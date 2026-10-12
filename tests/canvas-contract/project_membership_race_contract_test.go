package canvas_contract_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"canvas-contract/runner"
)

type workbenchMembershipOutcome struct {
	Role     string
	Response runner.Response
	Envelope runner.Envelope
	Err      error
}

func runWorkbenchReorderMembership(test *testing.T, owner, foreign *runner.Client, prefix, mutation string) {
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign membership "+mutation+prefix)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	for attempt := 0; attempt < 3; attempt++ {
		projectID, firstUnit := createMatrixProject(test, owner, fmt.Sprintf("Membership %s %s %d", mutation, prefix, attempt))
		identities := []string{firstUnit}
		for index := 0; index < 7; index++ {
			identities = append(identities, createWorkbenchUnit(test, owner, projectID, fmt.Sprintf("Membership chapter %d", index)))
		}
		projectPath := "/api/projects/" + projectID
		requireHTTPSuccess(test, owner, http.MethodPatch, projectPath+"/units/reorder", map[string]any{"unitIds": identities})
		before := readWorkbenchState(test, owner, projectID)
		reverse := make([]string, len(identities))
		for index := range identities {
			reverse[index] = identities[len(identities)-1-index]
		}
		mutationMethod, mutationPath := http.MethodPost, projectPath+"/units"
		var mutationPayload any
		input := []map[string]any{{"kind": "chapter", "title": "Membership appended chapter", "sourceText": "Membership authoritative source", "position": len(identities)}}
		switch mutation {
		case "create":
			mutationPayload = input[0]
		case "delete":
			mutationMethod, mutationPath = http.MethodDelete, projectPath+"/units/"+firstUnit
		case "import":
			input = append(input, map[string]any{"kind": "chapter", "title": "Membership second imported chapter", "sourceText": "Second authoritative source"})
			mutationPath = projectPath + "/units/import"
			mutationPayload = map[string]any{"units": input}
		default:
			test.Fatal("unknown membership mutation fixture")
		}
		start := make(chan struct{})
		results := make(chan workbenchMembershipOutcome, 2)
		go func() {
			<-start
			response, envelope, err := owner.Do(runner.Context(test), http.MethodPatch, projectPath+"/units/reorder", map[string]any{"unitIds": reverse}, nil)
			results <- workbenchMembershipOutcome{"reorder", response, envelope, err}
		}()
		go func() {
			<-start
			response, envelope, err := owner.Do(runner.Context(test), mutationMethod, mutationPath, mutationPayload, nil)
			results <- workbenchMembershipOutcome{"mutation", response, envelope, err}
		}()
		close(start)
		outcomes := []workbenchMembershipOutcome{<-results, <-results}
		var reorderOutcome, mutationOutcome workbenchMembershipOutcome
		committed := 0
		for _, outcome := range outcomes {
			if outcome.Err != nil {
				test.Fatal(outcome.Err)
			}
			if outcome.Response.StatusCode == http.StatusOK {
				runner.RequireSuccess(test, outcome.Response, outcome.Envelope)
				committed++
			} else if outcome.Response.StatusCode == http.StatusInternalServerError {
				runner.RequireError(test, outcome.Response, outcome.Envelope, http.StatusInternalServerError, "internal")
			} else if outcome.Role == "reorder" && (outcome.Response.StatusCode == http.StatusBadRequest || mutation == "delete" && outcome.Response.StatusCode == http.StatusNotFound) {
				runner.RequireError(test, outcome.Response, outcome.Envelope, outcome.Response.StatusCode, "")
			} else {
				test.Fatal("membership race returned an unclassified HTTP outcome")
			}
			if outcome.Role == "reorder" {
				reorderOutcome = outcome
			} else {
				mutationOutcome = outcome
			}
		}
		if committed == 0 {
			test.Fatal("membership race produced no successful operation")
		}
		removed := mutation == "delete" && mutationOutcome.Response.StatusCode == http.StatusOK
		newUnits := []map[string]any{}
		if mutation != "delete" && mutationOutcome.Response.StatusCode == http.StatusOK {
			if mutation == "create" {
				newUnits = append(newUnits, runner.DecodeData[struct {
					Unit map[string]any `json:"unit"`
				}](test, mutationOutcome.Envelope).Unit)
			} else {
				newUnits = runner.DecodeData[struct {
					Units []map[string]any `json:"units"`
				}](test, mutationOutcome.Envelope).Units
			}
			if len(newUnits) != len(input) {
				test.Fatal("membership write acknowledged an incomplete batch")
			}
		}
		after := readWorkbenchState(test, owner, projectID)
		runner.RequireNumber(test, after.Project["revision"], before.Project["revision"].(float64)+float64(committed), "membership race commit revision")
		if !reflect.DeepEqual(withoutWorkbenchFields(after.Project, "revision", "updatedAt"), withoutWorkbenchFields(before.Project, "revision", "updatedAt")) {
			test.Fatal("membership race changed unrelated project facts")
		}
		expectedExisting := map[string]map[string]any{}
		expectedPositions := map[string]int{}
		order := identities
		if reorderOutcome.Response.StatusCode == http.StatusOK {
			order = reverse
		}
		for index, identity := range order {
			expectedPositions[identity] = index
		}
		for _, unit := range before.Units {
			identity := unit["id"].(string)
			if !removed || identity != firstUnit {
				expectedExisting[identity] = unit
			}
		}
		newByID := map[string]int{}
		for index, unit := range newUnits {
			identity, ok := unit["id"].(string)
			_, existing := expectedExisting[identity]
			_, duplicate := newByID[identity]
			if !ok || identity == "" || existing || duplicate {
				test.Fatal("membership write returned missing, reused or duplicate unit identities")
			}
			newByID[identity] = index
		}
		if len(after.Units) != len(expectedExisting)+len(newByID) {
			test.Fatal("membership race persisted an unacknowledged, missing or partial unit")
		}
		seen := map[string]bool{}
		for _, unit := range after.Units {
			identity := unit["id"].(string)
			if seen[identity] {
				test.Fatal("membership race left duplicate units")
			}
			seen[identity] = true
			if previous, found := expectedExisting[identity]; found {
				if !reflect.DeepEqual(withoutWorkbenchFields(unit, "position", "updatedAt"), withoutWorkbenchFields(previous, "position", "updatedAt")) {
					test.Fatal("membership race changed an existing complete chapter DTO")
				}
				runner.RequireNumber(test, unit["position"], float64(expectedPositions[identity]), "membership existing position")
			} else if index, found := newByID[identity]; found {
				if unit["projectId"] != projectID || unit["title"] != input[index]["title"] || unit["sourceText"] != input[index]["sourceText"] || unit["kind"] != "chapter" || unit["status"] != "draft" {
					test.Fatal("membership write did not preserve complete owned source content")
				}
				runner.RequireNumber(test, unit["position"], float64(len(identities)+index), "membership appended position")
				runner.RequireNumber(test, unit["wordCount"], float64(len(input[index]["sourceText"].(string))), "membership source word count")
			} else {
				test.Fatal("membership race retained a deleted or unacknowledged identity")
			}
		}
		if removed {
			requireHTTPError(test, owner, http.MethodGet, projectPath+"/units/"+firstUnit, nil, http.StatusNotFound)
		}
		if mutationOutcome.Response.StatusCode == http.StatusOK {
			requireHTTPError(test, owner, http.MethodPatch, projectPath+"/units/reorder", map[string]any{"unitIds": reverse}, http.StatusBadRequest)
			requireWorkbenchState(test, owner, projectID, after)
		}
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
		test.Logf("membership_mutation=%s attempt=%d reorder_status=%d mutation_status=%d", mutation, attempt, reorderOutcome.Response.StatusCode, mutationOutcome.Response.StatusCode)
	}
}
