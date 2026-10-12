package canvas_contract_test

import (
	"bytes"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"canvas-contract/runner"
)

type timedResourceAccess struct {
	URL       string    `json:"url"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	RefreshAt time.Time `json:"refreshAt"`
	Revision  string    `json:"revision"`
}

func readTimedResourceAccess(test *testing.T, owner *runner.Client, resourceID string) timedResourceAccess {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/resources/access", []any{map[string]any{"resourceId": resourceID, "purpose": "display", "variant": "original"}})
	result := runner.DecodeData[struct {
		Items []struct {
			ResourceID string               `json:"resourceId"`
			Access     *timedResourceAccess `json:"access"`
		} `json:"items"`
	}](test, envelope)
	if len(result.Items) != 1 || result.Items[0].ResourceID != resourceID || result.Items[0].Access == nil {
		test.Fatal("timed access did not contain one authorized resource outcome")
	}
	access := *result.Items[0].Access
	lifetime := access.ExpiresAt.Sub(access.IssuedAt)
	if access.URL == "" || access.Revision == "" || lifetime <= 45*time.Second || lifetime > time.Minute || !access.RefreshAt.Equal(access.IssuedAt.Add(lifetime*4/5)) {
		test.Fatal("timed access did not preserve the configured one-minute lifetime and refresh ordering")
	}
	return access
}

func signedAccessRequestPath(test *testing.T, owner *runner.Client, resourceID string, access timedResourceAccess) string {
	test.Helper()
	parsed, err := url.Parse(access.URL)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" || parsed.Path != "/api/public/resources/"+resourceID+"/file" {
		test.Fatal("timed local access did not provide a safe public resource path")
	}
	if parsed.IsAbs() {
		origin, err := url.Parse(owner.Config.BaseURL)
		if err != nil || parsed.Scheme != origin.Scheme || parsed.Host != origin.Host {
			test.Fatal("timed local access did not preserve the target origin")
		}
	} else if parsed.Host != "" {
		test.Fatal("timed local access provided a protocol-relative origin")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || len(query) != 3 || len(query["expires"]) != 1 || query.Get("expires") != strconv.FormatInt(access.ExpiresAt.Unix(), 10) || len(query["signature"]) != 1 || query.Get("signature") == "" || len(query["variant"]) != 1 || query.Get("variant") != "original" {
		test.Fatal("timed local access did not bind its authorization to the requested variant and expiry")
	}
	return parsed.RequestURI()
}

func waitUntilResourceBoundary(test *testing.T, deadline time.Time) {
	test.Helper()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-test.Context().Done():
		test.Fatal("test context ended while waiting for the real resource expiry boundary")
	}
}

func runResourceRealExpiryAndConditionalCache(test *testing.T, admin *runner.Client) {
	owner := policyFixtureUser(test, admin, "resource-expiry-cache")
	withResourcePolicy(test, admin, func(setting map[string]any) {
		setting["storage"].(map[string]any)["accessURLTTLSeconds"] = 60
	})
	content := runner.ImageFixture(test)
	resourceID := uploadResource(test, owner, "expiry.png", "image", content)
	before := resourceState(test, owner)
	_, metadataEnvelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	metadata := runner.DecodeData[map[string]any](test, metadataEnvelope)
	access := readTimedResourceAccess(test, owner, resourceID)
	windowEnd := access.IssuedAt.Truncate(15 * time.Second).Add(15 * time.Second)
	if time.Until(windowEnd) < 5*time.Second {
		waitUntilResourceBoundary(test, windowEnd.Add(time.Second))
		access = readTimedResourceAccess(test, owner, resourceID)
	}
	replay := readTimedResourceAccess(test, owner, resourceID)
	if !replay.IssuedAt.Truncate(15 * time.Second).Equal(access.IssuedAt.Truncate(15 * time.Second)) {
		test.Fatal("target response latency prevented observation of the configured signing window")
	}
	if replay.URL != access.URL || replay.Revision != access.Revision || !replay.ExpiresAt.Equal(access.ExpiresAt) {
		test.Fatal("same-window access did not preserve its signed URL and resource revision")
	}
	path := signedAccessRequestPath(test, owner, resourceID, access)
	public, err := runner.NewClient("public-expiry-cache", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	defer public.HTTP.CloseIdleConnections()
	response, err := public.DoRaw(runner.Context(test), http.MethodGet, path, nil, "", nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusOK)
	if !bytes.Equal(response.Body, content) {
		test.Fatal("unexpired public resource returned different bytes")
	}
	runner.RequireHeader(test, response.Header, "Cache-Control", "public, max-age=0, must-revalidate")
	modified := response.Header.Get("Last-Modified")
	if _, err := http.ParseTime(modified); err != nil {
		test.Fatal("local delivery did not return a valid conditional-request timestamp")
	}
	headers := http.Header{"If-Modified-Since": {modified}}
	response, err = public.DoRaw(runner.Context(test), http.MethodGet, path, nil, "", headers)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusNotModified)
	if len(response.Body) != 0 {
		test.Fatal("conditional public resource response was not empty")
	}
	waitUntilResourceBoundary(test, access.ExpiresAt.Add(2*time.Second))
	for _, conditional := range []http.Header{nil, headers} {
		response, envelope, err := public.Do(runner.Context(test), http.MethodGet, path, nil, conditional)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusForbidden, "forbidden")
	}
	renewed := readTimedResourceAccess(test, owner, resourceID)
	if renewed.URL == access.URL || !renewed.ExpiresAt.After(access.ExpiresAt) || renewed.Revision != access.Revision {
		test.Fatal("post-expiry renewal did not change the authorization without changing the resource revision")
	}
	response, err = public.DoRaw(runner.Context(test), http.MethodGet, signedAccessRequestPath(test, owner, resourceID, renewed), nil, "", headers)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusNotModified)
	response, err = public.DoRaw(runner.Context(test), http.MethodGet, signedAccessRequestPath(test, owner, resourceID, renewed), nil, "", nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusOK)
	if !bytes.Equal(response.Body, content) {
		test.Fatal("authorization renewal changed the stored bytes")
	}
	_, metadataEnvelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	if !reflect.DeepEqual(metadata, runner.DecodeData[map[string]any](test, metadataEnvelope)) {
		test.Fatal("expiry or access renewal changed resource metadata")
	}
	requireResourceState(test, owner, before)
}
