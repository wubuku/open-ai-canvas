package canvas_contract_test

import (
	"bytes"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"canvas-contract/runner"
)

type describedResourceAccess struct {
	ResourceID       string     `json:"resourceId"`
	RequestedVariant string     `json:"requestedVariant"`
	ActualVariant    string     `json:"actualVariant"`
	URL              string     `json:"url"`
	Delivery         string     `json:"delivery"`
	IssuedAt         time.Time  `json:"issuedAt"`
	ExpiresAt        *time.Time `json:"expiresAt"`
	RefreshAt        time.Time  `json:"refreshAt"`
	Revision         string     `json:"revision"`
}

func resourceDescriptionPath(test *testing.T, path, variant string) string {
	test.Helper()
	parsed, err := url.Parse(path)
	if err != nil {
		test.Fatal("invalid description fixture path")
	}
	query := parsed.Query()
	query.Set("access", "1")
	if variant != "" {
		query.Set("variant", variant)
	}
	parsed.RawQuery = query.Encode()
	return parsed.RequestURI()
}

func requireResourceDescription(test *testing.T, client *runner.Client, path, resourceID, requestedVariant string) describedResourceAccess {
	test.Helper()
	response, envelope, err := client.Do(runner.Context(test), http.MethodGet, path, nil, http.Header{"Range": {"bytes=999999999-"}, "If-Modified-Since": {"Thu, 01 Jan 2099 00:00:00 GMT"}})
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	access := runner.DecodeData[struct {
		Access describedResourceAccess `json:"access"`
	}](test, envelope).Access
	if access.ResourceID != resourceID || access.RequestedVariant != requestedVariant || access.ActualVariant != "original" || access.Delivery != "platform-local" || access.Revision == "" || access.ExpiresAt == nil || !access.ExpiresAt.After(time.Now()) || !access.RefreshAt.After(access.IssuedAt) || !access.RefreshAt.Before(*access.ExpiresAt) {
		test.Fatal("local description changed identity, variant, delivery or authorization timing")
	}
	return access
}

func localDescriptionAccessPath(test *testing.T, owner *runner.Client, access describedResourceAccess) string {
	test.Helper()
	parsed, err := url.Parse(access.URL)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" || parsed.Path != "/api/public/resources/"+access.ResourceID+"/file" {
		test.Fatal("local description did not provide a safe signed resource path")
	}
	origin, err := url.Parse(owner.Config.BaseURL)
	if err != nil || parsed.IsAbs() && (parsed.Scheme != origin.Scheme || parsed.Host != origin.Host) || !parsed.IsAbs() && parsed.Host != "" {
		test.Fatal("local description changed the target origin")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || len(query["signature"]) != 1 || query.Get("signature") == "" || len(query["expires"]) != 1 || query.Get("expires") == "" || query.Get("variant") != access.ActualVariant {
		test.Fatal("local description did not bind its signed access authorization")
	}
	return parsed.RequestURI()
}

func requireResourcePathBytes(test *testing.T, client *runner.Client, path string, content []byte) runner.Response {
	test.Helper()
	response, err := client.DoRaw(runner.Context(test), http.MethodGet, path, nil, "", nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusOK)
	if !bytes.Equal(response.Body, content) {
		test.Fatal("description altered resource bytes or confused them with a JSON envelope")
	}
	return response
}

func runPrivateResourceDescription(test *testing.T, owner, foreign *runner.Client) {
	content := []byte(`{"code":0,"data":{"access":{"url":"file bytes, not an API response"}},"msg":"file"}`)
	resourceID := uploadResource(test, owner, "description-fixture.txt", "file", content)
	path := "/api/resources/" + resourceID + "/file"
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	_, envelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	metadata := runner.DecodeData[map[string]any](test, envelope)
	initial := requireResourcePathBytes(test, owner, path, content)
	anonymous, err := runner.NewClient("anonymous", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	defer anonymous.HTTP.CloseIdleConnections()
	var revision string
	for _, variant := range []string{"original", "thumbnail", "playback"} {
		descriptionPath := resourceDescriptionPath(test, path, variant)
		access := requireResourceDescription(test, owner, descriptionPath, resourceID, variant)
		if revision != "" && revision != access.Revision {
			test.Fatal("description changed the underlying resource revision")
		}
		revision = access.Revision
		requireResourcePathBytes(test, anonymous, localDescriptionAccessPath(test, owner, access), content)
		requireHTTPError(test, foreign, http.MethodGet, descriptionPath, nil, http.StatusNotFound)
		requireHTTPError(test, anonymous, http.MethodGet, descriptionPath, nil, http.StatusUnauthorized)
	}
	requireHTTPError(test, owner, http.MethodGet, resourceDescriptionPath(test, "/api/resources/missing-description-resource/file", "original"), nil, http.StatusNotFound)
	requireHTTPError(test, owner, http.MethodGet, resourceDescriptionPath(test, path, "invalid"), nil, http.StatusBadRequest)
	requireResourcePathBytes(test, owner, path+"?access=0", content)
	requireResourcePathBytes(test, owner, path+"?access=0&access=1", content)
	conditional, err := owner.DoRaw(runner.Context(test), http.MethodGet, path, nil, "", http.Header{"If-Modified-Since": {initial.Header.Get("Last-Modified")}})
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, conditional, http.StatusNotModified)
	_, envelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	if !reflect.DeepEqual(runner.DecodeData[map[string]any](test, envelope), metadata) {
		test.Fatal("description mutated persistent resource metadata")
	}
	requireResourceState(test, owner, before)
	requireResourceState(test, foreign, foreignBefore)
}

