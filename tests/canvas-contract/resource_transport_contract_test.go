package canvas_contract_test

import (
	"bytes"
	"net/http"
	"testing"

	"canvas-contract/runner"
)

func runResourceInterruptedChunkRecovery(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "interrupted-chunk")
	foreign := policyFixtureUser(test, admin, "foreign-interrupted-chunk")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["request"].(map[string]any)["resourceUploadPerMinute"] = 1000
	})
	content := bytes.Repeat([]byte("s"), (8<<20)+13)
	foreignBefore := resourceState(test, foreign)
	for _, phase := range []string{"missing", "overwrite"} {
		before := resourceState(test, owner)
		session := startChunkUpload(test, owner, len(content))
		if session.ChunkSize != 8<<20 || session.ChunkCount != 2 {
			test.Fatal("interrupted upload fixture requires the observed two-chunk boundary")
		}
		basePath := "/api/resources/uploads/" + session.UploadID
		last := content[session.ChunkSize:]
		writeChunk(test, owner, basePath+"/chunks/0", content[:session.ChunkSize], http.StatusOK)
		if phase == "overwrite" {
			writeChunk(test, owner, basePath+"/chunks/1", last, http.StatusOK)
		}
		response, envelope, err := owner.DoInterruptedChunk(runner.Context(test), basePath+"/chunks/1", int64(len(last)), last[:6])
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusBadRequest)
		requireResourceState(test, owner, before)
		requireResourceState(test, foreign, foreignBefore)
		writeChunk(test, foreign, basePath+"/chunks/1", last, http.StatusNotFound)
		writeChunk(test, owner, basePath+"/chunks/1", last, http.StatusOK)
		resourceID := completeChunkUpload(test, owner, session, content)
		requireCompletedUploadState(test, owner, foreign, before, foreignBefore, resourceID, basePath, content)
		test.Logf("interrupted_chunk=%s declared_bytes=13 sent_bytes=6 complete_failure=400; restored chunk publishes exact bytes and charges once", phase)
	}
}
