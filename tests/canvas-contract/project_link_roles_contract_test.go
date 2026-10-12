package canvas_contract_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"canvas-contract/runner"
)

func projectLinksByUnit(test *testing.T, value any, projectID, canvasID string) map[string]map[string]any {
	test.Helper()
	links := map[string]map[string]any{}
	if value == nil {
		return links
	}
	entries, ok := value.([]any)
	if !ok {
		test.Fatal("project association projection has no link array")
	}
	for _, entry := range entries {
		link, ok := entry.(map[string]any)
		if !ok {
			test.Fatal("project association projection has a malformed link")
		}
		unitID, ok := link["unitId"].(string)
		identity, validIdentity := link["id"].(string)
		if !ok || unitID == "" || !validIdentity || identity == "" || links[unitID] != nil || link["projectId"] != projectID || link["canvasId"] != canvasID {
			test.Fatal("project association projection has a missing, duplicate or cross-scope identity")
		}
		links[unitID] = link
	}
	return links
}

func associationStableFields(document map[string]any, fields ...string) map[string]any {
	stable := make(map[string]any, len(document))
	for field, value := range document {
		stable[field] = value
	}
	for _, field := range fields {
		delete(stable, field)
	}
	return stable
}

func requireProjectRoleState(test *testing.T, owner *runner.Client, projectID, canvasID string, before projectRelationState, units []map[string]any, roles map[string]string, canvasRevision, projectRevision float64, assigned bool) projectRelationState {
	test.Helper()
	state := readProjectRelationState(test, owner, projectID, canvasID)
	project, ok := state.Detail["project"].(map[string]any)
	priorProject, priorOK := before.Detail["project"].(map[string]any)
	if !ok || !priorOK || project["revision"] != projectRevision || !reflect.DeepEqual(associationStableFields(project, "revision", "updatedAt"), associationStableFields(priorProject, "revision", "updatedAt")) {
		test.Fatal("association transition changed unrelated project metadata or the expected revision")
	}
	wantProject, wantTotal := "", float64(0)
	if assigned {
		wantProject, wantTotal = projectID, 1
	}
	if state.Canvas["projectId"] != wantProject || state.Canvas["revision"] != canvasRevision || state.Page["total"] != wantTotal || state.Page["hasMore"] != false {
		test.Fatal("association transition has an inconsistent canvas owner, authoritative revision or page total")
	}
	if !reflect.DeepEqual(associationStableFields(state.Canvas, "projectId", "revision", "updatedAt"), associationStableFields(before.Canvas, "projectId", "revision", "updatedAt")) || state.History["currentRevision"] != canvasRevision || !reflect.DeepEqual(associationStableFields(state.History, "currentRevision"), associationStableFields(before.History, "currentRevision")) {
		test.Fatal("association transition changed canvas content or historical snapshots")
	}
	detailLinks := projectLinksByUnit(test, state.Detail["canvasUnitLinks"], projectID, canvasID)
	pageLinks := projectLinksByUnit(test, state.Page["canvasUnitLinks"], projectID, canvasID)
	if len(detailLinks) != len(roles) || !reflect.DeepEqual(detailLinks, pageLinks) {
		test.Fatal("full-detail and paged associations disagree or duplicate the linked canvas")
	}
	for unitID, role := range roles {
		if detailLinks[unitID] == nil || detailLinks[unitID]["role"] != role {
			test.Fatal("association role changed another unit or lost normalization")
		}
	}
	for _, value := range []any{state.Detail["canvases"], state.Page["canvases"]} {
		var canvases []any
		if value != nil {
			canvases, ok = value.([]any)
			if !ok {
				test.Fatal("association canvas projection has no array")
			}
		}
		if len(canvases) != int(wantTotal) {
			test.Fatal("multiple unit links duplicated or lost the canvas summary")
		}
		if assigned {
			canvas, ok := canvases[0].(map[string]any)
			if !ok || canvas["id"] != canvasID {
				test.Fatal("association canvas projection changed the canvas identity")
			}
		}
	}
	workbench := readWorkbenchState(test, owner, projectID)
	if !reflect.DeepEqual(workbench.Project, project) || !reflect.DeepEqual(workbench.Units, units) {
		test.Fatal("association transition changed authoritative unit bodies or project projections disagree")
	}
	return state
}

