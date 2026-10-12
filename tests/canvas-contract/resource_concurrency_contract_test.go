package canvas_contract_test

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"testing"

	"canvas-contract/runner"
)

type concurrentHTTPResult struct {
	Response runner.Response
	Envelope runner.Envelope
	Error    error
}

func concurrentResourceRequests(test *testing.T, count int, request func(context.Context, int) (runner.Response, runner.Envelope, error)) []concurrentHTTPResult {
	test.Helper()
	start := make(chan struct{})
	replies := make(chan concurrentHTTPResult, count)
	requestContext := runner.Context(test)
	for index := 0; index < count; index++ {
		go func(attempt int) {
			<-start
			response, envelope, err := request(requestContext, attempt)
			replies <- concurrentHTTPResult{Response: response, Envelope: envelope, Error: err}
		}(index)
	}
	close(start)
	results := make([]concurrentHTTPResult, 0, count)
	for index := 0; index < count; index++ {
		results = append(results, <-replies)
	}
	return results
}

func runResourceConcurrentSlotAdmission(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "concurrent-session-slots")
	foreign := policyFixtureUser(test, admin, "foreign-concurrent-session-slots")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	content := runner.ImageFixture(test)
	identities := map[string]bool{}
	for count := 0; count < 31; count++ {
		session := startChunkUpload(test, owner, len(content))
		if identities[session.UploadID] {
			test.Fatal("concurrent admission fixture received a duplicate session identity")
		}
		identities[session.UploadID] = true
	}
	var admitted uploadSession
	for phase := 0; phase < 2; phase++ {
		results := concurrentResourceRequests(test, 6, func(requestContext context.Context, attempt int) (runner.Response, runner.Envelope, error) {
			return owner.Do(requestContext, http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "concurrent-slot.png", "kind": "image", "size": len(content)}, nil)
		})
		successes, rejections := 0, 0
		for _, result := range results {
			if result.Error != nil {
				test.Fatal(result.Error)
			}
			if result.Response.StatusCode == http.StatusOK {
				runner.RequireSuccess(test, result.Response, result.Envelope)
				admitted = runner.DecodeData[uploadSession](test, result.Envelope)
				if admitted.UploadID == "" || identities[admitted.UploadID] || admitted.ChunkCount != 1 || admitted.ChunkSize != 8<<20 {
					test.Fatal("concurrent slot admission did not allocate a unique complete session contract")
				}
				identities[admitted.UploadID] = true
				successes++
			} else {
				runner.RequireError(test, result.Response, result.Envelope, http.StatusTooManyRequests, "rate_limited")
				if result.Response.Header.Get("Retry-After") != "" {
					test.Fatal("concurrent slot cap was incorrectly treated as frequency throttling")
				}
				rejections++
			}
		}
		if successes != 1 || rejections != 5 {
			test.Fatal("six concurrent candidates for one slot did not settle as exactly one admission and five rejections")
		}
		requireResourceState(test, owner, before)
		if phase == 0 {
			writeChunk(test, owner, "/api/resources/uploads/"+admitted.UploadID+"/chunks/0", content, http.StatusOK)
			resourceID := completeChunkUpload(test, owner, admitted, content)
			after := resourceState(test, owner)
			if len(after.IDs) != len(before.IDs)+1 || after.UsedBytes != before.UsedBytes+int64(len(content)) {
				test.Fatal("releasing the admitted slot did not charge exactly one completed resource")
			}
			requireResourceBytes(test, owner, resourceID, content)
			before = after
		}
	}
	foreignSession := startChunkUpload(test, foreign, len(content))
	if identities[foreignSession.UploadID] {
		test.Fatal("concurrent slot identity leaked into another user scope")
	}
	requireHTTPError(test, foreign, http.MethodPost, "/api/resources/uploads/"+admitted.UploadID+"/complete", nil, http.StatusNotFound)
	requireResourceState(test, owner, before)
	requireResourceState(test, foreign, foreignBefore)
}

func runResourceConcurrentChunksAndCompletion(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "concurrent-chunks")
	foreign := policyFixtureUser(test, admin, "foreign-concurrent-chunks")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	for phase := 0; phase < 2; phase++ {
		content := paddedImage(test, (8<<20)+37)
		if phase == 1 {
			content = paddedImage(test, 32<<10)
		}
		alternate := append([]byte{}, content...)
		alternate[len(alternate)-1] = 1
		session := startChunkUpload(test, owner, len(content))
		basePath := "/api/resources/uploads/" + session.UploadID
		if phase == 0 && (session.ChunkCount != 2 || session.ChunkSize != 8<<20) || phase == 1 && session.ChunkCount != 1 {
			test.Fatal("concurrent chunk fixture received unexpected chunk boundaries")
		}
		results := concurrentResourceRequests(test, 2, func(requestContext context.Context, attempt int) (runner.Response, runner.Envelope, error) {
			index, chunk := attempt, content
			if phase == 0 {
				if attempt == 0 {
					chunk = content[:session.ChunkSize]
				} else {
					chunk = content[session.ChunkSize:]
				}
			} else {
				index = 0
				if attempt == 1 {
					chunk = alternate
				}
			}
			return owner.DoBytes(requestContext, http.MethodPut, basePath+"/chunks/"+strconv.Itoa(index), "application/octet-stream", chunk, nil)
		})
		for _, result := range results {
			if result.Error != nil {
				test.Fatal(result.Error)
			}
			runner.RequireSuccess(test, result.Response, result.Envelope)
		}
		requireResourceState(test, owner, before)
		writeChunk(test, foreign, basePath+"/chunks/0", []byte("foreign"), http.StatusNotFound)
		requireHTTPError(test, foreign, http.MethodPost, basePath+"/complete", nil, http.StatusNotFound)
		results = concurrentResourceRequests(test, 4, func(requestContext context.Context, attempt int) (runner.Response, runner.Envelope, error) {
			return owner.Do(requestContext, http.MethodPost, basePath+"/complete", nil, nil)
		})
		resourceID, successes, rejections := "", 0, 0
		for _, result := range results {
			if result.Error != nil {
				test.Fatal(result.Error)
			}
			if result.Response.StatusCode == http.StatusOK {
				resourceID = readyResourceID(test, result.Response, result.Envelope)
				successes++
			} else {
				runner.RequireError(test, result.Response, result.Envelope, http.StatusNotFound, "")
				rejections++
			}
		}
		if successes != 1 || rejections != 3 {
			test.Fatal("concurrent completion did not settle as one ready resource and three closed-session rejections")
		}
		response, err := owner.DoRaw(runner.Context(test), http.MethodGet, "/api/resources/"+resourceID+"/file", nil, "", nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireStatus(test, response, http.StatusOK)
		if !bytes.Equal(response.Body, content) && (phase != 1 || !bytes.Equal(response.Body, alternate)) {
			test.Fatal("concurrent chunk writes produced mixed or truncated stored bytes")
		}
		after := resourceState(test, owner)
		if len(after.IDs) != len(before.IDs)+1 || after.UsedBytes != before.UsedBytes+int64(len(content)) {
			test.Fatal("concurrent completion duplicated resource identity or byte charges")
		}
		writeChunk(test, owner, basePath+"/chunks/0", content, http.StatusNotFound)
		requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusNotFound)
		requireResourceState(test, owner, after)
		requireResourceState(test, foreign, foreignBefore)
		before = after
	}
}
