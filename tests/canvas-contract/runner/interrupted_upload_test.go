package runner

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestInterruptedChunkReceivesCompleteHTTPFailureWithoutLoggingBodyOrCookie(test *testing.T) {
	for _, secured := range []bool{false, true} {
		name := "http"
		if secured {
			name = "https"
		}
		test.Run(name, func(test *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				body, err := io.ReadAll(request.Body)
				if err == nil || string(body) != "SECRET" || request.ContentLength != 13 || request.Method != http.MethodPut || request.Header.Get("Cookie") != "session=private-cookie" {
					test.Error("interrupted upload did not preserve framing, credentials and partial bytes")
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(`{"code":400,"data":null,"msg":"incomplete upload","reason":"bad_request"}`))
			}))
			if secured {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			client, err := NewClient("owner", Config{BaseURL: server.URL, Timeout: 2 * time.Second})
			if err != nil {
				test.Fatal(err)
			}
			defer client.HTTP.CloseIdleConnections()
			if secured {
				client.HTTP.Transport = server.Client().Transport
			}
			origin, _ := url.Parse(server.URL)
			client.HTTP.Jar.SetCookies(origin, []*http.Cookie{{Name: "session", Value: "private-cookie", Path: "/"}})
			logger := &evidenceLogger{}
			requestContext := context.WithValue(test.Context(), evidenceContextKey{}, logger)
			response, envelope, err := client.DoInterruptedChunk(requestContext, "/api/resources/uploads/test-session/chunks/1", 13, []byte("SECRET"))
			if err != nil || response.StatusCode != http.StatusBadRequest || envelope.Code != 400 {
				test.Fatalf("interrupted chunk did not receive a complete typed failure envelope: %s code=%d error=%v", response.String(), envelope.Code, err)
			}
			if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], `"request_fault":"upload-body-half-close","declared_body_bytes":13,"sent_body_bytes":6`) || !strings.Contains(logger.lines[0], `"body_complete":true,"transport_failure":false`) || strings.Contains(logger.lines[0], "SECRET") || strings.Contains(logger.lines[0], "private-cookie") {
				test.Fatal("fault evidence was incomplete, falsely classified or leaked request content")
			}
		})
	}
}

func TestInterruptedChunkRejectsUnsafePathsFramingAndTLS(test *testing.T) {
	client, err := NewClient("owner", Config{BaseURL: "http://127.0.0.1:1", Timeout: time.Second})
	if err != nil {
		test.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	for _, path := range []string{"/api/resources/uploads/test/chunks/1?signature=secret", "/api/resources/uploads/test/chunks/not-index", "/api/canvas-projects", "/api/resources/uploads/test\r\nsecret/chunks/1"} {
		if _, _, err := client.DoInterruptedChunk(test.Context(), path, 13, []byte("SECRET")); err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "SECRET") {
			test.Fatal("unsafe fault input was accepted or leaked")
		}
	}
	for _, declared := range []int64{-1, 0, 6} {
		if _, _, err := client.DoInterruptedChunk(test.Context(), "/api/resources/uploads/test/chunks/1", declared, []byte("SECRET")); err == nil {
			test.Fatal("non-interrupted framing accepted")
		}
	}
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	client.Config.BaseURL = server.URL
	client.HTTP.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if _, _, err := client.DoInterruptedChunk(test.Context(), "/api/resources/uploads/test/chunks/1", 13, []byte("SECRET")); err == nil || !strings.Contains(err.Error(), "verified TLS") {
		test.Fatal("interrupted upload bypassed TLS verification")
	}
}
