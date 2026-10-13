package canvas_contract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"canvas-contract/runner"
)

func importBodyAtBoundary(test *testing.T, size int) ([]byte, []map[string]any) {
	test.Helper()
	units := make([]map[string]any, 4)
	for index := range units {
		units[index] = map[string]any{"kind": "chapter", "title": fmt.Sprintf("Body boundary chapter %d", index), "sourceText": ""}
	}
	encode := func() []byte {
		body, err := json.Marshal(map[string]any{"units": units})
		if err != nil {
			test.Fatal("could not encode import body boundary fixture")
		}
		return body
	}
	sourceBytes := size - len(encode())
	if sourceBytes <= 0 {
		test.Fatal("body boundary is too small for its JSON framing")
	}
	for index := range units {
		length := sourceBytes / len(units)
		if index < sourceBytes%len(units) {
			length++
		}
		units[index]["sourceText"] = strings.Repeat(fmt.Sprintf("%d", index+1), length)
	}
	body := encode()
	if len(body) != size {
		test.Fatal("import fixture did not reach the exact requested byte boundary")
	}
	return body, units
}

func runWorkbenchImportBodyBoundary(test *testing.T, owner, foreign *runner.Client, prefix string) {
	const bodyBudget = 32 << 20
	projectID, _ := createMatrixProject(test, owner, "Import body boundary "+prefix)
	foreignProject, _ := createMatrixProject(test, foreign, "Foreign import body boundary "+prefix)
	before := readWorkbenchState(test, owner, projectID)
	foreignBefore := readWorkbenchState(test, foreign, foreignProject)
	path := "/api/projects/" + projectID + "/units/import"
	excess, _ := importBodyAtBoundary(test, bodyBudget+1)
	rejectOversize := func(want workbenchState) {
		response, envelope, err := owner.DoBytes(runner.Context(test), http.MethodPost, path, "application/json", excess, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		requireWorkbenchState(test, owner, projectID, want)
		requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
	}
	rejectOversize(before)
	body, input := importBodyAtBoundary(test, bodyBudget)
	response, envelope, err := owner.DoBytes(runner.Context(test), http.MethodPost, path, "application/json", body, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	acknowledged := runner.DecodeData[struct {
		Units []map[string]any `json:"units"`
	}](test, envelope).Units
	if len(acknowledged) != len(input) {
		test.Fatal("exact-body-budget import did not acknowledge all chapters")
	}
	after := readWorkbenchState(test, owner, projectID)
	if len(after.Units) != len(before.Units)+len(input) || !reflect.DeepEqual(after.Units[:len(before.Units)], before.Units) {
		test.Fatal("exact-body-budget import partially persisted or replaced original chapters")
	}
	runner.RequireNumber(test, after.Project["revision"], before.Project["revision"].(float64)+1, "body boundary commit revision")
	for index, unit := range after.Units[len(before.Units):] {
		if acknowledged[index]["id"] != unit["id"] || acknowledged[index]["sourceText"] != "" || unit["sourceText"] != input[index]["sourceText"] || unit["projectId"] != projectID || unit["title"] != input[index]["title"] {
			test.Fatal("exact-body-budget import changed identity or lost chapter source bytes")
		}
		runner.RequireNumber(test, unit["position"], float64(len(before.Units)+index), "body boundary append position")
		runner.RequireNumber(test, unit["wordCount"], float64(len(input[index]["sourceText"].(string))), "body boundary word count")
		requireHTTPError(test, foreign, http.MethodGet, "/api/projects/"+projectID+"/units/"+unit["id"].(string), nil, http.StatusNotFound)
	}
	rejectOversize(after)
	compactInput := map[string]any{"units": []any{map[string]any{"kind": "chapter", "title": "Trailing whitespace observed", "sourceText": "Small authoritative source"}}}
	compact, err := json.Marshal(compactInput)
	if err != nil {
		test.Fatal("could not encode trailing-byte fixture")
	}
	trailing := append(compact, bytes.Repeat([]byte(" "), bodyBudget+1-len(compact))...)
	response, envelope, err = owner.DoBytes(runner.Context(test), http.MethodPost, path, "application/json", trailing, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	acknowledged = runner.DecodeData[struct {
		Units []map[string]any `json:"units"`
	}](test, envelope).Units
	final := readWorkbenchState(test, owner, projectID)
	if len(acknowledged) != 1 || len(final.Units) != len(after.Units)+1 || !reflect.DeepEqual(final.Units[:len(after.Units)], after.Units) || final.Units[len(after.Units)]["id"] != acknowledged[0]["id"] || final.Units[len(after.Units)]["sourceText"] != "Small authoritative source" {
		test.Fatal("observed trailing-whitespace acceptance changed prior chapters or persisted padding")
	}
	runner.RequireNumber(test, final.Project["revision"], after.Project["revision"].(float64)+1, "trailing framing commit revision")
	nonWhitespaceTrailing := append(append([]byte{}, compact...), []byte(`{}`)...)
	response, envelope, err = owner.DoBytes(runner.Context(test), http.MethodPost, path, "application/json", nonWhitespaceTrailing, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	acknowledged = runner.DecodeData[struct {
		Units []map[string]any `json:"units"`
	}](test, envelope).Units
	nonWhitespaceAfter := readWorkbenchState(test, owner, projectID)
	if len(acknowledged) != 1 || len(nonWhitespaceAfter.Units) != len(final.Units)+1 || nonWhitespaceAfter.Units[len(final.Units)]["sourceText"] != "Small authoritative source" {
		test.Fatal("observed non-whitespace trailing JSON was not isolated from the decoded import payload")
	}
	runner.RequireNumber(test, nonWhitespaceAfter.Project["revision"], final.Project["revision"].(float64)+1, "non-whitespace trailing framing commit revision")
	test.Log("Go baseline currently accepts and ignores non-whitespace trailing JSON; Java target policy remains pending")
	requireHTTPError(test, foreign, http.MethodPost, path, compactInput, http.StatusNotFound)
	requireWorkbenchState(test, owner, projectID, nonWhitespaceAfter)
	requireWorkbenchState(test, foreign, foreignProject, foreignBefore)
}
