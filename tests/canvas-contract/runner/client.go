package runner

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"canvas-contract/contract"
)

type Config struct {
	BaseURL           string
	Profile           string
	RegisterUsers     bool
	UsernameA         string
	PasswordA         string
	UsernameB         string
	PasswordB         string
	AdminUsername     string
	AdminPassword     string
	ImportFixture     bool
	SettingsFixture   bool
	Timeout           time.Duration
	Implementation    string
	DatabaseProfile   string
	RunID             string
	ControlURL        string
	ControlToken      string
	ShareFixtureURL   string
	ShareFixtureToken string
}

func ConfigFromEnv() (Config, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_BASE_URL")), "/")
	if baseURL == "" {
		return Config{}, fmt.Errorf("CANVAS_CONTRACT_BASE_URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" && parsed.Path != "/" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Config{}, fmt.Errorf("CANVAS_CONTRACT_BASE_URL must be an HTTP origin without credentials, query, fragment or /api")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Config{}, fmt.Errorf("CANVAS_CONTRACT_BASE_URL must use http or https: %q", baseURL)
	}
	profile := strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_PROFILE"))
	if profile == "" {
		profile = "core"
	}
	switch profile {
	case "core", "share-public", "project-integration", "project-workbench-core", "resource-integrity", "import", "legacy-assets", "public-resource", "restart-recovery", "generation-boundary", "generation-writeback":
	default:
		return Config{}, fmt.Errorf("profile %q is not implemented; see contract/manifest.yaml", profile)
	}
	timeout := 20 * time.Second
	if profile == "import" {
		timeout = 35 * time.Second
	}
	if raw := strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_TIMEOUT")); raw != "" {
		parsedTimeout, parseErr := time.ParseDuration(raw)
		if parseErr != nil || parsedTimeout <= 0 {
			return Config{}, fmt.Errorf("CANVAS_CONTRACT_TIMEOUT must be a positive duration: %q", raw)
		}
		timeout = parsedTimeout
	}
	registerUsers := strings.EqualFold(strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_REGISTER_USERS")), "true")
	importFixture := strings.EqualFold(strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_IMPORT_FIXTURE")), "true")
	adminUsername := strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_ADMIN_USERNAME"))
	adminPassword := os.Getenv("CANVAS_CONTRACT_ADMIN_PASSWORD")
	settingsFixture := strings.EqualFold(strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_SETTINGS_FIXTURE")), "true")
	if RequiresSettingsFixture(profile) && (!settingsFixture || !registerUsers && (adminUsername == "" || adminPassword == "")) {
		return Config{}, fmt.Errorf("%s requires CANVAS_CONTRACT_SETTINGS_FIXTURE=true on an isolated target and bootstrap or explicit test-admin credentials", profile)
	}
	if profile == "import" && (!importFixture || !registerUsers && (adminUsername == "" || adminPassword == "")) {
		return Config{}, fmt.Errorf("import requires an isolated fixed-origin TLS fixture, CANVAS_CONTRACT_IMPORT_FIXTURE=true, and bootstrap or explicit test-admin credentials")
	}
	runID := strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_RUN_ID"))
	if runID == "" {
		runID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	controlURL := strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_CONTROL_URL"))
	controlToken := os.Getenv("CANVAS_CONTRACT_CONTROL_TOKEN")
	shareFixtureURL := strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_SHARE_FIXTURE_URL"))
	shareFixtureToken := os.Getenv("CANVAS_CONTRACT_SHARE_FIXTURE_TOKEN")
	if profile == "share-public" {
		fixture, fixtureErr := url.Parse(shareFixtureURL)
		if fixtureErr != nil || fixture.Scheme != "http" || fixture.Hostname() != "127.0.0.1" && fixture.Hostname() != "::1" || fixture.Port() == "" || fixture.Path != "" || fixture.RawQuery != "" || fixture.Fragment != "" || len(shareFixtureToken) < 24 {
			return Config{}, fmt.Errorf("share-public requires a private loopback share-expiry fixture and token (at least 24 bytes)")
		}
	}
	if profile == "restart-recovery" {
		control, parseErr := url.Parse(controlURL)
		if parseErr != nil || control.Scheme != "http" || control.Hostname() != "127.0.0.1" && control.Hostname() != "::1" || control.Port() == "" || control.User != nil || control.Path != "" || control.RawQuery != "" || control.Fragment != "" || len(controlToken) < 24 {
			return Config{}, fmt.Errorf("restart-recovery requires a private loopback control origin and token (at least 24 bytes)")
		}
	}
	return Config{
		BaseURL:           baseURL,
		Profile:           profile,
		RegisterUsers:     registerUsers,
		UsernameA:         strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_USER_A_USERNAME")),
		PasswordA:         os.Getenv("CANVAS_CONTRACT_USER_A_PASSWORD"),
		UsernameB:         strings.TrimSpace(os.Getenv("CANVAS_CONTRACT_USER_B_USERNAME")),
		PasswordB:         os.Getenv("CANVAS_CONTRACT_USER_B_PASSWORD"),
		AdminUsername:     adminUsername,
		AdminPassword:     adminPassword,
		ImportFixture:     importFixture,
		SettingsFixture:   settingsFixture,
		Timeout:           timeout,
		Implementation:    envOr("CANVAS_CONTRACT_IMPLEMENTATION", "unknown"),
		DatabaseProfile:   envOr("CANVAS_CONTRACT_DB_PROFILE", "unknown"),
		RunID:             runID,
		ControlURL:        controlURL,
		ControlToken:      controlToken,
		ShareFixtureURL:   shareFixtureURL,
		ShareFixtureToken: shareFixtureToken,
	}, nil
}

func RequiresSettingsFixture(profile string) bool {
	return profile == "project-integration" || profile == "project-workbench-core" || profile == "resource-integrity" || profile == "generation-writeback"
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

type Client struct {
	Name   string
	Config Config
	HTTP   *http.Client
}

func NewClient(name string, config Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		Name:   name,
		Config: config,
		HTTP: &http.Client{
			Jar:     jar,
			Timeout: config.Timeout,
			Transport: &http.Transport{
				DisableCompression:  true,
				MaxIdleConns:        8,
				MaxIdleConnsPerHost: 8,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type Envelope struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Msg     string          `json:"msg"`
	Reason  string          `json:"reason"`
	Details json.RawMessage `json:"details"`
}

func (c *Client) Do(ctx context.Context, method, path string, payload any, headers http.Header) (Response, Envelope, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return Response{}, Envelope{}, err
		}
		body = bytes.NewReader(encoded)
	}
	contentType := ""
	if payload != nil {
		contentType = "application/json"
	}
	return c.doEnvelope(ctx, method, path, body, contentType, headers)
}

func (c *Client) DoMultipart(ctx context.Context, method, path string, fields map[string]string, fieldName, fileName string, content []byte, headers http.Header) (Response, Envelope, error) {
	var encoded bytes.Buffer
	writer := multipart.NewWriter(&encoded)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return Response{}, Envelope{}, err
		}
	}
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return Response{}, Envelope{}, err
	}
	if _, err := part.Write(content); err != nil {
		return Response{}, Envelope{}, err
	}
	if err := writer.Close(); err != nil {
		return Response{}, Envelope{}, err
	}
	requestHeaders := headers.Clone()
	if requestHeaders == nil {
		requestHeaders = make(http.Header)
	}
	requestHeaders.Set("Content-Type", writer.FormDataContentType())
	return c.doEnvelope(ctx, method, path, bytes.NewReader(encoded.Bytes()), "", requestHeaders)
}

func (c *Client) DoBytes(ctx context.Context, method, path, contentType string, body []byte, headers http.Header) (Response, Envelope, error) {
	return c.doEnvelope(ctx, method, path, bytes.NewReader(body), contentType, headers)
}

func (c *Client) DoRaw(ctx context.Context, method, path string, body []byte, contentType string, headers http.Header) (Response, error) {
	return c.doRaw(ctx, method, path, bytes.NewReader(body), contentType, headers)
}

func (c *Client) doEnvelope(ctx context.Context, method, path string, body io.Reader, contentType string, headers http.Header) (Response, Envelope, error) {
	result, err := c.doRaw(ctx, method, path, body, contentType, headers)
	if err != nil {
		return result, Envelope{}, err
	}
	envelope, err := decodeEnvelope(result)
	if err != nil {
		return result, Envelope{}, fmt.Errorf("%s %s: %w", method, safeRequestPath(path), err)
	}
	return result, envelope, nil
}

func decodeEnvelope(response Response) (Envelope, error) {
	if response.StatusCode == http.StatusNotModified {
		if len(response.Body) != 0 {
			return Envelope{}, fmt.Errorf("304 response must have an empty body")
		}
		return Envelope{}, nil
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return Envelope{}, fmt.Errorf("status %d returned a non-JSON content type", response.StatusCode)
	}
	decodedBody, err := response.JSONBody()
	if err != nil {
		return Envelope{}, fmt.Errorf("decode response body: %w", err)
	}
	var required struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  *string         `json:"msg"`
	}
	if err := json.Unmarshal(decodedBody, &required); err != nil {
		return Envelope{}, fmt.Errorf("invalid JSON envelope: %w", err)
	}
	if required.Code == nil || required.Msg == nil || len(required.Data) == 0 {
		return Envelope{}, fmt.Errorf("response envelope must contain code, data and msg")
	}
	var envelope Envelope
	if err := json.Unmarshal(decodedBody, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("invalid response envelope fields: %w", err)
	}
	return envelope, nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, body io.Reader, contentType string, headers http.Header) (Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.Config.BaseURL+path, body)
	if err != nil {
		return Response{}, fmt.Errorf("%s %s: invalid HTTP request", method, safeRequestPath(path))
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("Accept", "application/json")
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		if fault, ok := ctx.Value(faultContextKey{}).(requestFault); ok && fault.Loss != nil {
			witness := fault.Loss()
			witness.ClientEOF = errors.Is(err, io.EOF)
			fault.Loss = func() ResponseLossWitness { return witness }
			ctx = context.WithValue(ctx, faultContextKey{}, fault)
		}
		recordRequest(ctx, method, path, 0, false, true)
		if urlError, ok := err.(*url.Error); ok {
			err = urlError.Err
		}
		return Response{}, fmt.Errorf("%s %s: %w", method, safeRequestPath(path), transportError{cause: err})
	}
	return readHTTPResponse(ctx, method, path, response)
}

