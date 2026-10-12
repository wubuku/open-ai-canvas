package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func RequireStatus(t *testing.T, response Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, want, response.String())
	}
}

func RequireStatuses(t *testing.T, response Response, wants ...int) {
	t.Helper()
	for _, want := range wants {
		if response.StatusCode == want {
			return
		}
	}
	t.Fatalf("status = %d, want one of %v: %s", response.StatusCode, wants, response.String())
}

func RequireSuccess(t *testing.T, response Response, envelope Envelope) {
	t.Helper()
	RequireStatus(t, response, http.StatusOK)
	if envelope.Code != 0 {
		t.Fatalf("success response code = %d: %s", envelope.Code, response.String())
	}
}

func RequireError(t *testing.T, response Response, envelope Envelope, status int, reason string) {
	t.Helper()
	RequireStatus(t, response, status)
	if envelope.Code == 0 {
		t.Fatalf("error response has code 0: %s", response.String())
	}
	if envelope.Reason == "" {
		t.Fatalf("error response has no machine-readable reason: %s", response.String())
	}
	if reason != "" && envelope.Reason != reason {
		t.Fatalf("error reason = %q, want %q: %s", envelope.Reason, reason, response.String())
	}
}

func DecodeData[T any](t *testing.T, envelope Envelope) T {
	t.Helper()
	var result T
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		t.Fatalf("response data is empty")
	}
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		t.Fatalf("decode response data: %v; data_bytes=%d", err, len(envelope.Data))
	}
	return result
}

func RequireHeader(t *testing.T, header http.Header, key, want string) {
	t.Helper()
	if got := header.Get(key); got != want {
		t.Fatalf("header %s = %q, want %q", key, got, want)
	}
}

func RequireJSONBody(t *testing.T, response Response) map[string]any {
	t.Helper()
	body, err := response.JSONBody()
	if err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode JSON response body: %v; body_bytes=%d", err, len(body))
	}
	return decoded
}

func RequireNumber(t *testing.T, value any, want float64, label string) {
	t.Helper()
	got, ok := value.(float64)
	if !ok || got != want {
		t.Fatalf("%s = %#v, want %s", label, value, fmt.Sprint(want))
	}
}
