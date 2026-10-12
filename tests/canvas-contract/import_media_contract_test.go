package canvas_contract_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"canvas-contract/fixtures"
	"canvas-contract/harness"
	"canvas-contract/runner"
)

func richImportConversion(test *testing.T, owner *runner.Client, canvasID, provider string) importedCanvasNodes {
	test.Helper()
	response, envelope := fixedImportRequest(test, owner, canvasID, provider, "rich-media")
	runner.RequireSuccess(test, response, envelope)
	converted := runner.DecodeData[importedCanvasNodes](test, envelope)
	requireRichImportConversion(test, provider, converted)
	return converted
}

func requireRichImportConversion(test *testing.T, provider string, result importedCanvasNodes) {
	test.Helper()
	if result.BatchID == "" || result.ImportedNodeCount != 6 || len(result.Nodes) != 6 || result.ImportedConnectionCount != 3 || len(result.Connections) != 3 || len(result.SkippedNodes) != 5 || len(result.SkippedConnections) != 3 || result.MultiResultNodeCount != 1 || result.ReusedFailedNodeCount != 1 || result.PlaceholderNodeCount != 2 {
		test.Fatalf("rich %s conversion changed node/edge/filter/placeholder counters", provider)
	}
	warningCount := 2
	if provider == "libtv" {
		warningCount = 3
		if result.ConvertedSpecialCount != 1 || result.StaleNodeCount != 1 {
			test.Fatal("LibTV style/stale conversion counters")
		}
	}
	if len(result.Warnings) != warningCount {
		test.Fatalf("rich %s conversion warnings = %d, want %d", provider, len(result.Warnings), warningCount)
	}
	identity, err := harness.ImportFixtureID(provider, "rich-media")
	if err != nil {
		test.Fatal(err)
	}
	seen := make(map[string]bool)
	for index, node := range result.Nodes {
		if node.ID == "" || seen[node.ID] || node.Title == "" || node.X != 12.5 || node.Y != -34.5 || node.Metadata["provider"] != provider || node.Metadata["batchId"] != result.BatchID {
			test.Fatal("converted node identity, position or provenance")
		}
		seen[node.ID] = true
		if index == 4 {
			if node.Width != 480 || node.Height != 300 || node.Content != "" || node.Status != "error" || node.ErrorDetails != "controlled placeholder failure" {
				test.Fatal("failed placeholder/default dimensions")
			}
		} else if node.Width != 640 || node.Height != 360 {
			test.Fatal("valid measured dimensions were not preserved")
		}
		identityField, sourceField := "shareId", "nodeId"
		if provider == "libtv" {
			identityField, sourceField = "projectUuid", "nodeKey"
		}
		if node.Metadata[identityField] != identity || node.Metadata[sourceField] == "" || node.Metadata[sourceField] == nil {
			test.Fatal("source identity provenance")
		}
	}
	picture, video := result.Nodes[0], result.Nodes[1]
	if picture.Type != "image" || picture.Content != fixtures.ImportMediaOrigin+"/contract/image.png" || picture.Prompt != "image prompt" || picture.Model != "fixture-image-model" || picture.NaturalWidth != 3 || picture.NaturalHeight != 2 || picture.MimeType != "image/png" || picture.Status != "success" {
		test.Fatal("image result selection, metadata or parameters")
	}
	if video.Type != "video" || video.Content != fixtures.ImportMediaOrigin+"/contract/video.mp4" || video.NaturalWidth != 16 || video.NaturalHeight != 16 || video.DurationMs != 40 || video.MimeType != "video/mp4" || video.Status != "success" {
		test.Fatal("video media/duration or failed-task historical result reuse")
	}
	if result.Nodes[5].Content != "" || result.Nodes[5].Status != "idle" {
		test.Fatal("unsafe media URL must not enter node content")
	}
	for _, issue := range append(result.SkippedNodes, result.SkippedConnections...) {
		if reason, ok := issue["reason"].(string); !ok || strings.TrimSpace(reason) == "" {
			test.Fatal("skipped node/connection lacks a reason")
		}
	}
	secondTarget := result.Nodes[3].ID
	lastSource := result.Nodes[3].ID
	if provider == "tapnow" {
		audio := result.Nodes[2]
		if audio.Type != "audio" || audio.Content != fixtures.ImportMediaOrigin+"/contract/audio.wav" || audio.MimeType != "audio/wav" || audio.DurationMs != 10 || audio.Metadata["nodeId"] != "103" || audio.Metadata["sourceType"] != "audio" {
			test.Fatal("audio short-id/data-type fallback or metadata URL/duration")
		}
		if result.Nodes[3].Type != "text" || result.Nodes[3].Content != "fallback text prompt" || result.Nodes[3].MimeType != "text/plain" || picture.Size != "3:2" || picture.Quality != "2k" || video.Seconds != "0.04" || video.VQuality != "720p" || video.GenerateAudio != "false" {
			test.Fatal("text fallback or provider parameter normalization")
		}
		if result.Connections[0].FromHandleID != "output" || result.Connections[0].ToHandleID != "image" || result.Connections[1].ToHandleID != "explicit-input" {
			test.Fatal("short-ID edges, valueKey fallback or explicit handle precedence")
		}
		secondTarget = audio.ID
	} else {
		style := result.Nodes[3]
		if result.Nodes[2].Status != "success" || style.Type != "image" || style.Content != picture.Content || style.Metadata["sourceType"] != "material-style" || style.Metadata["styleAssetUuid"] != "fixture-style-asset" || style.Metadata["styleVersionUuid"] != "fixture-style-version" || style.Metadata["styleName"] != "Fixture style" {
			test.Fatal("historical failed result or material-style fallback provenance")
		}
	}
	wantEdges := [][2]string{{picture.ID, video.ID}, {video.ID, secondTarget}, {lastSource, result.Nodes[4].ID}}
	edges := make(map[string]bool)
	for index, edge := range result.Connections {
		if edge.ID == "" || edges[edge.ID] || edge.FromNodeID != wantEdges[index][0] || edge.ToNodeID != wantEdges[index][1] {
			test.Fatal("converted topology must contain only valid, newly identified endpoints")
		}
		edges[edge.ID] = true
	}
}

