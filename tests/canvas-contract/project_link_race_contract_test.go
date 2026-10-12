package canvas_contract_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"canvas-contract/runner"
)

type associationLinkRequest struct {
	ProjectID string
	UnitID    string
	Role      string
}

type associationLinkOutcome struct {
	Request  associationLinkRequest
	Response runner.Response
	Envelope runner.Envelope
	Err      error
}

func concurrentCanvasLinks(test *testing.T, owner *runner.Client, canvasID string, requests []associationLinkRequest) []associationLinkOutcome {
	test.Helper()
	start := make(chan struct{})
	results := make(chan associationLinkOutcome, len(requests))
	for _, request := range requests {
		go func() {
			<-start
			response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/projects/"+request.ProjectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": request.UnitID, "role": request.Role}, nil)
			results <- associationLinkOutcome{request, response, envelope, err}
		}()
	}
	close(start)
	outcomes := make([]associationLinkOutcome, len(requests))
	for index := range outcomes {
		outcomes[index] = <-results
	}
	return outcomes
}

func successfulAssociationAck(test *testing.T, outcome associationLinkOutcome, canvasID string) map[string]any {
	test.Helper()
	if outcome.Err != nil {
		test.Fatal(outcome.Err)
	}
	runner.RequireSuccess(test, outcome.Response, outcome.Envelope)
	link := runner.DecodeData[struct {
		Link map[string]any `json:"link"`
	}](test, outcome.Envelope).Link
	identity, validIdentity := link["id"].(string)
	createdAt, validCreatedAt := link["createdAt"].(string)
	if !validIdentity || identity == "" || !validCreatedAt || createdAt == "" || link["canvasId"] != canvasID || link["projectId"] != outcome.Request.ProjectID || link["unitId"] != outcome.Request.UnitID || link["role"] != outcome.Request.Role {
		test.Fatal("concurrent association acknowledgement lost its identity, normalized role or scope")
	}
	return link
}

func requireContendedProjectState(test *testing.T, owner *runner.Client, projectID, canvasID string, before projectRelationState, units []map[string]any, links map[string]map[string]any, authoritativeProjectID string, canvasRevision, projectRevision float64) projectRelationState {
	test.Helper()
	state := readProjectRelationState(test, owner, projectID, canvasID)
	project, validProject := state.Detail["project"].(map[string]any)
	priorProject, validPrior := before.Detail["project"].(map[string]any)
	if !validProject || !validPrior || project["revision"] != projectRevision || !reflect.DeepEqual(associationStableFields(project, "revision", "updatedAt"), associationStableFields(priorProject, "revision", "updatedAt")) {
		test.Fatal("cross-project contention changed unrelated metadata or the expected project revision")
	}
	if state.Canvas["projectId"] != authoritativeProjectID || state.Canvas["revision"] != canvasRevision || !reflect.DeepEqual(associationStableFields(state.Canvas, "projectId", "revision", "updatedAt"), associationStableFields(before.Canvas, "projectId", "revision", "updatedAt")) || state.History["currentRevision"] != canvasRevision || !reflect.DeepEqual(associationStableFields(state.History, "currentRevision"), associationStableFields(before.History, "currentRevision")) {
		test.Fatal("cross-project contention lost authoritative assignment, content or history")
	}
	detailLinks := projectLinksByUnit(test, state.Detail["canvasUnitLinks"], projectID, canvasID)
	pageLinks := projectLinksByUnit(test, state.Page["canvasUnitLinks"], projectID, canvasID)
	wantPageLinks, wantTotal := map[string]map[string]any{}, float64(0)
	if authoritativeProjectID == projectID {
		wantPageLinks, wantTotal = links, 1
	}
	if !reflect.DeepEqual(detailLinks, links) || !reflect.DeepEqual(pageLinks, wantPageLinks) || state.Page["total"] != wantTotal || state.Page["hasMore"] != false {
		test.Fatal("cross-project full-detail links and authoritative page projection disagree with the observed settlement")
	}
	for _, projection := range []any{state.Detail["canvases"], state.Page["canvases"]} {
		var canvases []any
		if projection != nil {
			var validArray bool
			canvases, validArray = projection.([]any)
			if !validArray {
				test.Fatal("cross-project canvas summary is not an array")
			}
		}
		if len(canvases) != int(wantTotal) {
			test.Fatal("cross-project projection duplicated or lost the authoritative canvas")
		}
		if wantTotal == 1 {
			canvas, validCanvas := canvases[0].(map[string]any)
			if !validCanvas || canvas["id"] != canvasID {
				test.Fatal("cross-project projection has a foreign canvas identity")
			}
		}
	}
	if !reflect.DeepEqual(associationStableFields(state.Detail, "project", "canvases", "canvasUnitLinks"), associationStableFields(before.Detail, "project", "canvases", "canvasUnitLinks")) || !reflect.DeepEqual(associationStableFields(state.Page, "canvases", "canvasUnitLinks", "total", "hasMore"), associationStableFields(before.Page, "canvases", "canvasUnitLinks", "total", "hasMore")) {
		test.Fatal("cross-project contention changed unrelated full-detail or page fields")
	}
	workbench := readWorkbenchState(test, owner, projectID)
	if !reflect.DeepEqual(workbench.Project, project) || !reflect.DeepEqual(workbench.Units, units) {
		test.Fatal("cross-project contention changed complete unit bodies or disagreed with authoritative project facts")
	}
	return state
}

