package harness

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"

	"canvas-contract/fixtures"
)

type ResourceFixture struct {
	Server         *httptest.Server
	PNG            []byte
	Text           []byte
	mutex          sync.Mutex
	hits           map[string]int
	slowCanceled   chan struct{}
	slowCancelOnce sync.Once
}

func NewResourceFixture() (*ResourceFixture, error) {
	content, err := fixtures.PNG()
	if err != nil {
		return nil, err
	}
	fixture := &ResourceFixture{PNG: content, Text: []byte("contract resource fixture"), hits: make(map[string]int), slowCanceled: make(chan struct{})}
	fixture.Server = httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	return fixture, nil
}

func (fixture *ResourceFixture) Close() {
	fixture.Server.Close()
}

func (fixture *ResourceFixture) Hits(path string) int {
	fixture.mutex.Lock()
	defer fixture.mutex.Unlock()
	return fixture.hits[path]
}

func (fixture *ResourceFixture) SlowCancellation() <-chan struct{} {
	return fixture.slowCanceled
}

func (fixture *ResourceFixture) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	fixture.mutex.Lock()
	fixture.hits[request.URL.Path]++
	fixture.mutex.Unlock()
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch request.URL.Path {
	case "/slow":
		<-request.Context().Done()
		fixture.slowCancelOnce.Do(func() { close(fixture.slowCanceled) })
	case "/image.png":
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write(fixture.PNG)
	case "/note.txt":
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(fixture.Text)
	case "/different-size.txt":
		_, _ = writer.Write(append(append([]byte{}, fixture.Text...), '!'))
	case "/same-size.txt":
		_, _ = writer.Write(bytes.Repeat([]byte("x"), len(fixture.Text)))
	case "/redirect-image":
		http.Redirect(writer, request, "/image.png", http.StatusFound)
	case "/redirect-private":
		http.Redirect(writer, request, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	case "/redirect-loop":
		http.Redirect(writer, request, "/redirect-loop", http.StatusFound)
	case "/unavailable":
		writer.WriteHeader(http.StatusServiceUnavailable)
	case "/truncated":
		writer.Header().Set("Content-Length", strconv.Itoa(len(fixture.Text)+16))
		_, _ = writer.Write(fixture.Text)
	default:
		writer.WriteHeader(http.StatusNotFound)
	}
}
