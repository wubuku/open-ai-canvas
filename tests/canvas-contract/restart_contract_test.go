package canvas_contract_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"canvas-contract/runner"
)

func readCanvasDocument(test *testing.T, client *runner.Client, identity string) map[string]any {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, client, http.MethodGet, "/api/canvas-projects/"+identity, nil)
	return runner.DecodeData[struct {
		Project map[string]any `json:"project"`
	}](test, envelope).Project
}

func readCanvasHistory(test *testing.T, client *runner.Client, identity string) map[string]any {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, client, http.MethodGet, "/api/canvas-projects/"+identity+"/history", nil)
	return runner.DecodeData[map[string]any](test, envelope)
}

func restartOwnedTarget(test *testing.T, config runner.Config, mode string, generation int) {
	test.Helper()
	controlConfig := config
	controlConfig.BaseURL = config.ControlURL
	control, err := runner.NewClient("private-target-control", controlConfig)
	if err != nil {
		test.Fatal(err)
	}
	defer control.HTTP.CloseIdleConnections()
	control.HTTP.Timeout = config.Timeout + 45*time.Second
	body, err := json.Marshal(map[string]string{"mode": mode})
	if err != nil {
		test.Fatal(err)
	}
	response, err := control.DoRaw(runner.Context(test), http.MethodPost, "/restart", body, "application/json", http.Header{"Authorization": []string{"Bearer " + config.ControlToken}})
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusOK)
	var result struct {
		Restarted  bool   `json:"restarted"`
		Generation int    `json:"generation"`
		Mode       string `json:"mode"`
	}
	if json.Unmarshal(response.Body, &result) != nil || !result.Restarted || result.Generation != generation || result.Mode != mode {
		test.Fatal("target control did not prove the expected process generation")
	}
	test.Logf("CANVAS_CONTRACT_RESTART mode=%s generation=%d", mode, generation)
}