func runProjectConcurrentLinks(test *testing.T, owner, foreign *runner.Client, prefix string) {
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign concurrent links "+prefix)
	foreignCanvas := "foreign-link-race-" + prefix
	createAssociationCanvas(test, foreign, foreignCanvas)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	foreignRelation := readProjectRelationState(test, foreign, foreignProject, foreignCanvas)
	for shapeIndex, shape := range []string{"first-same-key", "existing-role-update", "different-units"} {
		committed, rejected := 0, 0
		for attempt := 0; attempt < 3; attempt++ {
			projectID, firstUnit := createMatrixProject(test, owner, fmt.Sprintf("Concurrent links %s %d %s", shape, attempt, prefix))
			secondUnit := firstUnit
			if shape == "different-units" {
				secondUnit = createWorkbenchUnit(test, owner, projectID, "Second concurrent association unit")
			}
			canvasID := fmt.Sprintf("link-race-%s-%d-%d", prefix, shapeIndex, attempt)
			createAssociationCanvas(test, owner, canvasID)
			if shape == "existing-role-update" {
				linkCanvasRole(test, owner, projectID, canvasID, firstUnit, "before-race", true)
			}
			before := readProjectRelationState(test, owner, projectID, canvasID)
			units := readWorkbenchState(test, owner, projectID).Units
			priorLinks := projectLinksByUnit(test, before.Detail["canvasUnitLinks"], projectID, canvasID)
			requests := []associationLinkRequest{{projectID, firstUnit, "primary-a"}, {projectID, secondUnit, "primary-b"}}
			acknowledgements := map[string][]map[string]any{}
			successes := 0
			for _, outcome := range concurrentCanvasLinks(test, owner, canvasID, requests) {
				if outcome.Err != nil {
					test.Fatal(outcome.Err)
				}
				if outcome.Response.StatusCode == http.StatusOK {
					acknowledgements[outcome.Request.UnitID] = append(acknowledgements[outcome.Request.UnitID], successfulAssociationAck(test, outcome, canvasID))
					successes++
					committed++
				} else {
					if shape != "first-same-key" {
						test.Fatal("existing or different-unit association unexpectedly failed")
					}
					runner.RequireError(test, outcome.Response, outcome.Envelope, http.StatusInternalServerError, "internal")
					rejected++
				}
			}
			if successes == 0 || shape != "first-same-key" && successes != 2 {
				test.Fatal("concurrent linking has no valid complete committed request")
			}
			settled := readProjectRelationState(test, owner, projectID, canvasID)
			links := projectLinksByUnit(test, settled.Detail["canvasUnitLinks"], projectID, canvasID)
			roles := map[string]string{}
			if len(links) != len(acknowledgements) {
				test.Fatal("concurrent linking duplicated a unit or persisted an unacknowledged identity")
			}
			for unitID, candidates := range acknowledgements {
				link := links[unitID]
				if link == nil {
					test.Fatal("concurrent linking lost an acknowledged unit")
				}
				matchedRole, matchedIdentity := false, false
				for _, acknowledged := range candidates {
					matchedRole = matchedRole || link["role"] == acknowledged["role"]
					matchedIdentity = matchedIdentity || link["id"] == acknowledged["id"]
				}
				if !matchedRole {
					test.Fatal("concurrent linking persisted a mixed, prior or rejected role")
				}
				if previous := priorLinks[unitID]; previous != nil {
					if !reflect.DeepEqual(associationStableFields(link, "role"), associationStableFields(previous, "role")) || matchedIdentity {
						test.Fatal("concurrent role update changed persisted identity metadata or lost the observed generated-ACK distinction")
					}
				} else if !matchedIdentity {
					test.Fatal("first-time concurrent association has no successfully acknowledged persisted identity")
				}
				roles[unitID] = link["role"].(string)
			}
			canvasRevision := before.Canvas["revision"].(float64) + 2
			projectRevision := before.Detail["project"].(map[string]any)["revision"].(float64) + float64(successes)
			settled = requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
			requireHTTPError(test, foreign, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": firstUnit, "role": "rejected-foreign"}, http.StatusNotFound)
			requireHTTPError(test, foreign, http.MethodDelete, "/api/projects/"+projectID+"/canvas-links/"+canvasID+"/units/"+firstUnit, nil, http.StatusNotFound)
			requireHTTPError(test, owner, http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": before.Canvas}, http.StatusConflict)
			if !reflect.DeepEqual(settled, requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)) {
				test.Fatal("foreign mutation or stale CAS changed the concurrent association settlement")
			}
			requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projectID+"/canvas-links/"+canvasID+"/units/"+firstUnit, nil)
			delete(roles, firstUnit)
			projectRevision++
			requireProjectRoleState(test, owner, projectID, canvasID, before, units, roles, canvasRevision, projectRevision, true)
			requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projectID+"/canvases/"+canvasID, nil)
			requireProjectRoleState(test, owner, projectID, canvasID, before, units, map[string]string{}, canvasRevision+1, projectRevision+1, false)
			requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
			if !reflect.DeepEqual(foreignRelation, readProjectRelationState(test, foreign, foreignProject, foreignCanvas)) {
				test.Fatal("concurrent owner associations changed the foreign canvas or history")
			}
		}
		test.Logf("association_shape=%s trials=3 committed_requests=%d internal_rejections=%d; a start barrier does not control database interleaving", shape, committed, rejected)
	}
}

