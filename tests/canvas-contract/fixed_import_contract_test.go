package canvas_contract_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"canvas-contract/harness"
	"canvas-contract/runner"
)

type importedCanvasNodes struct {
	BatchID                 string                     `json:"batchId"`
	ImportedNodeCount       int                        `json:"importedNodeCount"`
	ImportedConnectionCount int                        `json:"importedConnectionCount"`
	MultiResultNodeCount    int                        `json:"multiResultNodeCount"`
	ReusedFailedNodeCount   int                        `json:"reusedFailedNodeCount"`
	PlaceholderNodeCount    int                        `json:"placeholderNodeCount"`
	ConvertedSpecialCount   int                        `json:"convertedSpecialCount"`
	StaleNodeCount          int                        `json:"staleNodeCount"`
	Nodes                   []importedCanvasNode       `json:"nodes"`
	Connections             []importedCanvasConnection `json:"connections"`
	SkippedNodes            []map[string]any           `json:"skippedNodes"`
	SkippedConnections      []map[string]any           `json:"skippedConnections"`
	Warnings                []map[string]any           `json:"warnings"`
}

type importedCanvasNode struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Content       string         `json:"content"`
	X             float64        `json:"x"`
	Y             float64        `json:"y"`
	Width         float64        `json:"width"`
	Height        float64        `json:"height"`
	Prompt        string         `json:"prompt"`
	Model         string         `json:"model"`
	Status        string         `json:"status"`
	ErrorDetails  string         `json:"errorDetails"`
	NaturalWidth  int            `json:"naturalWidth"`
	NaturalHeight int            `json:"naturalHeight"`
	DurationMs    int64          `json:"durationMs"`
	MimeType      string         `json:"mimeType"`
	Size          string         `json:"size"`
	Quality       string         `json:"quality"`
	Seconds       string         `json:"seconds"`
	VQuality      string         `json:"vquality"`
	GenerateAudio string         `json:"generateAudio"`
	Metadata      map[string]any `json:"metadata"`
}

type importedCanvasConnection struct {
	ID           string `json:"id"`
	FromNodeID   string `json:"fromNodeId"`
	ToNodeID     string `json:"toNodeId"`
	FromHandleID string `json:"fromHandleId"`
	ToHandleID   string `json:"toHandleId"`
}

type visibleImportState struct {
	Canvas    map[string]any
	History   map[string]any
	Resources visibleResourceState
}

