package harness

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestResourceFixtureSlowResponseCancellation(test *testing.T) {
	fixture, err := NewResourceFixture()
	if err != nil {
		test.Fatal(err)
	}
	defer fixture.Close()
	requestContext, cancel := context.WithTimeout(test.Context(), 200*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, fixture.Server.URL+"/slow", nil)
	if err != nil {
		test.Fatal(err)
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Do(request)
	if response != nil {
		_ = response.Body.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) || fixture.Hits("/slow") != 1 {
		test.Fatal("slow fixture did not receive exactly one request and withhold its response until cancellation")
	}
	select {
	case <-fixture.SlowCancellation():
	case <-time.After(2 * time.Second):
		test.Fatal("slow fixture did not observe transport cancellation")
	}
}

func TestResourceFixtureDeterministicContentAndFailures(test *testing.T) {
	fixture, err := NewResourceFixture()
	if err != nil {
		test.Fatal(err)
	}
	defer fixture.Close()
	picture, err := png.Decode(bytes.NewReader(fixture.PNG))
	if err != nil || picture.Bounds().Dx() != 3 || picture.Bounds().Dy() != 2 {
		test.Fatal("fixture must be a decodable 3x2 PNG")
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for _, path := range []string{"/image.png", "/note.txt", "/different-size.txt", "/same-size.txt", "/redirect-image", "/redirect-private", "/redirect-loop", "/unavailable", "/truncated"} {
		response, err := client.Get(fixture.Server.URL + path)
		if err != nil {
			test.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if fixture.Hits(path) != 1 {
			test.Fatal("fixture request accounting")
		}
		switch path {
		case "/image.png":
			if !bytes.Equal(body, fixture.PNG) || response.Header.Get("Content-Type") != "text/plain" {
				test.Fatal("PNG fixture bytes and misleading type")
			}
		case "/note.txt":
			if !bytes.Equal(body, fixture.Text) {
				test.Fatal("text fixture bytes")
			}
		case "/different-size.txt":
			if len(body) != len(fixture.Text)+1 {
				test.Fatal("different-size fixture")
			}
		case "/same-size.txt":
			if len(body) != len(fixture.Text) || bytes.Equal(body, fixture.Text) {
				test.Fatal("same-size different-content fixture")
			}
		case "/truncated":
			if readErr != io.ErrUnexpectedEOF {
				test.Fatalf("truncated body error = %v", readErr)
			}
		case "/unavailable":
			if response.StatusCode != http.StatusServiceUnavailable {
				test.Fatal("unavailable fixture status")
			}
		default:
			if response.StatusCode != http.StatusFound {
				test.Fatal("fixture must return redirect without following it")
			}
		}
	}
}