func createAssociationCanvas(test *testing.T, owner *runner.Client, canvasID string) {
	test.Helper()
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Association initial document", 0, false))
	document := readCanvasDocument(test, owner, canvasID)
	document["title"] = "Association document with a historical preimage"
	requireHTTPSuccess(test, owner, http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": document})
	history := readCanvasHistory(test, owner, canvasID)
	snapshots, ok := history["snapshots"].([]any)
	if !ok || len(snapshots) == 0 {
		test.Fatal("association fixture has no historical preimage to protect")
	}
}

func linkCanvasRole(test *testing.T, owner *runner.Client, projectID, canvasID, unitID string, role any, includeRole bool) map[string]any {
	test.Helper()
	payload := map[string]any{"canvasId": " " + canvasID + " ", "unitId": " " + unitID + " "}
	if includeRole {
		payload["role"] = role
	}
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", payload)
	return runner.DecodeData[struct {
		Link map[string]any `json:"link"`
	}](test, envelope).Link
}

func runProjectMultiUnitRoles(test *testing.T, owner, foreign *runner.Client, prefix string) {
	projectID, firstUnit := createMatrixProject(test, owner, "Multi-role association "+prefix)
	secondUnit := createWorkbenchUnit(test, owner, projectID, "Reference association unit")
	thirdUnit := createWorkbenchUnit(test, owner, projectID, "Unicode role association unit")
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign multi-role association "+prefix)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	canvasID := "multi-role-" + prefix
	createAssociationCanvas(test, owner, canvasID)
	before := readProjectRelationState(test, owner, projectID, canvasID)
	units := readWorkbenchState(test, owner, projectID).Units
	canvasRevision := before.Canvas["revision"].(float64)
	projectRevision := before.Detail["project"].(map[string]any)["revision"].(float64)
	roles := map[string]string{}
	identities := map[string]map[string]any{}
	for _, request := range []struct {
		unitID      string
		role        any
		includeRole bool
		wantRole    string
	}{
		{firstUnit, nil, false, "storyboard"},
		{secondUnit, " reference ", true, "reference"},
		{thirdUnit, strings.Repeat("角", 32), true, strings.Repeat("角", 32)},
	} {
		acknowledged := linkCanvasRole(test, owner, projectID, canvasID, request.unitID, request.role, request.includeRole)
		roles[request.unitID] = request.wantRole
		canvasRevision++
		projectRevision++
		state := requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
		link := projectLinksByUnit(test, state.Detail["canvasUnitLinks"], projectID, canvasID)[request.unitID]
		if acknowledged["id"] != link["id"] || acknowledged["role"] != request.wantRole || acknowledged["canvasId"] != canvasID || acknowledged["unitId"] != request.unitID || acknowledged["projectId"] != projectID {
			test.Fatal("first association acknowledgement does not identify the persisted normalized link")
		}
		identities[request.unitID] = associationStableFields(link, "role")
	}
	for _, role := range []any{" revised-role ", " revised-role ", " \t ", nil} {
		acknowledged := linkCanvasRole(test, owner, projectID, canvasID, firstUnit, role, true)
		wantRole := "storyboard"
		if value, ok := role.(string); ok && strings.TrimSpace(value) != "" {
			wantRole = strings.TrimSpace(value)
		}
		roles[firstUnit] = wantRole
		canvasRevision++
		projectRevision++
		state := requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
		links := projectLinksByUnit(test, state.Detail["canvasUnitLinks"], projectID, canvasID)
		for unitID, identity := range identities {
			if !reflect.DeepEqual(associationStableFields(links[unitID], "role"), identity) {
				test.Fatal("role update or replay replaced persisted link identity or creation metadata")
			}
		}
		if acknowledged["id"] == links[firstUnit]["id"] || acknowledged["role"] != wantRole {
			test.Fatal("observed Go baseline changed: role updates return a fresh identity while persisting the original link")
		}
	}
	settled := readProjectRelationState(test, owner, projectID, canvasID)
	for _, role := range []any{true, 42, []string{"storyboard"}} {
		requireHTTPError(test, owner, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": firstUnit, "role": role}, http.StatusBadRequest)
		if !reflect.DeepEqual(settled, readProjectRelationState(test, owner, projectID, canvasID)) {
			test.Fatal("invalid role type changed an acknowledged association")
		}
	}
	requireHTTPError(test, foreign, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": secondUnit, "role": "foreign-role"}, http.StatusNotFound)
	requireHTTPError(test, foreign, http.MethodDelete, "/api/projects/"+projectID+"/canvas-links/"+canvasID+"/units/"+secondUnit, nil, http.StatusNotFound)
	if !reflect.DeepEqual(settled, readProjectRelationState(test, owner, projectID, canvasID)) {
		test.Fatal("foreign role update or unlink changed owner associations")
	}
	unitPath := "/api/projects/" + projectID + "/canvas-links/" + canvasID + "/units/" + firstUnit
	requireHTTPSuccess(test, owner, http.MethodDelete, unitPath, nil)
	delete(roles, firstUnit)
	projectRevision++
	settled = requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
	requireHTTPError(test, owner, http.MethodDelete, unitPath, nil, http.StatusNotFound)
	if !reflect.DeepEqual(settled, readProjectRelationState(test, owner, projectID, canvasID)) {
		test.Fatal("replayed single-unit unlink changed other roles or canvas state")
	}
	requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projectID+"/canvases/"+canvasID, nil)
	canvasRevision++
	projectRevision++
	settled = requireProjectRoleState(test, owner, projectID, canvasID, before, units, map[string]string{}, canvasRevision, projectRevision, false)
	requireHTTPError(test, owner, http.MethodDelete, "/api/projects/"+projectID+"/canvases/"+canvasID, nil, http.StatusBadRequest)
	if !reflect.DeepEqual(settled, readProjectRelationState(test, owner, projectID, canvasID)) {
		test.Fatal("replayed project detach changed settled canvas or history")
	}
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
}

func runProjectRoleOverflow(test *testing.T, owner, foreign *runner.Client, prefix string) {
	projectID, unitID := createMatrixProject(test, owner, "Role overflow association "+prefix)
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign overflow association "+prefix)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	canvasID := "role-overflow-" + prefix
	createAssociationCanvas(test, owner, canvasID)
	before := readProjectRelationState(test, owner, projectID, canvasID)
	units := readWorkbenchState(test, owner, projectID).Units
	canvasRevision := before.Canvas["revision"].(float64)
	projectRevision := before.Detail["project"].(map[string]any)["revision"].(float64)
	payload := map[string]any{"canvasId": canvasID, "unitId": unitID, "role": strings.Repeat("角", 33)}
	anonymous, err := runner.NewClient("anonymous", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	defer anonymous.HTTP.CloseIdleConnections()
	requireHTTPError(test, anonymous, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", payload, http.StatusUnauthorized)
	requireHTTPError(test, foreign, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", payload, http.StatusNotFound)
	if !reflect.DeepEqual(before, readProjectRelationState(test, owner, projectID, canvasID)) {
		test.Fatal("anonymous or foreign role overflow bypassed ownership before mutation")
	}
	for attempt := 0; attempt < 2; attempt++ {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/projects/"+projectID+"/canvas-links", payload, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
		canvasRevision++
		settled := requireProjectRoleState(test, owner, projectID, canvasID, before, units, map[string]string{}, canvasRevision, projectRevision, true)
		requireHTTPError(test, owner, http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": before.Canvas}, http.StatusConflict)
		if !reflect.DeepEqual(settled, readProjectRelationState(test, owner, projectID, canvasID)) {
			test.Fatal("stale document retry after an association error changed the partially committed state")
		}
	}
	role := strings.Repeat("角", 32)
	acknowledged := linkCanvasRole(test, owner, projectID, canvasID, unitID, role, true)
	canvasRevision++
	projectRevision++
	roles := map[string]string{unitID: role}
	settled := requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
	link := projectLinksByUnit(test, settled.Detail["canvasUnitLinks"], projectID, canvasID)[unitID]
	if acknowledged["id"] != link["id"] {
		test.Fatal("valid recovery did not create the first authoritative link")
	}
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/projects/"+projectID+"/canvas-links", payload, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
	canvasRevision++
	settled = requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
	if !reflect.DeepEqual(link, projectLinksByUnit(test, settled.Detail["canvasUnitLinks"], projectID, canvasID)[unitID]) {
		test.Fatal("role overflow changed the acknowledged link instead of only exposing the observed partial canvas assignment")
	}
	requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projectID+"/canvases/"+canvasID, nil)
	canvasRevision++
	projectRevision++
	requireProjectRoleState(test, owner, projectID, canvasID, before, units, map[string]string{}, canvasRevision, projectRevision, false)
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
}
