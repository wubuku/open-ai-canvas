package harness

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestControlRejectsUnauthorizedAndMalformedRequests(test *testing.T) {
	const token = "synthetic-control-test-token-2026"
	server := httptest.NewServer(ControlHandler(context.Background(), nil, token))
	defer server.Close()
	for _, fixture := range []struct {
		method string
		path   string
		body   string
		token  string
		status int
	}{
		{http.MethodPost, "/restart", `{"mode":"crash"}`, "", http.StatusUnauthorized},
		{http.MethodGet, "/restart", "", token, http.StatusMethodNotAllowed},
		{http.MethodPost, "/unrelated", `{"mode":"crash"}`, token, http.StatusNotFound},
		{http.MethodPost, "/restart", `{"mode":"unsupported"}`, token, http.StatusBadRequest},
		{http.MethodPost, "/restart", `{"mode":"crash","extra":true}`, token, http.StatusBadRequest},
		{http.MethodPost, "/restart", `{"mode":"crash"}{}`, token, http.StatusBadRequest},
		{http.MethodPost, "/restart", strings.Repeat(" ", 257) + `{}`, token, http.StatusBadRequest},
	} {
		request, err := http.NewRequest(fixture.method, server.URL+fixture.path, strings.NewReader(fixture.body))
		if err != nil {
			test.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+fixture.token)
		response, err := server.Client().Do(request)
		if err != nil {
			test.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != fixture.status {
			test.Fatalf("control status=%d want=%d", response.StatusCode, fixture.status)
		}
	}
}

func TestTargetRejectsOccupiedOriginAndRemoteControl(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { writer.WriteHeader(http.StatusOK) }))
	defer server.Close()
	target, err := NewTarget(Config{Binary: "not-an-executable", Origin: server.URL, Output: io.Discard, ReadyTimeout: time.Second})
	if err != nil {
		test.Fatal(err)
	}
	if err := target.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "occupied") {
		test.Fatal("occupied target was not rejected before process creation")
	}
	for _, origin := range []string{"http://example.invalid:80", "http://user:password@127.0.0.1:80", "http://127.0.0.1:80/api", "http://127.0.0.1:80?token=x", "https://127.0.0.1:80"} {
		if _, err := LocalOrigin(origin); err == nil {
			test.Fatal("unsafe control origin accepted")
		}
	}
}

func TestRestartOwnedTarget(test *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	executable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	target, err := NewTarget(Config{
		Binary: executable, Arguments: []string{"-test.run=^TestServingProcess$"},
		Environment: append(os.Environ(), "CANVAS_CONTRACT_HELPER=true", "CANVAS_CONTRACT_HELPER_ADDR="+address),
		Origin:      "http://" + address, Output: io.Discard, ReadyTimeout: 5 * time.Second,
	})
	if err != nil {
		test.Fatal(err)
	}
	defer target.Stop()
	if err := target.Start(test.Context()); err != nil {
		test.Fatal(err)
	}
	const token = "synthetic-control-test-token-2026"
	control := httptest.NewServer(ControlHandler(test.Context(), target, token))
	defer control.Close()
	for index, mode := range []string{"crash", "graceful"} {
		request, err := http.NewRequestWithContext(test.Context(), http.MethodPost, control.URL+"/restart", strings.NewReader(`{"mode":"`+mode+`"}`))
		if err != nil {
			test.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := control.Client().Do(request)
		if err != nil {
			test.Fatal(err)
		}
		var payload struct {
			Restarted  bool `json:"restarted"`
			Generation int  `json:"generation"`
		}
		err = json.NewDecoder(response.Body).Decode(&payload)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || !payload.Restarted || payload.Generation != index+2 {
			test.Fatal("owned process restart did not advance generation")
		}
	}
}

func TestServingProcess(test *testing.T) {
	if os.Getenv("CANVAS_CONTRACT_HELPER") != "true" {
		return
	}
	if os.Getenv("CANVAS_CONTRACT_HELPER_IGNORE_TERM") == "true" {
		signal.Ignore(syscall.SIGTERM)
	}
	server := &http.Server{Addr: os.Getenv("CANVAS_CONTRACT_HELPER_ADDR"), Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { writer.WriteHeader(http.StatusOK) })}
	if err := server.ListenAndServe(); err != nil {
		os.Exit(2)
	}
}

func TestGracefulShutdownTimeoutDoesNotBecomeSuccessfulRestart(test *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	executable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	target, err := NewTarget(Config{
		Binary: executable, Arguments: []string{"-test.run=^TestServingProcess$"},
		Environment: append(os.Environ(), "CANVAS_CONTRACT_HELPER=true", "CANVAS_CONTRACT_HELPER_ADDR="+address, "CANVAS_CONTRACT_HELPER_IGNORE_TERM=true"),
		Origin:      "http://" + address, Output: io.Discard, ReadyTimeout: 5 * time.Second, StopTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		test.Fatal(err)
	}
	defer target.Stop()
	if err := target.Start(test.Context()); err != nil {
		test.Fatal(err)
	}
	generation, err := target.Restart(test.Context(), "graceful")
	if err == nil || generation != 0 || target.generation != 1 || target.process != nil {
		test.Fatal("forced cleanup incorrectly became a successful graceful restart")
	}
}
