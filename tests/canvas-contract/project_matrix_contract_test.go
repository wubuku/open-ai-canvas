package canvas_contract_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"canvas-contract/runner"
)

type projectGateRequest struct {
	Method  string
	Path    string
	Payload any
}

func projectGateRequests(profile, projectID, unitID, canvasID string) []projectGateRequest {
	projectPath := "/api/projects/" + projectID
	if profile == "project-integration" {
		return []projectGateRequest{
			{http.MethodGet, projectPath + "/canvases", nil},
			{http.MethodPost, projectPath + "/canvas-links", map[string]any{"canvasId": canvasID, "unitId": unitID}},
			{http.MethodDelete, projectPath + "/canvas-links/" + canvasID + "/units/" + unitID, nil},
			{http.MethodDelete, projectPath + "/canvases/" + canvasID, nil},
		}
	}
	return []projectGateRequest{
		{http.MethodGet, "/api/projects", nil},
		{http.MethodPost, "/api/projects", map[string]any{"name": "Must not be created"}},
		{http.MethodGet, projectPath, nil},
		{http.MethodGet, projectPath + "/core", nil},
		{http.MethodGet, projectPath + "/overview", nil},
		{http.MethodPatch, projectPath, map[string]any{"name": "Must not be renamed"}},
		{http.MethodDelete, projectPath, nil},
		{http.MethodPost, projectPath + "/units", map[string]any{"title": "Must not be created"}},
		{http.MethodGet, projectPath + "/units", nil},
		{http.MethodGet, projectPath + "/units/" + unitID, nil},
		{http.MethodGet, projectPath + "/units/" + unitID + "/workspace", nil},
		{http.MethodPost, projectPath + "/units/import", map[string]any{"units": []any{}}},
		{http.MethodPatch, projectPath + "/units/reorder", map[string]any{"unitIds": []string{unitID}}},
		{http.MethodPatch, projectPath + "/units/" + unitID, map[string]any{"title": "Must not be renamed"}},
		{http.MethodDelete, projectPath + "/units/" + unitID, nil},
	}
}

func readProjectFeatures(test *testing.T, admin *runner.Client) map[string]any {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, admin, http.MethodGet, "/api/admin/settings/features", nil)
	return runner.DecodeData[struct {
		Features map[string]any `json:"features"`
	}](test, envelope).Features
}

func runProjectFeatureGate(test *testing.T, owner, foreign, admin *runner.Client, projectID, unitID, canvasID string) {
	if !owner.Config.SettingsFixture {
		test.Fatal("project feature tests must only modify settings on an explicitly isolated target")
	}
	if canvasID == "" {
		canvasID = "feature-gate-" + owner.Config.RunID
		createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Feature independent document", 0, false))
	}
	before := readWorkbenchState(test, owner, projectID)
	canvasBefore := readCanvasDocument(test, owner, canvasID)
	historyBefore := readCanvasHistory(test, owner, canvasID)
	featuresBefore := readProjectFeatures(test, admin)
	if featuresBefore["shortDramaEnabled"] != true {
		test.Fatal("project profile requires an initially enabled isolated short-drama feature")
	}
	anonymous, err := runner.NewClient("anonymous", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	for _, client := range []*runner.Client{owner, foreign, anonymous} {
		status := http.StatusForbidden
		if client == anonymous {
			status = http.StatusUnauthorized
		}
		requireHTTPError(test, client, http.MethodPatch, "/api/admin/settings/features", map[string]any{"shortDramaEnabled": false}, status)
	}
	if !reflect.DeepEqual(featuresBefore, readProjectFeatures(test, admin)) {
		test.Fatal("unprivileged feature changes mutated settings")
	}
	restored := false
	test.Cleanup(func() {
		if restored {
			return
		}
		cleanupContext, cancel := context.WithTimeout(context.Background(), admin.Config.Timeout)
		defer cancel()
		response, envelope, cleanupErr := admin.Do(cleanupContext, http.MethodPatch, "/api/admin/settings/features", map[string]any{"shortDramaEnabled": true}, nil)
		if cleanupErr != nil {
			test.Errorf("restore isolated project feature: %v", cleanupErr)
		} else if response.StatusCode != http.StatusOK || envelope.Code != 0 {
			test.Error("restore isolated project feature failed")
		}
	})
	requireHTTPSuccess(test, admin, http.MethodPatch, "/api/admin/settings/features", map[string]any{"shortDramaEnabled": false})
	for _, request := range projectGateRequests(owner.Config.Profile, projectID, unitID, canvasID) {
		for _, client := range []*runner.Client{owner, foreign, anonymous} {
			response, envelope, requestErr := client.Do(runner.Context(test), request.Method, request.Path, request.Payload, nil)
			if requestErr != nil {
				test.Fatal(requestErr)
			}
			if client == anonymous {
				runner.RequireError(test, response, envelope, http.StatusUnauthorized, "")
			} else {
				runner.RequireError(test, response, envelope, http.StatusForbidden, "forbidden")
			}
		}
	}
	if !reflect.DeepEqual(canvasBefore, readCanvasDocument(test, owner, canvasID)) || !reflect.DeepEqual(historyBefore, readCanvasHistory(test, owner, canvasID)) {
		test.Fatal("disabled project routes mutated the independent canvas or history")
	}
	requireHTTPSuccess(test, admin, http.MethodPatch, "/api/admin/settings/features", map[string]any{"shortDramaEnabled": true})
	restored = true
	if !reflect.DeepEqual(before, readWorkbenchState(test, owner, projectID)) {
		test.Fatal("disabled project routes mutated project/unit state")
	}
	featuresAfter := readProjectFeatures(test, admin)
	for field, value := range featuresBefore {
		if field != "configured" && field != "updatedAt" && field != "updatedBy" && !reflect.DeepEqual(value, featuresAfter[field]) {
			test.Fatal("partial feature toggle changed another feature")
		}
	}
}

func createMatrixProject(test *testing.T, owner *runner.Client, name string) (string, string) {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects", map[string]any{"name": name, "type": "short-drama", "aspectRatio": "16:9", "sourceType": "original"})
	project := runner.DecodeData[struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}](test, envelope)
	if project.Project.ID == "" {
		test.Fatal("cross-project fixture has no project identity")
	}
	_, envelope = requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+project.Project.ID+"/units", map[string]any{"kind": "chapter", "title": "Cross-project unit"})
	unit := runner.DecodeData[struct {
		Unit struct {
			ID string `json:"id"`
		} `json:"unit"`
	}](test, envelope)
	if unit.Unit.ID == "" {
		test.Fatal("cross-project fixture has no unit identity")
	}
	return project.Project.ID, unit.Unit.ID
}

