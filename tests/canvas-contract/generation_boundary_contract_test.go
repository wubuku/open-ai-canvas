package canvas_contract_test

import (
	"net/http"
	"testing"

	"canvas-contract/runner"
)

func runGenerationBoundaryContract(test *testing.T, config runner.Config) {
	admin, err := runner.NewClient("admin", config)
	if err != nil {
		test.Fatal(err)
	}
	users := runner.PrepareUsers(test, config, admin)

	owner, err := runner.NewClient("generation-owner", config)
	if err != nil {
		test.Fatal(err)
	}
	foreign, err := runner.NewClient("generation-foreign", config)
	if err != nil {
		test.Fatal(err)
	}
	login(test, owner, users.A)
	login(test, foreign, users.B)

	missingID := "missing-generation-boundary-" + config.RunID
	boundaryRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/tasks"},
		{http.MethodGet, "/api/tasks/" + missingID},
		{http.MethodGet, "/api/tasks/" + missingID + "/logs"},
		{http.MethodGet, "/api/creation-runs"},
		{http.MethodGet, "/api/creation-runs/" + missingID},
		{http.MethodGet, "/api/agent/capabilities"},
		{http.MethodGet, "/api/agent/skills/usage"},
		{http.MethodGet, "/api/agent/profile"},
		{http.MethodGet, "/api/agent/runs/" + missingID},
	}

	test.Run("generation.boundary.unauthenticated", func(test *testing.T) {
		anonymous, err := runner.NewClient("generation-anonymous", config)
		if err != nil {
			test.Fatal(err)
		}
		for _, route := range boundaryRoutes {
			response, envelope, err := anonymous.Do(runner.Context(test), route.method, route.path, nil, nil)
			if err != nil {
				test.Fatal(err)
			}
			runner.RequireError(test, response, envelope, http.StatusUnauthorized, "")
		}
	})

	var creationRunID string
	test.Run("generation.boundary.owner-read-only", func(test *testing.T) {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/creation-runs", map[string]any{
			"clientKey": "generation-boundary-" + config.RunID,
			"state":     map[string]any{"purpose": "http-boundary"},
		}, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		created := runner.DecodeData[struct {
			Run struct {
				ID string `json:"id"`
			} `json:"run"`
		}](test, envelope)
		creationRunID = created.Run.ID
		if creationRunID == "" {
			test.Fatal("creation run setup returned no id")
		}

		for _, route := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/tasks"},
			{http.MethodGet, "/api/tasks/" + missingID},
			{http.MethodGet, "/api/tasks/" + missingID + "/logs"},
			{http.MethodGet, "/api/creation-runs"},
			{http.MethodGet, "/api/agent/capabilities"},
			{http.MethodGet, "/api/agent/skills/usage"},
			{http.MethodGet, "/api/agent/profile"},
			{http.MethodGet, "/api/agent/runs/" + missingID},
		} {
			response, envelope, err := owner.Do(runner.Context(test), route.method, route.path, nil, nil)
			if err != nil {
				test.Fatal(err)
			}
			if route.path == "/api/tasks/"+missingID || route.path == "/api/agent/runs/"+missingID {
				runner.RequireError(test, response, envelope, http.StatusNotFound, "")
				continue
			}
			runner.RequireSuccess(test, response, envelope)
		}
		response, envelope, err = owner.Do(runner.Context(test), http.MethodGet, "/api/creation-runs/"+creationRunID, nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
	})

	test.Run("generation.boundary.cross-user-creation-read", func(test *testing.T) {
		if creationRunID == "" {
			test.Fatal("owner setup did not create a creation run")
		}
		response, envelope, err := foreign.Do(runner.Context(test), http.MethodGet, "/api/creation-runs/"+creationRunID, nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusNotFound, "")
	})
}
