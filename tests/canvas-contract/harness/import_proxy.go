package harness

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"canvas-contract/fixtures"
)

const ImportFixtureToken = "contract-fixture-only-not-a-real-token"

var importFixtureModes = []string{"success", "http-error", "business-error", "invalid-json", "oversized", "redirect", "truncated", "no-copy", "empty", "timeout", "rich-media", "node-limit", "node-over-limit", "edge-limit", "edge-over-limit", "no-read"}

type ImportProxy struct {
	CA          []byte
	certificate tls.Certificate
	mutex       sync.Mutex
	connections map[net.Conn]bool
	closed      chan struct{}
	closeOnce   sync.Once
}

func NewImportProxy() (*ImportProxy, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	issuer := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Canvas contract ephemeral fixture CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	issuerDER, err := x509.CreateCertificate(rand.Reader, issuer, issuer, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Canvas import contract fixture"},
		NotBefore: issuer.NotBefore, NotAfter: issuer.NotAfter,
		DNSNames: []string{"api.liblib.tv", "app.tapnow.media", "files.tapnow.media"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, issuer, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	privateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	certificate, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKey}))
	if err != nil {
		return nil, err
	}
	return &ImportProxy{CA: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: issuerDER}), certificate: certificate, connections: make(map[net.Conn]bool), closed: make(chan struct{})}, nil
}

func (proxy *ImportProxy) Close() {
	proxy.closeOnce.Do(func() {
		close(proxy.closed)
		proxy.mutex.Lock()
		defer proxy.mutex.Unlock()
		for connection := range proxy.connections {
			_ = connection.Close()
		}
	})
}