func importState(test *testing.T, owner *runner.Client, canvasID string) visibleImportState {
	test.Helper()
	response, envelope, err := owner.Do(runner.Context(test), http.MethodGet, "/api/canvas-projects/"+canvasID, nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	canvas := runner.DecodeData[struct {
		Project map[string]any `json:"project"`
	}](test, envelope).Project
	response, envelope, err = owner.Do(runner.Context(test), http.MethodGet, "/api/canvas-projects/"+canvasID+"/history", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	return visibleImportState{Canvas: canvas, History: runner.DecodeData[map[string]any](test, envelope), Resources: resourceState(test, owner)}
}

func requireImportState(test *testing.T, owner *runner.Client, canvasID string, want visibleImportState) {
	test.Helper()
	if got := importState(test, owner, canvasID); !reflect.DeepEqual(got, want) {
		test.Fatal("conversion changed the full persisted canvas, history, resource identities or stored-byte usage")
	}
}

func fixedImportRequest(test *testing.T, client *runner.Client, canvasID, provider, mode string) (runner.Response, runner.Envelope) {
	test.Helper()
	identity, err := harness.ImportFixtureID(provider, mode)
	if err != nil {
		test.Fatal(err)
	}
	field := "shareId"
	if provider == "libtv" {
		field = "uuid"
	}
	response, envelope, err := client.Do(runner.Context(test), http.MethodPost, "/api/canvas-projects/"+canvasID+"/import/"+provider, map[string]string{field: identity}, nil)
	if err != nil {
		test.Fatal(err)
	}
	return response, envelope
}

func runImportScopeMatrix(test *testing.T, owner, foreign, admin *runner.Client, canvasID string) {
	before := importState(test, owner, canvasID)
	foreignBefore := resourceState(test, foreign)
	response, envelope := fixedImportRequest(test, owner, canvasID, "libtv", "success")
	runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
	anonymous, err := runner.NewClient("anonymous", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	for _, provider := range []string{"libtv", "tapnow"} {
		response, envelope = fixedImportRequest(test, anonymous, canvasID, provider, "success")
		runner.RequireError(test, response, envelope, http.StatusUnauthorized, "")
		response, envelope = fixedImportRequest(test, foreign, canvasID, provider, "success")
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
		response, envelope = fixedImportRequest(test, owner, "missing-"+canvasID, provider, "success")
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
	}
	response, envelope, err = admin.Do(runner.Context(test), http.MethodPatch, "/api/admin/settings/libtv", map[string]any{"enabled": true, "token": harness.ImportFixtureToken}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	requireImportState(test, owner, canvasID, before)
	requireResourceState(test, foreign, foreignBefore)
}

func runFixedImportSuccessMatrix(test *testing.T, owner *runner.Client, canvasID string) {
	before := importState(test, owner, canvasID)
	for _, provider := range []string{"libtv", "tapnow"} {
		var previous importedCanvasNodes
		for attempt := 0; attempt < 2; attempt++ {
			response, envelope := fixedImportRequest(test, owner, canvasID, provider, "success")
			runner.RequireSuccess(test, response, envelope)
			converted := runner.DecodeData[importedCanvasNodes](test, envelope)
			if converted.BatchID == "" || converted.ImportedNodeCount != 2 || len(converted.Nodes) != 2 || converted.ImportedConnectionCount != 1 || len(converted.SkippedNodes) != 1 {
				test.Fatal("controlled import did not preserve the successful partial-conversion shape")
			}
			if converted.Nodes[0].ID == "" || converted.Nodes[1].ID == "" || converted.Nodes[0].ID == converted.Nodes[1].ID {
				test.Fatal("converted node identities are empty or duplicated")
			}
			if len(converted.Connections) != 1 || converted.Connections[0].ID == "" || converted.Connections[0].FromNodeID != converted.Nodes[0].ID || converted.Connections[0].ToNodeID != converted.Nodes[1].ID {
				test.Fatal("converted connection must point to the converted source and target node identities")
			}
			if provider == "libtv" && converted.Nodes[0].Content != "https://fixture.invalid/first.png" || provider == "tapnow" && converted.Nodes[0].Content != "contract fixture first text" {
				test.Fatal("response is not from the controlled fixed-origin upstream")
			}
			if attempt > 0 && (converted.BatchID == previous.BatchID || converted.Nodes[0].ID == previous.Nodes[0].ID || converted.Nodes[0].Content != previous.Nodes[0].Content) {
				test.Fatal("Go repeated conversion must allocate a new batch/node identity while preserving source content")
			}
			previous = converted
			requireImportState(test, owner, canvasID, before)
		}
	}
}

func runFixedImportFailureMatrix(test *testing.T, owner *runner.Client, canvasID string) {
	before := importState(test, owner, canvasID)
	for _, provider := range []string{"libtv", "tapnow"} {
		modes := []string{"http-error", "business-error", "invalid-json", "oversized", "redirect", "truncated", "empty"}
		if provider == "libtv" {
			modes = append(modes, "no-copy", "no-read")
		}
		for _, mode := range modes {
			test.Logf("controlled import failure: provider=%s mode=%s", provider, mode)
			response, envelope := fixedImportRequest(test, owner, canvasID, provider, mode)
			runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
			requireImportState(test, owner, canvasID, before)
		}
	}
}

func runImportConversionLimitMatrix(test *testing.T, owner *runner.Client, canvasID string) {
	before := importState(test, owner, canvasID)
	for _, mode := range []string{"node-limit", "node-over-limit", "edge-limit", "edge-over-limit"} {
		response, envelope := fixedImportRequest(test, owner, canvasID, "tapnow", mode)
		if mode == "node-over-limit" {
			runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
		} else {
			runner.RequireSuccess(test, response, envelope)
			result := runner.DecodeData[importedCanvasNodes](test, envelope)
			wantNodes, wantEdges, wantWarnings := 2, 5000, 0
			if mode == "node-limit" {
				wantNodes, wantEdges = 1000, 1
			} else if mode == "edge-over-limit" {
				wantWarnings = 1
			}
			if len(result.Nodes) != wantNodes || result.ImportedNodeCount != wantNodes || len(result.Connections) != wantEdges || result.ImportedConnectionCount != wantEdges || len(result.SkippedNodes) != 0 || len(result.SkippedConnections) != 0 || len(result.Warnings) != wantWarnings {
				test.Fatalf("%s changed node/edge limit or truncation warning policy", mode)
			}
			seen := make(map[string]bool)
			for index, node := range result.Nodes {
				if node.ID == "" || seen[node.ID] || node.Type != "text" || node.Content != "controlled boundary text" || node.Metadata["nodeId"] != fmt.Sprintf("boundary-%d", index) {
					test.Fatal("conversion limit fixture lost/duplicated/reordered a node")
				}
				seen[node.ID] = true
			}
			seenEdges := make(map[string]bool)
			for _, edge := range result.Connections {
				if edge.ID == "" || seenEdges[edge.ID] || edge.FromNodeID != result.Nodes[0].ID || edge.ToNodeID != result.Nodes[len(result.Nodes)-1].ID {
					test.Fatal("conversion limit fixture lost edge identity or endpoints")
				}
				seenEdges[edge.ID] = true
			}
		}
		requireImportState(test, owner, canvasID, before)
	}
}

func runFixedImportTimeoutMatrix(test *testing.T, owner *runner.Client, canvasID string) {
	before := importState(test, owner, canvasID)
	for _, provider := range []string{"libtv", "tapnow"} {
		started := time.Now()
		response, envelope := fixedImportRequest(test, owner, canvasID, provider, "timeout")
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
		if elapsed := time.Since(started); elapsed < 18*time.Second || elapsed >= owner.Config.Timeout {
			test.Fatal("slow fixture did not reach the target upstream deadline before the runner deadline")
		}
		requireImportState(test, owner, canvasID, before)
	}
}
