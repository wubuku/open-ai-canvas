package harness

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPGatewayPreservesHTTPAndCookieWithoutChangingDestination(test *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != "/api/import?fixture=synthetic" || request.Method != http.MethodPost || request.Header.Get("Cookie") != "session=synthetic" {
			test.Error("gateway changed HTTP request contract")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Set-Cookie", "session=updated; HttpOnly")
		writer.WriteHeader(http.StatusConflict)
		_, _ = writer.Write([]byte(`{"code":409,"data":null,"msg":"fixture"}`))
	}))
	defer target.Close()
	handler, err := HTTPGateway(target.URL)
	if err != nil {
		test.Fatal(err)
	}
	gateway := httptest.NewServer(handler)
	defer gateway.Close()
	request, _ := http.NewRequest(http.MethodPost, gateway.URL+"/api/import?fixture=synthetic", strings.NewReader(`{}`))
	request.Header.Set("Cookie", "session=synthetic")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusConflict || response.Header.Get("Set-Cookie") != "session=updated; HttpOnly" || !strings.Contains(string(body), `"code":409`) {
		test.Fatal("gateway changed response status/cookie/envelope")
	}
	for _, origin := range []string{"http://outside.invalid:8080", "http://canvas-contract-target-outside.invalid:8080", "http://user:secret@127.0.0.1:8080", "http://127.0.0.1:8080/api", "http://127.0.0.1:8080?token=secret"} {
		if _, err := HTTPGateway(origin); err == nil || strings.Contains(err.Error(), "secret") {
			test.Fatal("gateway accepted an unsafe origin or leaked credentials")
		}
	}
}