func runRichImportConversionMatrix(test *testing.T, owner *runner.Client, canvasID string) {
	before := importState(test, owner, canvasID)
	for _, provider := range []string{"libtv", "tapnow"} {
		first := richImportConversion(test, owner, canvasID, provider)
		repeated := richImportConversion(test, owner, canvasID, provider)
		if repeated.BatchID == first.BatchID {
			test.Fatal("pure conversion replay reused the batch identity")
		}
		for index, node := range repeated.Nodes {
			if node.ID == first.Nodes[index].ID || node.Content != first.Nodes[index].Content {
				test.Fatal("conversion replay must allocate new identities with stable source content")
			}
		}
		requireImportState(test, owner, canvasID, before)
	}
}

func importedNodeProject(node importedCanvasNode, resourceID string) map[string]any {
	metadata := map[string]any{
		"content": node.Content, "prompt": node.Prompt, "model": node.Model, "status": node.Status, "errorDetails": node.ErrorDetails,
		"naturalWidth": node.NaturalWidth, "naturalHeight": node.NaturalHeight, "durationMs": node.DurationMs, "mimeType": node.MimeType,
		"importSource": node.Metadata, "size": node.Size, "quality": node.Quality, "seconds": node.Seconds, "vquality": node.VQuality, "generateAudio": node.GenerateAudio,
	}
	if resourceID != "" {
		metadata["storageKey"] = "resource:" + resourceID
		metadata["assetId"] = resourceID
		metadata["content"] = "/api/resources/" + resourceID + "/file"
	}
	return map[string]any{"id": node.ID, "type": node.Type, "title": node.Title, "position": map[string]any{"x": node.X, "y": node.Y}, "width": node.Width, "height": node.Height, "metadata": metadata}
}

