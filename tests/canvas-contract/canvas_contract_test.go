package canvas_contract_test

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"canvas-contract/harness"
	"canvas-contract/runner"
)

func TestCanvasContract(t *testing.T) {
	config, err := runner.ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("implementation=%s database=%s profile=%s run_id=%s", config.Implementation, config.DatabaseProfile, config.Profile, config.RunID)
	switch config.Profile {
	case "core":
		runCoreContract(t, config)
	case "share-public":
		runSharePublicContract(t, config)
	case "project-integration":
		runProjectIntegrationContract(t, config)
	case "project-workbench-core":
		runProjectWorkbenchCoreContract(t, config)
	case "resource-integrity":
		runResourceIntegrityContract(t, config)
	case "import":
		runImportContract(t, config)
	case "legacy-assets":
		runLegacyAssetsContract(t, config)
	case "public-resource":
		runPublicResourceContract(t, config)
	case "restart-recovery":
		runRestartRecoveryContract(t, config)
	case "generation-boundary":
		runGenerationBoundaryContract(t, config)
	default:
		t.Fatalf("profile %q has no executable runner", config.Profile)
	}
}

func runCoreContract(t *testing.T, config runner.Config) {

	admin, err := runner.NewClient("admin", config)
	if err != nil {
		t.Fatal(err)
	}
	users := runner.PrepareUsers(t, config, admin)
	login(t, admin, users.A)

	clientA, err := runner.NewClient("user-a", config)
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := runner.NewClient("user-b", config)
	if err != nil {
		t.Fatal(err)
	}
	login(t, clientA, users.A)
	login(t, clientB, users.B)

	t.Run("canvas.authz.unauthenticated", func(t *testing.T) {
		anonymous, err := runner.NewClient("anonymous", config)
		if err != nil {
			t.Fatal(err)
		}
		for _, request := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/canvas-projects"},
			{http.MethodGet, "/api/canvas-projects/unauthenticated"},
			{http.MethodPut, "/api/canvas-projects/unauthenticated"},
			{http.MethodDelete, "/api/canvas-projects/unauthenticated"},
			{http.MethodGet, "/api/canvas-projects/unauthenticated/history"},
			{http.MethodGet, "/api/canvas-projects/unauthenticated/history/snapshot"},
			{http.MethodPost, "/api/canvas-projects/unauthenticated/history/snapshot/restore"},
		} {
			response, envelope, err := anonymous.Do(runner.Context(t), request.method, request.path, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireError(t, response, envelope, http.StatusUnauthorized, "")
		}
	})

	canvasID := fmt.Sprintf("contract-%s", config.RunID)
	project := runner.FixtureProject(canvasID, "Contract initial", 0, true)

	t.Run("canvas.create.read", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": project}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		var summary struct {
			Project struct {
				ID       string `json:"id"`
				Revision int    `json:"revision"`
			} `json:"project"`
		}
		data := runner.DecodeData[struct {
			Project struct {
				ID       string `json:"id"`
				Revision int    `json:"revision"`
			} `json:"project"`
		}](t, envelope)
		summary = data
		if summary.Project.ID != canvasID || summary.Project.Revision != 1 {
			t.Fatal("create summary")
		}

		getCanvasAndAssert(t, clientA, canvasID, 1, "Contract initial")
	})

	t.Run("canvas.list", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		data := runner.DecodeData[struct {
			Projects []map[string]any `json:"projects"`
		}](t, envelope)
		if !containsID(data.Projects, canvasID) {
			t.Fatal("canvas missing from its owner list")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects?page=1&pageSize=40", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		var page map[string]any
		page = runner.DecodeData[map[string]any](t, envelope)
		if _, ok := page["projects"]; !ok {
			t.Fatal("paginated list has no projects field")
		}
	})

	t.Run("canvas.conditional-read", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		etag := response.Header.Get("ETag")
		if etag != `W/"canvas-1"` {
			t.Fatalf("ETag = %q", etag)
		}
		if response.Header.Get("Cache-Control") != "private, no-cache" {
			t.Fatalf("Cache-Control = %q", response.Header.Get("Cache-Control"))
		}
		if response.Header.Get("Vary") != "Accept-Encoding" {
			t.Fatalf("Vary = %q", response.Header.Get("Vary"))
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, http.Header{"If-None-Match": []string{`"canvas-1"`}})
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusNotModified)
		if len(response.Body) != 0 || envelope.Code != 0 {
			t.Fatal("304 response must have an empty body")
		}
	})

	t.Run("canvas.update.cas", func(t *testing.T) {
		updated := runner.FixtureProject(canvasID, "Contract updated", 1, true)
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": updated}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		data := runner.DecodeData[struct {
			Project struct {
				Revision int `json:"revision"`
			} `json:"project"`
		}](t, envelope)
		if data.Project.Revision != 2 {
			t.Fatalf("updated revision = %d", data.Project.Revision)
		}
		getCanvasAndAssert(t, clientA, canvasID, 2, "Contract updated")

		stale := runner.FixtureProject(canvasID, "Contract stale", 1, false)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": stale}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "conflict")

		missingRevision := runner.FixtureProject(canvasID, "Contract missing revision", 0, false)
		delete(missingRevision, "revision")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": missingRevision}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusPreconditionRequired, "")

		mismatched := runner.FixtureProject("different-id", "Contract mismatch", 2, false)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": mismatched}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusBadRequest, "")
		getCanvasAndAssert(t, clientA, canvasID, 2, "Contract updated")
	})

	t.Run("canvas.authz.foreign-user", func(t *testing.T) {
		response, envelope, err := clientB.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
		response, envelope, err = clientB.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": runner.FixtureProject(canvasID, "Foreign write", 2, false)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "conflict")
	})

	t.Run("canvas.authz.foreign-delete", func(t *testing.T) {
		response, envelope, err := clientB.Do(runner.Context(t), http.MethodDelete, "/api/canvas-projects/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		getCanvasAndAssert(t, clientA, canvasID, 2, "Contract updated")
	})

	t.Run("canvas.history.lifecycle", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID+"/history", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		data := runner.DecodeData[struct {
			Snapshots []struct {
				ID       string `json:"id"`
				Revision int    `json:"revision"`
				Reason   string `json:"reason"`
			} `json:"snapshots"`
			CurrentRevision int `json:"currentRevision"`
		}](t, envelope)
		if len(data.Snapshots) == 0 || data.CurrentRevision != 2 {
			t.Fatal("history")
		}
		snapshotID := data.Snapshots[0].ID
		for _, request := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/canvas-projects/" + canvasID + "/history"},
			{http.MethodGet, "/api/canvas-projects/" + canvasID + "/history/" + snapshotID},
			{http.MethodPost, "/api/canvas-projects/" + canvasID + "/history/" + snapshotID + "/restore"},
		} {
			foreignResponse, foreignEnvelope, err := clientB.Do(runner.Context(t), request.method, request.path, map[string]int{"revision": 2}, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireError(t, foreignResponse, foreignEnvelope, http.StatusNotFound, "")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID+"/history/"+snapshotID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		snapshot := runner.DecodeData[struct {
			Snapshot map[string]any `json:"snapshot"`
			Project  map[string]any `json:"project"`
		}](t, envelope)
		if snapshot.Snapshot["id"] != snapshotID || snapshot.Project["title"] != "Contract initial" {
			t.Fatal("history snapshot")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/canvas-projects/"+canvasID+"/history/"+snapshotID+"/restore", map[string]int{"revision": 1}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "conflict")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/canvas-projects/"+canvasID+"/history/"+snapshotID+"/restore", map[string]any{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusPreconditionRequired, "")

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/canvas-projects/"+canvasID+"/history/"+snapshotID+"/restore", map[string]int{"revision": 2}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		data2 := runner.DecodeData[struct {
			Project struct {
				Revision int `json:"revision"`
			} `json:"project"`
		}](t, envelope)
		if data2.Project.Revision != 3 {
			t.Fatalf("restore revision = %d", data2.Project.Revision)
		}
		getCanvasAndAssert(t, clientA, canvasID, 3, "Contract initial")
	})

	t.Run("canvas.compression", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, http.Header{"Accept-Encoding": []string{"gzip"}})
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		if response.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("Content-Encoding = %q", response.Header.Get("Content-Encoding"))
		}
		body := runner.RequireJSONBody(t, response)
		if body["code"] != float64(0) {
			t.Fatal("decompressed envelope")
		}
	})

	t.Run("canvas.concurrent-cas", func(t *testing.T) {
		clientA1, err := runner.NewClient("user-a-1", config)
		if err != nil {
			t.Fatal(err)
		}
		clientA2, err := runner.NewClient("user-a-2", config)
		if err != nil {
			t.Fatal(err)
		}
		login(t, clientA1, users.A)
		login(t, clientA2, users.A)
		const baseRevision = 3
		start := make(chan struct{})
		type casResult struct {
			response runner.Response
			envelope runner.Envelope
			err      error
		}
		results := make(chan casResult, 2)
		var wait sync.WaitGroup
		requestContext := runner.Context(t)
		for index, client := range []*runner.Client{clientA1, clientA2} {
			wait.Add(1)
			go func(index int, client *runner.Client) {
				defer wait.Done()
				<-start
				response, envelope, requestErr := client.Do(requestContext, http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": runner.FixtureProject(canvasID, fmt.Sprintf("Concurrent %d", index), baseRevision, false)}, nil)
				results <- casResult{response: response, envelope: envelope, err: requestErr}
			}(index, client)
		}
		close(start)
		wait.Wait()
		close(results)
		successes, conflicts := 0, 0
		for result := range results {
			if result.err != nil {
				t.Fatal(result.err)
			}
			switch result.response.StatusCode {
			case http.StatusOK:
				successes++
				runner.RequireSuccess(t, result.response, result.envelope)
			case http.StatusConflict:
				conflicts++
				runner.RequireError(t, result.response, result.envelope, http.StatusConflict, "conflict")
			default:
				t.Fatalf("concurrent CAS status = %d: %s", result.response.StatusCode, result.response.String())
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent CAS outcomes: successes=%d conflicts=%d", successes, conflicts)
		}
		getCanvasAndAssert(t, clientA, canvasID, 4, "Concurrent ")
	})

	t.Run("canvas.delete.lifecycle", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodDelete, "/api/canvas-projects/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID+"/history", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/canvas-projects/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
	})

	t.Run("canvas.update.repair-missing-resources", func(t *testing.T) {
		repairCanvasID := "repair-" + config.RunID
		createCanvas(t, clientA, repairCanvasID, runner.FixtureProject(repairCanvasID, "Repair preimage", 0, false))
		invalid := runner.FixtureProject(repairCanvasID, "Invalid repair", 1, false)
		attachResource(invalid, "missing-repair-resource-"+config.RunID)
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+repairCanvasID, map[string]any{"project": invalid, "repairMissingResources": true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "")
		getCanvasAndAssert(t, clientA, repairCanvasID, 1, "Repair preimage")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+repairCanvasID+"/history", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		failedHistory := runner.DecodeData[struct {
			Snapshots []map[string]any `json:"snapshots"`
		}](t, envelope)
		if len(failedHistory.Snapshots) != 0 {
			t.Fatal("failed repair created history")
		}
		repaired := runner.FixtureProject(repairCanvasID, "Repaired document", 1, false)
		response, envelope, err = clientB.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+repairCanvasID, map[string]any{"project": repaired, "repairMissingResources": true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "conflict")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+repairCanvasID, map[string]any{"project": repaired, "repairMissingResources": true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		getCanvasAndAssert(t, clientA, repairCanvasID, 2, "Repaired document")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+repairCanvasID, map[string]any{"project": repaired, "repairMissingResources": true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "conflict")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+repairCanvasID+"/history", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		history := runner.DecodeData[struct {
			Snapshots []struct {
				ID       string `json:"id"`
				Revision int    `json:"revision"`
				Reason   string `json:"reason"`
			} `json:"snapshots"`
		}](t, envelope)
		if len(history.Snapshots) != 1 || history.Snapshots[0].Reason != "before_resource_repair" || history.Snapshots[0].Revision != 1 {
			t.Fatal("repair did not preserve exactly one preimage at revision 1")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+repairCanvasID+"/history/"+history.Snapshots[0].ID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		snapshot := runner.DecodeData[struct {
			Project map[string]any `json:"project"`
		}](t, envelope)
		if snapshot.Project["title"] != "Repair preimage" {
			t.Fatal("repair preimage title changed")
		}
		getCanvasAndAssert(t, clientA, repairCanvasID, 2, "Repaired document")
	})

	t.Run("canvas.list.pagination", func(t *testing.T) {
		runCanvasPageMatrix(t, clientA, clientB, config.RunID)
	})
	t.Run("canvas.write.invalid-input", func(t *testing.T) {
		runCanvasInvalidWriteMatrix(t, clientA, config.RunID)
	})
	t.Run("canvas.history.retention-and-forced-preimage", func(t *testing.T) {
		runCanvasHistoryRetention(t, clientA, clientB, config.RunID)
	})
	t.Run("canvas.authz.logout", func(t *testing.T) {
		origin, err := url.Parse(config.BaseURL)
		if err != nil {
			t.Fatal(err)
		}
		cookies := clientA.HTTP.Jar.Cookies(origin)
		if len(cookies) == 0 {
			t.Fatal("logout fixture has no authenticated session cookie")
		}
		runner.Logout(t, clientA)
		revoked, err := runner.NewClient("revoked-session", config)
		if err != nil {
			t.Fatal(err)
		}
		revoked.HTTP.Jar.SetCookies(origin, cookies)
		for _, client := range []*runner.Client{clientA, revoked} {
			for _, path := range []string{"/api/canvas-projects", "/api/canvas-projects/" + canvasID} {
				response, envelope, requestErr := client.Do(runner.Context(t), http.MethodGet, path, nil, nil)
				if requestErr != nil {
					t.Fatal(requestErr)
				}
				runner.RequireError(t, response, envelope, http.StatusUnauthorized, "")
			}
		}
	})
}

func runSharePublicContract(t *testing.T, config runner.Config) {
	clientA, clientB, admin := prepareClients(t, config)
	anonymous, err := runner.NewClient("anonymous", config)
	if err != nil {
		t.Fatal(err)
	}
	canvasID := fmt.Sprintf("share-%s", config.RunID)
	resourceContent := runner.ImageFixture(t)
	resourceID := uploadResource(t, clientA, "share-fixture.png", "image", resourceContent)
	createImageAsset(t, clientA, resourceID, len(resourceContent))
	project := runner.FixtureProject(canvasID, "Share contract", 0, false)
	attachResource(project, resourceID)
	createCanvas(t, clientA, canvasID, project)

	t.Run("canvas.share.authz", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			response, envelope, err := anonymous.Do(runner.Context(t), method, "/api/canvas-projects/"+canvasID+"/share", map[string]any{"expiresDays": 1}, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireError(t, response, envelope, http.StatusUnauthorized, "")
			response, envelope, err = clientB.Do(runner.Context(t), method, "/api/canvas-projects/"+canvasID+"/share", map[string]any{"expiresDays": 1}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if method == http.MethodGet {
				runner.RequireError(t, response, envelope, http.StatusNotFound, "")
			} else {
				runner.RequireError(t, response, envelope, http.StatusInternalServerError, "internal")
			}
		}
		getCanvasAndAssert(t, clientA, canvasID, 1, "Share contract")
	})

	t.Run("canvas.share.lifecycle", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID+"/share", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		status := runner.DecodeData[struct {
			Share struct {
				Enabled bool `json:"enabled"`
			} `json:"share"`
		}](t, envelope)
		if status.Share.Enabled {
			t.Fatalf("share is enabled before creation")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/canvas-projects/"+canvasID+"/share", map[string]any{"expiresDays": 1}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		created := runner.DecodeData[struct {
			Share struct {
				Enabled bool   `json:"enabled"`
				Token   string `json:"token"`
			} `json:"share"`
		}](t, envelope)
		if !created.Share.Enabled || created.Share.Token == "" {
			t.Fatal("share create did not return an enabled share with a token")
		}
		oldToken := created.Share.Token

		response, envelope, err = anonymous.Do(runner.Context(t), http.MethodGet, "/api/public/canvas-shares/"+oldToken, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Referrer-Policy") != "no-referrer" || response.Header.Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatal("public share headers")
		}
		public := runner.DecodeData[struct {
			Project map[string]any `json:"project"`
		}](t, envelope)
		if public.Project["id"] != canvasID || public.Project["customExtension"] != nil {
			t.Fatal("public project identity/scrubbing")
		}
		publicResourceURL := publicResourceURLFromProject(t, public.Project)
		resourceResponse, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, publicResourceURL, nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, resourceResponse, http.StatusOK)
		if string(resourceResponse.Body) != string(resourceContent) || resourceResponse.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("public resource response = %s", resourceResponse.String())
		}

		response, envelope, err = clientB.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID+"/share", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/canvas-projects/"+canvasID+"/share", map[string]any{"expiresDays": 1, "rotate": true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		rotated := runner.DecodeData[struct {
			Share struct {
				Token string `json:"token"`
			} `json:"share"`
		}](t, envelope)
		if rotated.Share.Token == "" || rotated.Share.Token == oldToken {
			t.Fatal("share rotation did not issue a new non-empty token")
		}
		response, envelope, err = anonymous.Do(runner.Context(t), http.MethodGet, "/api/public/canvas-shares/"+oldToken, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/canvas-projects/"+canvasID+"/share", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = anonymous.Do(runner.Context(t), http.MethodGet, "/api/public/canvas-shares/"+rotated.Share.Token, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
	})

	t.Run("canvas.share.invalid-token", func(t *testing.T) {
		response, envelope, err := anonymous.Do(runner.Context(t), http.MethodGet, "/api/public/canvas-shares/not-a-valid-token", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
	})
	t.Run("canvas.share.invalid-input-and-replay", func(t *testing.T) {
		runShareInvalidInputMatrix(t, clientA, anonymous, config.RunID)
	})
	t.Run("canvas.share.resource-allowlist-and-live-document", func(t *testing.T) {
		runShareResourceMatrix(t, clientA, clientB, anonymous, config.RunID)
	})
	t.Run("canvas.share.concurrent-rotate-and-revoke", func(t *testing.T) {
		runShareConcurrentMatrix(t, clientA, anonymous, config.RunID)
	})
	t.Run("canvas.share.concurrent-first-create", func(t *testing.T) {
		runShareFirstCreateConcurrentMatrix(t, clientA, clientB, anonymous, config.RunID)
	})
	t.Run("canvas.share.resource-description-and-revocation", func(test *testing.T) {
		runSharedResourceDescription(test, clientA, clientB, anonymous, config.RunID)
	})
	t.Run("canvas.share.expiry-and-recovery", func(test *testing.T) {
		runShareExpiryRecovery(test, clientA, clientB, anonymous, config, config.RunID)
	})
	t.Run("canvas.share.resource-object-storage-failure", func(test *testing.T) {
		runShareResourceObjectStorageFailure(test, clientA, admin, anonymous, config.RunID)
	})
}