func runRestartRecoveryContract(test *testing.T, config runner.Config) {
	owner, foreign, _ := prepareClients(test, config)
	anonymous, err := runner.NewClient("anonymous", config)
	if err != nil {
		test.Fatal(err)
	}
	resourceContent := runner.ImageFixture(test)
	resourceID := uploadResource(test, owner, "restart.png", "image", resourceContent)
	createImageAsset(test, owner, resourceID, len(resourceContent))
	identity := "restart-" + config.RunID
	project := runner.FixtureProject(identity, "Restart initial", 0, false)
	attachResource(project, resourceID)
	createCanvas(test, owner, identity, project)
	project = runner.FixtureProject(identity, "Restart saved", 1, false)
	attachResource(project, resourceID)
	createCanvas(test, owner, identity, project)
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects", map[string]any{"name": "Restart project " + config.RunID, "type": "short-drama", "aspectRatio": "16:9", "sourceType": "original"})
	workbenchID := runner.DecodeData[struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}](test, envelope).Project.ID
	if workbenchID == "" {
		test.Fatal("restart workbench fixture has no identity")
	}
	_, envelope = requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+workbenchID+"/units", map[string]any{"kind": "chapter", "title": "Restart chapter", "sourceText": "restart source text"})
	unitID := runner.DecodeData[struct {
		Unit struct {
			ID string `json:"id"`
		} `json:"unit"`
	}](test, envelope).Unit.ID
	if unitID == "" {
		test.Fatal("restart unit fixture has no identity")
	}
	requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+workbenchID+"/canvas-links", map[string]any{"canvasId": identity, "unitId": unitID})
	_, envelope = requireHTTPSuccess(test, owner, http.MethodPost, "/api/canvas-projects/"+identity+"/share", map[string]any{"expiresDays": 1})
	shareToken := runner.DecodeData[struct {
		Share struct {
			Token string `json:"token"`
		} `json:"share"`
	}](test, envelope).Share.Token
	if shareToken == "" {
		test.Fatal("restart share fixture has no token")
	}
	signedResourcePath := publicResourceAccessPath(test, owner, resourceID)
	before := readCanvasDocument(test, owner, identity)
	historyBefore := readCanvasHistory(test, owner, identity)
	workbenchBefore := readWorkbenchState(test, owner, workbenchID)
	uploadContent := []byte("pending restart upload fixture")
	pending := startChunkUpload(test, owner, len(uploadContent))
	writeChunk(test, owner, "/api/resources/uploads/"+pending.UploadID+"/chunks/0", uploadContent, http.StatusOK)
	deletedID := "deleted-restart-" + config.RunID
	createCanvas(test, owner, deletedID, runner.FixtureProject(deletedID, "Deleted before restart", 0, false))
	requireHTTPSuccess(test, owner, http.MethodDelete, "/api/canvas-projects/"+deletedID, nil)

	if !test.Run("canvas.recovery.document-and-history", func(test *testing.T) {
		restartOwnedTarget(test, config, "crash", 2)
		if !reflect.DeepEqual(before, readCanvasDocument(test, owner, identity)) || !reflect.DeepEqual(historyBefore, readCanvasHistory(test, owner, identity)) {
			test.Fatal("committed document or history changed after process crash")
		}
		requireHTTPError(test, foreign, http.MethodGet, "/api/canvas-projects/"+identity, nil, http.StatusNotFound)
		response, _, err := owner.Do(runner.Context(test), http.MethodGet, "/api/canvas-projects/"+identity, nil, http.Header{"If-None-Match": []string{`"canvas-3"`}})
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireStatus(test, response, http.StatusNotModified)
		stale := runner.FixtureProject(identity, "Stale before restart", 2, false)
		attachResource(stale, resourceID)
		requireHTTPError(test, owner, http.MethodPut, "/api/canvas-projects/"+identity, map[string]any{"project": stale}, http.StatusConflict)
		if !reflect.DeepEqual(before, readCanvasDocument(test, owner, identity)) {
			test.Fatal("stale save after crash changed the durable document")
		}
	}) {
		return
	}
	test.Run("canvas.recovery.share-and-resource", func(test *testing.T) {
		_, envelope := requireHTTPSuccess(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+shareToken, nil)
		public := runner.DecodeData[struct {
			Project map[string]any `json:"project"`
		}](test, envelope)
		if public.Project["id"] != identity || public.Project["title"] != "Restart saved" {
			test.Fatal("share lost its durable target after crash")
		}
		for _, request := range []struct {
			client *runner.Client
			path   string
		}{
			{owner, "/api/resources/" + resourceID + "/file"},
			{anonymous, signedResourcePath},
			{anonymous, "/api/public/canvas-shares/" + shareToken + "/resources/" + resourceID + "/file"},
		} {
			response, err := request.client.DoRaw(runner.Context(test), http.MethodGet, request.path, nil, "", nil)
			if err != nil {
				test.Fatal(err)
			}
			runner.RequireStatus(test, response, http.StatusOK)
			if !bytes.Equal(response.Body, resourceContent) {
				test.Fatal("durable resource bytes changed after crash")
			}
		}
		requireHTTPError(test, foreign, http.MethodGet, "/api/resources/"+resourceID, nil, http.StatusNotFound)
	})
	test.Run("canvas.recovery.project-links", func(test *testing.T) {
		if !reflect.DeepEqual(workbenchBefore, readWorkbenchState(test, owner, workbenchID)) {
			test.Fatal("project or unit state changed after crash")
		}
		_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects/"+workbenchID+"/canvases?page=1", nil)
		page := runner.DecodeData[struct {
			Canvases []map[string]any `json:"canvases"`
			Links    []map[string]any `json:"canvasUnitLinks"`
		}](test, envelope)
		if !containsID(page.Canvases, identity) || len(page.Links) != 1 || page.Links[0]["unitId"] != unitID {
			test.Fatal("project canvas link changed after crash")
		}
	})
	test.Run("canvas.recovery.upload-session-loss", func(test *testing.T) {
		basePath := "/api/resources/uploads/" + pending.UploadID
		writeChunk(test, owner, basePath+"/chunks/0", uploadContent, http.StatusNotFound)
		requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusNotFound)
		fresh := startChunkUpload(test, owner, len(uploadContent))
		writeChunk(test, owner, "/api/resources/uploads/"+fresh.UploadID+"/chunks/0", uploadContent, http.StatusOK)
		completeChunkUpload(test, owner, fresh, uploadContent)
	})
	test.Run("canvas.recovery.deleted-state", func(test *testing.T) {
		requireHTTPError(test, owner, http.MethodGet, "/api/canvas-projects/"+deletedID, nil, http.StatusNotFound)
	})
	test.Run("canvas.recovery.graceful-and-write", func(test *testing.T) {
		restartOwnedTarget(test, config, "graceful", 3)
		if !reflect.DeepEqual(before, readCanvasDocument(test, owner, identity)) || !reflect.DeepEqual(historyBefore, readCanvasHistory(test, owner, identity)) {
			test.Fatal("document or history changed after graceful restart")
		}
		updated := runner.FixtureProject(identity, "Saved after two restarts", 3, false)
		updated["projectId"] = workbenchID
		attachResource(updated, resourceID)
		createCanvas(test, owner, identity, updated)
		getCanvasAndAssert(test, owner, identity, 4, "Saved after two restarts")
	})
	test.Run("canvas.recovery.response-loss-and-restart", func(test *testing.T) {
		runResponseLossRestartMatrix(test, owner, foreign, config)
	})
}
