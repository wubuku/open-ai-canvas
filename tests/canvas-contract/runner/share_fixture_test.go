package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExpireShareUsesPrivateFixtureContract(t *testing.T) {
	var authorization, body string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		if request.Method != http.MethodPost || request.URL.Path != "/expire-share" {
			t.Error("fixture request did not use the private POST contract")
		}
		buffer := make([]byte, 128)
		read, _ := request.Body.Read(buffer)
		body = string(buffer[:read])
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"expired":true,"projectId":"project-1"}`))
	}))
	defer server.Close()
	config := Config{FixtureURL: server.URL, FixtureToken: "fixture-token-with-at-least-24-bytes", Timeout: 2 * time.Second}
	if err := ExpireShare(context.Background(), config, "project-1"); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer "+config.FixtureToken || !strings.Contains(body, `"projectId":"project-1"`) {
		t.Fatalf("fixture request leaked or omitted its scoped authorization: auth=%q body=%q", authorization, body)
	}
}

func TestShareConfigRequiresPrivateExpiryFixture(t *testing.T) {
	t.Setenv("CANVAS_CONTRACT_BASE_URL", "http://127.0.0.1:18183")
	t.Setenv("CANVAS_CONTRACT_PROFILE", "share-public")
	t.Setenv("CANVAS_CONTRACT_REGISTER_USERS", "true")
	t.Setenv("CANVAS_CONTRACT_FIXTURE_URL", "http://example.test:1234")
	t.Setenv("CANVAS_CONTRACT_FIXTURE_TOKEN", "fixture-token-with-at-least-24-bytes")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("share profile accepted a non-loopback expiry fixture")
	}
	t.Setenv("CANVAS_CONTRACT_FIXTURE_URL", "http://127.0.0.1:18185")
	if _, err := ConfigFromEnv(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseFixtureConfigRejectsCredentials(t *testing.T) {
	t.Setenv("CANVAS_CONTRACT_BASE_URL", "http://127.0.0.1:18183")
	t.Setenv("CANVAS_CONTRACT_PROFILE", "core")
	t.Setenv("CANVAS_CONTRACT_REGISTER_USERS", "true")
	t.Setenv("CANVAS_CONTRACT_FIXTURE_TOKEN", "fixture-token-with-at-least-24-bytes")
	t.Setenv("CANVAS_CONTRACT_FIXTURE_URL", "http://fixture-user:fixture-password@127.0.0.1:18185")
	if _, err := ConfigFromEnv(); err == nil || strings.Contains(err.Error(), "fixture-password") {
		t.Fatal("database fixture accepted credentials or exposed them in an error")
	}
}
