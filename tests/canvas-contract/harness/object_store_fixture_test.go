package harness

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestObjectStoreFixtureResolvesPathStyleOriginAndCDNKeys(test *testing.T) {
	fixture := NewObjectStoreFixture()
	defer fixture.Close()
	content := []byte("fixture-object")

	request, err := http.NewRequest(http.MethodPut, fixture.URL()+"/canvas-contract/contract/object.txt", bytes.NewReader(content))
	if err != nil {
		test.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		test.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		test.Fatalf("path-style PUT status = %d", response.StatusCode)
	}

	response, err = http.Get(fixture.URL() + "/contract/object.txt")
	if err != nil {
		test.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		test.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !bytes.Equal(body, content) {
		test.Fatalf("CDN object response = status %d body %q", response.StatusCode, string(body))
	}
}