func runProjectIntegrationContract(t *testing.T, config runner.Config) {
	clientA, clientB, admin := prepareClients(t, config)
	response, envelope, err := clientA.Do(runner.Context(t), http.MethodPost, "/api/projects", map[string]any{
		"name":        "Contract project " + config.RunID,
		"type":        "short-drama",
		"aspectRatio": "16:9",
		"sourceType":  "original",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	project := runner.DecodeData[struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}](t, envelope)
	projectID := project.Project.ID
	if projectID == "" {
		t.Fatal("project create response")
	}

	response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/projects/"+projectID+"/units", map[string]any{
		"kind":       "chapter",
		"title":      "Contract chapter",
		"sourceText": "contract source",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	unit := runner.DecodeData[struct {
		Unit struct {
			ID string `json:"id"`
		} `json:"unit"`
	}](t, envelope)
	unitID := unit.Unit.ID
	if unitID == "" {
		t.Fatal("unit create response")
	}

	canvasID := fmt.Sprintf("project-%s", config.RunID)
	createCanvas(t, clientA, canvasID, runner.FixtureProject(canvasID, "Project canvas", 0, false))

	t.Run("project.canvas.foreign", func(t *testing.T) {
		response, envelope, err := clientB.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/canvases", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusInternalServerError, "internal")
	})

	t.Run("project.canvas.validation", func(t *testing.T) {
		runProjectCanvasValidationMatrix(t, clientA, clientB, projectID, unitID, canvasID)
	})
	t.Run("project.canvas.feature-gate", func(t *testing.T) {
		runProjectFeatureGate(t, clientA, clientB, admin, projectID, unitID, canvasID)
	})
	t.Run("project.canvas.lifecycle", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": unitID, "role": "storyboard"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/canvases?page=1&pageSize=20", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		page := runner.DecodeData[struct {
			Canvases []map[string]any `json:"canvases"`
			Links    []map[string]any `json:"canvasUnitLinks"`
		}](t, envelope)
		if !containsID(page.Canvases, canvasID) || len(page.Links) != 1 {
			t.Fatal("project canvas page")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects?page=1&pageSize=20&projectId="+projectID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		linkedPage := runner.DecodeData[struct {
			Projects []map[string]any `json:"projects"`
			Total    int              `json:"total"`
		}](t, envelope)
		if linkedPage.Total != 1 || !containsID(linkedPage.Projects, canvasID) {
			t.Fatal("project-linked canvas filter did not return the linked document")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects?page=1&pageSize=20&projectId=independent", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		independentPage := runner.DecodeData[struct {
			Projects []map[string]any `json:"projects"`
			Total    int              `json:"total"`
		}](t, envelope)
		if independentPage.Total != 0 || containsID(independentPage.Projects, canvasID) {
			t.Fatal("project-linked canvas leaked into the independent filter")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": unitID, "role": "storyboard"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/projects/"+projectID+"/canvas-links/"+canvasID+"/units/"+unitID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/projects/"+projectID+"/canvases/"+canvasID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		getCanvasAndAssert(t, clientA, canvasID, 4, "Project canvas")
	})
	t.Run("project.canvas.cross-project-and-reassociation", func(t *testing.T) {
		runProjectReassociationMatrix(t, clientA, clientB, config.RunID)
	})
	t.Run("project.canvas.multi-unit-roles-and-replay", func(test *testing.T) {
		runProjectMultiUnitRoles(test, clientA, clientB, config.RunID+"-a")
		runProjectMultiUnitRoles(test, clientB, clientA, config.RunID+"-b")
	})
	t.Run("project.canvas.role-overflow-partial-state", func(test *testing.T) {
		runProjectRoleOverflow(test, clientA, clientB, config.RunID+"-a")
		runProjectRoleOverflow(test, clientB, clientA, config.RunID+"-b")
	})
	t.Run("project.canvas.concurrent-link-settlement", func(test *testing.T) {
		runProjectConcurrentLinks(test, clientA, clientB, config.RunID+"-a")
		runProjectConcurrentLinks(test, clientB, clientA, config.RunID+"-b")
	})
	t.Run("project.canvas.concurrent-cross-project-reassociation", func(test *testing.T) {
		runProjectConcurrentReassociation(test, clientA, clientB, config.RunID+"-a")
		runProjectConcurrentReassociation(test, clientB, clientA, config.RunID+"-b")
	})
}

