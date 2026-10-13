package canvas_contract_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"canvas-contract/harness"
	"canvas-contract/runner"
)

func policyFixtureUser(test *testing.T, admin *runner.Client, role string) *runner.Client {
	test.Helper()
	if !admin.Config.SettingsFixture {
		test.Fatal("quota/rate fixtures must only create users on an explicitly isolated target")
	}
	digest := sha256.Sum256([]byte(role + admin.Config.RunID))
	credentials := runner.Credentials{Username: fmt.Sprintf("canvas_limit_%x", digest[:8]), Password: "CanvasContract-Limits-2026!"}
	requireHTTPSuccess(test, admin, http.MethodPost, "/api/admin/users", map[string]any{
		"username": credentials.Username, "password": credentials.Password, "displayName": "Isolated limit fixture", "role": "user", "status": "active",
	})
	client, err := runner.NewClient(role, admin.Config)
	if err != nil {
		test.Fatal(err)
	}
	login(test, client, credentials)
	return client
}

func withResourcePolicy(test *testing.T, admin *runner.Client, configure func(map[string]any)) {
	test.Helper()
	if !admin.Config.SettingsFixture {
		test.Fatal("runtime policy changes require an explicitly isolated target")
	}
	_, envelope := requireHTTPSuccess(test, admin, http.MethodGet, "/api/admin/settings/runtime-policy", nil)
	setting := runner.DecodeData[struct {
		Setting map[string]any `json:"setting"`
	}](test, envelope).Setting
	original := map[string]any{}
	for _, field := range []string{"resource", "storage", "task", "request"} {
		if _, ok := setting[field].(map[string]any); !ok {
			test.Fatal("runtime policy is missing a required object")
		}
		original[field] = setting[field]
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		test.Fatal(err)
	}
	var modified map[string]any
	if err := json.Unmarshal(encoded, &modified); err != nil {
		test.Fatal(err)
	}
	configure(modified)
	test.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), admin.Config.Timeout)
		defer cancel()
		response, result, cleanupErr := admin.Do(cleanupContext, http.MethodPut, "/api/admin/settings/runtime-policy", original, nil)
		if cleanupErr != nil {
			test.Errorf("restore isolated runtime policy: %v", cleanupErr)
		} else if response.StatusCode != http.StatusOK || result.Code != 0 {
			test.Error("restore isolated runtime policy failed")
		}
	})
	requireHTTPSuccess(test, admin, http.MethodPut, "/api/admin/settings/runtime-policy", modified)
}

func paddedImage(test *testing.T, size int) []byte {
	test.Helper()
	content := runner.ImageFixture(test)
	if size < len(content) {
		test.Fatal("padded image size is smaller than the valid PNG")
	}
	return append(content, bytes.Repeat([]byte{0}, size-len(content))...)
}

func uploadLimitFixture(test *testing.T, owner *runner.Client, content []byte, key string) (runner.Response, runner.Envelope) {
	test.Helper()
	headers := make(http.Header)
	if key != "" {
		headers.Set("X-Idempotency-Key", key)
	}
	response, envelope, err := owner.DoMultipart(runner.Context(test), http.MethodPost, "/api/resources", map[string]string{"kind": "image"}, "file", "limits.png", content, headers)
	if err != nil {
		test.Fatal(err)
	}
	return response, envelope
}

func readyResourceID(test *testing.T, response runner.Response, envelope runner.Envelope) string {
	test.Helper()
	runner.RequireSuccess(test, response, envelope)
	resource := runner.DecodeData[struct {
		Resource importedResource `json:"resource"`
	}](test, envelope).Resource
	if resource.ID == "" || resource.Status != "ready" {
		test.Fatal("successful upload did not return a ready resource identity")
	}
	return resource.ID
}

