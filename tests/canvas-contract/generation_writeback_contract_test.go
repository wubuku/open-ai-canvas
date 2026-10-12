package canvas_contract_test

import (
	"net/http"
	"testing"

	"canvas-contract/runner"
)

func runGenerationWritebackContract(test *testing.T, config runner.Config) {
	owner, foreign, admin := prepareClients(test, config)
	channelID, modelKey := createTextReplayModel(test, admin, config.RunID)

	anonymous, err := runner.NewClient("generation-writeback-anonymous", config)
	if err != nil {
		test.Fatal(err)
	}

	missingID := "missing-generation-writeback-" + config.RunID
	test.Run("generation.writeback.unauthenticated", func(test *testing.T) {
		for _, request := range []struct {
			method string
			path   string
			body   any
		}{
			{http.MethodPost, "/api/tasks", map[string]any{"type": "canvas_text", "prompt": "unauthenticated", "input": map[string]any{"mode": "text", "replay": true}}},
			{http.MethodPost, "/api/tasks/" + missingID + "/text-deltas", map[string]any{"content": "unauthenticated"}},
			{http.MethodGet, "/api/tasks/" + missingID + "/text-deltas", nil},
			{http.MethodPost, "/api/tasks/" + missingID + "/text-replay-complete", map[string]any{"text": "unauthenticated"}},
		} {
			response, envelope, err := anonymous.Do(runner.Context(test), request.method, request.path, request.body, nil)
			if err != nil {
				test.Fatal(err)
			}
			runner.RequireError(test, response, envelope, http.StatusUnauthorized, "")
		}
	})

	taskID := ""
	test.Run("generation.writeback.owner-foreign-cursor", func(test *testing.T) {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/tasks", map[string]any{
			"type":      "canvas_text",
			"operation": "text_generation",
			"prompt":    "generation writeback contract",
			"input": map[string]any{
				"mode":   "text",
				"replay": true,
				"config": map[string]any{"channelId": channelID, "model": modelKey},
			},
		}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		created := runner.DecodeData[struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}](test, envelope)
		taskID = created.ID
		if taskID == "" || created.Status != "text_replay" {
			test.Fatalf("unexpected replay task: %#v", created)
		}

		response, envelope, err = foreign.Do(runner.Context(test), http.MethodGet, "/api/tasks/"+taskID+"/text-deltas", nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "")
		response, envelope, err = foreign.Do(runner.Context(test), http.MethodPost, "/api/tasks/"+taskID+"/text-deltas", map[string]any{"content": "foreign"}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusInternalServerError, "")

		for _, content := range []string{"第一段", "第二段"} {
			response, envelope, err = owner.Do(runner.Context(test), http.MethodPost, "/api/tasks/"+taskID+"/text-deltas", map[string]any{"content": content}, nil)
			if err != nil {
				test.Fatal(err)
			}
			runner.RequireSuccess(test, response, envelope)
		}

		response, envelope, err = owner.Do(runner.Context(test), http.MethodGet, "/api/tasks/"+taskID+"/text-deltas?after=0", nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		all := runner.DecodeData[struct {
			Deltas []struct {
				Sequence int64  `json:"sequence"`
				Content  string `json:"content"`
			} `json:"deltas"`
			Complete bool   `json:"complete"`
			Status   string `json:"status"`
		}](test, envelope)
		if len(all.Deltas) != 2 || all.Deltas[0].Sequence != 1 || all.Deltas[1].Sequence != 2 || all.Complete || all.Status != "text_replay" {
			test.Fatalf("unexpected initial replay: %#v", all)
		}

		response, envelope, err = owner.Do(runner.Context(test), http.MethodGet, "/api/tasks/"+taskID+"/text-deltas?after=1", nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		tail := runner.DecodeData[struct {
			Deltas []struct {
				Sequence int64 `json:"sequence"`
			} `json:"deltas"`
		}](test, envelope)
		if len(tail.Deltas) != 1 || tail.Deltas[0].Sequence != 2 {
			test.Fatalf("cursor after=1 returned %#v", tail)
		}

		response, envelope, err = owner.Do(runner.Context(test), http.MethodGet, "/api/tasks/"+taskID+"/text-deltas?after=invalid", nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
	})

	test.Run("generation.writeback.completion-lifecycle", func(test *testing.T) {
		if taskID == "" {
			test.Fatal("owner setup did not create a replay task")
		}
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/tasks/"+taskID+"/text-replay-complete", map[string]any{"text": "第一段第二段"}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		completed := runner.DecodeData[struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}](test, envelope)
		if completed.ID != taskID || completed.Status != "succeeded" {
			test.Fatalf("unexpected completion: %#v", completed)
		}

		response, envelope, err = owner.Do(runner.Context(test), http.MethodGet, "/api/tasks/"+taskID+"/text-deltas?after=0", nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		replay := runner.DecodeData[struct {
			Deltas    []any  `json:"deltas"`
			Complete  bool   `json:"complete"`
			Status    string `json:"status"`
			FinalText string `json:"finalText"`
		}](test, envelope)
		if len(replay.Deltas) != 2 || !replay.Complete || replay.Status != "succeeded" || replay.FinalText != "第一段第二段" {
			test.Fatalf("unexpected completed replay: %#v", replay)
		}

		response, envelope, err = owner.Do(runner.Context(test), http.MethodPost, "/api/tasks/"+taskID+"/text-deltas", map[string]any{"content": "late"}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		response, envelope, err = owner.Do(runner.Context(test), http.MethodPost, "/api/tasks/"+taskID+"/text-replay-complete", map[string]any{"text": "重复完成"}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		response, envelope, err = foreign.Do(runner.Context(test), http.MethodPost, "/api/tasks/"+taskID+"/text-replay-complete", map[string]any{"text": "foreign"}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
	})
}

func createTextReplayModel(test *testing.T, admin *runner.Client, runID string) (string, string) {
	test.Helper()
	modelKey := "canvas-contract-text-" + runID
	response, envelope, err := admin.Do(runner.Context(test), http.MethodPost, "/api/admin/channels", map[string]any{
		"name":    "Canvas Contract Text " + runID,
		"baseUrl": "https://example.com/v1",
		"models":  []string{},
		"enabled": true,
	}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	channel := runner.DecodeData[struct {
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}](test, envelope)
	if channel.Channel.ID == "" {
		test.Fatal("admin channel fixture returned no id")
	}
	response, envelope, err = admin.Do(runner.Context(test), http.MethodPost, "/api/admin/channels/"+channel.Channel.ID+"/models", map[string]any{
		"modelKey":              modelKey,
		"providerModelKey":      modelKey,
		"displayName":           "Canvas Contract Text",
		"channelLabel":          "Canvas Contract",
		"capability":            "text",
		"protocol":              "chat-completion",
		"billingMode":           "fixed_request",
		"unitPriceMicrocredits": 0,
		"priceConfigured":       true,
		"capabilityConfig": map[string]any{
			"version": 1,
			"text": map[string]any{
				"streaming":           true,
				"contextWindowTokens": 128000,
				"maxOutputTokens":     16384,
				"references":          map[string]any{"promptMaxChars": 32000},
			},
		},
	}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	return channel.Channel.ID, modelKey
}
