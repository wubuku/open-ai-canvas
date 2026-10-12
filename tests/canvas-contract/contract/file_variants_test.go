package contract

import (
	"net/http"
	"strings"
	"testing"
)

func accessFixture() map[string]any {
	return map[string]any{
		"resourceId": "fixture", "requestedVariant": "original", "actualVariant": "original",
		"url": "/api/public/resources/fixture/file?signature=secret", "delivery": "platform-local",
		"issuedAt": "2026-10-12T00:00:00Z", "expiresAt": "2026-10-12T00:01:00Z",
		"refreshAt": "2026-10-12T00:00:48Z", "revision": "fixture-revision",
	}
}

func accessHeaders() http.Header {
	return http.Header{"Cache-Control": {"private, no-store"}, "Referrer-Policy": {"no-referrer"}}
}

func TestFileDescriptionIsBoundToTheRequestNotContentType(test *testing.T) {
	for _, path := range []string{
		"/api/resources/fixture/file", "/api/public/resources/fixture/file",
		"/api/public/resources/fixture/file/name", "/api/public/canvas-shares/fixture/resources/fixture/file",
	} {
		body := encodedSuccess(test, map[string]any{"access": accessFixture()})
		for _, query := range []string{"?access=1", "?signature=secret&access=%31", "?access=1&access=0"} {
			requestPath := path + query
			evidence, err := ValidateResponse("GET", requestPath, 200, "application/json", body, accessHeaders())
			if err != nil || !evidence.Valid || evidence.Mode != "access-description" || evidence.Schema != "resource.v1.schema.json#/$defs/access-single" || !RequiresJSON("GET", requestPath, 200, "application/json") || RequestVariant("GET", requestPath) != "access-description" {
				test.Fatal("file description was not selected and validated by its explicit request variant")
			}
		}
		for _, query := range []string{"", "?access=0", "?access=0&access=1", "?access=2"} {
			evidence, err := ValidateResponse("GET", path+query, 200, "application/json", body)
			if err != nil || evidence.Mode != "binary" || RequiresJSON("GET", path+query, 200, "application/json") || RequestVariant("GET", path+query) != "" {
				test.Fatal("JSON file bytes were incorrectly reinterpreted as a business envelope")
			}
		}
	}
	if RequestVariant("GET", "/api/resources/fixture?access=1") != "" {
		test.Fatal("description variant escaped its file route bindings")
	}
}

func TestFileDescriptionRejectsMalformedDataAndConditionalOrRedirectResponses(test *testing.T) {
	path := "/api/resources/fixture/file?access=1&signature=secret"
	for field := range accessFixture() {
		for _, wrongType := range []bool{false, true} {
			access := accessFixture()
			delete(access, field)
			if wrongType {
				access[field] = []any{"secret"}
			}
			evidence, err := ValidateResponse("GET", path, 200, "application/json", encodedSuccess(test, map[string]any{"access": access}), accessHeaders())
			if err == nil || evidence.Valid || strings.Contains(err.Error(), "secret") {
				test.Fatal("invalid description was accepted or its values leaked")
			}
		}
	}
	for _, status := range []int{204, 206, 304, 307} {
		if evidence, err := ValidateResponse("GET", path, status, "application/json", nil, accessHeaders()); err == nil || evidence.Valid {
			test.Fatal("description request accepted a binary, conditional or redirect status")
		}
	}
	for _, header := range []http.Header{nil, {}, {"Cache-Control": {"public"}, "Referrer-Policy": {"no-referrer"}}} {
		if _, err := ValidateResponse("GET", path, 200, "application/json", encodedSuccess(test, map[string]any{"access": accessFixture()}), header); err == nil {
			test.Fatal("description omitted its private no-store/referrer protections")
		}
	}
	for _, status := range []int{401, 403, 404} {
		evidence, err := ValidateResponse("GET", path, status, "application/json", []byte(`{"code":1,"data":null,"msg":"denied","reason":"not_found"}`))
		if err != nil || !evidence.Valid || evidence.Mode != "json" || evidence.Schema != "common.v1.schema.json#/$defs/error-envelope" {
			test.Fatal("description authorization errors were not validated as errors")
		}
	}
}