func runResourceStoredReservationLimitMatrix(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "stored-reservation")
	foreign := policyFixtureUser(test, admin, "foreign-stored-reservation")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		resource := setting["resource"].(map[string]any)
		resource["resourceUploadMB"], resource["generatedFileMB"] = 600, 600
		resource["storedFileGB"], resource["dailyUploadMB"] = 1, 2048
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	first := startChunkUpload(test, owner, 512<<20)
	if first.ChunkCount != 64 {
		test.Fatal("declared half-GiB reservation did not return the expected chunk boundary")
	}
	for _, size := range []int{512 << 20, (512 << 20) + 1} {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "reservation.txt", "kind": "file", "size": size}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusForbidden, "quota_exceeded")
		requireResourceState(test, owner, before)
	}
	second := startChunkUpload(test, owner, (512<<20)-1)
	if second.UploadID == first.UploadID {
		test.Fatal("different reservations reused an upload identity")
	}
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "exact-capacity.txt", "kind": "file", "size": 1}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusForbidden, "quota_exceeded")
	response, envelope = uploadLimitFixture(test, owner, runner.ImageFixture(test), "blocked-by-reservations")
	runner.RequireError(test, response, envelope, http.StatusForbidden, "quota_exceeded")
	requireResourceState(test, owner, before)
	foreignSession := startChunkUpload(test, foreign, 1)
	if foreignSession.UploadID == first.UploadID || foreignSession.UploadID == second.UploadID {
		test.Fatal("reservation identity leaked across users")
	}
	requireResourceState(test, foreign, foreignBefore)
	response, envelope, err = foreign.Do(runner.Context(test), http.MethodPost, "/api/resources/uploads/"+first.UploadID+"/complete", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusNotFound, "")
	requireResourceState(test, owner, before)
}

func runResourceReservationExpiryMatrix(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "reservation-expiry")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		resource := setting["resource"].(map[string]any)
		resource["resourceUploadMB"] = 1
		resource["dailyUploadMB"] = 1
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	content := paddedImage(test, 600<<10)
	before := resourceState(test, owner)
	session := startChunkUpload(test, owner, len(content))
	if err := runner.ExpireUploadReservation(runner.Context(test), owner.Config, session.UploadID); err != nil {
		test.Fatal(err)
	}
	basePath := "/api/resources/uploads/" + session.UploadID
	writeChunk(test, owner, basePath+"/chunks/0", content, http.StatusOK)
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, basePath+"/complete", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
	requireResourceState(test, owner, before)

	recovered := startChunkUpload(test, owner, len(content))
	recoveredPath := "/api/resources/uploads/" + recovered.UploadID
	writeChunk(test, owner, recoveredPath+"/chunks/0", content, http.StatusOK)
	resourceID := completeChunkUpload(test, owner, recovered, content)
	if resourceID == "" {
		test.Fatal("expired reservation recovery did not publish a resource")
	}
	after := resourceState(test, owner)
	if len(after.IDs) != len(before.IDs)+1 || after.UsedBytes != before.UsedBytes+int64(len(content)) {
		test.Fatalf("recovered upload changed visible resource state unexpectedly: before=%+v after=%+v", before, after)
	}
	test.Log("expired reservation was rejected at commit, released, and replaced by a successful retry")
}

func runResourceSessionSlotLimitMatrix(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "session-slots")
	foreign := policyFixtureUser(test, admin, "foreign-session-slots")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	content := runner.ImageFixture(test)
	sessions := make([]uploadSession, 0, 32)
	identities := map[string]bool{}
	for count := 0; count < 32; count++ {
		session := startChunkUpload(test, owner, len(content))
		if identities[session.UploadID] {
			test.Fatal("session-slot fixture received a duplicate upload identity")
		}
		identities[session.UploadID] = true
		sessions = append(sessions, session)
	}
	for attempt := 0; attempt < 2; attempt++ {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "over-slot-cap.txt", "kind": "file", "size": len(content)}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusTooManyRequests, "rate_limited")
		if response.Header.Get("Retry-After") != "" {
			test.Fatal("session-slot rejection was incorrectly treated as the request-frequency bucket")
		}
		requireResourceState(test, owner, before)
	}
	foreignSession := startChunkUpload(test, foreign, len(content))
	if identities[foreignSession.UploadID] {
		test.Fatal("session identity leaked across user scopes")
	}
	requireResourceState(test, foreign, foreignBefore)
	basePath := "/api/resources/uploads/" + sessions[0].UploadID
	writeChunk(test, owner, basePath+"/chunks/0", content, http.StatusOK)
	resourceID := completeChunkUpload(test, owner, sessions[0], content)
	after := resourceState(test, owner)
	if len(after.IDs) != len(before.IDs)+1 || after.UsedBytes != before.UsedBytes+int64(len(content)) {
		test.Fatal("completed slot did not produce exactly one ready resource with its real byte usage")
	}
	newSession := startChunkUpload(test, owner, len(content))
	if identities[newSession.UploadID] {
		test.Fatal("released slot reused a closed upload identity")
	}
	requireResourceState(test, owner, after)
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, basePath+"/complete", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusNotFound, "")
	requireResourceState(test, owner, after)
	requireResourceBytes(test, owner, resourceID, content)
}

