package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileRedirectIsValidatedWithoutFollowingOrDisclosingLocation(test *testing.T) {
	var followed atomic.Int64
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		followed.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Location", destination.URL+"/secret-location?signature=secret-signature")
		writer.Header().Set("Cache-Control", "private, no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := NewClient("synthetic", Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		test.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	logger := &evidenceLogger{}
	response, err := client.DoRaw(context.WithValue(test.Context(), evidenceContextKey{}, logger), "GET", "/api/public/resources/fixture/file/secret-name?signature=secret-input", nil, "", nil)
	if err != nil || response.StatusCode != 307 || len(response.Body) != 0 || followed.Load() != 0 || len(logger.lines) != 1 || !strings.Contains(logger.lines[0], `"mode":"redirect","valid":true`) || strings.Contains(logger.lines[0], "secret") {
		test.Fatal("redirect was followed, invalid, or disclosed its authorization")
	}
}

func TestInvalidDescriptionProducesSafeRequestVariantAndFailedWireEvidence(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "private, no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		fmt.Fprint(writer, `{"code":0,"data":{"access":{"url":"secret-location"}},"msg":"secret-message"}`)
	}))
	defer server.Close()
	client, err := NewClient("synthetic", Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		test.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	logger := &evidenceLogger{}
	_, err = client.DoRaw(context.WithValue(test.Context(), evidenceContextKey{}, logger), "GET", "/api/public/canvas-shares/secret-token/resources/fixture/file?signature=secret-input&access=1", nil, "", nil)
	if err == nil || strings.Contains(err.Error(), "secret") || len(logger.lines) != 1 || strings.Contains(logger.lines[0], "secret") || !strings.Contains(logger.lines[0], `"request_variant":"access-description"`) || !strings.Contains(logger.lines[0], `"valid":false`) {
		test.Fatal("invalid description lost its request facet, was accepted, or leaked secrets")
	}
}