func runProjectConcurrentReassociation(test *testing.T, owner, foreign *runner.Client, prefix string) {
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign project contention "+prefix)
	foreignCanvas := "foreign-project-race-" + prefix
	createAssociationCanvas(test, foreign, foreignCanvas)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	foreignRelation := readProjectRelationState(test, foreign, foreignProject, foreignCanvas)
	for attempt := 0; attempt < 3; attempt++ {
		firstProject, firstUnit := createMatrixProject(test, owner, fmt.Sprintf("First contending project %d %s", attempt, prefix))
		secondProject, secondUnit := createMatrixProject(test, owner, fmt.Sprintf("Second contending project %d %s", attempt, prefix))
		canvasID := fmt.Sprintf("project-link-race-%s-%d", prefix, attempt)
		createAssociationCanvas(test, owner, canvasID)
		projects := []string{firstProject, secondProject}
		unitIDs := []string{firstUnit, secondUnit}
		before := []projectRelationState{readProjectRelationState(test, owner, firstProject, canvasID), readProjectRelationState(test, owner, secondProject, canvasID)}
		units := [][]map[string]any{readWorkbenchState(test, owner, firstProject).Units, readWorkbenchState(test, owner, secondProject).Units}
		requests := []associationLinkRequest{{firstProject, firstUnit, "first-project"}, {secondProject, secondUnit, "second-project"}}
		acknowledgements := map[string]map[string]any{}
		for _, outcome := range concurrentCanvasLinks(test, owner, canvasID, requests) {
			acknowledgements[outcome.Request.ProjectID] = successfulAssociationAck(test, outcome, canvasID)
		}
		canvas := readCanvasDocument(test, owner, canvasID)
		winner, loser := 0, 1
		if canvas["projectId"] == secondProject {
			winner, loser = 1, 0
		} else if canvas["projectId"] != firstProject {
			test.Fatal("cross-project contention assigned the canvas outside the successful candidates")
		}
		canvasRevision := before[0].Canvas["revision"].(float64) + 2
		projectRevisions := []float64{before[0].Detail["project"].(map[string]any)["revision"].(float64) + 1, before[1].Detail["project"].(map[string]any)["revision"].(float64) + 1}
		links := make([]map[string]map[string]any, 2)
		settled := make([]projectRelationState, 2)
		for index, projectID := range projects {
			actual := readProjectRelationState(test, owner, projectID, canvasID)
			links[index] = projectLinksByUnit(test, actual.Detail["canvasUnitLinks"], projectID, canvasID)
			link := links[index][unitIDs[index]]
			if len(links[index]) != 1 || link == nil || !reflect.DeepEqual(associationStableFields(link, "createdAt"), associationStableFields(acknowledgements[projectID], "createdAt")) {
				test.Fatal("cross-project contention lost either acknowledged association identity or role")
			}
			settled[index] = requireContendedProjectState(test, owner, projectID, canvasID, before[index], units[index], links[index], projects[winner], canvasRevision, projectRevisions[index])
		}
		requireHTTPError(test, owner, http.MethodDelete, "/api/projects/"+projects[loser]+"/canvas-links/"+canvasID+"/units/"+unitIDs[loser], nil, http.StatusBadRequest)
		requireHTTPError(test, owner, http.MethodDelete, "/api/projects/"+projects[loser]+"/canvases/"+canvasID, nil, http.StatusBadRequest)
		requireHTTPError(test, owner, http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": before[0].Canvas}, http.StatusConflict)
		for index, projectID := range projects {
			requireHTTPError(test, foreign, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": unitIDs[index], "role": "foreign-project"}, http.StatusNotFound)
			requireHTTPError(test, foreign, http.MethodDelete, "/api/projects/"+projectID+"/canvas-links/"+canvasID+"/units/"+unitIDs[index], nil, http.StatusNotFound)
			if !reflect.DeepEqual(settled[index], requireContendedProjectState(test, owner, projectID, canvasID, before[index], units[index], links[index], projects[winner], canvasRevision, projectRevisions[index])) {
				test.Fatal("old-project unlink, foreign mutation or stale CAS changed contended state")
			}
		}
		requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projects[winner]+"/canvases/"+canvasID, nil)
		canvasRevision++
		projectRevisions[winner]++
		links[winner] = map[string]map[string]any{}
		for index, projectID := range projects {
			settled[index] = requireContendedProjectState(test, owner, projectID, canvasID, before[index], units[index], links[index], "", canvasRevision, projectRevisions[index])
		}
		requireHTTPError(test, owner, http.MethodDelete, "/api/projects/"+projects[loser]+"/canvases/"+canvasID, nil, http.StatusBadRequest)
		requireHTTPError(test, owner, http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": before[0].Canvas}, http.StatusConflict)
		for index, projectID := range projects {
			if !reflect.DeepEqual(settled[index], requireContendedProjectState(test, owner, projectID, canvasID, before[index], units[index], links[index], "", canvasRevision, projectRevisions[index])) {
				test.Fatal("post-detach rejection changed the independent canvas or retained old association")
			}
		}
		recovered := linkCanvasRole(test, owner, projects[loser], canvasID, unitIDs[loser], "recovered", true)
		retained := links[loser][unitIDs[loser]]
		if recovered["id"] == retained["id"] {
			test.Fatal("stale-link recovery lost the observed generated-ACK identity distinction")
		}
		retained["role"] = "recovered"
		canvasRevision++
		projectRevisions[loser]++
		for index, projectID := range projects {
			requireContendedProjectState(test, owner, projectID, canvasID, before[index], units[index], links[index], projects[loser], canvasRevision, projectRevisions[index])
		}
		requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projects[loser]+"/canvases/"+canvasID, nil)
		links[loser] = map[string]map[string]any{}
		for index, projectID := range projects {
			if index == loser {
				projectRevisions[index]++
			}
			requireContendedProjectState(test, owner, projectID, canvasID, before[index], units[index], links[index], "", canvasRevision+1, projectRevisions[index])
		}
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
		if !reflect.DeepEqual(foreignRelation, readProjectRelationState(test, foreign, foreignProject, foreignCanvas)) {
			test.Fatal("cross-project contention or recovery changed foreign canvas state")
		}
	}
	test.Log("cross_project_trials=3 committed_requests=6; authoritative assignment is read from the canvas, not response completion order")
}
