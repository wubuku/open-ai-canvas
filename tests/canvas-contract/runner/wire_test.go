package runner

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRawAndEnvelopeRequestsRejectSchemaDriftWithoutLeakingBodies(test *testing.T) {
	for _, raw := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprint(writer, `{"code":0,"data":{"project":{"prompt":"secret-response"}},"msg":"ok"}`)
		}))
		client, err := NewClient("synthetic", Config{BaseURL: server.URL, Timeout: time.Second})
		if err != nil {
			test.Fatal(err)
		}
		logger := &evidenceLogger{}
		ctx := context.WithValue(test.Context(), evidenceContextKey{}, logger)
		var response Response
		if raw {
			response, err = client.DoRaw(ctx, "GET", "/api/canvas-projects/fixture", nil, "", nil)
		} else {
			response, _, err = client.Do(ctx, "GET", "/api/canvas-projects/fixture", nil, nil)
		}
		client.HTTP.CloseIdleConnections()
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret") || response.StatusCode != 200 {
			test.Fatal("schema drift was accepted, leaked, or mislabeled as transport failure")
		}
		if len(logger.lines) != 1 || strings.Contains(logger.lines[0], "secret") || !strings.Contains(logger.lines[0], `"body_complete":true,"transport_failure":false`) || !strings.Contains(logger.lines[0], `"valid":false`) {
			test.Fatal("invalid wire shape did not retain safe and complete response evidence")
		}
	}
}

func TestWireFailureDiagnosticsRedactNamedFilesAndInvalidURLs(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprint(writer, `{"code":0,"data":null,"msg":"secret-message"}`)
	}))
	defer server.Close()
	client, err := NewClient("synthetic", Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		test.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	for _, path := range []string{
		"/api/public/resources/fixture/file/secret-name?signature=secret-signature",
		"/api/public/canvas-shares/secret-token/resources/fixture/file/secret-name?signature=secret-url\x7f",
	} {
		if _, err := client.DoRaw(test.Context(), "GET", path, nil, "", nil); err == nil || strings.Contains(err.Error(), "secret") {
			test.Fatal("wire or invalid-URL diagnostics leaked a filename, token, query or response value")
		}
	}
}

func TestTransportDiagnosticsHideCausesButPreserveCancellation(test *testing.T) {
	cause := errors.New("secret-upstream-location")
	wrapped := transportError{cause: cause}
	if strings.Contains(wrapped.Error(), "secret") || !errors.Is(wrapped, cause) {
		test.Fatal("transport wrapper disclosed the cause or lost error identity")
	}
	client, err := NewClient("synthetic", Config{BaseURL: "http://127.0.0.1:1", Timeout: time.Second})
	if err != nil {
		test.Fatal(err)
	}
	ctx, cancel := context.WithCancel(test.Context())
	cancel()
	_, err = client.DoRaw(ctx, "GET", "/api/public/resources/fixture/file/secret-name?signature=secret-query", nil, "", nil)
	if err == nil || !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "secret") {
		test.Fatal("cancellation identity was lost or transport diagnostics exposed the request")
	}
}

func TestGzipWireValidationAndDecodedSizeLimit(test *testing.T) {
	for _, oversized := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("Content-Encoding", "gzip")
			compressed := gzip.NewWriter(writer)
			defer compressed.Close()
			if oversized {
				fmt.Fprintf(compressed, `{"code":0,"data":{"projects":[],"prompt":"%s"},"msg":"ok"}`, strings.Repeat("x", 12<<20))
			} else {
				fmt.Fprint(compressed, `{"code":0,"data":{"projects":[]},"msg":"ok"}`)
			}
		}))
		client, err := NewClient("synthetic", Config{BaseURL: server.URL, Timeout: time.Second})
		if err != nil {
			test.Fatal(err)
		}
		logger := &evidenceLogger{}
		ctx := context.WithValue(test.Context(), evidenceContextKey{}, logger)
		_, err = client.DoRaw(ctx, "GET", "/api/canvas-projects", nil, "", http.Header{"Accept-Encoding": {"gzip"}})
		client.HTTP.CloseIdleConnections()
		server.Close()
		if (err != nil) != oversized {
			test.Fatal("gzip decoding or decompressed response limit was not applied before validation")
		}
		if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], fmt.Sprintf(`"valid":%t`, !oversized)) {
			test.Fatal("gzip wire validation was not reported")
		}
	}
}
