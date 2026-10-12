package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResponseLossProvesOneCommitAndRedactsEvidence(test *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(request.Body)
		if request.Method != http.MethodPut || request.URL.Path != "/api/canvas-projects/fixture" || request.Header.Get("X-Canvas-Contract-Fault") != "" || request.Header.Get("Cookie") != "session=secret-cookie" || !strings.Contains(string(body), "secret-prompt") {
			test.Error("fixed target received an altered method/path/body/cookie or fixture credential")
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"code":0,"msg":"ok","data":{"project":{"id":"fixture","title":"secret-response","revision":2,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer server.Close()
	client, err := NewClient("owner", Config{BaseURL: server.URL, Timeout: time.Second, SettingsFixture: true})
	if err != nil {
		test.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	origin, _ := url.Parse(server.URL)
	client.HTTP.Jar.SetCookies(origin, []*http.Cookie{{Name: "session", Value: "secret-cookie"}})
	logger := &evidenceLogger{}
	ctx := context.WithValue(test.Context(), evidenceContextKey{}, logger)
	witness, err := client.DoWithResponseLoss(ctx, "document-response-loss", http.MethodPut, "/api/canvas-projects/fixture", func(ctx context.Context, proxy *Client) (Response, Envelope, error) {
		return proxy.Do(ctx, http.MethodPut, "/api/canvas-projects/fixture", map[string]string{"prompt": "secret-prompt"}, http.Header{"X-Idempotency-Key": []string{"secret-key"}})
	})
	if err != nil || calls.Load() != 1 || !witness.ClientEOF || !witness.TargetWire.Valid || witness.ClientBytesSent != 0 || witness.RequestBodyBytes <= 0 {
		test.Fatal("response loss did not prove a single complete target commit followed by client EOF", err)
	}
	if len(logger.lines) != 1 || strings.Contains(logger.lines[0], "secret") {
		test.Fatal("loss evidence missing or containing credentials/content")
	}
	var fact requestEvidence
	if err := json.Unmarshal([]byte(strings.TrimPrefix(logger.lines[0], "CANVAS_CONTRACT_HTTP ")), &fact); err != nil {
		test.Fatal(err)
	}
	if fact.Status != 0 || fact.BodyComplete || !fact.TransportFail || fact.Wire != nil || fact.RequestFault != "document-response-loss" || fact.ResponseLoss == nil || !fact.ResponseLoss.ClientEOF || fact.ResponseLoss.TargetStatus != 200 {
		test.Fatal("the lost client response must not masquerade as a successful wire validation")
	}
}

func TestResponseLossRejectsUnisolatedOrUnboundedOperations(test *testing.T) {
	client, _ := NewClient("owner", Config{BaseURL: "http://127.0.0.1:9", Timeout: time.Second})
	called := false
	send := func(context.Context, *Client) (Response, Envelope, error) {
		called = true
		return Response{}, Envelope{}, nil
	}
	for _, path := range []string{"/api/canvas-projects/fixture", "/api/tasks", "/api/resources?unsafe=1", "/api/resources/uploads/fixture/complete/extra"} {
		if _, err := client.DoWithResponseLoss(test.Context(), "document-response-loss", http.MethodPut, path, send); err == nil {
			test.Fatal("response loss accepted an unisolated or unsupported operation")
		}
	}
	if called {
		test.Fatal("invalid fixture configuration invoked the caller")
	}
}

func TestResponseLossCannotClassifyUpstreamFailuresAsCommitted(test *testing.T) {
	for _, mode := range []string{"upstream-error", "invalid-wire", "truncated"} {
		test.Run(mode, func(test *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				if mode == "upstream-error" {
					writer.WriteHeader(http.StatusInternalServerError)
				}
				if mode == "truncated" {
					writer.Header().Set("Content-Length", "128")
				}
				fmt.Fprint(writer, `{"code":0,"data":{},"msg":"ok"}`)
			}))
			defer server.Close()
			client, _ := NewClient("owner", Config{BaseURL: server.URL, Timeout: time.Second, SettingsFixture: true})
			defer client.HTTP.CloseIdleConnections()
			witness, err := client.DoWithResponseLoss(test.Context(), "document-response-loss", http.MethodPut, "/api/canvas-projects/fixture", func(ctx context.Context, proxy *Client) (Response, Envelope, error) {
				return proxy.Do(ctx, http.MethodPut, "/api/canvas-projects/fixture", map[string]any{"project": map[string]any{}}, nil)
			})
			if err == nil || witness.TargetBodyComplete || witness.ConnectionClosed {
				test.Fatal("failed or unvalidated target response was classified as a proven lost commit")
			}
		})
	}
}
