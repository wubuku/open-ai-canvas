package harness

import (
	"encoding/json"
	"fmt"
	"strings"

	"canvas-contract/fixtures"
)

func richImportFixtureBody(provider, identity string) []byte {
	var detail map[string]any
	if provider == "libtv" {
		detail = richLibTVFixture(identity)
	} else {
		detail = richTapNowFixture(identity)
	}
	encoded, _ := json.Marshal(map[string]any{"code": 0, "msg": "controlled rich media fixture", "data": detail})
	return encoded
}

func richLibTVFixture(identity string) map[string]any {
	picture, video := fixtures.ImportMediaOrigin+"/contract/image.png", fixtures.ImportMediaOrigin+"/contract/video.mp4"
	imageNode := libTVFixtureNode("fixture-image", "image", map[string]any{
		"url":           []string{"http://files.tapnow.media/blocked.png", "https://credential@files.tapnow.media/blocked.png", picture, picture},
		"params":        map[string]any{"prompt": " image prompt ", "model": " fixture-image-model "},
		"_resourceMeta": map[string]any{"items": []any{map[string]any{}, map[string]any{}, map[string]any{"width": 3, "height": 2}}},
	})
	imageNode["isStale"] = true
	videoNode := libTVFixtureNode("fixture-video", "video", map[string]any{
		"url": []string{video}, "_resourceMeta": map[string]any{"items": []any{map[string]any{"width": 16, "height": 16, "durationSec": 0.040}}},
	})
	reused := libTVFixtureNode("fixture-reused", "image", map[string]any{"url": []string{picture}, "taskInfo": map[string]any{"status": 3, "failedReason": "latest attempt failed"}})
	style := libTVFixtureNode("fixture-style", "material-style", map[string]any{"coverUrl": picture, "styleAssetUuid": "fixture-style-asset", "styleVersionUuid": "fixture-style-version", "styleName": "Fixture style"})
	failed := libTVFixtureNode("fixture-failed", "video", map[string]any{})
	failed["taskInfo"] = map[string]any{"status": 3, "failedReason": "controlled placeholder failure"}
	failed["measured"] = map[string]any{"width": "0", "height": "NaN"}
	unsafe := libTVFixtureNode("fixture-unsafe", "image", map[string]any{"url": []string{"http://files.tapnow.media/not-allowed", "https://credential@files.tapnow.media/not-allowed"}})
	invalidPosition := libTVFixtureNode("fixture-position", "image", map[string]any{"url": []string{picture}})
	invalidPosition["position"] = map[string]any{"positionX": "NaN", "positionY": "1"}
	return map[string]any{
		"projectMeta": map[string]any{"uuid": identity, "name": "Contract LibTV rich media", "effective": map[string]any{"canRead": true, "canCopy": true}},
		"nodeList":    []any{imageNode, videoNode, reused, style, failed, unsafe, imageNode, libTVFixtureNode("", "image", nil), map[string]any{"nodeKey": "fixture-bad-json", "data": "{"}, libTVFixtureNode("fixture-unsupported", "unsupported", nil), invalidPosition},
		"connectionList": []any{
			map[string]any{"connectionId": "fixture-image-video", "source": "fixture-image", "target": "fixture-video"},
			map[string]any{"connectionId": "fixture-video-style", "source": "fixture-video", "target": "fixture-style"},
			map[string]any{"connectionId": "fixture-style-failed", "source": "fixture-style", "target": "fixture-failed"},
			map[string]any{"connectionId": "fixture-dangling", "source": "fixture-unsupported", "target": "fixture-video"},
			map[string]any{"connectionId": "fixture-image-video", "source": "fixture-image", "target": "fixture-video"},
			map[string]any{"source": "fixture-image", "target": "fixture-video"},
		},
	}
}

func libTVFixtureNode(identity, kind string, data map[string]any) map[string]any {
	if data == nil {
		data = make(map[string]any)
	}
	data["type"] = kind
	encoded, _ := json.Marshal(data)
	return map[string]any{"nodeKey": identity, "name": identity, "data": string(encoded), "position": map[string]any{"positionX": "12.5", "positionY": "-34.5"}, "measured": map[string]any{"width": "640", "height": "360"}}
}

