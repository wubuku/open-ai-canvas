package canvas_contract_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"canvas-contract/runner"
)

func requireWorkbenchState(test *testing.T, client *runner.Client, projectID string, want workbenchState) {
	test.Helper()
	if !reflect.DeepEqual(readWorkbenchState(test, client, projectID), want) {
		test.Fatal("rejected or unrelated write changed project metadata, revision or units")
	}
}

func createWorkbenchUnit(test *testing.T, client *runner.Client, projectID, title string) string {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, client, http.MethodPost, "/api/projects/"+projectID+"/units", map[string]any{"kind": "chapter", "title": title, "sourceText": "synthetic source"})
	unit := runner.DecodeData[struct {
		Unit struct {
			ID string `json:"id"`
		} `json:"unit"`
	}](test, envelope)
	if unit.Unit.ID == "" {
		test.Fatal("created unit has no identity")
	}
	return unit.Unit.ID
}

func runWorkbenchBulkImportBoundaries(test *testing.T, owner, foreign *runner.Client, prefix string) {
	projectID, _ := createMatrixProject(test, owner, "Bulk import boundary "+prefix)
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign bulk boundary "+prefix)
	before := readWorkbenchState(test, owner, projectID)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	path := "/api/projects/" + projectID + "/units/import"
	inputs := make([]map[string]any, 2500)
	for index := range inputs {
		inputs[index] = map[string]any{"kind": "chapter", "title": fmt.Sprintf("Bulk unit %04d", index), "sourceText": fmt.Sprintf("Bulk source %04d", index)}
	}
	inputs[len(inputs)-1]["title"] = "  " + strings.Repeat("章", 240) + "  "
	tooMany := append(append([]map[string]any{}, inputs...), map[string]any{"kind": "chapter", "title": "Over batch bound", "sourceText": "Must not persist"})
	invalidLast := append([]map[string]any{}, inputs...)
	invalidLast[len(invalidLast)-1] = map[string]any{"kind": "chapter", "title": strings.Repeat("章", 241), "sourceText": "Invalid final row"}
	for _, batch := range [][]map[string]any{nil, {}, tooMany, invalidLast} {
		requireHTTPError(test, owner, http.MethodPost, path, map[string]any{"units": batch}, http.StatusBadRequest)
		requireWorkbenchState(test, owner, projectID, before)
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	}
	requireHTTPError(test, foreign, http.MethodPost, path, map[string]any{"units": inputs}, http.StatusNotFound)
	requireWorkbenchState(test, owner, projectID, before)
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, path, map[string]any{"units": inputs})
	imported := runner.DecodeData[struct {
		Units []map[string]any `json:"units"`
	}](test, envelope).Units
	if len(imported) != len(inputs) {
		test.Fatal("exact-boundary batch did not return all twenty-five hundred units")
	}
	identities := map[string]bool{}
	for _, unit := range before.Units {
		identity, ok := unit["id"].(string)
		if !ok || identity == "" {
			test.Fatal("bulk fixture contains a unit without an identity")
		}
		identities[identity] = true
	}
	for index, unit := range imported {
		identity, ok := unit["id"].(string)
		if !ok || identity == "" || identities[identity] || unit["projectId"] != projectID || unit["title"] != strings.TrimSpace(inputs[index]["title"].(string)) || unit["sourceText"] != "" || unit["kind"] != "chapter" || unit["status"] != "draft" {
			test.Fatal("bulk import returned duplicate, cross-project or invalid summary content")
		}
		identities[identity] = true
		runner.RequireNumber(test, unit["position"], float64(len(before.Units)+index), "bulk append position")
		runner.RequireNumber(test, unit["wordCount"], float64(len(inputs[index]["sourceText"].(string))), "bulk source word count")
	}
	after := readWorkbenchState(test, owner, projectID)
	if len(after.Units) != len(before.Units)+len(inputs) || !reflect.DeepEqual(after.Units[:len(before.Units)], before.Units) {
		test.Fatal("bulk import replaced original units or persisted only part of the batch")
	}
	runner.RequireNumber(test, after.Project["revision"], before.Project["revision"].(float64)+1, "bulk commit revision")
	for index, persisted := range after.Units[len(before.Units):] {
		for _, field := range []string{"id", "projectId", "kind", "title", "wordCount", "status", "position"} {
			if !reflect.DeepEqual(persisted[field], imported[index][field]) {
				test.Fatal("complete per-unit reads did not preserve bulk identities, ordering and source text")
			}
		}
		if persisted["sourceText"] != inputs[index]["sourceText"] {
			test.Fatal("per-unit read did not retain the complete imported source text omitted from the summary acknowledgement")
		}
	}
	for _, batch := range [][]map[string]any{tooMany, invalidLast} {
		requireHTTPError(test, owner, http.MethodPost, path, map[string]any{"units": batch}, http.StatusBadRequest)
		requireWorkbenchState(test, owner, projectID, after)
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	}
	for _, unit := range []map[string]any{imported[0], imported[len(imported)-1]} {
		identity := unit["id"].(string)
		requireHTTPError(test, foreign, http.MethodGet, "/api/projects/"+projectID+"/units/"+identity, nil, http.StatusNotFound)
		requireHTTPError(test, foreign, http.MethodGet, "/api/projects/"+foreignProject+"/units/"+identity, nil, http.StatusNotFound)
	}
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
}