func TestFileRedirectRequiresExplicitBindingSafeLocationAndEmptyBody(test *testing.T) {
	header := accessHeaders()
	header.Set("Location", "https://fixture.invalid/file?signature=secret")
	header.Set("X-Content-Type-Options", "nosniff")
	for _, path := range []string{
		"/api/resources/fixture/file", "/api/public/resources/fixture/file",
		"/api/public/resources/fixture/file/name", "/api/public/canvas-shares/fixture/resources/fixture/file",
	} {
		evidence, err := ValidateResponse("GET", path, 307, "", nil, header)
		if err != nil || !evidence.Valid || evidence.Mode != "redirect" || evidence.Schema != "" || RequiresJSON("GET", path, 307, "") {
			test.Fatal("explicit file redirect contract was not applied")
		}
	}
	for _, location := range []string{"", "/relative", "//fixture.invalid/file", "https://user:secret@fixture.invalid/file", "javascript:secret", "https://fixture.invalid/file#secret", "https:///secret", "https://fixture.invalid/secret space"} {
		invalidHeader := header.Clone()
		invalidHeader.Set("Location", location)
		if evidence, err := ValidateResponse("GET", "/api/resources/fixture/file", 307, "", nil, invalidHeader); err == nil || evidence.Valid || strings.Contains(err.Error(), "secret") {
			test.Fatal("unsafe redirect accepted or exposed its Location")
		}
	}
	for _, field := range []string{"Location", "Cache-Control", "Referrer-Policy", "X-Content-Type-Options"} {
		invalidHeader := header.Clone()
		invalidHeader.Del(field)
		if _, err := ValidateResponse("GET", "/api/resources/fixture/file", 307, "", nil, invalidHeader); err == nil {
			test.Fatal("redirect accepted missing required headers")
		}
	}
	duplicate := header.Clone()
	duplicate.Add("Location", "https://other.invalid/file")
	if _, err := ValidateResponse("GET", "/api/resources/fixture/file", 307, "", nil, duplicate); err == nil {
		test.Fatal("multiple redirect targets were accepted")
	}
	for _, path := range []string{"/api/resources/fixture", "/api/canvas-projects/fixture", "/api/resources/fixture/file?access=1"} {
		if _, err := ValidateResponse("GET", path, 307, "", nil, header); err == nil {
			test.Fatal("redirect escaped its explicit byte-delivery contract")
		}
	}
	if _, err := ValidateResponse("GET", "/api/resources/fixture/file", 307, "text/html", []byte("secret body"), header); err == nil || strings.Contains(err.Error(), "secret") {
		test.Fatal("redirect accepted a body or leaked it")
	}
}

func TestFilePrivacyContractChecksDirectivesWithoutFreezingGoHeaderOrder(test *testing.T) {
	body := encodedSuccess(test, map[string]any{"access": accessFixture()})
	for _, values := range [][]string{{"private, no-store"}, {"no-store, private"}, {" Private , NO-STORE "}, {"private", "no-store"}} {
		header := accessHeaders()
		header["Cache-Control"] = values
		if _, err := ValidateResponse("GET", "/api/resources/fixture/file?access=1", 200, "application/json", body, header); err != nil {
			test.Fatal("equivalent privacy directives were incorrectly bound to Go header serialization")
		}
	}
	for _, values := range [][]string{{"no-cache"}, {"private"}, {"no-store"}, {"private no-store"}, {"private, no-store, public"}, {"private, no-store", "public"}, {"private, no-store=invalid"}} {
		header := accessHeaders()
		header["Cache-Control"] = values
		if _, err := ValidateResponse("GET", "/api/resources/fixture/file?access=1", 200, "application/json", body, header); err == nil {
			test.Fatal("description accepted missing or contradictory privacy directives")
		}
	}
}
