package runner

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSetCanvasUpdatedAtUsesScopedFixtureRequest(t *testing.T) {
	var authorization, body string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		if request.Method != http.MethodPost || request.URL.Path != "/set-canvas-updated-at" {
			t.Errorf("fixture request did not use the private POST contract: %s %s", request.Method, request.URL.Path)
		}
		body = readRequestBody(t, request)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"updated":2,"updatedAt":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	config := Config{FixtureURL: server.URL, FixtureToken: "fixture-token-with-at-least-24-bytes", Timeout: 2 * time.Second}
	err := SetCanvasUpdatedAt(context.Background(), config, []string{"canvas-a", "canvas-b"}, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer "+config.FixtureToken || !strings.Contains(body, `"projectIds":["canvas-a","canvas-b"]`) || !strings.Contains(body, `"updatedAt":"2026-01-01T00:00:00Z"`) {
		t.Fatalf("fixture request was not scoped or did not carry the expected payload: auth=%q body=%q", authorization, body)
	}
}

func TestExpireUploadReservationUsesScopedFixtureRequest(t *testing.T) {
	var authorization, body string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		if request.Method != http.MethodPost || request.URL.Path != "/expire-upload-reservation" {
			t.Errorf("fixture request did not use the private POST contract: %s %s", request.Method, request.URL.Path)
		}
		body = readRequestBody(t, request)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"expired":true,"uploadId":"upload-1"}`))
	}))
	defer server.Close()

	config := Config{FixtureURL: server.URL, FixtureToken: "fixture-token-with-at-least-24-bytes", Timeout: 2 * time.Second}
	if err := ExpireUploadReservation(context.Background(), config, "upload-1"); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer "+config.FixtureToken || body != `{"uploadId":"upload-1"}` {
		t.Fatalf("fixture request was not scoped or did not carry the expected payload: auth=%q body=%q", authorization, body)
	}
}

func TestDatabaseFixtureRejectsNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "fixture failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	config := Config{FixtureURL: server.URL, FixtureToken: "fixture-token-with-at-least-24-bytes", Timeout: 2 * time.Second}
	err := SetCanvasUpdatedAt(context.Background(), config, []string{"canvas-a"}, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") || strings.Contains(err.Error(), "fixture failure") {
		t.Fatalf("unexpected fixture failure: %v", err)
	}
}

func readRequestBody(t *testing.T, request *http.Request) string {
	t.Helper()
	var body strings.Builder
	if _, err := io.Copy(&body, request.Body); err != nil {
		t.Fatal(err)
	}
	return body.String()
}