func runWorkbenchDuplicateAndReplay(test *testing.T, owner, foreign *runner.Client, prefix string) {
	name := "Duplicate workbench " + prefix
	projectID, _ := createMatrixProject(test, owner, name)
	before := readWorkbenchState(test, owner, projectID)
	_, listEnvelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects", nil)
	beforeList := runner.DecodeData[map[string]any](test, listEnvelope)
	for attempt := 0; attempt < 2; attempt++ {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/projects", map[string]any{"name": "  " + name + "  "}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusConflict, "project_name_conflict")
		requireWorkbenchState(test, owner, projectID, before)
		_, listEnvelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects", nil)
		if !reflect.DeepEqual(beforeList, runner.DecodeData[map[string]any](test, listEnvelope)) {
			test.Fatal("duplicate project creation changed the owner's project list")
		}
	}
	foreignProject, _ := createMatrixProject(test, foreign, name)
	if foreignProject == projectID {
		test.Fatal("same name across users reused a project identity")
	}
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	first := createWorkbenchUnit(test, owner, projectID, "Repeated chapter")
	second := createWorkbenchUnit(test, owner, projectID, "Repeated chapter")
	identities := map[string]bool{first: true}
	if first == second {
		test.Fatal("unit POST replay was incorrectly assumed to be idempotent")
	}
	identities[second] = true
	for attempt := 0; attempt < 2; attempt++ {
		_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+projectID+"/units/import", map[string]any{"units": []any{map[string]any{"kind": "chapter", "title": "Repeated import", "sourceText": "synthetic import"}}})
		imported := runner.DecodeData[struct {
			Units []map[string]any `json:"units"`
		}](test, envelope)
		if len(imported.Units) != 1 {
			test.Fatal("replayed import returned the wrong number of units")
		}
		identity, ok := imported.Units[0]["id"].(string)
		if !ok || identity == "" || identities[identity] {
			test.Fatal("import replay reused or omitted a unit identity")
		}
		identities[identity] = true
	}
	after := readWorkbenchState(test, owner, projectID)
	if len(after.Units) != len(before.Units)+4 {
		test.Fatal("unit creation/import replay did not append exactly four units")
	}
	runner.RequireNumber(test, after.Project["revision"], before.Project["revision"].(float64)+4, "project revision after four unit writes")
	for identity := range identities {
		_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects/"+projectID+"/units/"+identity, nil)
		unit := runner.DecodeData[struct {
			Unit map[string]any `json:"unit"`
		}](test, envelope).Unit
		if unit["id"] != identity || unit["projectId"] != projectID || unit["sourceText"] == "" {
			test.Fatal("replayed unit was not readable with its own source text")
		}
	}
	_, listEnvelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects", nil)
	listBeforeRace := runner.DecodeData[struct {
		Projects []map[string]any `json:"projects"`
	}](test, listEnvelope)
	type createOutcome struct {
		response runner.Response
		envelope runner.Envelope
		err      error
	}
	start := make(chan struct{})
	results := make(chan createOutcome, 2)
	for count := 0; count < 2; count++ {
		go func() {
			<-start
			response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/projects", map[string]any{"name": "Concurrent duplicate " + prefix}, nil)
			results <- createOutcome{response, envelope, err}
		}()
	}
	close(start)
	winner := ""
	conflicts := 0
	for count := 0; count < 2; count++ {
		result := <-results
		if result.err != nil {
			test.Fatal(result.err)
		}
		if result.response.StatusCode == http.StatusConflict {
			runner.RequireError(test, result.response, result.envelope, http.StatusConflict, "project_name_conflict")
			conflicts++
		} else {
			runner.RequireSuccess(test, result.response, result.envelope)
			created := runner.DecodeData[struct {
				Project struct {
					ID string `json:"id"`
				} `json:"project"`
			}](test, result.envelope)
			if winner != "" || created.Project.ID == "" {
				test.Fatal("concurrent duplicate name creation produced multiple or empty identities")
			}
			winner = created.Project.ID
		}
	}
	if winner == "" || conflicts != 1 {
		test.Fatal("concurrent project creation did not have exactly one winner and one name conflict")
	}
	_, listEnvelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects", nil)
	listAfterRace := runner.DecodeData[struct {
		Projects []map[string]any `json:"projects"`
	}](test, listEnvelope)
	if len(listAfterRace.Projects) != len(listBeforeRace.Projects)+1 || !containsProjectSummaryID(listAfterRace.Projects, winner) {
		test.Fatal("concurrent duplicate project creation left missing or extra projects")
	}
	_, _ = requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects/"+winner, nil)
	requireWorkbenchState(test, owner, projectID, after)
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
}