func runProjectWorkbenchCoreContract(t *testing.T, config runner.Config) {
	clientA, clientB, admin := prepareClients(t, config)
	anonymous, err := runner.NewClient("anonymous", config)
	if err != nil {
		t.Fatal(err)
	}
	projectID := ""
	unitIDs := make([]string, 0, 3)

	if !t.Run("project.workbench.lifecycle", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPost, "/api/projects", map[string]any{
			"name":        "Contract workbench " + config.RunID,
			"type":        "short-drama",
			"aspectRatio": "9:16",
			"sourceType":  "blank",
			"description": "project workbench contract fixture",
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		created := runner.DecodeData[struct {
			Project struct {
				ID string `json:"id"`
			} `json:"project"`
		}](t, envelope)
		projectID = created.Project.ID
		if projectID == "" {
			t.Fatal("project create response")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		list := runner.DecodeData[struct {
			Projects []map[string]any `json:"projects"`
		}](t, envelope)
		if !containsProjectSummaryID(list.Projects, projectID) {
			t.Fatal("project list does not contain created project")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects?page=1&pageSize=20", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		page := runner.DecodeData[struct {
			Projects []map[string]any `json:"projects"`
			Page     int              `json:"page"`
			PageSize int              `json:"pageSize"`
		}](t, envelope)
		if page.Page != 1 || page.PageSize != 20 || !containsProjectSummaryID(page.Projects, projectID) {
			t.Fatal("project page")
		}

		for _, path := range []string{
			"/api/projects/" + projectID,
			"/api/projects/" + projectID + "/core",
			"/api/projects/" + projectID + "/overview",
		} {
			response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, path, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireSuccess(t, response, envelope)
			data := runner.DecodeData[map[string]any](t, envelope)
			if strings.HasSuffix(path, "/overview") {
				metrics, ok := data["metrics"].(map[string]any)
				if !ok {
					t.Fatal("project overview has no metrics object")
				}
				runner.RequireNumber(t, metrics["unitCount"], 0, "overview unit count")
			} else {
				project, ok := data["project"].(map[string]any)
				if !ok || project["id"] != projectID || project["name"] != "Contract workbench "+config.RunID {
					t.Fatal("project read did not preserve identity and name")
				}
			}
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPatch, "/api/projects/"+projectID, map[string]any{
			"name": "Contract workbench updated " + config.RunID,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		state := readWorkbenchState(t, clientA, projectID)
		if state.Project["name"] != "Contract workbench updated "+config.RunID {
			t.Fatal("project update was not persisted")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/projects/"+projectID+"/units", map[string]any{
			"kind":       "chapter",
			"title":      "第一章",
			"sourceText": "第一章正文",
			"position":   0,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		firstUnit := runner.DecodeData[struct {
			Unit struct {
				ID string `json:"id"`
			} `json:"unit"`
		}](t, envelope)
		unitIDs = append(unitIDs, firstUnit.Unit.ID)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/projects/"+projectID+"/units", map[string]any{
			"kind":       "episode",
			"title":      "第二集",
			"sourceText": "第二集正文",
			"position":   1,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		secondUnit := runner.DecodeData[struct {
			Unit struct {
				ID string `json:"id"`
			} `json:"unit"`
		}](t, envelope)
		unitIDs = append(unitIDs, secondUnit.Unit.ID)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		unitPage := runner.DecodeData[struct {
			Units []map[string]any `json:"units"`
		}](t, envelope)
		if len(unitPage.Units) != 2 || !containsID(unitPage.Units, unitIDs[0]) || !containsID(unitPage.Units, unitIDs[1]) {
			t.Fatal("project unit list")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units/"+unitIDs[0], nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		unit := runner.DecodeData[struct {
			Unit map[string]any `json:"unit"`
		}](t, envelope)
		if unit.Unit["id"] != unitIDs[0] {
			t.Fatal("project unit read")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units/"+unitIDs[0]+"/workspace", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		workspace := runner.DecodeData[struct {
			Unit map[string]any `json:"unit"`
		}](t, envelope)
		if workspace.Unit["id"] != unitIDs[0] || workspace.Unit["sourceText"] != "第一章正文" {
			t.Fatal("unit workspace did not return the owned unit正文")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/projects/"+projectID+"/units/import", map[string]any{
			"units": []map[string]any{{"kind": "chapter", "title": "第三章", "sourceText": "第三章正文", "position": 2}},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		imported := runner.DecodeData[struct {
			Units []struct {
				ID string `json:"id"`
			} `json:"units"`
		}](t, envelope)
		if len(imported.Units) != 1 || imported.Units[0].ID == "" {
			t.Fatal("project unit import")
		}
		unitIDs = append(unitIDs, imported.Units[0].ID)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPatch, "/api/projects/"+projectID+"/units/reorder", map[string]any{
			"unitIds": []string{unitIDs[1], unitIDs[2], unitIDs[0]},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		reordered := readWorkbenchState(t, clientA, projectID)
		for position, expectedID := range []string{unitIDs[1], unitIDs[2], unitIDs[0]} {
			if len(reordered.Units) != 3 || reordered.Units[position]["id"] != expectedID {
				t.Fatal("unit reorder response did not persist the requested order")
			}
			runner.RequireNumber(t, reordered.Units[position]["position"], float64(position), "unit position")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPatch, "/api/projects/"+projectID+"/units/"+unitIDs[0], map[string]any{
			"title":      "第一章（已更新）",
			"sourceText": "更新后的第一章正文",
			"status":     "draft",
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units/"+unitIDs[0], nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		updatedUnit := runner.DecodeData[struct {
			Unit map[string]any `json:"unit"`
		}](t, envelope)
		if updatedUnit.Unit["title"] != "第一章（已更新）" || updatedUnit.Unit["sourceText"] != "更新后的第一章正文" {
			t.Fatal("unit update was not persisted")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/projects/"+projectID+"/units/"+unitIDs[2], nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		remaining := readWorkbenchState(t, clientA, projectID)
		if len(remaining.Units) != 2 || containsID(remaining.Units, unitIDs[2]) {
			t.Fatal("deleted unit remains in the owner list")
		}
	}) {
		t.Fatal("project workbench fixture setup failed; dependent scenarios cannot run")
	}
	if projectID == "" || len(unitIDs) < 3 {
		t.Fatal("project workbench lifecycle did not establish project and unit fixtures")
	}
	t.Run("project.workbench.feature-gate", func(t *testing.T) {
		runProjectFeatureGate(t, clientA, clientB, admin, projectID, unitIDs[0], "")
	})

	t.Run("project.workbench.authz", func(t *testing.T) {
		before := readWorkbenchState(t, clientA, projectID)
		for _, request := range []struct {
			method  string
			path    string
			payload any
		}{
			{http.MethodGet, "/api/projects", nil},
			{http.MethodPost, "/api/projects", map[string]any{"name": "anonymous project"}},
			{http.MethodGet, "/api/projects/" + projectID, nil},
			{http.MethodGet, "/api/projects/" + projectID + "/core", nil},
			{http.MethodGet, "/api/projects/" + projectID + "/overview", nil},
			{http.MethodPatch, "/api/projects/" + projectID, map[string]any{"name": "anonymous update"}},
			{http.MethodDelete, "/api/projects/" + projectID, nil},
			{http.MethodGet, "/api/projects/" + projectID + "/units", nil},
			{http.MethodGet, "/api/projects/" + projectID + "/units/" + unitIDs[0], nil},
			{http.MethodGet, "/api/projects/" + projectID + "/units/" + unitIDs[0] + "/workspace", nil},
			{http.MethodPost, "/api/projects/" + projectID + "/units", map[string]any{"title": "anonymous unit"}},
			{http.MethodPost, "/api/projects/" + projectID + "/units/import", map[string]any{"units": []any{}}},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/reorder", map[string]any{"unitIds": unitIDs[:2]}},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/" + unitIDs[0], map[string]any{"title": "anonymous update"}},
			{http.MethodDelete, "/api/projects/" + projectID + "/units/" + unitIDs[0], nil},
		} {
			response, envelope, err := anonymous.Do(runner.Context(t), request.method, request.path, request.payload, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireError(t, response, envelope, http.StatusUnauthorized, "")
		}

		for _, request := range []struct {
			method  string
			path    string
			payload any
			status  int
			reason  string
		}{
			{http.MethodGet, "/api/projects/" + projectID, nil, http.StatusNotFound, ""},
			{http.MethodGet, "/api/projects/" + projectID + "/core", nil, http.StatusNotFound, ""},
			{http.MethodGet, "/api/projects/" + projectID + "/overview", nil, http.StatusInternalServerError, "internal"},
			{http.MethodPatch, "/api/projects/" + projectID, map[string]any{"name": "foreign update"}, http.StatusNotFound, ""},
			{http.MethodDelete, "/api/projects/" + projectID, nil, http.StatusNotFound, ""},
			{http.MethodGet, "/api/projects/" + projectID + "/units", nil, http.StatusInternalServerError, "internal"},
			{http.MethodGet, "/api/projects/" + projectID + "/units/" + unitIDs[0], nil, http.StatusNotFound, ""},
			{http.MethodGet, "/api/projects/" + projectID + "/units/" + unitIDs[0] + "/workspace", nil, http.StatusInternalServerError, "internal"},
			{http.MethodPost, "/api/projects/" + projectID + "/units", map[string]any{"title": "foreign unit"}, http.StatusNotFound, ""},
			{http.MethodPost, "/api/projects/" + projectID + "/units/import", map[string]any{"units": []any{}}, http.StatusNotFound, ""},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/reorder", map[string]any{"unitIds": unitIDs[:2]}, http.StatusNotFound, ""},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/" + unitIDs[0], map[string]any{"title": "foreign update"}, http.StatusNotFound, ""},
			{http.MethodDelete, "/api/projects/" + projectID + "/units/" + unitIDs[0], nil, http.StatusNotFound, ""},
		} {
			response, envelope, err := clientB.Do(runner.Context(t), request.method, request.path, request.payload, nil)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != request.status {
				t.Fatalf("foreign request %s %s returned status=%d reason=%q, want status=%d", request.method, request.path, response.StatusCode, envelope.Reason, request.status)
			}
			runner.RequireError(t, response, envelope, request.status, request.reason)
		}
		if !reflect.DeepEqual(before, readWorkbenchState(t, clientA, projectID)) {
			t.Fatal("rejected foreign requests mutated the owner's project or units")
		}
	})
	t.Run("project.workbench.invalid-mutations", func(t *testing.T) {
		before := readWorkbenchState(t, clientA, projectID)
		for _, request := range []struct {
			method  string
			path    string
			payload any
		}{
			{http.MethodPatch, "/api/projects/" + projectID, map[string]any{"status": "invalid-contract-status"}},
			{http.MethodPost, "/api/projects/" + projectID + "/units/import", map[string]any{"units": []any{}}},
			{http.MethodPost, "/api/projects/" + projectID + "/units/import", map[string]any{"units": []any{map[string]any{"title": "valid-row-before-invalid", "kind": "chapter"}, map[string]any{"title": "", "kind": "chapter"}}}},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/reorder", map[string]any{"unitIds": []string{unitIDs[0]}}},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/reorder", map[string]any{"unitIds": []string{unitIDs[0], unitIDs[0]}}},
			{http.MethodPatch, "/api/projects/" + projectID + "/units/reorder", map[string]any{"unitIds": []string{unitIDs[0], "missing-unit"}}},
		} {
			response, envelope, err := clientA.Do(runner.Context(t), request.method, request.path, request.payload, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireError(t, response, envelope, http.StatusBadRequest, "")
			if !reflect.DeepEqual(before, readWorkbenchState(t, clientA, projectID)) {
				t.Fatal("invalid mutation changed project revision, unit membership or ordering")
			}
		}
	})
	t.Run("project.workbench.delete", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodDelete, "/api/projects/"+projectID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units/"+unitIDs[0], nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/projects/"+projectID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
	})
	t.Run("project.workbench.duplicate-creation-and-replay", func(t *testing.T) {
		runWorkbenchDuplicateAndReplay(t, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.cross-project-units", func(t *testing.T) {
		runWorkbenchCrossProjectUnits(t, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.concurrent-reorder", func(t *testing.T) {
		runWorkbenchConcurrentReorder(t, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.deletion-cascade", func(t *testing.T) {
		runWorkbenchDeletionCascade(t, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.bulk-import-boundaries", func(t *testing.T) {
		runWorkbenchBulkImportBoundaries(t, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.import-body-budget-and-framing", func(test *testing.T) {
		runWorkbenchImportBodyBoundary(test, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.concurrent-opposite-order", func(test *testing.T) {
		runWorkbenchOppositeOrder(test, clientA, clientB, config.RunID)
	})
	t.Run("project.workbench.reorder-versus-create", func(test *testing.T) {
		runWorkbenchReorderMembership(test, clientA, clientB, config.RunID, "create")
	})
	t.Run("project.workbench.reorder-versus-delete", func(test *testing.T) {
		runWorkbenchReorderMembership(test, clientA, clientB, config.RunID, "delete")
	})
	t.Run("project.workbench.reorder-versus-import", func(test *testing.T) {
		runWorkbenchReorderMembership(test, clientA, clientB, config.RunID, "import")
	})
}

type workbenchState struct {
	Project map[string]any
	Units   []map[string]any
}

func readWorkbenchState(t *testing.T, client *runner.Client, projectID string) workbenchState {
	t.Helper()
	response, envelope, err := client.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	core := runner.DecodeData[struct {
		Project map[string]any `json:"project"`
	}](t, envelope)
	if core.Project["id"] != projectID {
		t.Fatal("project core identity missing")
	}
	response, envelope, err = client.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	units := runner.DecodeData[struct {
		Units []map[string]any `json:"units"`
	}](t, envelope)
	for index, summary := range units.Units {
		identity, ok := summary["id"].(string)
		if !ok || identity == "" {
			t.Fatal("unit summary has no identity for a complete state comparison")
		}
		response, envelope, err = client.Do(runner.Context(t), http.MethodGet, "/api/projects/"+projectID+"/units/"+identity, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		detail := runner.DecodeData[struct {
			Unit map[string]any `json:"unit"`
		}](t, envelope).Unit
		if detail["id"] != identity || detail["projectId"] != projectID {
			t.Fatal("unit detail did not preserve the summary identity and project scope")
		}
		units.Units[index] = detail
	}
	return workbenchState{Project: core.Project, Units: units.Units}
}

func runResourceIntegrityContract(t *testing.T, config runner.Config) {
	clientA, clientB, admin := prepareClients(t, config)
	resourceID := "unauthenticated"

	t.Run("canvas.resource.unauthenticated", func(t *testing.T) {
		anonymous, err := runner.NewClient("anonymous", config)
		if err != nil {
			t.Fatal(err)
		}
		for _, request := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/resources"},
			{http.MethodGet, "/api/resources/storage-usage"},
			{http.MethodGet, "/api/resources/" + resourceID},
			{http.MethodGet, "/api/resources/" + resourceID + "/file"},
			{http.MethodPost, "/api/resources/access"},
			{http.MethodPost, "/api/resources"},
			{http.MethodPost, "/api/resources/import"},
			{http.MethodPost, "/api/resources/uploads"},
			{http.MethodPut, "/api/resources/uploads/unauthenticated/chunks/0"},
			{http.MethodPost, "/api/resources/uploads/unauthenticated/complete"},
		} {
			response, envelope, err := anonymous.Do(runner.Context(t), request.method, request.path, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner.RequireError(t, response, envelope, http.StatusUnauthorized, "")
		}
	})

	if !t.Run("canvas.resource.metadata-and-access", func(t *testing.T) {
		resourceID = uploadResource(t, clientA, "resource-fixture.txt", "file", []byte("resource fixture"))
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/resources", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		resources := runner.DecodeData[struct {
			Resources []map[string]any `json:"resources"`
		}](t, envelope)
		if !containsID(resources.Resources, resourceID) {
			t.Fatal("resource list")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/resources/storage-usage", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		usage := runner.DecodeData[map[string]any](t, envelope)
		if _, ok := usage["usage"]; !ok {
			t.Fatal("storage usage")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		resource := runner.DecodeData[struct {
			Resource struct {
				ID string `json:"id"`
			} `json:"resource"`
		}](t, envelope)
		if resource.Resource.ID != resourceID {
			t.Fatal("resource metadata")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/resources/access", []map[string]any{{"resourceId": resourceID, "purpose": "display", "variant": "original"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		items := runner.DecodeData[struct {
			Items []map[string]any `json:"items"`
		}](t, envelope)
		if len(items.Items) != 1 {
			t.Fatal("resource access")
		}
		response, err = clientA.DoRaw(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID+"/file", nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusOK)
		if string(response.Body) != "resource fixture" {
			t.Fatal("resource file body")
		}
	}) {
		t.Fatal("resource fixture setup failed; dependent scenarios cannot run")
	}

	t.Run("canvas.resource.foreign-access", func(t *testing.T) {
		response, envelope, err := clientB.Do(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
		response, err = clientB.DoRaw(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID+"/file", nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusNotFound)
	})

	t.Run("canvas.resource.import.invalid-input", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPost, "/api/resources/import", map[string]any{
			"url":  "not-a-url",
			"kind": "file",
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusBadRequest, "")
	})
	fixture, err := harness.NewResourceFixture()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fixture.Close)
	t.Run("canvas.resource.import.success", func(t *testing.T) {
		runResourceImportSuccessMatrix(t, clientA, clientB, fixture, config.RunID)
	})
	t.Run("canvas.resource.import.replay", func(t *testing.T) {
		runResourceImportReplayMatrix(t, clientA, clientB, fixture, config.RunID)
	})
	t.Run("canvas.resource.import.failure-and-ssrf", func(t *testing.T) {
		runResourceImportFailureMatrix(t, clientA, fixture, config.RunID)
	})

	t.Run("canvas.resource.reference-validation", func(t *testing.T) {
		content := runner.ImageFixture(t)
		referencedResourceID := uploadResource(t, clientA, "referenced.png", "image", content)
		createImageAsset(t, clientA, referencedResourceID, len(content))
		canvasID := fmt.Sprintf("resource-reference-%s", config.RunID)
		project := runner.FixtureProject(canvasID, "Resource reference", 0, false)
		attachResource(project, referencedResourceID)
		createCanvas(t, clientA, canvasID, project)

		broken := runner.FixtureProject(canvasID, "Broken resource reference", 1, false)
		attachResource(broken, "missing-resource-"+config.RunID)
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": broken}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "")
		getCanvasAndAssert(t, clientA, canvasID, 1, "Resource reference")

		foreignResourceID := uploadResource(t, clientB, "foreign.png", "image", content)
		createImageAsset(t, clientB, foreignResourceID, len(content))
		foreignReference := runner.FixtureProject(canvasID, "Foreign reference", 1, false)
		attachResource(foreignReference, foreignResourceID)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": foreignReference}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "")
		getCanvasAndAssert(t, clientA, canvasID, 1, "Resource reference")

		replacementResourceID := uploadResource(t, clientA, "replacement.png", "image", content)
		replacement := legacyAssetPayload(referencedResourceID, "Forbidden replacement", len(content))
		replacement["coverUrl"] = "/api/resources/" + replacementResourceID + "/file"
		replacement["data"].(map[string]any)["storageKey"] = "resource:" + replacementResourceID
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/assets/"+referencedResourceID, map[string]any{"asset": replacement}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusBadRequest, "")
		getCanvasAndAssert(t, clientA, canvasID, 1, "Resource reference")
		response, err = clientA.DoRaw(runner.Context(t), http.MethodGet, "/api/resources/"+referencedResourceID+"/file", nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusOK)
		if string(response.Body) != string(content) {
			t.Fatal("referenced resource bytes changed after rejected replacement")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/assets/"+referencedResourceID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		getCanvasAndAssert(t, clientA, canvasID, 1, "Resource reference")
		project["revision"] = 1
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": project}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "canvas_history_resources_missing")
		getCanvasAndAssert(t, clientA, canvasID, 1, "Resource reference")
		unlinked := runner.FixtureProject(canvasID, "Resource repaired after purge", 1, false)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": unlinked, "repairMissingResources": true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		getCanvasAndAssert(t, clientA, canvasID, 2, "Resource repaired after purge")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID+"/history", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		history := runner.DecodeData[struct {
			Snapshots []struct {
				ID     string `json:"id"`
				Reason string `json:"reason"`
			} `json:"snapshots"`
		}](t, envelope)
		if len(history.Snapshots) != 1 || history.Snapshots[0].Reason != "before_resource_repair" {
			t.Fatal("purge repair did not retain the damaged preimage")
		}
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/canvas-projects/"+canvasID+"/history/"+history.Snapshots[0].ID+"/restore", map[string]int{"revision": 2}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusConflict, "canvas_history_resources_missing")
		getCanvasAndAssert(t, clientA, canvasID, 2, "Resource repaired after purge")
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets/"+referencedResourceID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
	})

	t.Run("canvas.resource.chunk-validation", func(t *testing.T) {
		runChunkUploadFailureMatrix(t, clientA, clientB)
	})
	t.Run("canvas.resource.multichunk-replay", func(t *testing.T) {
		runChunkUploadBoundaryMatrix(t, clientA)
	})
	t.Run("canvas.resource.chunked-upload", func(t *testing.T) {
		content := []byte("chunked resource fixture")
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "chunked.txt", "kind": "file", "size": len(content)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		session := runner.DecodeData[struct {
			UploadID   string `json:"uploadId"`
			ChunkCount int    `json:"chunkCount"`
		}](t, envelope)
		if session.UploadID == "" || session.ChunkCount != 1 {
			t.Fatal("upload session")
		}
		response, envelope, err = clientA.DoBytes(runner.Context(t), http.MethodPut, "/api/resources/uploads/"+session.UploadID+"/chunks/0", "application/octet-stream", content, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/resources/uploads/"+session.UploadID+"/complete", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		resource := runner.DecodeData[struct {
			Resource struct {
				ID string `json:"id"`
			} `json:"resource"`
		}](t, envelope)
		if resource.Resource.ID == "" {
			t.Fatal("chunked resource")
		}
	})
	t.Run("canvas.resource.limits.single-file", func(t *testing.T) {
		runResourceFileLimitMatrix(t, admin)
	})
	t.Run("canvas.resource.limits.daily-replay-and-concurrency", func(t *testing.T) {
		runResourceDailyLimitMatrix(t, admin)
	})
	t.Run("canvas.resource.limits.rate-and-scope", func(t *testing.T) {
		runResourceRateLimitMatrix(t, admin)
	})
	t.Run("canvas.resource.limits.stored-reservations", func(t *testing.T) {
		runResourceStoredReservationLimitMatrix(t, admin)
	})
	t.Run("canvas.resource.limits.session-slots", func(t *testing.T) {
		runResourceSessionSlotLimitMatrix(t, admin)
	})
	t.Run("canvas.resource.delivery.real-expiry-and-conditional-cache", func(t *testing.T) {
		runResourceRealExpiryAndConditionalCache(t, admin)
	})
	t.Run("canvas.resource.limits.concurrent-slot-admission", func(t *testing.T) {
		runResourceConcurrentSlotAdmission(t, admin)
	})
	t.Run("canvas.resource.upload.concurrent-chunks-and-completion", func(t *testing.T) {
		runResourceConcurrentChunksAndCompletion(t, admin)
	})
	t.Run("canvas.resource.import.slow-timeout", func(t *testing.T) {
		runResourceImportSlowTimeout(t, admin, fixture)
	})
	t.Run("canvas.resource.file.access-description", func(test *testing.T) {
		runPrivateResourceDescription(test, clientA, clientB)
	})
	t.Run("canvas.resource.upload.chunk-versus-completion", func(test *testing.T) {
		runResourceChunkVersusCompletion(test, admin)
	})
	t.Run("canvas.resource.upload.invalid-overwrite-recovery", func(test *testing.T) {
		runResourceInvalidOverwriteRecovery(test, admin)
	})
	t.Run("canvas.resource.upload.interrupted-chunk-recovery", func(test *testing.T) {
		runResourceInterruptedChunkRecovery(test, admin)
	})
	t.Run("canvas.resource.object-storage-http-origin", func(test *testing.T) {
		runObjectStorageHTTPOrigin(test, clientA, admin)
	})
}

func runPublicResourceContract(t *testing.T, config runner.Config) {
	clientA, _, admin := prepareClients(t, config)
	anonymous, err := runner.NewClient("anonymous", config)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("public resource fixture with range support")
	resourceID := uploadResource(t, clientA, "public-fixture.txt", "file", content)
	signedPath := publicResourceAccessPath(t, clientA, resourceID)

	t.Run("canvas.public-resource.access-description", func(test *testing.T) {
		runPublicResourceDescription(test, clientA, anonymous, resourceID, signedPath, content)
	})

	t.Run("canvas.public-resource.delivery", func(t *testing.T) {
		response, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, signedPath, nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusOK)
		if string(response.Body) != string(content) {
			t.Fatalf("public resource bytes differ: %s", response.String())
		}
		for key, want := range map[string]string{
			"Cache-Control":          "public, max-age=0, must-revalidate",
			"Referrer-Policy":        "no-referrer",
			"X-Content-Type-Options": "nosniff",
			"Accept-Ranges":          "bytes",
		} {
			runner.RequireHeader(t, response.Header, key, want)
		}

		replay, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, signedPath, nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, replay, http.StatusOK)
		if string(replay.Body) != string(content) {
			t.Fatal("signed public resource URL was not replayable before expiry")
		}
	})

	t.Run("canvas.public-resource.named-file", func(t *testing.T) {
		parsed, err := url.Parse(signedPath)
		if err != nil {
			t.Fatal(err)
		}
		parsed.Path += "/download.txt"
		response, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, parsed.RequestURI(), nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusOK)
		if string(response.Body) != string(content) {
			t.Fatalf("named public resource bytes differ: %s", response.String())
		}
	})

	t.Run("canvas.public-resource.range", func(t *testing.T) {
		headers := make(http.Header)
		headers.Set("Range", "bytes=0-6")
		response, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, signedPath, nil, "", headers)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireStatus(t, response, http.StatusPartialContent)
		if string(response.Body) != string(content[:7]) {
			t.Fatalf("ranged public resource bytes differ: %s", response.String())
		}
		runner.RequireHeader(t, response.Header, "Content-Range", fmt.Sprintf("bytes 0-6/%d", len(content)))
		runner.RequireHeader(t, response.Header, "Accept-Ranges", "bytes")
	})

	t.Run("canvas.public-resource.invalid-signature", func(t *testing.T) {
		parsed, err := url.Parse(signedPath)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("signature", strings.Repeat("A", len(query.Get("signature"))))
		parsed.RawQuery = query.Encode()
		response, envelope, err := anonymous.Do(runner.Context(t), http.MethodGet, parsed.RequestURI(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusForbidden, "")
	})

	t.Run("canvas.public-resource.missing-and-expired-signature", func(t *testing.T) {
		parsed, err := url.Parse(signedPath)
		if err != nil {
			t.Fatal(err)
		}
		missing := parsed.Query()
		missing.Del("signature")
		parsed.RawQuery = missing.Encode()
		response, envelope, err := anonymous.Do(runner.Context(t), http.MethodGet, parsed.RequestURI(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusForbidden, "")

		parsed, err = url.Parse(signedPath)
		if err != nil {
			t.Fatal(err)
		}
		expired := parsed.Query()
		expired.Set("expires", "0")
		parsed.RawQuery = expired.Encode()
		response, envelope, err = anonymous.Do(runner.Context(t), http.MethodGet, parsed.RequestURI(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusForbidden, "")
	})

	t.Run("canvas.public-resource.resource-binding", func(t *testing.T) {
		parsed, err := url.Parse(signedPath)
		if err != nil {
			t.Fatal(err)
		}
		parsed.Path = strings.Replace(parsed.Path, resourceID, "missing-public-resource-"+config.RunID, 1)
		response, envelope, err := anonymous.Do(runner.Context(t), http.MethodGet, parsed.RequestURI(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusForbidden, "")
	})

	t.Run("canvas.public-resource.remote-origin-http-failure", func(test *testing.T) {
		runPublicRemoteOriginHTTPFailure(test, clientA, admin, anonymous)
	})
}

func runObjectStorageHTTPOrigin(t *testing.T, client *runner.Client, admin *runner.Client) {
	t.Helper()
	fixture := harness.NewObjectStoreFixture()
	t.Cleanup(fixture.Close)
	configureAdminS3(t, admin, fixture.URL())

	content := []byte("object storage contract fixture")
	resourceID := uploadResource(t, client, "object-storage-success.txt", "file", content)
	if fixture.ObjectCount() == 0 {
		t.Fatal("S3 fixture did not receive the uploaded object")
	}

	response, envelope, err := client.Do(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	resource := runner.DecodeData[struct {
		Resource struct {
			Provider string `json:"provider"`
			Endpoint string `json:"endpoint"`
			Bucket   string `json:"bucket"`
		} `json:"resource"`
	}](t, envelope)
	if resource.Resource.Provider != "s3" || resource.Resource.Endpoint != fixture.URL() || resource.Resource.Bucket != "canvas-contract" {
		t.Fatalf("resource storage binding = %#v", resource.Resource)
	}

	response, err = client.DoRaw(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID+"/file", nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireStatus(t, response, http.StatusOK)
	if string(response.Body) != string(content) {
		t.Fatal("S3-backed resource bytes differ")
	}

	fixture.SetFailure(http.MethodGet, true)
	response, err = client.DoRaw(runner.Context(t), http.MethodGet, "/api/resources/"+resourceID+"/file", nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < http.StatusInternalServerError {
		t.Fatalf("S3 GET failure was not surfaced: status=%d", response.StatusCode)
	}
	fixture.SetFailure(http.MethodGet, false)

	key := "object-storage-failure-retry"
	fixture.SetFailure(http.MethodPut, true)
	headers := http.Header{"X-Idempotency-Key": []string{key}}
	response, _, err = client.DoMultipart(runner.Context(t), http.MethodPost, "/api/resources", map[string]string{"kind": "file"}, "file", "object-storage-retry.txt", content, headers)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < http.StatusInternalServerError {
		t.Fatalf("S3 PUT failure was not surfaced: status=%d", response.StatusCode)
	}
	fixture.SetFailure(http.MethodPut, false)
	response, envelope, err = client.DoMultipart(runner.Context(t), http.MethodPost, "/api/resources", map[string]string{"kind": "file"}, "file", "object-storage-retry.txt", content, headers)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	retried := runner.DecodeData[struct {
		Resource struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		} `json:"resource"`
	}](t, envelope)
	if retried.Resource.ID == "" || retried.Resource.Provider != "s3" {
		t.Fatalf("S3 retry resource = %#v", retried.Resource)
	}
}

func runPublicRemoteOriginHTTPFailure(t *testing.T, client *runner.Client, admin *runner.Client, anonymous *runner.Client) {
	t.Helper()
	fixture := harness.NewObjectStoreFixture()
	t.Cleanup(fixture.Close)
	configureAdminS3(t, admin, fixture.URL())

	content := []byte("remote public object fixture")
	resourceID := uploadResource(t, client, "remote-public.txt", "file", content)
	signedPath := publicResourceAccessPath(t, client, resourceID)
	response, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, signedPath, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireStatus(t, response, http.StatusOK)
	if string(response.Body) != string(content) {
		t.Fatalf("remote origin response body = %q", string(response.Body))
	}

	fixture.SetFailure(http.MethodGet, true)
	response, err = anonymous.DoRaw(runner.Context(t), http.MethodGet, signedPath, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < http.StatusInternalServerError {
		t.Fatalf("remote origin failure status=%d", response.StatusCode)
	}
	fixture.SetFailure(http.MethodGet, false)
	response, err = anonymous.DoRaw(runner.Context(t), http.MethodGet, signedPath, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireStatus(t, response, http.StatusOK)
	if string(response.Body) != string(content) {
		t.Fatalf("remote origin recovery body = %q", string(response.Body))
	}
}

func runShareResourceObjectStorageFailure(t *testing.T, owner, admin, anonymous *runner.Client, runID string) {
	t.Helper()
	fixture := harness.NewObjectStoreFixture()
	t.Cleanup(fixture.Close)
	configureAdminS3WithDelivery(t, admin, fixture.URL(), fixture.URL())

	content := runner.ImageFixture(t)
	resourceID := uploadResource(t, owner, "shared-object-storage.png", "image", content)
	if fixture.ObjectCount() == 0 {
		t.Fatal("S3 fixture did not receive the shared resource")
	}
	createImageAsset(t, owner, resourceID, len(content))

	canvasID := "share-resource-storage-" + runID
	project := runner.FixtureProject(canvasID, "Shared object storage", 0, false)
	attachResource(project, resourceID)
	createCanvas(t, owner, canvasID, project)
	share := createShare(t, owner, canvasID, 0, false)
	path := "/api/public/canvas-shares/" + share.Token + "/resources/" + resourceID + "/file"
	accessResponse, accessEnvelope, err := anonymous.Do(runner.Context(t), http.MethodGet, path+"?access=1&variant=original", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, accessResponse, accessEnvelope)
	access := runner.DecodeData[struct {
		Access describedResourceAccess `json:"access"`
	}](t, accessEnvelope).Access
	if access.ResourceID != resourceID || access.Delivery != "cdn" || access.URL == "" {
		t.Fatalf("shared object storage access = %#v", access)
	}

	response, err := anonymous.DoRaw(runner.Context(t), http.MethodGet, path, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireStatus(t, response, http.StatusTemporaryRedirect)
	runner.RequireHeader(t, response.Header, "Location", access.URL)
	status, body := fetchExternalObject(t, access.URL)
	if status != http.StatusOK || string(body) != string(content) {
		t.Fatalf("shared object storage CDN response = status %d body %q", status, string(body))
	}

	fixture.SetFailure(http.MethodGet, true)
	status, _ = fetchExternalObject(t, access.URL)
	if status < http.StatusInternalServerError {
		t.Fatalf("shared object storage failure was not surfaced: status=%d", status)
	}

	fixture.SetFailure(http.MethodGet, false)
	status, body = fetchExternalObject(t, access.URL)
	if status != http.StatusOK || string(body) != string(content) {
		t.Fatalf("shared object storage recovery = status %d body %q", status, string(body))
	}
}

func fetchExternalObject(t *testing.T, rawURL string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(runner.Context(t), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body
}

func configureAdminS3(t *testing.T, admin *runner.Client, endpoint string) {
	configureAdminS3WithDelivery(t, admin, endpoint, "")
}

func configureAdminS3WithDelivery(t *testing.T, admin *runner.Client, endpoint, cdnBaseURL string) {
	t.Helper()
	setting := map[string]any{
		"enabled":         true,
		"provider":        "s3",
		"region":          "us-east-1",
		"endpoint":        endpoint,
		"bucket":          "canvas-contract",
		"accessKeyId":     "contract-access",
		"accessKeySecret": "contract-secret",
		"pathPrefix":      "contract",
		"s3Preset":        "custom",
		"pathStyle":       true,
	}
	if cdnBaseURL != "" {
		setting["cdnBaseUrl"] = cdnBaseURL
		setting["cdnAuthMode"] = "public"
	}
	response, envelope, err := admin.Do(runner.Context(t), http.MethodPost, "/api/admin/settings/oss/test", setting, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	response, envelope, err = admin.Do(runner.Context(t), http.MethodPatch, "/api/admin/settings/oss", setting, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
}

func runImportContract(t *testing.T, config runner.Config) {
	clientA, clientB, admin := prepareClients(t, config)
	canvasID := fmt.Sprintf("import-%s", config.RunID)
	createCanvas(t, clientA, canvasID, runner.FixtureProject(canvasID, "Import contract", 0, false))

	t.Run("canvas.import.scope-and-feature-gate", func(t *testing.T) {
		runImportScopeMatrix(t, clientA, clientB, admin, canvasID)
	})
	t.Run("canvas.import.invalid-input", func(t *testing.T) {
		for _, testCase := range []struct {
			name string
			path string
			body map[string]string
		}{
			{name: "libtv", path: "/api/canvas-projects/" + canvasID + "/import/libtv", body: map[string]string{"uuid": ""}},
			{name: "tapnow", path: "/api/canvas-projects/" + canvasID + "/import/tapnow", body: map[string]string{"shareId": ""}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				response, envelope, err := clientA.Do(runner.Context(t), http.MethodPost, testCase.path, testCase.body, nil)
				if err != nil {
					t.Fatal(err)
				}
				runner.RequireError(t, response, envelope, http.StatusBadRequest, "")
			})
		}
	})
	t.Run("canvas.import.success-and-replay", func(t *testing.T) {
		runFixedImportSuccessMatrix(t, clientA, canvasID)
	})
	t.Run("canvas.import.media-topology", func(t *testing.T) {
		runRichImportConversionMatrix(t, clientA, canvasID)
	})
	t.Run("canvas.import.conversion-limits", func(t *testing.T) {
		runImportConversionLimitMatrix(t, clientA, canvasID)
	})
	t.Run("canvas.import.resource-save-and-isolation", func(t *testing.T) {
		runImportResourceSaveMatrix(t, clientA, clientB, config.RunID)
	})
	t.Run("canvas.import.upstream-failures", func(t *testing.T) {
		runFixedImportFailureMatrix(t, clientA, canvasID)
	})
	t.Run("canvas.import.upstream-timeout", func(t *testing.T) {
		runFixedImportTimeoutMatrix(t, clientA, canvasID)
	})
}

func runLegacyAssetsContract(t *testing.T, config runner.Config) {
	clientA, clientB, _ := prepareClients(t, config)
	content := runner.ImageFixture(t)
	resourceID := uploadResource(t, clientA, "legacy-asset-fixture.png", "image", content)
	secondResourceID := uploadResource(t, clientA, "legacy-asset-fixture-2.png", "image", content)
	assetID := resourceID
	secondAssetID := secondResourceID
	upsertAsset(t, clientA, assetID, legacyAssetPayload(assetID, "Legacy contract asset", len(content)))
	upsertAsset(t, clientA, secondAssetID, legacyAssetPayload(secondAssetID, "Legacy contract asset 2", len(content)))
	t.Run("canvas.legacy-asset.foreign-write-matrix", func(test *testing.T) {
		runLegacyForeignWriteMatrix(test, clientA, clientB)
	})
	t.Run("canvas.legacy-asset.mixed-owner-rollback", func(test *testing.T) {
		runLegacyMixedOwnerRollback(test, clientA, clientB)
	})

	t.Run("canvas.legacy-asset.unauthenticated", func(t *testing.T) {
		anonymous, err := runner.NewClient("anonymous", config)
		if err != nil {
			t.Fatal(err)
		}
		response, envelope, err := anonymous.Do(runner.Context(t), http.MethodGet, "/api/assets", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusUnauthorized, "")
	})

	t.Run("canvas.legacy-asset.folder-lifecycle", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/asset-folders", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/asset-folders", map[string]string{"name": "Contract folder"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		created := runner.DecodeData[struct {
			Folder struct {
				ID string `json:"id"`
			} `json:"folder"`
		}](t, envelope)
		folderID := created.Folder.ID
		if folderID == "" {
			t.Fatal("folder create response")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPatch, "/api/asset-folders/"+folderID, map[string]string{"name": "Contract folder renamed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPatch, "/api/assets/folder", map[string]any{"assetIds": []string{assetID}, "folderId": folderID}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets?page=1&pageSize=20&folderId="+folderID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		page := runner.DecodeData[struct {
			Assets []map[string]any `json:"assets"`
		}](t, envelope)
		if !containsID(page.Assets, assetID) {
			t.Fatal("folder-filtered assets")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodDelete, "/api/asset-folders/"+folderID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets/"+assetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
	})

	t.Run("canvas.legacy-asset.read-update-batch", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/assets", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		list := runner.DecodeData[struct {
			Assets []map[string]any `json:"assets"`
		}](t, envelope)
		if !containsID(list.Assets, assetID) || !containsID(list.Assets, secondAssetID) {
			t.Fatal("asset list")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets/"+assetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		read := runner.DecodeData[struct {
			Asset map[string]any `json:"asset"`
		}](t, envelope)
		if read.Asset["id"] != assetID {
			t.Fatal("asset read")
		}

		upsertAsset(t, clientA, assetID, legacyAssetPayload(assetID, "Legacy contract asset updated", len(content)))
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets/"+assetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		updated := runner.DecodeData[struct {
			Asset map[string]any `json:"asset"`
		}](t, envelope)
		if updated.Asset["title"] != "Legacy contract asset updated" {
			t.Fatal("asset update")
		}

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/assets/batch", map[string]any{"ids": []string{assetID, secondAssetID}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		batch := runner.DecodeData[struct {
			Assets []map[string]any `json:"assets"`
		}](t, envelope)
		if len(batch.Assets) != 2 || !containsID(batch.Assets, assetID) || !containsID(batch.Assets, secondAssetID) {
			t.Fatal("asset batch")
		}

		response, envelope, err = clientB.Do(runner.Context(t), http.MethodGet, "/api/assets/"+assetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
	})

	t.Run("canvas.legacy-user-data.snapshot", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodGet, "/api/user-data/snapshot", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		if len(envelope.Data) == 0 {
			t.Fatalf("empty user data snapshot")
		}
	})

	t.Run("canvas.legacy-asset.delete", func(t *testing.T) {
		response, envelope, err := clientA.Do(runner.Context(t), http.MethodDelete, "/api/assets/"+assetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets/"+assetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")

		response, envelope, err = clientA.Do(runner.Context(t), http.MethodPost, "/api/assets/batch-delete", []string{secondAssetID}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireSuccess(t, response, envelope)
		response, envelope, err = clientA.Do(runner.Context(t), http.MethodGet, "/api/assets/"+secondAssetID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner.RequireError(t, response, envelope, http.StatusNotFound, "")
	})
}

func prepareClients(t *testing.T, config runner.Config) (*runner.Client, *runner.Client, *runner.Client) {
	t.Helper()
	admin, err := runner.NewClient("admin", config)
	if err != nil {
		t.Fatal(err)
	}
	users := runner.PrepareUsers(t, config, admin)
	clientA, err := runner.NewClient("user-a", config)
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := runner.NewClient("user-b", config)
	if err != nil {
		t.Fatal(err)
	}
	login(t, clientA, users.A)
	login(t, clientB, users.B)
	if !config.RegisterUsers && (config.Profile == "import" || runner.RequiresSettingsFixture(config.Profile)) {
		login(t, admin, runner.Credentials{Username: config.AdminUsername, Password: config.AdminPassword})
	}
	return clientA, clientB, admin
}

func createCanvas(t *testing.T, client *runner.Client, canvasID string, project map[string]any) {
	t.Helper()
	response, envelope, err := client.Do(runner.Context(t), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": project}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
}

func uploadResource(t *testing.T, client *runner.Client, fileName, kind string, content []byte) string {
	t.Helper()
	response, envelope, err := client.DoMultipart(runner.Context(t), http.MethodPost, "/api/resources", map[string]string{"kind": kind}, "file", fileName, content, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	resource := runner.DecodeData[struct {
		Resource struct {
			ID string `json:"id"`
		} `json:"resource"`
	}](t, envelope)
	if resource.Resource.ID == "" {
		t.Fatal("upload response")
	}
	return resource.Resource.ID
}

func createImageAsset(t *testing.T, client *runner.Client, resourceID string, size int) {
	t.Helper()
	upsertAsset(t, client, resourceID, legacyAssetPayload(resourceID, "Contract share image", size))
}

func legacyAssetPayload(assetID, title string, size int) map[string]any {
	return map[string]any{
		"id":       assetID,
		"kind":     "image",
		"title":    title,
		"coverUrl": "/api/resources/" + assetID + "/file",
		"tags":     []string{},
		"data": map[string]any{
			"storageKey": "resource:" + assetID,
			"width":      1,
			"height":     1,
			"bytes":      size,
			"mimeType":   "image/png",
		},
	}
}

func upsertAsset(t *testing.T, client *runner.Client, assetID string, asset map[string]any) {
	t.Helper()
	response, envelope, err := client.Do(runner.Context(t), http.MethodPut, "/api/assets/"+assetID, map[string]any{"asset": asset}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
}

func attachResource(project map[string]any, resourceID string) {
	nodes, ok := project["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		return
	}
	node, ok := nodes[0].(map[string]any)
	if !ok {
		return
	}
	metadata, ok := node["metadata"].(map[string]any)
	if !ok {
		metadata = map[string]any{}
		node["metadata"] = metadata
	}
	metadata["storageKey"] = "resource:" + resourceID
	metadata["assetId"] = resourceID
}

func publicResourceURLFromProject(t *testing.T, project map[string]any) string {
	t.Helper()
	nodes, ok := project["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		t.Fatal("public project has no nodes")
	}
	node, ok := nodes[0].(map[string]any)
	if !ok {
		t.Fatal("public node has unexpected type")
	}
	metadata, ok := node["metadata"].(map[string]any)
	if !ok {
		t.Fatal("public node has no metadata")
	}
	value, ok := metadata["content"].(string)
	if !ok || !strings.HasPrefix(value, "/api/public/canvas-shares/") {
		t.Fatal("public resource URL")
	}
	return value
}

func publicResourceAccessPath(t *testing.T, client *runner.Client, resourceID string) string {
	t.Helper()
	response, envelope, err := client.Do(runner.Context(t), http.MethodPost, "/api/resources/access", []map[string]any{{
		"resourceId": resourceID,
		"purpose":    "display",
		"variant":    "original",
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	access := runner.DecodeData[struct {
		Items []struct {
			ResourceID string `json:"resourceId"`
			Access     *struct {
				URL string `json:"url"`
			} `json:"access"`
		} `json:"items"`
	}](t, envelope)
	if len(access.Items) != 1 || access.Items[0].ResourceID != resourceID || access.Items[0].Access == nil || access.Items[0].Access.URL == "" {
		t.Fatal("resource access fixture did not provide one matching signed access item")
	}
	parsed, err := url.Parse(access.Items[0].Access.URL)
	if err != nil {
		t.Fatal("malformed signed resource URL")
	}
	if parsed.Path != "/api/public/resources/"+resourceID+"/file" || parsed.Query().Get("expires") == "" || parsed.Query().Get("signature") == "" {
		t.Fatal("signed public resource URL has an unexpected path or missing signature fields")
	}
	return parsed.RequestURI()
}

func login(t *testing.T, client *runner.Client, credentials runner.Credentials) {
	t.Helper()
	response, envelope, err := client.Do(runner.Context(t), http.MethodPost, "/api/auth/login", map[string]string{
		"username": credentials.Username,
		"password": credentials.Password,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
}

func getCanvasAndAssert(t *testing.T, client *runner.Client, canvasID string, revision int, titlePrefix string) {
	t.Helper()
	response, envelope, err := client.Do(runner.Context(t), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner.RequireSuccess(t, response, envelope)
	data := runner.DecodeData[struct {
		Project map[string]any `json:"project"`
	}](t, envelope)
	project := data.Project
	if project["id"] != canvasID {
		t.Fatal("project id")
	}
	runner.RequireNumber(t, project["revision"], float64(revision), "project revision")
	title, _ := project["title"].(string)
	if titlePrefix != "" && len(titlePrefix) > 0 && !strings.HasPrefix(title, titlePrefix) {
		t.Fatalf("project title = %q, want prefix %q", title, titlePrefix)
	}
	if _, ok := project["customExtension"]; !ok {
		t.Fatal("unknown top-level extension was dropped")
	}
}

func containsID(projects []map[string]any, id string) bool {
	for _, project := range projects {
		if project["id"] == id {
			return true
		}
	}
	return false
}

func containsProjectSummaryID(projects []map[string]any, id string) bool {
	for _, summary := range projects {
		project, ok := summary["project"].(map[string]any)
		if ok && project["id"] == id {
			return true
		}
	}
	return false
}