type projectRelationState struct {
	Detail  map[string]any
	Page    map[string]any
	Canvas  map[string]any
	History map[string]any
}

func readProjectRelationState(test *testing.T, owner *runner.Client, projectID, canvasID string) projectRelationState {
	test.Helper()
	_, detail := requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects/"+projectID, nil)
	_, page := requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects/"+projectID+"/canvases?page=1&pageSize=50", nil)
	return projectRelationState{
		Detail: runner.DecodeData[map[string]any](test, detail), Page: runner.DecodeData[map[string]any](test, page),
		Canvas: readCanvasDocument(test, owner, canvasID), History: readCanvasHistory(test, owner, canvasID),
	}
}

func runProjectReassociationMatrix(test *testing.T, owner, foreign *runner.Client, prefix string) {
	firstProject, firstUnit := createMatrixProject(test, owner, "First relation "+prefix)
	secondProject, secondUnit := createMatrixProject(test, owner, "Second relation "+prefix)
	foreignProject, foreignUnit := createMatrixProject(test, foreign, "Foreign relation "+prefix)
	canvasID, foreignCanvasID := "relation-"+prefix, "foreign-relation-"+prefix
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Relation canvas", 0, false))
	createCanvas(test, foreign, foreignCanvasID, runner.FixtureProject(foreignCanvasID, "Foreign relation canvas", 0, false))
	firstBefore := readProjectRelationState(test, owner, firstProject, canvasID)
	secondBefore := readProjectRelationState(test, owner, secondProject, canvasID)
	foreignBefore := readProjectRelationState(test, foreign, foreignProject, foreignCanvasID)
	for _, request := range []struct {
		projectID string
		canvasID  string
		unitID    string
	}{
		{firstProject, canvasID, secondUnit},
		{firstProject, canvasID, foreignUnit},
		{firstProject, foreignCanvasID, firstUnit},
		{firstProject, "missing-" + canvasID, firstUnit},
		{firstProject, canvasID, "missing-" + firstUnit},
		{foreignProject, canvasID, foreignUnit},
		{"missing-" + firstProject, canvasID, firstUnit},
	} {
		requireHTTPError(test, owner, http.MethodPost, "/api/projects/"+request.projectID+"/canvas-links", map[string]any{"canvasId": request.canvasID, "unitId": request.unitID}, http.StatusNotFound)
		if !reflect.DeepEqual(firstBefore, readProjectRelationState(test, owner, firstProject, canvasID)) || !reflect.DeepEqual(secondBefore, readProjectRelationState(test, owner, secondProject, canvasID)) || !reflect.DeepEqual(foreignBefore, readProjectRelationState(test, foreign, foreignProject, foreignCanvasID)) {
			test.Fatal("rejected cross-project link mutated project, canvas, unit, history or foreign state")
		}
	}
	requireHTTPError(test, owner, http.MethodDelete, "/api/projects/"+firstProject+"/canvases/"+canvasID, nil, http.StatusBadRequest)
	requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+firstProject+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": firstUnit})
	firstLinked := readProjectRelationState(test, owner, firstProject, canvasID)
	if firstLinked.Canvas["projectId"] != firstProject || firstLinked.Canvas["revision"] != float64(2) {
		test.Fatal("first link did not update authoritative canvas project/revision")
	}
	firstLinks, ok := firstLinked.Detail["canvasUnitLinks"].([]any)
	if !ok || len(firstLinks) != 1 {
		test.Fatal("first association must create one full-detail unit link")
	}
	requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+secondProject+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": secondUnit})
	firstMoved := readProjectRelationState(test, owner, firstProject, canvasID)
	secondMoved := readProjectRelationState(test, owner, secondProject, canvasID)
	if firstMoved.Canvas["projectId"] != secondProject || firstMoved.Canvas["revision"] != float64(3) || firstMoved.History["currentRevision"] != float64(3) || !reflect.DeepEqual(firstBefore.History["snapshots"], firstMoved.History["snapshots"]) {
		test.Fatal("Go reassociation must update canvas metadata without making a document history snapshot")
	}
	if !reflect.DeepEqual(firstLinked.Detail["canvasUnitLinks"], firstMoved.Detail["canvasUnitLinks"]) {
		test.Fatal("observed Go baseline changed: reassociation currently retains the old full-detail link")
	}
	if firstMoved.Page["total"] != float64(0) || secondMoved.Page["total"] != float64(1) {
		test.Fatal("canvas paging must use the new project identity despite the stale old full-detail link")
	}
	links, ok := secondMoved.Page["canvasUnitLinks"].([]any)
	if !ok || len(links) != 1 || links[0].(map[string]any)["role"] != "storyboard" {
		test.Fatal("new association must expose one default-storyboard link")
	}
	for _, path := range []string{
		"/api/projects/" + firstProject + "/canvas-links/" + canvasID + "/units/" + firstUnit,
		"/api/projects/" + firstProject + "/canvases/" + canvasID,
	} {
		requireHTTPError(test, owner, http.MethodDelete, path, nil, http.StatusBadRequest)
		if !reflect.DeepEqual(firstMoved, readProjectRelationState(test, owner, firstProject, canvasID)) || !reflect.DeepEqual(secondMoved, readProjectRelationState(test, owner, secondProject, canvasID)) {
			test.Fatal("old-project unlink rejection mutated either relation")
		}
	}
	unitPath := "/api/projects/" + secondProject + "/canvas-links/" + canvasID + "/units/" + secondUnit
	requireHTTPSuccess(test, owner, http.MethodDelete, unitPath, nil)
	afterUnitUnlink := readProjectRelationState(test, owner, secondProject, canvasID)
	requireHTTPError(test, owner, http.MethodDelete, unitPath, nil, http.StatusNotFound)
	if !reflect.DeepEqual(afterUnitUnlink, readProjectRelationState(test, owner, secondProject, canvasID)) {
		test.Fatal("replayed unit unlink mutated settled state")
	}
	canvasPath := "/api/projects/" + secondProject + "/canvases/" + canvasID
	requireHTTPSuccess(test, owner, http.MethodDelete, canvasPath, nil)
	afterDetach := readProjectRelationState(test, owner, secondProject, canvasID)
	if afterDetach.Canvas["projectId"] != "" || afterDetach.Canvas["revision"] != float64(4) || afterDetach.Page["total"] != float64(0) {
		test.Fatal("detach must clear canvas project identity once while retaining the document")
	}
	requireHTTPError(test, owner, http.MethodDelete, canvasPath, nil, http.StatusBadRequest)
	if !reflect.DeepEqual(afterDetach, readProjectRelationState(test, owner, secondProject, canvasID)) || !reflect.DeepEqual(foreignBefore, readProjectRelationState(test, foreign, foreignProject, foreignCanvasID)) {
		test.Fatal("detach replay changed settled or foreign state")
	}
}
