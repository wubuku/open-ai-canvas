package canvas_contract_test

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"testing"

	"canvas-contract/harness"
	"canvas-contract/runner"
)

func runResponseLossRestartMatrix(test *testing.T, owner, foreign *runner.Client, config runner.Config, fixture *harness.ResourceFixture) {
	identity := "lost-response-" + config.RunID
	createCanvas(test, owner, identity, runner.FixtureProject(identity, "Before lost response", 0, false))
	updated := runner.FixtureProject(identity, "Committed without acknowledgement", 1, true)
	_, err := owner.DoWithResponseLoss(runner.Context(test), "document-response-loss", http.MethodPut, "/api/canvas-projects/"+identity, func(ctx context.Context, proxy *runner.Client) (runner.Response, runner.Envelope, error) {
		return proxy.Do(ctx, http.MethodPut, "/api/canvas-projects/"+identity, map[string]any{"project": updated}, nil)
	})
	if err != nil {
		test.Fatal(err)
	}
	saved := readCanvasDocument(test, owner, identity)
	runner.RequireNumber(test, saved["revision"], 2, "lost response committed revision")
	if saved["title"] != updated["title"] {
		test.Fatal("lost document response did not preserve the committed title")
	}
	for _, field := range []string{"nodes", "connections", "customExtension"} {
		if !bytes.Equal(importJSON(test, saved[field]), importJSON(test, updated[field])) {
			test.Fatalf("lost document response did not preserve committed %s", field)
		}
	}
	history := readCanvasHistory(test, owner, identity)
	resourcesBefore := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	content := runner.ImageFixture(test)
	directKey, completeKey := "lost-direct-"+config.RunID, "lost-complete-"+config.RunID
	_, err = owner.DoWithResponseLoss(runner.Context(test), "resource-create-response-loss", http.MethodPost, "/api/resources", func(ctx context.Context, proxy *runner.Client) (runner.Response, runner.Envelope, error) {
		return proxy.DoMultipart(ctx, http.MethodPost, "/api/resources", map[string]string{"kind": "image"}, "file", "lost-direct.png", content, http.Header{"X-Idempotency-Key": []string{directKey}})
	})
	if err != nil {
		test.Fatal(err)
	}
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "lost-complete.png", "kind": "image", "size": len(content), "idempotencyKey": completeKey})
	session := runner.DecodeData[uploadSession](test, envelope)
	if session.UploadID == "" || session.ChunkCount != 1 {
		test.Fatal("lost completion fixture requires one valid upload chunk")
	}
	writeChunk(test, owner, "/api/resources/uploads/"+session.UploadID+"/chunks/0", content, http.StatusOK)
	completePath := "/api/resources/uploads/" + session.UploadID + "/complete"
	_, err = owner.DoWithResponseLoss(runner.Context(test), "upload-complete-response-loss", http.MethodPost, completePath, func(ctx context.Context, proxy *runner.Client) (runner.Response, runner.Envelope, error) {
		return proxy.Do(ctx, http.MethodPost, completePath, nil, nil)
	})
	if err != nil {
		test.Fatal(err)
	}
	importKey := "lost-import-" + config.RunID
	_, err = owner.DoWithResponseLoss(runner.Context(test), "resource-import-response-loss", http.MethodPost, "/api/resources/import", func(ctx context.Context, proxy *runner.Client) (runner.Response, runner.Envelope, error) {
		return proxy.Do(ctx, http.MethodPost, "/api/resources/import", map[string]any{"url": fixture.Server.URL + "/image.png", "kind": "image"}, http.Header{"X-Idempotency-Key": []string{importKey}})
	})
	if err != nil {
		test.Fatal(err)
	}
	committed := resourceState(test, owner)
	if len(committed.IDs) != len(resourcesBefore.IDs)+3 || committed.UsedBytes != resourcesBefore.UsedBytes+2*int64(len(content))+int64(len(fixture.PNG)) {
		test.Fatal("lost resource responses must leave exactly two committed resources and their byte usage")
	}
	if len(differenceIDs(committed.IDs, resourcesBefore.IDs)) != 3 {
		test.Fatal("lost import response did not commit exactly one additional resource")
	}
	restartOwnedTarget(test, config, "crash", 4)
	if !reflect.DeepEqual(saved, readCanvasDocument(test, owner, identity)) || !reflect.DeepEqual(history, readCanvasHistory(test, owner, identity)) {
		test.Fatal("lost acknowledged document commit/history did not survive process crash")
	}
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPut, "/api/canvas-projects/"+identity, map[string]any{"project": updated}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusConflict, "conflict")
	if !reflect.DeepEqual(saved, readCanvasDocument(test, owner, identity)) || !reflect.DeepEqual(history, readCanvasHistory(test, owner, identity)) {
		test.Fatal("retrying a lost canvas write duplicated revision or history")
	}
	requireHTTPError(test, owner, http.MethodPost, completePath, nil, http.StatusNotFound)
	response, envelope, err = owner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": fixture.Server.URL + "/image.png", "kind": "image"}, http.Header{"X-Idempotency-Key": []string{importKey}})
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	replayed := runner.DecodeData[struct {
		Resource importedResource `json:"resource"`
	}](test, envelope).Resource
	if !containsString(committed.IDs, replayed.ID) || replayed.Size != int64(len(fixture.PNG)) || replayed.Kind != "image" || replayed.MimeType != "image/png" || replayed.Status != "ready" {
		test.Fatal("lost import response replay did not resolve the committed resource identity")
	}
	requireResourceBytes(test, owner, replayed.ID, fixture.PNG)
	requireHTTPError(test, foreign, http.MethodGet, "/api/resources/"+replayed.ID, nil, http.StatusNotFound)
	resourceIDs := make(map[string]bool)
	for _, key := range []string{directKey, completeKey} {
		response, envelope, err := owner.DoMultipart(runner.Context(test), http.MethodPost, "/api/resources", map[string]string{"kind": "image"}, "file", "retry-after-response-loss.png", content, http.Header{"X-Idempotency-Key": []string{key}})
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		resource := runner.DecodeData[struct {
			Resource importedResource `json:"resource"`
		}](test, envelope).Resource
		if resource.ID == "" || resourceIDs[resource.ID] || resource.Size != int64(len(content)) || resource.Kind != "image" || resource.MimeType != "image/png" || resource.Status != "ready" {
			test.Fatal("same-key recovery did not return two distinct committed ready images")
		}
		found := false
		for _, committedID := range committed.IDs {
			if committedID == resource.ID {
				found = true
			}
		}
		if !found {
			test.Fatal("lost-response replay created an identity absent from the committed resource set")
		}
		for _, priorID := range resourcesBefore.IDs {
			if priorID == resource.ID {
				test.Fatal("lost-response recovery incorrectly resolved an unrelated pre-existing resource")
			}
		}
		resourceIDs[resource.ID] = true
		requireResourceBytes(test, owner, resource.ID, content)
		requireHTTPError(test, foreign, http.MethodGet, "/api/resources/"+resource.ID, nil, http.StatusNotFound)
		foreignResponse, err := foreign.DoRaw(runner.Context(test), http.MethodGet, "/api/resources/"+resource.ID+"/file", nil, "", nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireStatus(test, foreignResponse, http.StatusNotFound)
		requireResourceState(test, owner, committed)
	}
	requireHTTPError(test, foreign, http.MethodGet, "/api/canvas-projects/"+identity, nil, http.StatusNotFound)
	requireResourceState(test, foreign, foreignBefore)
	requireResourceState(test, owner, committed)
}

func differenceIDs(after, before []string) []string {
	known := make(map[string]struct{}, len(before))
	for _, id := range before {
		known[id] = struct{}{}
	}
	var difference []string
	for _, id := range after {
		if _, ok := known[id]; !ok {
			difference = append(difference, id)
		}
	}
	return difference
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
