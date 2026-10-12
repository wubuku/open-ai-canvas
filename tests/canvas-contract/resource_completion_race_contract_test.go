package canvas_contract_test

import (
	"bytes"
	"net/http"
	"reflect"
	"sort"
	"testing"

	"canvas-contract/runner"
)

type uploadCompletionOutcome struct {
	Role   string
	Result concurrentHTTPResult
}

func raceLastChunkAndComplete(test *testing.T, owner *runner.Client, basePath string, content []byte) (concurrentHTTPResult, concurrentHTTPResult) {
	test.Helper()
	start := make(chan struct{})
	results := make(chan uploadCompletionOutcome, 2)
	requestContext := runner.Context(test)
	go func() {
		<-start
		response, envelope, err := owner.DoBytes(requestContext, http.MethodPut, basePath+"/chunks/1", "application/octet-stream", content, nil)
		results <- uploadCompletionOutcome{"chunk", concurrentHTTPResult{Response: response, Envelope: envelope, Error: err}}
	}()
	go func() {
		<-start
		response, envelope, err := owner.Do(requestContext, http.MethodPost, basePath+"/complete", nil, nil)
		results <- uploadCompletionOutcome{"complete", concurrentHTTPResult{Response: response, Envelope: envelope, Error: err}}
	}()
	close(start)
	outcomes := []uploadCompletionOutcome{<-results, <-results}
	var chunk, complete concurrentHTTPResult
	for _, outcome := range outcomes {
		if outcome.Result.Error != nil {
			test.Fatal(outcome.Result.Error)
		}
		if outcome.Role == "chunk" {
			chunk = outcome.Result
		} else {
			complete = outcome.Result
		}
	}
	return chunk, complete
}

func requireCompletedUploadState(test *testing.T, owner, foreign *runner.Client, before, foreignBefore visibleResourceState, resourceID, basePath string, content []byte) {
	test.Helper()
	expected := visibleResourceState{IDs: append(append([]string{}, before.IDs...), resourceID), UsedBytes: before.UsedBytes + int64(len(content))}
	sort.Strings(expected.IDs)
	requireResourceState(test, owner, expected)
	requireResourceBytes(test, owner, resourceID, content)
	_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	metadata := runner.DecodeData[map[string]any](test, envelope)
	writeChunk(test, owner, basePath+"/chunks/1", content[len(content)-13:], http.StatusNotFound)
	requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusNotFound)
	requireHTTPError(test, foreign, http.MethodGet, "/api/resources/"+resourceID, nil, http.StatusNotFound)
	_, envelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	if !reflect.DeepEqual(runner.DecodeData[map[string]any](test, envelope), metadata) {
		test.Fatal("closed upload replay changed resource metadata")
	}
	requireResourceBytes(test, owner, resourceID, content)
	requireResourceState(test, owner, expected)
	requireResourceState(test, foreign, foreignBefore)
}

