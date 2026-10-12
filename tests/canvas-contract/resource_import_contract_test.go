package canvas_contract_test

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	"canvas-contract/harness"
	"canvas-contract/runner"
)

type importedResource struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Status   string `json:"status"`
}

type visibleResourceState struct {
	IDs       []string
	UsedBytes int64
}

func resourceState(test *testing.T, client *runner.Client) visibleResourceState {
	test.Helper()
	response, envelope, err := client.Do(runner.Context(test), http.MethodGet, "/api/resources", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	listing := runner.DecodeData[struct {
		Resources []importedResource `json:"resources"`
	}](test, envelope)
	state := visibleResourceState{IDs: make([]string, 0, len(listing.Resources))}
	for _, resource := range listing.Resources {
		state.IDs = append(state.IDs, resource.ID)
	}
	sort.Strings(state.IDs)
	response, envelope, err = client.Do(runner.Context(test), http.MethodGet, "/api/resources/storage-usage", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	usage := runner.DecodeData[struct {
		Usage struct {
			UsedBytes int64 `json:"usedBytes"`
		} `json:"usage"`
	}](test, envelope)
	state.UsedBytes = usage.Usage.UsedBytes
	return state
}

func requireResourceState(test *testing.T, client *runner.Client, want visibleResourceState) {
	test.Helper()
	if got := resourceState(test, client); !reflect.DeepEqual(got, want) {
		test.Fatalf("resource list/usage changed: count=%d bytes=%d, want count=%d bytes=%d", len(got.IDs), got.UsedBytes, len(want.IDs), want.UsedBytes)
	}
}

func importResource(test *testing.T, client *runner.Client, address, key string) importedResource {
	test.Helper()
	headers := make(http.Header)
	headers.Set("X-Idempotency-Key", key)
	response, envelope, err := client.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": address, "kind": "video"}, headers)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	resource := runner.DecodeData[struct {
		Resource importedResource `json:"resource"`
	}](test, envelope).Resource
	if resource.ID == "" || resource.Status != "ready" {
		test.Fatal("import did not produce a ready resource")
	}
	return resource
}

func requireResourceBytes(test *testing.T, client *runner.Client, resourceID string, content []byte) {
	test.Helper()
	response, err := client.DoRaw(runner.Context(test), http.MethodGet, "/api/resources/"+resourceID+"/file", nil, "", nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusOK)
	if !bytes.Equal(response.Body, content) {
		test.Fatal("stored resource bytes differ")
	}
}