func runResourceFileLimitMatrix(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "single-file")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["resource"].(map[string]any)["resourceUploadMB"] = 1
	})
	before := resourceState(test, owner)
	for _, size := range []int{0, 1 << 20, (1 << 20) + 1} {
		content := []byte{}
		if size > 0 {
			content = paddedImage(test, size)
		}
		response, envelope := uploadLimitFixture(test, owner, content, "")
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		requireResourceState(test, owner, before)
	}
	requireHTTPError(test, owner, http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "limits.png", "size": 1 << 20}, http.StatusBadRequest)
	content := paddedImage(test, (1<<20)-1)
	response, envelope := uploadLimitFixture(test, owner, content, "")
	resourceID := readyResourceID(test, response, envelope)
	stored := resourceState(test, owner)
	if len(stored.IDs) != 1 || stored.UsedBytes != int64(len(content)) {
		test.Fatal("single-file near-limit success did not account exact bytes once")
	}
	requireResourceBytes(test, owner, resourceID, content)
	oversized := paddedImage(test, (1<<20)+1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Content-Length", strconv.Itoa((1<<20)+1))
		_, _ = writer.Write(oversized)
	}))
	defer upstream.Close()
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": upstream.URL + "/large.png"}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
	requireResourceState(test, owner, stored)
}

func runResourceDailyLimitMatrix(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "daily-owner")
	foreign := policyFixtureUser(test, admin, "daily-foreign")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["resource"].(map[string]any)["resourceUploadMB"] = 1
		setting["resource"].(map[string]any)["dailyUploadMB"] = 1
	})
	content := paddedImage(test, 600<<10)
	response, envelope := uploadLimitFixture(test, owner, content, "daily-first")
	resourceID := readyResourceID(test, response, envelope)
	before := resourceState(test, owner)
	response, envelope = uploadLimitFixture(test, owner, content, "daily-first")
	if readyResourceID(test, response, envelope) != resourceID {
		test.Fatal("idempotent upload consumed another resource identity")
	}
	requireResourceState(test, owner, before)
	response, envelope = uploadLimitFixture(test, owner, paddedImage(test, 424<<10), "daily-exact-boundary")
	runner.RequireError(test, response, envelope, http.StatusForbidden, "quota_exceeded")
	requireHTTPError(test, owner, http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "limits.png", "size": 600 << 10}, http.StatusForbidden)
	requireResourceState(test, owner, before)
	response, envelope = uploadLimitFixture(test, foreign, content, "daily-first")
	foreignID := readyResourceID(test, response, envelope)
	if foreignID == resourceID {
		test.Fatal("daily quota/upload key leaked across users")
	}
	foreignBefore := resourceState(test, foreign)
	candidate := paddedImage(test, 400<<10)
	start := make(chan struct{})
	type result struct {
		Response runner.Response
		Envelope runner.Envelope
		Error    error
	}
	results := make(chan result, 2)
	requestContext := runner.Context(test)
	for index := 0; index < 2; index++ {
		go func(attempt int) {
			<-start
			headers := make(http.Header)
			headers.Set("X-Idempotency-Key", fmt.Sprintf("daily-concurrent-%d", attempt))
			response, envelope, requestErr := owner.DoMultipart(requestContext, http.MethodPost, "/api/resources", map[string]string{"kind": "image"}, "file", "limits.png", candidate, headers)
			results <- result{Response: response, Envelope: envelope, Error: requestErr}
		}(index)
	}
	close(start)
	successes, rejections := 0, 0
	for index := 0; index < 2; index++ {
		result := <-results
		if result.Error != nil {
			test.Fatal(result.Error)
		}
		if result.Response.StatusCode == http.StatusOK {
			identity := readyResourceID(test, result.Response, result.Envelope)
			requireResourceBytes(test, owner, identity, candidate)
			successes++
		} else {
			runner.RequireError(test, result.Response, result.Envelope, http.StatusForbidden, "quota_exceeded")
			rejections++
		}
	}
	if successes != 1 || rejections != 1 {
		test.Fatal("two concurrent candidates must settle as one upload and one daily-quota rejection")
	}
	after := resourceState(test, owner)
	if len(after.IDs) != 2 || after.UsedBytes != 1000<<10 {
		test.Fatal("daily reservation rollback or concurrency charged unexpected bytes/records")
	}
	requireResourceBytes(test, owner, resourceID, content)
	requireResourceState(test, foreign, foreignBefore)
}