func runResourceChunkVersusCompletion(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "chunk-completion-overlap")
	foreign := policyFixtureUser(test, admin, "foreign-chunk-completion-overlap")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	content := bytes.Repeat([]byte("p"), (8<<20)+13)
	foreignBefore := resourceState(test, foreign)
	for _, phase := range []string{"last-missing", "valid-overwrite"} {
		before := resourceState(test, owner)
		session := startChunkUpload(test, owner, len(content))
		if session.ChunkSize != 8<<20 || session.ChunkCount != 2 {
			test.Fatal("completion overlap fixture requires the observed two-chunk boundary")
		}
		basePath := "/api/resources/uploads/" + session.UploadID
		first, last := content[:session.ChunkSize], content[session.ChunkSize:]
		writeChunk(test, owner, basePath+"/chunks/0", first, http.StatusOK)
		requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusBadRequest)
		requireResourceState(test, owner, before)
		writeChunk(test, foreign, basePath+"/chunks/1", last, http.StatusNotFound)
		if phase == "valid-overwrite" {
			writeChunk(test, owner, basePath+"/chunks/1", last, http.StatusOK)
			last = bytes.Repeat([]byte("q"), len(last))
		}
		chunk, complete := raceLastChunkAndComplete(test, owner, basePath, last)
		resourceID := ""
		expectedContent := content
		if phase == "last-missing" {
			runner.RequireSuccess(test, chunk.Response, chunk.Envelope)
			if complete.Response.StatusCode == http.StatusBadRequest {
				runner.RequireError(test, complete.Response, complete.Envelope, http.StatusBadRequest, "")
				requireResourceState(test, owner, before)
				resourceID = completeChunkUpload(test, owner, session, content)
			} else {
				resourceID = readyResourceID(test, complete.Response, complete.Envelope)
			}
		} else {
			resourceID = readyResourceID(test, complete.Response, complete.Envelope)
			if chunk.Response.StatusCode == http.StatusOK {
				runner.RequireSuccess(test, chunk.Response, chunk.Envelope)
				expectedContent = append(append([]byte{}, first...), last...)
			} else {
				runner.RequireError(test, chunk.Response, chunk.Envelope, http.StatusNotFound, "")
			}
		}
		requireCompletedUploadState(test, owner, foreign, before, foreignBefore, resourceID, basePath, expectedContent)
		test.Logf("completion_overlap=%s chunk_status=%d complete_status=%d", phase, chunk.Response.StatusCode, complete.Response.StatusCode)
	}
}

func runResourceInvalidOverwriteRecovery(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "invalid-chunk-overwrite")
	foreign := policyFixtureUser(test, admin, "foreign-invalid-chunk-overwrite")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	content := bytes.Repeat([]byte("r"), (8<<20)+13)
	foreignBefore := resourceState(test, foreign)
	for _, phase := range []string{"short", "long"} {
		before := resourceState(test, owner)
		session := startChunkUpload(test, owner, len(content))
		if session.ChunkSize != 8<<20 || session.ChunkCount != 2 {
			test.Fatal("invalid overwrite fixture requires the observed two-chunk boundary")
		}
		basePath := "/api/resources/uploads/" + session.UploadID
		first, last := content[:session.ChunkSize], content[session.ChunkSize:]
		writeChunk(test, owner, basePath+"/chunks/0", first, http.StatusOK)
		writeChunk(test, owner, basePath+"/chunks/1", last, http.StatusOK)
		invalid := last[:len(last)-1]
		if phase == "long" {
			invalid = append(append([]byte{}, last...), 'x')
		}
		writeChunk(test, owner, basePath+"/chunks/1", invalid, http.StatusBadRequest)
		requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusBadRequest)
		requireResourceState(test, owner, before)
		writeChunk(test, foreign, basePath+"/chunks/1", last, http.StatusNotFound)
		writeChunk(test, owner, basePath+"/chunks/1", last, http.StatusOK)
		chunk, complete := raceLastChunkAndComplete(test, owner, basePath, invalid)
		resourceID := ""
		if chunk.Response.StatusCode == http.StatusBadRequest {
			runner.RequireError(test, chunk.Response, chunk.Envelope, http.StatusBadRequest, "")
			runner.RequireError(test, complete.Response, complete.Envelope, http.StatusBadRequest, "")
			requireResourceState(test, owner, before)
			writeChunk(test, owner, basePath+"/chunks/1", last, http.StatusOK)
			resourceID = completeChunkUpload(test, owner, session, content)
		} else {
			runner.RequireError(test, chunk.Response, chunk.Envelope, http.StatusNotFound, "")
			resourceID = readyResourceID(test, complete.Response, complete.Envelope)
		}
		requireCompletedUploadState(test, owner, foreign, before, foreignBefore, resourceID, basePath, content)
		test.Logf("invalid_overwrite=%s chunk_status=%d complete_status=%d; failed overwrite removes an acknowledged staged chunk in the observed Go baseline", phase, chunk.Response.StatusCode, complete.Response.StatusCode)
	}
}