func runPublicResourceDescription(test *testing.T, owner, anonymous *runner.Client, resourceID, signedPath string, content []byte) {
	before := resourceState(test, owner)
	_, metadataEnvelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	metadata := runner.DecodeData[map[string]any](test, metadataEnvelope)
	parsed, err := url.Parse(signedPath)
	if err != nil {
		test.Fatal("invalid public description fixture")
	}
	named := *parsed
	named.Path += "/description-name.txt"
	for _, path := range []string{signedPath, named.RequestURI()} {
		access := requireResourceDescription(test, anonymous, resourceDescriptionPath(test, path, ""), resourceID, "original")
		requireResourcePathBytes(test, anonymous, localDescriptionAccessPath(test, owner, access), content)
		requireResourcePathBytes(test, anonymous, path, content)
		invalid, err := url.Parse(resourceDescriptionPath(test, path, ""))
		if err != nil {
			test.Fatal("invalid public authorization fixture")
		}
		authorizedQuery := invalid.RawQuery
		for _, mutation := range []string{"missing", "invalid", "tampered-expiry"} {
			query, err := url.ParseQuery(authorizedQuery)
			if err != nil {
				test.Fatal("invalid public authorization query fixture")
			}
			switch mutation {
			case "missing":
				query.Del("signature")
			case "invalid":
				query.Set("signature", "invalid-signature")
			case "tampered-expiry":
				query.Set("expires", "1")
			}
			invalid.RawQuery = query.Encode()
			requireHTTPError(test, anonymous, http.MethodGet, invalid.RequestURI(), nil, http.StatusForbidden)
		}
	}
	_, metadataEnvelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/resources/"+resourceID, nil)
	if !reflect.DeepEqual(runner.DecodeData[map[string]any](test, metadataEnvelope), metadata) {
		test.Fatal("public description mutated resource metadata")
	}
	requireResourceState(test, owner, before)
}

func runSharedResourceDescription(test *testing.T, owner, foreign, anonymous *runner.Client, runID string) {
	content := runner.ImageFixture(test)
	resourceID := uploadResource(test, owner, "share-description.png", "image", content)
	createImageAsset(test, owner, resourceID, len(content))
	excludedID := uploadResource(test, foreign, "excluded-description.png", "image", content)
	canvasID := "share-description-" + runID
	project := runner.FixtureProject(canvasID, "Share description", 0, false)
	attachResource(project, resourceID)
	createCanvas(test, owner, canvasID, project)
	share := createShare(test, owner, canvasID, 1, false)
	path := "/api/public/canvas-shares/" + share.Token + "/resources/" + resourceID + "/file"
	before := resourceState(test, owner)
	foreignBefore := resourceState(test, foreign)
	_, canvasEnvelope := requireHTTPSuccess(test, owner, http.MethodGet, "/api/canvas-projects/"+canvasID, nil)
	canvas := runner.DecodeData[map[string]any](test, canvasEnvelope)
	access := requireResourceDescription(test, anonymous, resourceDescriptionPath(test, path, "original"), resourceID, "original")
	if share.ExpiresAt == nil || access.ExpiresAt.After(*share.ExpiresAt) {
		test.Fatal("share description exceeded the share expiry")
	}
	signedPath := localDescriptionAccessPath(test, owner, access)
	requireResourcePathBytes(test, anonymous, path, content)
	requireResourcePathBytes(test, anonymous, signedPath, content)
	requireHTTPError(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+share.Token+"/resources/"+excludedID+"/file?access=1", nil, http.StatusNotFound)
	requireHTTPError(test, anonymous, http.MethodGet, "/api/public/canvas-shares/invalid/resources/"+resourceID+"/file?access=1", nil, http.StatusNotFound)
	requireHTTPSuccess(test, owner, http.MethodDelete, "/api/canvas-projects/"+canvasID+"/share", nil)
	requireHTTPError(test, anonymous, http.MethodGet, resourceDescriptionPath(test, path, "original"), nil, http.StatusNotFound)
	requireHTTPError(test, anonymous, http.MethodGet, path, nil, http.StatusNotFound)
	requireResourcePathBytes(test, anonymous, signedPath, content)
	requireResourcePathBytes(test, owner, "/api/resources/"+resourceID+"/file", content)
	if current := readShare(test, owner, canvasID); current.Enabled {
		test.Fatal("share description or derived access reenabled a revoked share")
	}
	getCanvasAndAssert(test, owner, canvasID, 1, "Share description")
	_, canvasEnvelope = requireHTTPSuccess(test, owner, http.MethodGet, "/api/canvas-projects/"+canvasID, nil)
	if !reflect.DeepEqual(runner.DecodeData[map[string]any](test, canvasEnvelope), canvas) {
		test.Fatal("description or revocation mutated the complete canvas document")
	}
	requireResourceState(test, owner, before)
	requireResourceState(test, foreign, foreignBefore)
}
