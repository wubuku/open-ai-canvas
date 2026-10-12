package harness

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func HTTPGateway(origin string) (http.Handler, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "http" || parsed.Port() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Hostname() != "127.0.0.1" && (!strings.HasPrefix(parsed.Hostname(), "canvas-contract-target-") || strings.Contains(parsed.Hostname(), ".")) {
		return nil, errors.New("gateway requires one fixed private contract-target HTTP origin")
	}
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	proxy.Transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 40 * time.Second}
	proxy.ErrorLog = log.New(io.Discard, "", 0)
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(writer, "private contract target unavailable", http.StatusBadGateway)
	}
	return proxy, nil
}