func runResourceRateLimitMatrix(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "rate-owner")
	foreign := policyFixtureUser(test, admin, "rate-foreign")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 2
		setting["request"].(map[string]any)["resourceImportPerMinute"] = 1
	})
	content := runner.ImageFixture(test)
	response, envelope := uploadLimitFixture(test, owner, content, "rate-first")
	resourceID := readyResourceID(test, response, envelope)
	_, sessionEnvelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "limits.png", "size": len(content)})
	session := runner.DecodeData[struct {
		UploadID string `json:"uploadId"`
	}](test, sessionEnvelope)
	if session.UploadID == "" {
		test.Fatal("rate fixture has no upload session")
	}
	before := resourceState(test, owner)
	response, envelope = uploadLimitFixture(test, owner, content, "rate-rejected")
	requireRateLimited(test, response, envelope)
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "limits.png", "size": len(content)}, nil)
	if err != nil {
		test.Fatal(err)
	}
	requireRateLimited(test, response, envelope)
	requireResourceState(test, owner, before)
	response, envelope = uploadLimitFixture(test, foreign, content, "rate-first")
	readyResourceID(test, response, envelope)
	response, envelope, err = owner.DoBytes(runner.Context(test), http.MethodPut, "/api/resources/uploads/"+session.UploadID+"/chunks/0", "application/octet-stream", content, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	response, envelope = requireHTTPSuccess(test, owner, http.MethodPost, "/api/resources/uploads/"+session.UploadID+"/complete", map[string]any{})
	completedID := readyResourceID(test, response, envelope)
	requireResourceBytes(test, owner, completedID, content)
	fixture, err := harness.NewResourceFixture()
	if err != nil {
		test.Fatal(err)
	}
	defer fixture.Close()
	importResource(test, owner, fixture.Server.URL+"/image.png", "rate-import")
	before = resourceState(test, owner)
	response, envelope, err = owner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": fixture.Server.URL + "/image.png"}, nil)
	if err != nil {
		test.Fatal(err)
	}
	requireRateLimited(test, response, envelope)
	if fixture.Hits("/image.png") != 1 {
		test.Fatal("rate-limited import must not contact the upstream")
	}
	requireResourceState(test, owner, before)
	requireResourceBytes(test, owner, resourceID, content)
}

func requireRateLimited(test *testing.T, response runner.Response, envelope runner.Envelope) {
	test.Helper()
	runner.RequireError(test, response, envelope, http.StatusTooManyRequests, "rate_limited")
	seconds, err := strconv.Atoi(response.Header.Get("Retry-After"))
	if err != nil || seconds < 1 || seconds > 60 {
		test.Fatal("rate limit must include a positive Retry-After within its one-minute window")
	}
}