func (proxy *ImportProxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodConnect || request.Host != "api.liblib.tv:443" && request.Host != "app.tapnow.media:443" && request.Host != "files.tapnow.media:443" {
		http.Error(writer, "fixture only accepts CONNECT for the fixed import and media origins", http.StatusForbidden)
		return
	}
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "fixture requires HTTP/1.1 CONNECT", http.StatusInternalServerError)
		return
	}
	connection, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer connection.Close()
	proxy.mutex.Lock()
	select {
	case <-proxy.closed:
		proxy.mutex.Unlock()
		return
	default:
		proxy.connections[connection] = true
	}
	proxy.mutex.Unlock()
	defer func() {
		proxy.mutex.Lock()
		delete(proxy.connections, connection)
		proxy.mutex.Unlock()
	}()
	_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	if err := buffered.Flush(); err != nil {
		return
	}
	secure := tls.Server(connection, &tls.Config{Certificates: []tls.Certificate{proxy.certificate}, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if err := secure.Handshake(); err != nil {
		return
	}
	if secure.ConnectionState().ServerName != strings.TrimSuffix(request.Host, ":443") {
		return
	}
	reader := bufio.NewReader(secure)
	for {
		_ = connection.SetDeadline(time.Now().Add(45 * time.Second))
		upstreamRequest, err := http.ReadRequest(reader)
		if err != nil {
			return
		}
		_ = upstreamRequest.Body.Close()
		response := proxy.response(upstreamRequest, strings.TrimSuffix(request.Host, ":443"))
		if response == nil {
			return
		}
		if err := response.Write(secure); err != nil || response.Close {
			return
		}
	}
}

func ImportFixtureID(provider, mode string) (string, error) {
	for index, candidate := range importFixtureModes {
		if candidate != mode {
			continue
		}
		if provider == "libtv" {
			return fmt.Sprintf("%032x", index), nil
		}
		if provider == "tapnow" {
			return "contract_" + strings.ReplaceAll(mode, "-", "_"), nil
		}
	}
	return "", errors.New("unknown import fixture provider or mode")
}

func (proxy *ImportProxy) response(request *http.Request, host string) *http.Response {
	if host == "files.tapnow.media" {
		return importMediaResponse(request, host)
	}
	provider, identity := "", ""
	if request.Method == http.MethodGet && request.Host == host && host == "api.liblib.tv" && request.URL.Path == "/api/canvas/project/detail" && request.Header.Get("token") == ImportFixtureToken {
		provider, identity = "libtv", request.URL.Query().Get("uuid")
	} else if request.Method == http.MethodGet && request.Host == host && host == "app.tapnow.media" && strings.HasPrefix(request.URL.Path, "/api/conversation/v1/canvas-share/") {
		provider, identity = "tapnow", strings.TrimPrefix(request.URL.Path, "/api/conversation/v1/canvas-share/")
	}
	mode := ""
	for _, candidate := range importFixtureModes {
		value, _ := ImportFixtureID(provider, candidate)
		if value != "" && value == identity {
			mode = candidate
		}
	}
	status := http.StatusOK
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	body := []byte(`{"code":77,"msg":"unknown fixture request","data":null}`)
	if mode != "" {
		body = importFixtureBody(provider, identity, mode)
	}
	switch mode {
	case "http-error":
		status = http.StatusServiceUnavailable
	case "invalid-json":
		body = []byte(`{"code":`)
	case "oversized":
		body = bytes.Repeat([]byte("x"), (16<<20)+1)
	case "redirect":
		status = http.StatusFound
		headers.Set("Location", "https://fixture.invalid/must-not-be-contacted")
	case "timeout":
		select {
		case <-proxy.closed:
			return nil
		case <-time.After(40 * time.Second):
		}
	}
	response := &http.Response{StatusCode: status, ProtoMajor: 1, ProtoMinor: 1, Header: headers, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}
	if mode == "truncated" {
		response.ContentLength += 16
		response.Close = true
	}
	return response
}

func importFixtureBody(provider, identity, mode string) []byte {
	if mode == "rich-media" {
		return richImportFixtureBody(provider, identity)
	}
	if provider == "tapnow" && (mode == "node-limit" || mode == "node-over-limit" || mode == "edge-limit" || mode == "edge-over-limit") {
		return importLimitFixtureBody(identity, mode)
	}
	code := 0
	if mode == "business-error" {
		code = 42
	}
	var detail map[string]any
	if provider == "libtv" {
		nodes := []any{
			map[string]any{"nodeKey": "fixture-image", "name": "Fixture image", "data": `{"type":"image","url":["https://fixture.invalid/first.png"],"params":{"prompt":"fixture prompt","model":"fixture model"}}`, "position": map[string]any{"positionX": "12", "positionY": "34"}, "measured": map[string]any{"width": "640", "height": "360"}},
			map[string]any{"nodeKey": "fixture-video", "name": "Fixture placeholder", "data": `{"type":"video","url":[],"taskInfo":{"status":3,"failedReason":"fixture upstream failure"}}`, "position": map[string]any{"positionX": "56", "positionY": "78"}},
			map[string]any{"nodeKey": "fixture-unsupported", "data": `{"type":"unsupported"}`},
		}
		if mode == "empty" {
			nodes = []any{}
		}
		detail = map[string]any{"projectMeta": map[string]any{"uuid": identity, "name": "Contract LibTV fixture", "effective": map[string]any{"canRead": mode != "no-read", "canCopy": mode != "no-copy"}}, "nodeList": nodes, "connectionList": []any{map[string]any{"connectionId": "fixture-link", "source": "fixture-image", "target": "fixture-video"}}}
	} else {
		nodes := []any{
			map[string]any{"id": "fixture-text-1", "type": "text", "data": map[string]any{"title": "Fixture text 1", "text": "contract fixture first text"}, "position": map[string]any{"x": 12, "y": 34}},
			map[string]any{"id": "fixture-text-2", "type": "text", "data": map[string]any{"title": "Fixture text 2", "text": "contract fixture second text"}, "position": map[string]any{"x": 56, "y": 78}},
			map[string]any{"id": "fixture-unsupported", "type": "unsupported", "data": map[string]any{}},
		}
		if mode == "empty" {
			nodes = []any{}
		}
		detail = map[string]any{"id": identity, "name": "Contract TapNow fixture", "nodes": nodes, "connections": []any{map[string]any{"id": "fixture-link", "source": "fixture-text-1", "target": "fixture-text-2"}}}
	}
	encoded, _ := json.Marshal(map[string]any{"code": code, "msg": "contract fixture business result", "data": detail})
	return encoded
}

func importMediaResponse(request *http.Request, host string) *http.Response {
	status := http.StatusNotFound
	body := []byte("unknown controlled import media request")
	mime := "text/plain"
	if request.Method == http.MethodGet && request.Host == host && request.URL.RawQuery == "" {
		if content, contentType, err := fixtures.ImportMedia(request.URL.Path); err == nil {
			status, body, mime = http.StatusOK, content, contentType
		}
	}
	return &http.Response{StatusCode: status, ProtoMajor: 1, ProtoMinor: 1, Header: http.Header{"Content-Type": []string{mime}}, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}
}
