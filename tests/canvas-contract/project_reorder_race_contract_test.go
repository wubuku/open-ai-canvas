package canvas_contract_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"canvas-contract/runner"
)

type workbenchReorderOutcome struct {
	Order    []string
	Response runner.Response
	Envelope runner.Envelope
	Err      error
}

func withoutWorkbenchFields(value map[string]any, fields ...string) map[string]any {
	copy := make(map[string]any, len(value))
	for key, field := range value {
		copy[key] = field
	}
	for _, field := range fields {
		delete(copy, field)
	}
	return copy
}

func runWorkbenchOppositeOrder(test *testing.T, owner, foreign *runner.Client, prefix string) {
	projectID, firstUnit := createMatrixProject(test, owner, "Opposite order "+prefix)
	identities := []string{firstUnit}
	for index := 0; index < 7; index++ {
		identities = append(identities, createWorkbenchUnit(test, owner, projectID, fmt.Sprintf("Opposite order chapter %d", index)))
	}
	firstOrder := append([]string(nil), identities...)
	secondOrder := make([]string, len(identities))
	for index := range identities {
		secondOrder[index] = identities[len(identities)-1-index]
	}
	if firstOrder[0] == secondOrder[0] {
		test.Fatal("opposite-order fixture accidentally shares the leading unit")
	}
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign opposite order "+prefix)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	path := "/api/projects/" + projectID + "/units/reorder"
	requireHTTPSuccess(test, owner, http.MethodPatch, path, map[string]any{"unitIds": firstOrder})
	committed, rejected := 0, 0
	for attempt := 0; attempt < 6; attempt++ {
		before := readWorkbenchState(test, owner, projectID)
		start := make(chan struct{})
		results := make(chan workbenchReorderOutcome, 2)
		for _, order := range [][]string{firstOrder, secondOrder} {
			go func() {
				<-start
				response, envelope, err := owner.Do(runner.Context(test), http.MethodPatch, path, map[string]any{"unitIds": order}, nil)
				results <- workbenchReorderOutcome{order, response, envelope, err}
			}()
		}
		close(start)
		outcomes := []workbenchReorderOutcome{<-results, <-results}
		winningOrders := [][]string{}
		for _, outcome := range outcomes {
			if outcome.Err != nil {
				test.Fatal(outcome.Err)
			}
			if outcome.Response.StatusCode == http.StatusOK {
				runner.RequireSuccess(test, outcome.Response, outcome.Envelope)
				winningOrders = append(winningOrders, outcome.Order)
				committed++
			} else {
				runner.RequireError(test, outcome.Response, outcome.Envelope, http.StatusInternalServerError, "internal")
				rejected++
			}
		}
		if len(winningOrders) == 0 {
			test.Fatal("opposite-order contention produced no committed complete permutation")
		}
		after := readWorkbenchState(test, owner, projectID)
		runner.RequireNumber(test, after.Project["revision"], before.Project["revision"].(float64)+float64(len(winningOrders)), "opposite-order committed revision")
		if !reflect.DeepEqual(withoutWorkbenchFields(after.Project, "revision", "updatedAt"), withoutWorkbenchFields(before.Project, "revision", "updatedAt")) || len(after.Units) != len(before.Units) {
			test.Fatal("opposite-order contention changed unrelated project facts or unit count")
		}
		beforeByID := make(map[string]map[string]any, len(before.Units))
		for _, unit := range before.Units {
			beforeByID[unit["id"].(string)] = unit
		}
		actualOrder := make([]string, len(after.Units))
		seen := map[string]bool{}
		for position, unit := range after.Units {
			identity := unit["id"].(string)
			previous, found := beforeByID[identity]
			if !found || seen[identity] || !reflect.DeepEqual(withoutWorkbenchFields(unit, "position", "updatedAt"), withoutWorkbenchFields(previous, "position", "updatedAt")) {
				test.Fatal("opposite-order contention duplicated, removed or changed a complete unit DTO")
			}
			seen[identity] = true
			actualOrder[position] = identity
			runner.RequireNumber(test, unit["position"], float64(position), "opposite-order contiguous position")
		}
		matched := false
		for _, winner := range winningOrders {
			matched = matched || reflect.DeepEqual(actualOrder, winner)
		}
		if !matched {
			test.Fatal("rejected or mixed permutation survived instead of a complete successful request")
		}
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	}
	test.Logf("reorder_trials=6 committed_requests=%d internal_rejections=%d; HTTP outcomes do not prove a database deadlock or approve Java retry semantics", committed, rejected)
}