func runWorkbenchCrossProjectUnits(test *testing.T, owner, foreign *runner.Client, prefix string) {
	firstProject, firstUnit := createMatrixProject(test, owner, "First unit scope "+prefix)
	secondProject, secondUnit := createMatrixProject(test, owner, "Second unit scope "+prefix)
	foreignProject, foreignUnit := createMatrixProject(test, foreign, "Foreign unit scope "+prefix)
	firstBefore := readWorkbenchState(test, owner, firstProject)
	secondBefore := readWorkbenchState(test, owner, secondProject)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	checkUnchanged := func() {
		requireWorkbenchState(test, owner, firstProject, firstBefore)
		requireWorkbenchState(test, owner, secondProject, secondBefore)
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	}
	for _, combination := range []struct {
		client            *runner.Client
		projectID, unitID string
	}{
		{owner, firstProject, secondUnit}, {owner, firstProject, foreignUnit},
		{owner, secondProject, firstUnit}, {owner, firstProject, "missing-" + firstUnit},
		{owner, foreignProject, firstUnit}, {foreign, firstProject, foreignUnit},
	} {
		for _, request := range []struct {
			method, suffix string
			payload        any
			status         int
			reason         string
		}{
			{http.MethodGet, "", nil, http.StatusNotFound, ""},
			{http.MethodPatch, "", map[string]any{"title": "Rejected cross-scope edit", "sourceText": "not persisted"}, http.StatusNotFound, ""},
			{http.MethodDelete, "", nil, http.StatusNotFound, ""},
			{http.MethodGet, "/workspace", nil, http.StatusInternalServerError, "internal"},
		} {
			response, envelope, err := combination.client.Do(runner.Context(test), request.method, "/api/projects/"+combination.projectID+"/units/"+combination.unitID+request.suffix, request.payload, nil)
			if err != nil {
				test.Fatal(err)
			}
			runner.RequireError(test, response, envelope, request.status, request.reason)
			checkUnchanged()
		}
	}
	for _, identities := range [][]string{{secondUnit}, {foreignUnit}, {"missing-" + firstUnit}, {firstUnit, firstUnit}} {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPatch, "/api/projects/"+firstProject+"/units/reorder", map[string]any{"unitIds": identities}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		checkUnchanged()
	}
	for _, request := range []struct {
		client    *runner.Client
		projectID string
		units     []any
		status    int
	}{
		{owner, firstProject, []any{map[string]any{"title": "Valid preimage"}, map[string]any{"title": "Invalid row", "kind": "unknown-kind"}}, http.StatusBadRequest},
		{owner, foreignProject, []any{map[string]any{"title": "Rejected import"}}, http.StatusNotFound},
		{foreign, firstProject, []any{map[string]any{"title": "Rejected import"}}, http.StatusNotFound},
	} {
		response, envelope, err := request.client.Do(runner.Context(test), http.MethodPost, "/api/projects/"+request.projectID+"/units/import", map[string]any{"units": request.units}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, request.status, "")
		checkUnchanged()
	}
	_, imported := requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+secondProject+"/units/import", map[string]any{"units": []any{map[string]any{"title": "Scoped successful import", "sourceText": "synthetic source"}}})
	units := runner.DecodeData[struct {
		Units []map[string]any `json:"units"`
	}](test, imported).Units
	if len(units) != 1 || units[0]["projectId"] != secondProject || units[0]["id"] == firstUnit || units[0]["id"] == foreignUnit {
		test.Fatal("import leaked a unit from another project")
	}
	after := readWorkbenchState(test, owner, secondProject)
	if len(after.Units) != len(secondBefore.Units)+1 {
		test.Fatal("scoped import changed the wrong unit membership")
	}
	runner.RequireNumber(test, after.Project["revision"], secondBefore.Project["revision"].(float64)+1, "scoped import revision")
	requireWorkbenchState(test, owner, firstProject, firstBefore)
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
}

