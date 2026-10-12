package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type evidenceLogger struct {
	lines []string
}

func (logger *evidenceLogger) Logf(format string, values ...any) {
	logger.lines = append(logger.lines, fmt.Sprintf(format, values...))
}

func TestRequestEvidenceRedactsCredentialsAndContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Set-Cookie", "session=secret-cookie")
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"code":0,"data":{"prompt":"secret-response"},"msg":"ok"}`)
	}))
	defer server.Close()
	client, err := NewClient("user-a", Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	logger := &evidenceLogger{}
	ctx := context.WithValue(t.Context(), evidenceContextKey{}, logger)
	_, _, err = client.Do(ctx, http.MethodPost, "/api/public/canvas-shares/secret-token/resources/test-resource/file/secret-name?signature=secret-signature", map[string]string{"prompt": "secret-prompt"}, http.Header{"Authorization": {"secret-auth"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(logger.lines) != 1 || strings.Contains(logger.lines[0], "secret") || !strings.Contains(logger.lines[0], `"path":"/api/public/canvas-shares/{token}/resources/test-resource/file/{filename}"`) || !strings.Contains(logger.lines[0], `"status":200,"body_complete":true`) {
		t.Fatal("HTTP request evidence is missing or contains sensitive content")
	}
}

func TestRequestEvidenceDistinguishesTransportFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	client, err := NewClient("user-a", Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	logger := &evidenceLogger{}
	ctx := context.WithValue(t.Context(), evidenceContextKey{}, logger)
	if _, _, err := client.Do(ctx, http.MethodGet, "/api/canvas-projects", nil, nil); err == nil {
		t.Fatal("closed server unexpectedly returned a response")
	}
	if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], `"status":0,"body_complete":false,"transport_failure":true`) {
		t.Fatal("transport failure was incorrectly recorded as an HTTP response")
	}
}

func TestOversizedResponseDoesNotCountAsCompleteEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, strings.Repeat("x", (12<<20)+1))
	}))
	defer server.Close()
	client, err := NewClient("user-a", Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	logger := &evidenceLogger{}
	ctx := context.WithValue(t.Context(), evidenceContextKey{}, logger)
	if _, err := client.DoRaw(ctx, http.MethodGet, "/api/resources/test-resource/file", nil, "", nil); err == nil {
		t.Fatal("oversized response accepted")
	}
	if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], `"body_complete":false`) {
		t.Fatal("truncated response counted as complete HTTP evidence")
	}
}