func richTapNowFixture(identity string) map[string]any {
	picture, video, audio := fixtures.ImportMediaOrigin+"/contract/image.png", fixtures.ImportMediaOrigin+"/contract/video.mp4", fixtures.ImportMediaOrigin+"/contract/audio.wav"
	imageNode := tapNowFixtureNode("fixture-image", "image", map[string]any{
		"options": []string{"https://not-allowed.invalid/image.png", picture, picture},
		"prompt":  " image prompt ", "params": map[string]any{"model": "fixture-image-model", "aspectRatio": "3:2", "imageSize": "2K"},
		"__metadata": map[string]any{"width": 3, "height": 2},
	})
	imageNode["short_id"] = 101
	videoNode := tapNowFixtureNode("fixture-video", "video", map[string]any{
		"src": video, "params": map[string]any{"duration": 0.040, "resolution": "720p", "generateAudio": false},
		"taskInfo": map[string]any{"status": "failed"}, "__metadata": map[string]any{"width": 16, "height": 16, "duration": 0.040},
	})
	encoded, _ := json.Marshal(videoNode["data"])
	videoNode["data"] = string(encoded)
	videoNode["short_id"] = 102
	audioNode := tapNowFixtureNode("", "", map[string]any{"type": "audio", "__metadata": map[string]any{"url": audio, "duration": 0.010}})
	audioNode["short_id"] = 103
	text := tapNowFixtureNode("fixture-text", "text", map[string]any{"text": "   ", "prompt": " fallback text prompt "})
	failed := tapNowFixtureNode("fixture-failed", "video", map[string]any{"taskInfo": map[string]any{"status": "error", "message": "controlled placeholder failure"}})
	failed["measured"] = map[string]any{"width": 0, "height": -1}
	unsafe := tapNowFixtureNode("fixture-unsafe", "image", map[string]any{"src": "http://files.tapnow.media/blocked.png", "options": []string{"https://credential@files.tapnow.media/blocked.png", "https://not-allowed.invalid/blocked.png"}})
	deleted := tapNowFixtureNode("fixture-deleted", "text", map[string]any{"text": "must be ignored"})
	deleted["deleted_at"] = "2026-01-01T00:00:00Z"
	return map[string]any{
		"id": identity, "name": "Contract TapNow rich media",
		"nodes": []any{imageNode, videoNode, audioNode, text, failed, unsafe, imageNode, tapNowFixtureNode("", "text", nil), tapNowFixtureNode("fixture-bad-json", "text", "{"), tapNowFixtureNode("fixture-unsupported", "unsupported", nil), tapNowFixtureNode(strings.Repeat("x", 129), "text", nil), deleted},
		"connections": []any{
			map[string]any{"id": "fixture-image-video", "source": "101", "target": "102", "sourceHandle": "output", "data": map[string]any{"valueKey": "image"}},
			map[string]any{"id": "fixture-video-audio", "source": "fixture-video", "target": "103", "targetHandle": "explicit-input", "data": map[string]any{"valueKey": "ignored-fallback"}},
			map[string]any{"id": "fixture-text-failed", "source": "fixture-text", "target": "fixture-failed"},
			map[string]any{"id": "fixture-dangling", "source": "fixture-deleted", "target": "fixture-video"},
			map[string]any{"id": "fixture-image-video", "source": "101", "target": "102"},
			map[string]any{"source": "101", "target": "102"},
			map[string]any{"id": "fixture-deleted-link", "source": "101", "target": "102", "deleted_at": "2026-01-01T00:00:00Z"},
		},
	}
}

func tapNowFixtureNode(identity, kind string, data any) map[string]any {
	return map[string]any{"id": identity, "type": kind, "data": data, "position": map[string]any{"x": 12.5, "y": -34.5}, "measured": map[string]any{"width": 640, "height": 360}}
}

func importLimitFixtureBody(identity, mode string) []byte {
	nodeCount, edgeCount := 2, 1
	switch mode {
	case "node-limit":
		nodeCount = 1000
	case "node-over-limit":
		nodeCount = 1001
	case "edge-limit":
		edgeCount = 5000
	case "edge-over-limit":
		edgeCount = 5001
	}
	nodes := make([]any, 0, nodeCount)
	for index := 0; index < nodeCount; index++ {
		nodes = append(nodes, tapNowFixtureNode(fmt.Sprintf("boundary-%d", index), "text", map[string]any{"text": "controlled boundary text"}))
	}
	edges := make([]any, 0, edgeCount)
	for index := 0; index < edgeCount; index++ {
		edges = append(edges, map[string]any{"id": fmt.Sprintf("boundary-edge-%d", index), "source": "boundary-0", "target": fmt.Sprintf("boundary-%d", nodeCount-1)})
	}
	encoded, _ := json.Marshal(map[string]any{"code": 0, "msg": "controlled conversion limit fixture", "data": map[string]any{"id": identity, "name": "Contract TapNow conversion limits", "nodes": nodes, "connections": edges}})
	return encoded
}