func runWorkbenchConcurrentReorder(test *testing.T, owner, foreign *runner.Client, prefix string) {
	projectID, firstUnit := createMatrixProject(test, owner, "Concurrent reorder "+prefix)
	identities := []string{firstUnit}
	for count := 0; count < 7; count++ {
		identities = append(identities, createWorkbenchUnit(test, owner, projectID, fmt.Sprintf("Concurrent chapter %d", count)))
	}
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign concurrent control "+prefix)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	firstOrder := append([]string(nil), identities...)
	secondOrder := append([]string(nil), identities...)
	for index := 1; index < len(identities); index++ {
		secondOrder[index] = identities[len(identities)-index]
	}
	type outcome struct {
		response runner.Response
		envelope runner.Envelope
		err      error
	}
	for attempt := 0; attempt < 3; attempt++ {
		before := readWorkbenchState(test, owner, projectID)
		start := make(chan struct{})
		results := make(chan outcome, 2)
		for _, order := range [][]string{firstOrder, secondOrder} {
			go func() {
				<-start
				response, envelope, err := owner.Do(runner.Context(test), http.MethodPatch, "/api/projects/"+projectID+"/units/reorder", map[string]any{"unitIds": order}, nil)
				results <- outcome{response, envelope, err}
			}()
		}
		close(start)
		for count := 0; count < 2; count++ {
			result := <-results
			if result.err != nil {
				test.Fatal(result.err)
			}
			runner.RequireSuccess(test, result.response, result.envelope)
		}
		after := readWorkbenchState(test, owner, projectID)
		runner.RequireNumber(test, after.Project["revision"], before.Project["revision"].(float64)+2, "concurrent reorder committed revision")
		actualOrder := []string{}
		for position, unit := range after.Units {
			identity, ok := unit["id"].(string)
			if !ok {
				test.Fatal("concurrent reorder lost a unit identity")
			}
			actualOrder = append(actualOrder, identity)
			runner.RequireNumber(test, unit["position"], float64(position), "atomic unit position")
		}
		if !reflect.DeepEqual(actualOrder, firstOrder) && !reflect.DeepEqual(actualOrder, secondOrder) {
			test.Fatal("concurrent reorder left a mixed or incomplete ordering")
		}
		for _, previous := range before.Units {
			matched := false
			for _, unit := range after.Units {
				if unit["id"] == previous["id"] {
					matched = true
					for _, field := range []string{"projectId", "title", "kind", "sourceText", "wordCount", "status", "createdAt"} {
						if !reflect.DeepEqual(unit[field], previous[field]) {
							test.Fatal("concurrent reorder changed non-ordering unit content")
						}
					}
				}
			}
			if !matched {
				test.Fatal("concurrent reorder deleted a unit")
			}
		}
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	}
}