func readHTTPResponse(ctx context.Context, method, path string, response *http.Response) (Response, error) {
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, (12<<20)+1))
	if err != nil {
		recordRequest(ctx, method, path, response.StatusCode, false, false)
		return Response{}, err
	}
	if len(responseBody) > 12<<20 {
		recordRequest(ctx, method, path, response.StatusCode, false, false)
		return Response{}, fmt.Errorf("response exceeded the 12 MiB limit")
	}
	result := Response{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: responseBody}
	validationBody := responseBody
	if contract.RequiresJSON(method, path, result.StatusCode, result.Header.Get("Content-Type")) {
		validationBody, err = result.JSONBody()
		if err != nil {
			recordRequest(ctx, method, path, result.StatusCode, true, false, contract.WireEvidence{Version: contract.WireVersion, Mode: "json", Valid: false})
			return result, fmt.Errorf("%s %s: invalid encoded JSON body", method, safeRequestPath(path))
		}
	}
	wire, err := contract.ValidateResponse(method, path, result.StatusCode, result.Header.Get("Content-Type"), validationBody, result.Header)
	recordRequest(ctx, method, path, result.StatusCode, true, false, wire)
	if err != nil {
		return result, fmt.Errorf("%s %s: %w", method, safeRequestPath(path), err)
	}
	return result, nil
}

