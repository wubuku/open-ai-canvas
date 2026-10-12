package runner

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"canvas-contract/fixtures"
)

func ImageFixture(test *testing.T) []byte {
	test.Helper()
	content, err := fixtures.PNG()
	if err != nil {
		test.Fatal(err)
	}
	return content
}

type TestUsers struct {
	A Credentials
	B Credentials
}

type Credentials struct {
	Username string
	Password string
}

func PrepareUsers(t *testing.T, config Config, admin *Client) TestUsers {
	t.Helper()
	if config.RegisterUsers {
		if config.UsernameA != "" || config.UsernameB != "" || config.PasswordA != "" || config.PasswordB != "" {
			t.Fatalf("do not combine CANVAS_CONTRACT_REGISTER_USERS=true with explicit user credentials")
		}
		bootstrap := Credentials{Username: contractUsername("admin", config.RunID), Password: "CanvasContract-Bootstrap-2026!"}
		userA := Credentials{Username: contractUsername("a", config.RunID), Password: "CanvasContract-A-2026!"}
		userB := Credentials{Username: contractUsername("b", config.RunID), Password: "CanvasContract-B-2026!"}
		register(t, admin, bootstrap, "Canvas Contract Bootstrap")
		createAdminUser(t, admin, userA)
		createAdminUser(t, admin, userB)
		return TestUsers{A: userA, B: userB}
	}
	if config.UsernameA == "" || config.PasswordA == "" || config.UsernameB == "" || config.PasswordB == "" {
		t.Fatalf("provide both user credentials or set CANVAS_CONTRACT_REGISTER_USERS=true")
	}
	if config.UsernameA == config.UsernameB {
		t.Fatal("user A and B must be different accounts")
	}
	return TestUsers{
		A: Credentials{Username: config.UsernameA, Password: config.PasswordA},
		B: Credentials{Username: config.UsernameB, Password: config.PasswordB},
	}
}

func contractUsername(role, runID string) string {
	digest := sha256.Sum256([]byte(runID))
	return fmt.Sprintf("canvas_%s_%x", role, digest[:6])
}

func register(t *testing.T, client *Client, credentials Credentials, displayName string) {
	t.Helper()
	response, envelope, err := client.Do(Context(t), http.MethodPost, "/api/auth/register", map[string]any{
		"username":      credentials.Username,
		"displayName":   displayName,
		"password":      credentials.Password,
		"acceptedTerms": true,
	}, nil)
	if err != nil {
		t.Fatalf("register test user: %v", err)
	}
	RequireSuccess(t, response, envelope)
}

func createAdminUser(t *testing.T, admin *Client, credentials Credentials) {
	t.Helper()
	response, envelope, err := admin.Do(Context(t), http.MethodPost, "/api/admin/users", map[string]any{
		"username":    credentials.Username,
		"displayName": "Canvas Contract User",
		"password":    credentials.Password,
		"role":        "user",
		"status":      "active",
	}, nil)
	if err != nil {
		t.Fatalf("create second test user through admin HTTP API: %v", err)
	}
	RequireSuccess(t, response, envelope)
}

func login(t *testing.T, client *Client, credentials Credentials) {
	t.Helper()
	response, envelope, err := client.Do(Context(t), http.MethodPost, "/api/auth/login", map[string]string{
		"username": credentials.Username,
		"password": credentials.Password,
	}, nil)
	if err != nil {
		t.Fatalf("login %s: %v", client.Name, err)
	}
	RequireSuccess(t, response, envelope)
}

func Logout(t *testing.T, client *Client) {
	t.Helper()
	response, envelope, err := client.Do(Context(t), http.MethodPost, "/api/auth/logout", nil, nil)
	if err != nil {
		t.Fatalf("logout %s: %v", client.Name, err)
	}
	RequireSuccess(t, response, envelope)
}

func expectedConflictReason(envelope Envelope) string {
	if strings.TrimSpace(envelope.Reason) != "" {
		return envelope.Reason
	}
	return "<missing>"
}

func FixtureProject(id, title string, revision int, large bool) map[string]any {
	metadata := map[string]any{
		"contractUnknown": map[string]any{"preserve": true, "label": "wire-extension"},
		"prompt":          "contract fixture prompt",
	}
	if large {
		metadata["largeExtension"] = strings.Repeat("x", 4096)
	}
	return map[string]any{
		"id":       id,
		"revision": revision,
		"title":    title,
		"nodes": []any{
			map[string]any{
				"id":       "node-contract-1",
				"type":     "image",
				"title":    "Contract node",
				"position": map[string]any{"x": 0, "y": 0},
				"width":    320,
				"height":   240,
				"metadata": metadata,
			},
		},
		"connections": []any{},
		"customExtension": map[string]any{
			"nested": true,
			"source": "contract-suite",
		},
		"viewport":          map[string]any{"x": 999, "y": 12, "k": 2},
		"remoteContentHash": "client-only-hash",
	}
}
