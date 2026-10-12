package harness

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

type ObjectStoreFixture struct {
	server *httptest.Server
	mu     sync.Mutex
	data   map[string][]byte
	fail   map[string]bool
}

func NewObjectStoreFixture() *ObjectStoreFixture {
	fixture := &ObjectStoreFixture{data: make(map[string][]byte), fail: make(map[string]bool)}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.handle))
	return fixture
}

func (fixture *ObjectStoreFixture) URL() string {
	return fixture.server.URL
}

func (fixture *ObjectStoreFixture) Close() {
	fixture.server.Close()
}

func (fixture *ObjectStoreFixture) SetFailure(method string, enabled bool) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.fail[strings.ToUpper(strings.TrimSpace(method))] = enabled
}

func (fixture *ObjectStoreFixture) ObjectCount() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return len(fixture.data)
}

func (fixture *ObjectStoreFixture) handle(writer http.ResponseWriter, request *http.Request) {
	key := strings.TrimPrefix(request.URL.Path, "/")
	if key == "" {
		http.Error(writer, "object key is required", http.StatusBadRequest)
		return
	}
	method := strings.ToUpper(request.Method)
	fixture.mu.Lock()
	failed := fixture.fail[method]
	fixture.mu.Unlock()
	if failed {
		writer.Header().Set("Content-Type", "application/xml")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(writer, "<Error><Code>SlowDown</Code><Message>contract fixture failure</Message></Error>")
		return
	}

	switch method {
	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(request.Body, 64<<20))
		if err != nil {
			http.Error(writer, "read failed", http.StatusBadRequest)
			return
		}
		fixture.mu.Lock()
		fixture.data[key] = append([]byte(nil), body...)
		fixture.mu.Unlock()
		digest := md5.Sum(body)
		writer.Header().Set("ETag", `"`+hex.EncodeToString(digest[:])+`"`)
		writer.WriteHeader(http.StatusOK)
	case http.MethodGet:
		fixture.mu.Lock()
		body, ok := fixture.data[key]
		if !ok {
			for storedKey, storedBody := range fixture.data {
				if strings.HasSuffix(storedKey, "/"+key) {
					body = storedBody
					ok = true
					break
				}
			}
		}
		body = append([]byte(nil), body...)
		fixture.mu.Unlock()
		if !ok {
			http.Error(writer, "not found", http.StatusNotFound)
			return
		}
		serveObjectRange(writer, request, body)
	case http.MethodDelete:
		fixture.mu.Lock()
		delete(fixture.data, key)
		fixture.mu.Unlock()
		writer.WriteHeader(http.StatusNoContent)
	default:
		writer.Header().Set("Allow", "GET, PUT, DELETE")
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func serveObjectRange(writer http.ResponseWriter, request *http.Request, body []byte) {
	writer.Header().Set("Accept-Ranges", "bytes")
	start, end, ranged, valid := parseObjectRange(request.Header.Get("Range"), int64(len(body)))
	if !valid {
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(body)))
		writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if !ranged {
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body)
		return
	}
	part := body[start : end+1]
	writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
	writer.Header().Set("Content-Length", strconv.Itoa(len(part)))
	writer.WriteHeader(http.StatusPartialContent)
	_, _ = writer.Write(part)
}

func parseObjectRange(value string, size int64) (int, int, bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, false, true
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return 0, 0, true, false
	}
	parts := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)
	if len(parts) != 2 || parts[0] == "" {
		return 0, 0, true, false
	}
	start64, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start64 < 0 || start64 >= size {
		return 0, 0, true, false
	}
	end64 := size - 1
	if parts[1] != "" {
		end64, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end64 < start64 {
			return 0, 0, true, false
		}
		if end64 >= size {
			end64 = size - 1
		}
	}
	return int(start64), int(end64), true, true
}
