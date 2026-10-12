package canvas_contract_test

import (
	"net/http"
	"reflect"
	"testing"

	"canvas-contract/runner"
)

type retentionSnapshot struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
}

type retentionHistory struct {
	Snapshots       []retentionSnapshot `json:"snapshots"`
	CurrentRevision int                 `json:"currentRevision"`
}

func readRetentionHistory(test *testing.T, owner *runner.Client, canvasID string) retentionHistory {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/canvas-projects/"+canvasID+"/history", nil)
	return runner.DecodeData[retentionHistory](test, envelope)
}

func retainedCanvasContent(document map[string]any) map[string]any {
	content := make(map[string]any, len(document))
	for key, value := range document {
		if key != "revision" && key != "updatedAt" {
			content[key] = value
		}
	}
	return content
}

func runCanvasHistoryRetention(test *testing.T, owner, foreign *runner.Client, prefix string) {
	canvasID := "history-retention-" + prefix
	path := "/api/canvas-projects/" + canvasID
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Retention original", 0, false))
	original := readCanvasDocument(test, owner, canvasID)
	changed := readCanvasDocument(test, owner, canvasID)
	changed["title"] = "Retention changed"
	requireHTTPSuccess(test, owner, http.MethodPut, path, map[string]any{"project": changed})
	initial := readRetentionHistory(test, owner, canvasID)
	if initial.CurrentRevision != 2 || len(initial.Snapshots) != 1 || initial.Snapshots[0].Revision != 1 {
		test.Fatal("retention fixture did not capture exactly one original preimage")
	}
	seedID := initial.Snapshots[0].ID
	changed = readCanvasDocument(test, owner, canvasID)
	changed["title"] = "Retention rapidly changed again"
	requireHTTPSuccess(test, owner, http.MethodPut, path, map[string]any{"project": changed})
	coalesced := readRetentionHistory(test, owner, canvasID)
	if coalesced.CurrentRevision != 3 || !reflect.DeepEqual(coalesced.Snapshots, initial.Snapshots) {
		test.Fatal("rapid ordinary content writes did not preserve the observed history coalescing window")
	}
	for _, request := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, path + "/history/" + seedID},
		{http.MethodPost, path + "/history/" + seedID + "/restore"},
	} {
		requireHTTPError(test, foreign, request.method, request.path, map[string]int{"revision": 3}, http.StatusNotFound)
	}
	if !reflect.DeepEqual(readRetentionHistory(test, owner, canvasID), coalesced) {
		test.Fatal("foreign history requests changed the retained snapshots")
	}
	currentRevision := 3
	createdIdentities := map[string]bool{seedID: true}
	for step := 0; step < 20; step++ {
		before := readCanvasDocument(test, owner, canvasID)
		requireHTTPSuccess(test, owner, http.MethodPost, path+"/history/"+seedID+"/restore", map[string]int{"revision": currentRevision})
		currentRevision++
		document := readCanvasDocument(test, owner, canvasID)
		runner.RequireNumber(test, document["revision"], float64(currentRevision), "restored revision")
		if !reflect.DeepEqual(retainedCanvasContent(document), retainedCanvasContent(original)) {
			test.Fatal("retention restore did not preserve the complete original content and extensions")
		}
		history := readRetentionHistory(test, owner, canvasID)
		if history.CurrentRevision != currentRevision || len(history.Snapshots) != min(20, step+2) {
			test.Fatal("forced restore did not capture its preimage or maintain the observed twenty-snapshot bound")
		}
		identities := map[string]bool{}
		for index, snapshot := range history.Snapshots {
			wantedRevision := currentRevision - 1 - index
			if step < 19 && index == len(history.Snapshots)-1 {
				wantedRevision = 1
			}
			if snapshot.ID == "" || identities[snapshot.ID] || snapshot.Revision != wantedRevision {
				test.Fatal("retained history identities or descending committed revisions differ from the capture sequence")
			}
			identities[snapshot.ID] = true
		}
		newest := history.Snapshots[0]
		if createdIdentities[newest.ID] || newest.Reason != "before_restore" || newest.Title != before["title"] {
			test.Fatal("restore preimage reused an identity or captured the wrong title or reason")
		}
		createdIdentities[newest.ID] = true
		_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, path+"/history/"+newest.ID, nil)
		snapshot := runner.DecodeData[struct {
			Snapshot retentionSnapshot `json:"snapshot"`
			Project  map[string]any    `json:"project"`
		}](test, envelope)
		if snapshot.Snapshot != newest || !reflect.DeepEqual(snapshot.Project, before) {
			test.Fatal("readable history did not retain the complete pre-restore document")
		}
	}
	beforeDocument, beforeHistory := readCanvasDocument(test, owner, canvasID), readCanvasHistory(test, owner, canvasID)
	for _, request := range []struct {
		method   string
		path     string
		revision int
		status   int
	}{
		{http.MethodGet, path + "/history/" + seedID, currentRevision, http.StatusNotFound},
		{http.MethodPost, path + "/history/" + seedID + "/restore", currentRevision, http.StatusNotFound},
		{http.MethodPost, path + "/history/" + seedID + "/restore", currentRevision - 1, http.StatusConflict},
	} {
		requireHTTPError(test, owner, request.method, request.path, map[string]int{"revision": request.revision}, request.status)
		if !reflect.DeepEqual(readCanvasDocument(test, owner, canvasID), beforeDocument) || !reflect.DeepEqual(readCanvasHistory(test, owner, canvasID), beforeHistory) {
			test.Fatal("evicted or stale history access changed document content, revision or retained history")
		}
	}
	requireHTTPSuccess(test, owner, http.MethodDelete, path, nil)
	requireHTTPError(test, owner, http.MethodGet, path+"/history", nil, http.StatusNotFound)
}
