package runner

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestResponseDiagnosticsExcludeSecrets(t *testing.T) {
	response := Response{
		StatusCode: http.StatusBadRequest,
		Header: http.Header{
			"Set-Cookie": {"session=secret-cookie"},
			"Location":   {"https://example.test/file?signature=secret-signature"},
		},
		Body: []byte(`{"token":"secret-token","prompt":"secret-prompt"}`),
	}
	if strings.Contains(response.String(), "secret") {
		t.Fatal("response diagnostics leaked credentials or content")
	}
}

func TestSafeRequestPath(t *testing.T) {
	requestPath := "/api/public/canvas-shares/secret-token/resources/resource/file?signature=secret-signature"
	if got := safeRequestPath(requestPath); got != "/api/public/canvas-shares/{token}/resources/resource/file" {
		t.Fatal("request diagnostics did not redact share token and query")
	}
}

func TestConfigRejectsNonOriginURLs(t *testing.T) {
	for _, baseURL := range []string{
		"http://username:secret-password@example.test",
		"http://example.test?token=secret-token",
		"http://example.test#secret-fragment",
		"http://example.test/api",
	} {
		t.Setenv("CANVAS_CONTRACT_BASE_URL", baseURL)
		if _, err := ConfigFromEnv(); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("configuration accepted a non-origin URL or logged its secret")
		}
	}
}

func TestImportConfigRequiresExplicitFixtureAndTestAdmin(t *testing.T) {
	t.Setenv("CANVAS_CONTRACT_BASE_URL", "http://127.0.0.1:18183")
	t.Setenv("CANVAS_CONTRACT_PROFILE", "import")
	t.Setenv("CANVAS_CONTRACT_IMPORT_FIXTURE", "false")
	t.Setenv("CANVAS_CONTRACT_REGISTER_USERS", "true")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("import must not run fixed production URLs without an explicit isolated-fixture declaration")
	}
	t.Setenv("CANVAS_CONTRACT_IMPORT_FIXTURE", "true")
	if config, err := ConfigFromEnv(); err != nil || config.Timeout != 35*time.Second {
		t.Fatal("fixture bootstrap must provide a runner deadline longer than the target's 20-second upstream deadline")
	}
	t.Setenv("CANVAS_CONTRACT_REGISTER_USERS", "false")
	t.Setenv("CANVAS_CONTRACT_ADMIN_USERNAME", "")
	t.Setenv("CANVAS_CONTRACT_ADMIN_PASSWORD", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("explicit-account import needs isolated test-admin credentials")
	}
	t.Setenv("CANVAS_CONTRACT_ADMIN_USERNAME", "fixture-admin")
	t.Setenv("CANVAS_CONTRACT_ADMIN_PASSWORD", "synthetic-fixture-password")
	if _, err := ConfigFromEnv(); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsProfilesRequireIsolatedTargetAndAdmin(t *testing.T) {
	t.Setenv("CANVAS_CONTRACT_BASE_URL", "http://127.0.0.1:18183")
	for _, profile := range []string{"project-integration", "project-workbench-core", "resource-integrity"} {
		t.Setenv("CANVAS_CONTRACT_PROFILE", profile)
		t.Setenv("CANVAS_CONTRACT_REGISTER_USERS", "true")
		t.Setenv("CANVAS_CONTRACT_SETTINGS_FIXTURE", "false")
		if _, err := ConfigFromEnv(); err == nil {
			t.Fatal("settings-changing profile accepted a target without explicit isolation")
		}
		t.Setenv("CANVAS_CONTRACT_SETTINGS_FIXTURE", "true")
		if config, err := ConfigFromEnv(); err != nil || !config.SettingsFixture {
			t.Fatal("isolated bootstrap settings configuration rejected")
		}
		t.Setenv("CANVAS_CONTRACT_REGISTER_USERS", "false")
		t.Setenv("CANVAS_CONTRACT_ADMIN_USERNAME", "")
		t.Setenv("CANVAS_CONTRACT_ADMIN_PASSWORD", "")
		if _, err := ConfigFromEnv(); err == nil {
			t.Fatal("explicit-account settings-changing profile accepted no admin credentials")
		}
		t.Setenv("CANVAS_CONTRACT_ADMIN_USERNAME", "fixture-admin")
		t.Setenv("CANVAS_CONTRACT_ADMIN_PASSWORD", "synthetic-fixture-password")
		if _, err := ConfigFromEnv(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEnvelopeRejectsMissingOrInvalidRequiredFields(t *testing.T) {
	for _, body := range []string{
		"",
		`{}`,
		`{"code":0,"data":{}}`,
		`{"code":0,"msg":"ok"}`,
		`{"code":null,"data":{},"msg":"ok"}`,
		`{"code":0,"data":{},"msg":null}`,
		`{"code":"0","data":{},"msg":"ok"}`,
		`{"code":0,"data":{},"msg":1}`,
	} {
		response := Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(body)}
		if _, err := decodeEnvelope(response); err == nil {
			t.Fatal("invalid envelope accepted as code-zero success")
		}
	}
	response := Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: []byte(`{"code":0,"data":{},"msg":"ok"}`)}
	if _, err := decodeEnvelope(response); err != nil {
		t.Fatal(err)
	}
	response.Header.Set("Content-Type", "text/plain")
	if _, err := decodeEnvelope(response); err == nil {
		t.Fatal("non-JSON content type accepted")
	}
}

func TestNotModifiedEnvelopeMustBeEmpty(t *testing.T) {
	if _, err := decodeEnvelope(Response{StatusCode: http.StatusNotModified}); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeEnvelope(Response{StatusCode: http.StatusNotModified, Body: []byte("unexpected")}); err == nil {
		t.Fatal("304 with a body accepted")
	}
}