func (response Response) JSONBody() ([]byte, error) {
	if strings.EqualFold(response.Header.Get("Content-Encoding"), "gzip") {
		reader, err := gzip.NewReader(bytes.NewReader(response.Body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		decoded, err := io.ReadAll(io.LimitReader(reader, (12<<20)+1))
		if err != nil {
			return nil, err
		}
		if len(decoded) > 12<<20 {
			return nil, fmt.Errorf("decoded response exceeded the 12 MiB limit")
		}
		return decoded, nil
	}
	return response.Body, nil
}

func (response Response) String() string {
	return fmt.Sprintf("status=%d content_type=%q content_encoding=%q body_bytes=%d", response.StatusCode, response.Header.Get("Content-Type"), response.Header.Get("Content-Encoding"), len(response.Body))
}

func safeRequestPath(path string) string {
	path = strings.SplitN(path, "?", 2)[0]
	path = strings.SplitN(path, "#", 2)[0]
	const sharePrefix = "/api/public/canvas-shares/"
	if strings.HasPrefix(path, sharePrefix) {
		parts := strings.SplitN(strings.TrimPrefix(path, sharePrefix), "/", 2)
		path = sharePrefix + "{token}"
		if len(parts) == 2 {
			path += "/" + parts[1]
		}
	}
	if index := strings.Index(path, "/file/"); index >= 0 {
		path = path[:index] + "/file/{filename}"
	}
	return path
}

type transportError struct {
	cause error
}

func (transportError) Error() string {
	return "HTTP transport failed"
}

func (failure transportError) Unwrap() error {
	return failure.cause
}
