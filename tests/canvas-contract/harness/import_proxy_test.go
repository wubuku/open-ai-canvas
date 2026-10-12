package harness

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"bytes"
	"canvas-contract/fixtures"
)

func TestImportProxyUsesVerifiedTLSAndNeverForwards(test *testing.T) {
	proxy, err := NewImportProxy()
	if err != nil {
		test.Fatal(err)
	}
	server := httptest.NewServer(proxy)
	defer server.Close()
	defer proxy.Close()
	origin, _ := url.Parse(server.URL)
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(proxy.CA) {
		test.Fatal("fixture CA certificate")
	}
	transport := &http.Transport{Proxy: http.ProxyURL(origin), TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for _, provider := range []string{"libtv", "tapnow"} {
		for _, mode := range []string{"success", "business-error", "invalid-json", "redirect", "truncated", "no-copy", "empty", "rich-media"} {
			identity, _ := ImportFixtureID(provider, mode)
			endpoint := "https://app.tapnow.media/api/conversation/v1/canvas-share/" + identity
			if provider == "libtv" {
				endpoint = "https://api.liblib.tv/api/canvas/project/detail?uuid=" + identity
			}
			request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
			request.Header.Set("token", ImportFixtureToken)
			response, err := client.Do(request)
			if err != nil {
				test.Fatal("fixture HTTPS with CA verification failed")
			}
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if mode == "truncated" && readErr != io.ErrUnexpectedEOF {
				test.Fatal("fixture must truncate the declared HTTP body")
			}
			if mode == "success" && (!strings.Contains(string(body), "Contract") || response.TLS == nil || len(response.TLS.VerifiedChains) == 0) {
				test.Fatal("success must use the synthetic fixture and a verified TLS chain")
			}
			if mode == "redirect" && response.StatusCode != http.StatusFound {
				test.Fatal("redirect fixture")
			}
		}
	}
	for _, path := range []string{"/contract/image.png", "/contract/audio.wav", "/contract/video.mp4", "/not-a-fixture", "/contract/image.png?not-allowed=1"} {
		response, err := client.Get(fixtures.ImportMediaOrigin + path)
		if err != nil {
			test.Fatal("controlled media TLS request failed", err)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil || response.TLS == nil || len(response.TLS.VerifiedChains) == 0 {
			test.Fatal("media fixture must use verified TLS with complete bytes")
		}
		if want, mime, fixtureErr := fixtures.ImportMedia(path); fixtureErr == nil {
			if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != mime || !bytes.Equal(body, want) {
				test.Fatal("media fixture did not serve exact bytes/MIME")
			}
		} else if response.StatusCode != http.StatusNotFound {
			test.Fatal("media fixture accepted a non-fixture path/query")
		}
	}
	if _, err := client.Get("https://must-not-be-forwarded.invalid/"); err == nil {
		test.Fatal("fixture accepted an unallowlisted CONNECT destination")
	}
	untrusted := &http.Transport{Proxy: http.ProxyURL(origin), TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}}
	defer untrusted.CloseIdleConnections()
	if _, err := (&http.Client{Transport: untrusted, Timeout: time.Second}).Get("https://app.tapnow.media/"); err == nil {
		test.Fatal("ephemeral fixture CA must not be trusted implicitly")
	}
}

func TestImportFixtureIdentitiesAreDistinct(test *testing.T) {
	for _, provider := range []string{"libtv", "tapnow"} {
		seen := make(map[string]bool)
		for _, mode := range importFixtureModes {
			identity, err := ImportFixtureID(provider, mode)
			if err != nil || seen[identity] || provider == "libtv" && len(identity) != 32 {
				test.Fatal("invalid or duplicate fixed import identity")
			}
			seen[identity] = true
		}
	}
	if _, err := ImportFixtureID("unknown", "rich-media"); err == nil {
		test.Fatal("unknown provider accepted")
	}
}