func runResourceImportSuccessMatrix(test *testing.T, owner, foreign *runner.Client, fixture *harness.ResourceFixture, runID string) {
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	picture := importResource(test, owner, fixture.Server.URL+"/image.png", "picture-"+runID)
	if picture.Kind != "image" || picture.MimeType != "image/png" || picture.Width != 3 || picture.Height != 2 || picture.Size != int64(len(fixture.PNG)) {
		test.Fatal("import did not detect actual image bytes/dimensions independently of declared MIME/kind")
	}
	requireResourceBytes(test, owner, picture.ID, fixture.PNG)
	response, envelope, err := owner.Do(runner.Context(test), http.MethodGet, "/api/resources/"+picture.ID, nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	metadata := runner.DecodeData[struct {
		Resource importedResource `json:"resource"`
	}](test, envelope).Resource
	if metadata != picture {
		test.Fatal("import response and persistent metadata differ")
	}
	redirected := importResource(test, owner, fixture.Server.URL+"/redirect-image", "redirect-"+runID)
	requireResourceBytes(test, owner, redirected.ID, fixture.PNG)
	if redirected.Kind != "image" || redirected.Size != picture.Size {
		test.Fatal("allowed redirect did not preserve image metadata")
	}
	after := resourceState(test, owner)
	if len(after.IDs) != len(before.IDs)+2 || after.UsedBytes != before.UsedBytes+2*picture.Size {
		test.Fatal("two successful imports must add exactly two resources and their stored byte usage")
	}
	requireHTTPError(test, foreign, http.MethodGet, "/api/resources/"+picture.ID, nil, http.StatusNotFound)
	foreignResponse, err := foreign.DoRaw(runner.Context(test), http.MethodGet, "/api/resources/"+picture.ID+"/file", nil, "", nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, foreignResponse, http.StatusNotFound)
	requireResourceState(test, foreign, foreignBefore)
}

func runResourceImportReplayMatrix(test *testing.T, owner, foreign *runner.Client, fixture *harness.ResourceFixture, runID string) {
	key := "resource-replay-" + runID
	original := importResource(test, owner, fixture.Server.URL+"/note.txt", key)
	if original.Kind != "file" || original.MimeType != "text/plain" || original.Size != int64(len(fixture.Text)) {
		test.Fatal("text import must use actual MIME rather than the misleading image header")
	}
	before := resourceState(test, owner)
	hits := fixture.Hits("/note.txt")
	replayed := importResource(test, owner, fixture.Server.URL+"/note.txt", key)
	if replayed != original || fixture.Hits("/note.txt") != hits+1 {
		test.Fatal("Go replay must return the stored resource after downloading the upstream again")
	}
	requireResourceState(test, owner, before)
	for _, failure := range []struct {
		path   string
		status int
	}{
		{"/different-size.txt", http.StatusConflict},
		{"/unavailable", http.StatusInternalServerError},
	} {
		headers := make(http.Header)
		headers.Set("X-Idempotency-Key", key)
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": fixture.Server.URL + failure.path}, headers)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, failure.status, "")
		requireResourceState(test, owner, before)
		requireResourceBytes(test, owner, original.ID, fixture.Text)
	}
	sameSize := importResource(test, owner, fixture.Server.URL+"/same-size.txt", key)
	if sameSize != original {
		test.Fatal("Go's current identity check uses kind/MIME/size, not a content fingerprint")
	}
	requireResourceState(test, owner, before)
	requireResourceBytes(test, owner, original.ID, fixture.Text)
	foreignBefore := resourceState(test, foreign)
	foreignImport := importResource(test, foreign, fixture.Server.URL+"/note.txt", key)
	if foreignImport.ID == original.ID {
		test.Fatal("resource import idempotency key is not user-scoped")
	}
	requireResourceBytes(test, foreign, foreignImport.ID, fixture.Text)
	foreignAfter := resourceState(test, foreign)
	if len(foreignAfter.IDs) != len(foreignBefore.IDs)+1 || foreignAfter.UsedBytes != foreignBefore.UsedBytes+original.Size {
		test.Fatal("foreign import must produce exactly one foreign-owned resource")
	}
	requireResourceState(test, owner, before)
}

func runResourceImportFailureMatrix(test *testing.T, owner *runner.Client, fixture *harness.ResourceFixture, runID string) {
	before := resourceState(test, owner)
	for index, failure := range []struct {
		address string
		status  int
	}{
		{fixture.Server.URL + "/unavailable", http.StatusInternalServerError},
		{fixture.Server.URL + "/truncated", http.StatusInternalServerError},
		{fixture.Server.URL + "/redirect-loop", http.StatusInternalServerError},
		{fixture.Server.URL + "/redirect-private", http.StatusInternalServerError},
		{"http://169.254.169.254/latest/meta-data/", http.StatusBadRequest},
		{"http://127.0.0.2/", http.StatusBadRequest},
		{"http://user:synthetic-fixture-password@127.0.0.1/", http.StatusBadRequest},
		{"file:///tmp/not-a-resource", http.StatusBadRequest},
	} {
		headers := make(http.Header)
		headers.Set("X-Idempotency-Key", fmt.Sprintf("failed-import-%s-%d", runID, index))
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": failure.address}, headers)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, failure.status, "")
		requireResourceState(test, owner, before)
	}
	for _, path := range []string{"/unavailable", "/truncated", "/redirect-loop", "/redirect-private"} {
		if fixture.Hits(path) == 0 {
			test.Fatal("controlled upstream failure was not exercised")
		}
	}
}

func runResourceImportSlowTimeout(test *testing.T, admin *runner.Client, fixture *harness.ResourceFixture) {
	owner := policyFixtureUser(test, admin, "slow-url-import")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["resource"].(map[string]any)["resourceUploadDailyMB"] = 1
	})
	config := owner.Config
	if config.Timeout < 110*time.Second {
		config.Timeout = 110 * time.Second
	}
	slowOwner, err := runner.NewClient("slow-url-import", config)
	if err != nil {
		test.Fatal(err)
	}
	slowOwner.HTTP.Jar = owner.HTTP.Jar
	defer slowOwner.HTTP.CloseIdleConnections()
	before := resourceState(test, owner)
	headers := make(http.Header)
	key := "slow-import-" + config.RunID
	headers.Set("X-Idempotency-Key", key)
	started := time.Now()
	response, envelope, err := slowOwner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": fixture.Server.URL + "/slow"}, headers)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
	if elapsed := time.Since(started); elapsed < 85*time.Second || elapsed >= config.Timeout {
		test.Fatal("resource import did not return its observed server-side 90-second timeout before the HTTP runner deadline")
	}
	if fixture.Hits("/slow") != 1 {
		test.Fatal("slow URL import contacted its controlled upstream more or less than once")
	}
	select {
	case <-fixture.SlowCancellation():
	case <-time.After(2 * time.Second):
		test.Fatal("timed-out import did not close its upstream transport")
	}
	requireResourceState(test, owner, before)
	beforeImageHits := fixture.Hits("/image.png")
	resource := importResource(test, owner, fixture.Server.URL+"/image.png", key)
	requireResourceBytes(test, owner, resource.ID, fixture.PNG)
	after := resourceState(test, owner)
	if len(after.IDs) != len(before.IDs)+1 || after.UsedBytes != before.UsedBytes+int64(len(fixture.PNG)) || fixture.Hits("/image.png") != beforeImageHits+1 {
		test.Fatal("explicit retry after a proven download failure did not create exactly one correctly charged resource")
	}
}
