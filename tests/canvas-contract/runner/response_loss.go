package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"

	"canvas-contract/contract"
)

type ResponseLossWitness struct {
	TargetRequests     int                   `json:"target_requests"`
	TargetStatus       int                   `json:"target_status"`
	TargetBodyComplete bool                  `json:"target_body_complete"`
	TargetBodyBytes    int                   `json:"target_body_bytes"`
	RequestBodyBytes   int                   `json:"request_body_bytes"`
	RequestSHA256      string                `json:"request_sha256"`
	ClientBytesSent    int                   `json:"client_bytes_sent"`
	ConnectionClosed   bool                  `json:"connection_closed"`
	ClientEOF          bool                  `json:"client_eof"`
	TargetWire         contract.WireEvidence `json:"target_wire"`
}

var responseLossCanvasPath = regexp.MustCompile(`^/api/canvas-projects/[A-Za-z0-9_-]+$`)
var responseLossCompletePath = regexp.MustCompile(`^/api/resources/uploads/[A-Za-z0-9_-]+/complete$`)

func (client *Client) DoWithResponseLoss(ctx context.Context, faultID, method, path string, send func(context.Context, *Client) (Response, Envelope, error)) (ResponseLossWitness, error) {
	origin, err := url.Parse(client.Config.BaseURL)
	allowed := method == http.MethodPut && responseLossCanvasPath.MatchString(path) || method == http.MethodPost && (path == "/api/resources" || path == "/api/resources/import" || responseLossCompletePath.MatchString(path))
	if err != nil || origin.Scheme != "http" && origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || !allowed || !client.Config.SettingsFixture || faultID == "" || send == nil {
		return ResponseLossWitness{}, fmt.Errorf("response loss requires an explicitly isolated target, safe origin and exact write operation")
	}
	transport, ok := client.HTTP.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		return ResponseLossWitness{}, fmt.Errorf("response loss requires a standard HTTP transport with verified target TLS")
	}
	targetTransport := transport.Clone()
	targetTransport.DisableKeepAlives = true
	targetTransport.DisableCompression = true
	defer targetTransport.CloseIdleConnections()
	target := &http.Client{Transport: targetTransport, Timeout: client.Config.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return ResponseLossWitness{}, err
	}
	token := hex.EncodeToString(secret)
	var mutex sync.Mutex
	var witness ResponseLossWitness
	snapshot := func() ResponseLossWitness {
		mutex.Lock()
		defer mutex.Unlock()
		return witness
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != method || request.URL.Path != path || request.URL.RawQuery != "" || request.Header.Get("X-Canvas-Contract-Fault") != token {
			http.Error(writer, "fixture rejected undeclared request", http.StatusForbidden)
			return
		}
		mutex.Lock()
		if witness.TargetRequests != 0 {
			mutex.Unlock()
			http.Error(writer, "fixture never retries a target request", http.StatusConflict)
			return
		}
		witness.TargetRequests = 1
		mutex.Unlock()
		body, err := io.ReadAll(io.LimitReader(request.Body, (6<<20)+1))
		if err != nil || len(body) > 6<<20 {
			http.Error(writer, "fixture request body failed", http.StatusBadRequest)
			return
		}
		upstream, err := http.NewRequestWithContext(request.Context(), method, origin.String()+path, bytes.NewReader(body))
		if err != nil {
			http.Error(writer, "fixture target request failed", http.StatusBadGateway)
			return
		}
		upstream.Header = request.Header.Clone()
		upstream.Header.Del("X-Canvas-Contract-Fault")
		upstream.Header.Del("Connection")
		upstream.Header.Del("Proxy-Authorization")
		upstream.Close = true
		response, err := target.Do(upstream)
		if err != nil {
			http.Error(writer, "fixture target transport failed", http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		content, err := io.ReadAll(io.LimitReader(response.Body, (12<<20)+1))
		if err != nil || len(content) > 12<<20 {
			http.Error(writer, "fixture target response incomplete", http.StatusBadGateway)
			return
		}
		result := Response{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: content}
		decoded, err := result.JSONBody()
		if err != nil {
			http.Error(writer, "fixture target encoding failed", http.StatusBadGateway)
			return
		}
		wire, err := contract.ValidateResponse(method, path, response.StatusCode, response.Header.Get("Content-Type"), decoded, response.Header)
		envelope, envelopeErr := decodeEnvelope(result)
		if err != nil || envelopeErr != nil || response.StatusCode != http.StatusOK || envelope.Code != 0 {
			http.Error(writer, "fixture target did not acknowledge success", http.StatusBadGateway)
			return
		}
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			http.Error(writer, "fixture requires HTTP/1.1", http.StatusInternalServerError)
			return
		}
		connection, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		digest := sha256.Sum256(body)
		mutex.Lock()
		witness = ResponseLossWitness{TargetRequests: 1, TargetStatus: response.StatusCode, TargetBodyComplete: true, TargetBodyBytes: len(content), RequestBodyBytes: len(body), RequestSHA256: hex.EncodeToString(digest[:]), ClientBytesSent: 0, ConnectionClosed: true, TargetWire: wire}
		mutex.Unlock()
		_ = connection.Close()
	}))
	defer server.Close()
	proxyConfig := client.Config
	proxyConfig.BaseURL = server.URL
	proxyClient, err := NewClient(client.Name+"-response-loss", proxyConfig)
	if err != nil {
		return ResponseLossWitness{}, err
	}
	defer proxyClient.HTTP.CloseIdleConnections()
	proxyClient.HTTP.Transport.(*http.Transport).DisableKeepAlives = true
	proxyClient.HTTP.Jar = nil
	proxyClient.HTTP.Transport = &responseLossTransport{base: proxyClient.HTTP.Transport, token: token, origin: origin, jar: client.HTTP.Jar}
	faultContext := context.WithValue(ctx, faultContextKey{}, requestFault{ID: faultID, Loss: snapshot})
	response, _, sendErr := send(faultContext, proxyClient)
	observed := snapshot()
	observed.ClientEOF = errors.Is(sendErr, io.EOF)
	if !observed.ClientEOF || response.StatusCode != 0 || observed.TargetRequests != 1 || observed.TargetStatus != http.StatusOK || !observed.TargetBodyComplete || observed.TargetBodyBytes == 0 || !observed.ConnectionClosed || !observed.TargetWire.Valid {
		return observed, fmt.Errorf("response loss did not prove one successful target response and zero client response bytes")
	}
	return observed, nil
}

type responseLossTransport struct {
	base   http.RoundTripper
	token  string
	origin *url.URL
	jar    http.CookieJar
}

func (transport *responseLossTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	forwarded := request.Clone(request.Context())
	forwarded.Header = request.Header.Clone()
	forwarded.Header.Set("X-Canvas-Contract-Fault", transport.token)
	forwarded.Header.Del("Cookie")
	if transport.jar != nil {
		for _, cookie := range transport.jar.Cookies(transport.origin) {
			forwarded.AddCookie(cookie)
		}
	}
	return transport.base.RoundTrip(forwarded)
}

func (transport *responseLossTransport) CloseIdleConnections() {
	if closer, ok := transport.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