func runImportResourceSaveMatrix(test *testing.T, owner, foreign *runner.Client, runID string) {
	for _, provider := range []string{"libtv", "tapnow"} {
		canvasID := "import-media-" + provider + "-" + runID
		foreignID := "foreign-" + canvasID
		createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Imported media", 0, false))
		createCanvas(test, foreign, foreignID, runner.FixtureProject(foreignID, "Foreign media", 0, false))
		before := importState(test, owner, canvasID)
		foreignBefore := importState(test, foreign, foreignID)
		converted := richImportConversion(test, owner, canvasID, provider)
		requireImportState(test, owner, canvasID, before)
		project := runner.FixtureProject(canvasID, "Imported media saved", 1, false)
		nodes := project["nodes"].([]any)
		resources := make(map[string][]byte)
		assets := make(map[string]map[string]any)
		var addedBytes int64
		for _, node := range converted.Nodes {
			resourceID := ""
			if node.Type != "text" && node.Content != "" {
				path := strings.TrimPrefix(node.Content, fixtures.ImportMediaOrigin)
				content, mime, err := fixtures.ImportMedia(path)
				if err != nil {
					test.Fatal(err)
				}
				if path == "/contract/audio.wav" {
					mime = "audio/wav"
				}
				key := "media-" + provider + "-" + node.ID
				resource := importResource(test, owner, node.Content, key)
				if resource.Kind != node.Type || resource.MimeType != mime || resource.Size != int64(len(content)) {
					test.Fatalf("media resource kind/MIME/bytes = %s/%s/%d, want %s/%s/%d", resource.Kind, resource.MimeType, resource.Size, node.Type, mime, len(content))
				}
				state := resourceState(test, owner)
				if replayed := importResource(test, owner, node.Content, key); replayed != resource {
					test.Fatal("media resource replay changed the persisted identity/metadata")
				}
				requireResourceState(test, owner, state)
				requireResourceBytes(test, owner, resource.ID, content)
				resources[resource.ID] = content
				asset := legacyAssetPayload(resource.ID, node.Title, len(content))
				asset["kind"] = node.Type
				data := asset["data"].(map[string]any)
				data["mimeType"] = resource.MimeType
				data["width"], data["height"], data["durationMs"] = node.NaturalWidth, node.NaturalHeight, node.DurationMs
				assets[resource.ID] = asset
				resourceID = resource.ID
				addedBytes += resource.Size
				requireHTTPError(test, foreign, http.MethodGet, "/api/resources/"+resource.ID, nil, http.StatusNotFound)
				response, err := foreign.DoRaw(runner.Context(test), http.MethodGet, "/api/resources/"+resource.ID+"/file", nil, "", nil)
				if err != nil {
					test.Fatal(err)
				}
				runner.RequireStatus(test, response, http.StatusNotFound)
			}
			nodes = append(nodes, importedNodeProject(node, resourceID))
		}
		project["nodes"] = nodes
		project["connections"] = converted.Connections
		state := resourceState(test, owner)
		if len(state.IDs) != len(before.Resources.IDs)+len(resources) || state.UsedBytes != before.Resources.UsedBytes+addedBytes {
			test.Fatal("media import/replay must charge only the committed resource bytes once")
		}
		if got := importState(test, owner, canvasID); !bytes.Equal(importJSON(test, got.Canvas), importJSON(test, before.Canvas)) || !bytes.Equal(importJSON(test, got.History), importJSON(test, before.History)) {
			test.Fatal("resource import must not implicitly save the converted canvas/history")
		}
		headers := http.Header{"X-Idempotency-Key": []string{"missing-media-" + canvasID}}
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/resources/import", map[string]any{"url": fixtures.ImportMediaOrigin + "/contract/missing.mp4", "kind": "video"}, headers)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "internal")
		requireResourceState(test, owner, state)
		recovered := importResource(test, owner, fixtures.ImportMediaOrigin+"/contract/image.png", "missing-media-"+canvasID)
		recoveredBytes, _, err := fixtures.ImportMedia("/contract/image.png")
		if err != nil {
			test.Fatal(err)
		}
		if recovered.Kind != "image" || recovered.MimeType != "image/png" || recovered.Size != int64(len(recoveredBytes)) {
			test.Fatal("explicit retry after a proven download failure did not recover")
		}
		recoveredState := resourceState(test, owner)
		if len(recoveredState.IDs) != len(state.IDs)+1 || recoveredState.UsedBytes != state.UsedBytes+recovered.Size {
			test.Fatal("recovery must preserve staged resources and charge only the new commit")
		}
		if replayed := importResource(test, owner, fixtures.ImportMediaOrigin+"/contract/image.png", "missing-media-"+canvasID); replayed != recovered {
			test.Fatal("recovered resource replay changed its identity")
		}
		requireResourceState(test, owner, recoveredState)
		resources[recovered.ID] = recoveredBytes
		staged := importState(test, owner, canvasID)
		response, envelope, err = owner.Do(runner.Context(test), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": project}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		requireImportState(test, owner, canvasID, staged)
		for resourceID, asset := range assets {
			upsertAsset(test, owner, resourceID, asset)
		}
		response, envelope, err = owner.Do(runner.Context(test), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": project}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		saved := importState(test, owner, canvasID)
		runner.RequireNumber(test, saved.Canvas["revision"], 2, "imported canvas revision")
		for _, field := range []string{"nodes", "connections", "customExtension"} {
			if !bytes.Equal(importJSON(test, saved.Canvas[field]), importJSON(test, project[field])) {
				test.Fatalf("save/reopen lost imported %s", field)
			}
		}
		if bytes.Equal(importJSON(test, saved.History), importJSON(test, before.History)) {
			test.Fatal("successful imported canvas save did not create its preimage")
		}
		response, envelope, err = owner.Do(runner.Context(test), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": project}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusConflict, "conflict")
		requireImportState(test, owner, canvasID, saved)
		foreignProject := runner.FixtureProject(foreignID, "Rejected imported media", 1, false)
		foreignProject["nodes"] = project["nodes"]
		foreignProject["connections"] = project["connections"]
		response, envelope, err = foreign.Do(runner.Context(test), http.MethodPut, "/api/canvas-projects/"+foreignID, map[string]any{"project": foreignProject}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusConflict, "")
		requireImportState(test, foreign, foreignID, foreignBefore)
		requireHTTPError(test, foreign, http.MethodGet, "/api/canvas-projects/"+canvasID, nil, http.StatusNotFound)
		requireImportState(test, owner, canvasID, saved)
		for resourceID, content := range resources {
			requireResourceBytes(test, owner, resourceID, content)
		}
	}
}

func importJSON(test *testing.T, value any) []byte {
	test.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		test.Fatal(err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		test.Fatal(err)
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		test.Fatal(err)
	}
	return canonical
}

func TestImportedMediaProjectMapping(test *testing.T) {
	node := importedCanvasNode{ID: "image", Type: "image", X: 12.5, Y: -34.5, Width: 640, Height: 360, Content: fixtures.ImportMediaOrigin + "/contract/image.png", Metadata: map[string]any{"provider": "tapnow", "nodeId": "source", "batchId": "batch"}}
	projectNode := importedNodeProject(node, "owned-resource")
	metadata := projectNode["metadata"].(map[string]any)
	if metadata["storageKey"] != "resource:owned-resource" || metadata["assetId"] != "owned-resource" || metadata["content"] != "/api/resources/owned-resource/file" || !bytes.Equal(importJSON(test, metadata["importSource"]), importJSON(test, node.Metadata)) {
		test.Fatal("resource mapping lost its locator or conversion provenance")
	}
	placeholder := importedNodeProject(importedCanvasNode{ID: "placeholder", Type: "video", Status: "error"}, "")
	if _, exists := placeholder["metadata"].(map[string]any)["assetId"]; exists {
		test.Fatal("placeholder must not acquire an empty or invented resource identity")
	}
	connection := importedCanvasConnection{ID: "edge", FromNodeID: "image", ToNodeID: "placeholder"}
	var generic map[string]any
	if err := json.Unmarshal(importJSON(test, connection), &generic); err != nil {
		test.Fatal(err)
	}
	if !bytes.Equal(importJSON(test, connection), importJSON(test, generic)) {
		test.Fatal("semantic JSON comparison must not depend on struct/map key order")
	}
}