func runWorkbenchDeletionCascade(test *testing.T, owner, foreign *runner.Client, prefix string) {
	projectID, unitID := createMatrixProject(test, owner, "Deletion cascade "+prefix)
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign cascade control "+prefix)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	content := runner.ImageFixture(test)
	resourceID := uploadResource(test, owner, "cascade.png", "image", content)
	createImageAsset(test, owner, resourceID, len(content))
	resourceBefore := resourceState(test, owner)
	canvasID := "cascade-" + prefix
	payload := runner.FixtureProject(canvasID, "Retained cascade canvas", 0, false)
	attachResource(payload, resourceID)
	createCanvas(test, owner, canvasID, payload)
	_, _ = requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": unitID})
	_, shareEnvelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/canvas-projects/"+canvasID+"/share", map[string]any{})
	share := runner.DecodeData[struct {
		Share struct {
			Token string `json:"token"`
		} `json:"share"`
	}](test, shareEnvelope)
	if share.Share.Token == "" {
		test.Fatal("cascade share fixture has no token")
	}
	beforeCanvas, beforeHistory := readCanvasDocument(test, owner, canvasID), readCanvasHistory(test, owner, canvasID)
	_, _ = requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projectID+"/units/"+unitID, nil)
	if !reflect.DeepEqual(readCanvasDocument(test, owner, canvasID), beforeCanvas) || !reflect.DeepEqual(readCanvasHistory(test, owner, canvasID), beforeHistory) {
		test.Fatal("unit deletion changed or deleted the linked canvas")
	}
	_, detailEnvelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/projects/"+projectID, nil)
	detail := runner.DecodeData[map[string]any](test, detailEnvelope)
	if units, ok := detail["units"].([]any); detail["units"] != nil && (!ok || len(units) != 0) {
		test.Fatal("unit deletion left project units")
	}
	if links, ok := detail["canvasUnitLinks"].([]any); detail["canvasUnitLinks"] != nil && (!ok || len(links) != 0) {
		test.Fatal("unit deletion left canvas-unit links")
	}
	newUnit := createWorkbenchUnit(test, owner, projectID, "Retained until project deletion")
	_, _ = requireHTTPSuccess(test, owner, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", map[string]any{"canvasId": canvasID, "unitId": newUnit})
	beforeCanvas, beforeHistory = readCanvasDocument(test, owner, canvasID), readCanvasHistory(test, owner, canvasID)
	_, _ = requireHTTPSuccess(test, owner, http.MethodDelete, "/api/projects/"+projectID, nil)
	afterCanvas, afterHistory := readCanvasDocument(test, owner, canvasID), readCanvasHistory(test, owner, canvasID)
	if afterCanvas["projectId"] != nil && afterCanvas["projectId"] != "" {
		test.Fatal("project deletion did not detach the retained canvas")
	}
	for _, field := range []string{"id", "title", "nodes", "connections", "createdAt"} {
		if !reflect.DeepEqual(afterCanvas[field], beforeCanvas[field]) {
			test.Fatal("project deletion destroyed retained canvas content")
		}
	}
	runner.RequireNumber(test, afterCanvas["revision"], beforeCanvas["revision"].(float64)+1, "detached canvas revision")
	if !reflect.DeepEqual(afterHistory["snapshots"], beforeHistory["snapshots"]) {
		test.Fatal("project deletion destroyed canvas history")
	}
	runner.RequireNumber(test, afterHistory["currentRevision"], afterCanvas["revision"].(float64), "detached history revision")
	for _, path := range []string{"/api/projects/" + projectID, "/api/projects/" + projectID + "/core", "/api/projects/" + projectID + "/units/" + newUnit} {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodGet, path, nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusNotFound, "")
	}
	_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/canvas-projects/"+canvasID+"/share", nil)
	retainedShare := runner.DecodeData[struct {
		Share struct {
			Token   string `json:"token"`
			Enabled bool   `json:"enabled"`
		} `json:"share"`
	}](test, envelope)
	if !retainedShare.Share.Enabled || retainedShare.Share.Token != share.Share.Token {
		test.Fatal("project deletion revoked an independently owned canvas share")
	}
	public, err := runner.NewClient("public-cascade", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	defer public.HTTP.CloseIdleConnections()
	_, _ = requireHTTPSuccess(test, public, http.MethodGet, "/api/public/canvas-shares/"+share.Share.Token, nil)
	_, _ = requireHTTPSuccess(test, owner, http.MethodGet, "/api/assets/"+resourceID, nil)
	requireResourceState(test, owner, resourceBefore)
	requireResourceBytes(test, owner, resourceID, content)
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	response, envelope, err := owner.Do(runner.Context(test), http.MethodDelete, "/api/projects/"+projectID, nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusNotFound, "")
	if !reflect.DeepEqual(readCanvasDocument(test, owner, canvasID), afterCanvas) {
		test.Fatal("project DELETE replay modified the detached canvas")
	}
}
