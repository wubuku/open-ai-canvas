package contract

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/*.json
var schemaFiles embed.FS

const WireVersion = "v1-observed"
const schemaOrigin = "https://canvas-contract.invalid/schemas/"

type Binding struct {
	Method           string `json:"method"`
	Path             string `json:"path"`
	Schema           string `json:"schema"`
	Binary           bool   `json:"binary"`
	AllowNotModified bool   `json:"not_modified"`
	DescriptorSchema string `json:"descriptor_schema"`
	AllowRedirect    bool   `json:"redirect"`
}

type WireEvidence struct {
	Version string `json:"version"`
	Schema  string `json:"schema"`
	Mode    string `json:"mode"`
	Valid   bool   `json:"valid"`
}

type wireValidator struct {
	bindings []Binding
	schemas  map[string]*jsonschema.Schema
}

type offlineLoader struct{}

func (offlineLoader) Load(string) (any, error) {
	return nil, fmt.Errorf("external schema loading is disabled")
}

var loadWireValidator = sync.OnceValues(newWireValidator)

func newWireValidator() (*wireValidator, error) {
	var registry struct {
		Version  string    `json:"version"`
		Bindings []Binding `json:"bindings"`
	}
	encoded, err := schemaFiles.ReadFile("schemas/bindings.v1.json")
	if err != nil || json.Unmarshal(encoded, &registry) != nil || registry.Version != WireVersion {
		return nil, fmt.Errorf("invalid embedded wire registry")
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(offlineLoader{})
	compiler.AssertFormat()
	entries, err := schemaFiles.ReadDir("schemas")
	if err != nil {
		return nil, fmt.Errorf("cannot read embedded schemas")
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		encoded, err := schemaFiles.ReadFile("schemas/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("cannot read schema %s", entry.Name())
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
		if err != nil || compiler.AddResource(schemaOrigin+entry.Name(), document) != nil {
			return nil, fmt.Errorf("invalid schema %s", entry.Name())
		}
	}
	validator := &wireValidator{bindings: registry.Bindings, schemas: make(map[string]*jsonschema.Schema)}
	identities := map[string]bool{}
	for _, binding := range registry.Bindings {
		identity := binding.Method + " " + binding.Path
		if identities[identity] || !strings.HasPrefix(binding.Path, "/api/") || binding.Binary == (binding.Schema != "") || !binding.Binary && (binding.DescriptorSchema != "" || binding.AllowRedirect) {
			return nil, fmt.Errorf("invalid wire binding registry")
		}
		identities[identity] = true
	}
	for _, reference := range append([]string{"common.v1.schema.json", "common.v1.schema.json#/$defs/error-envelope"}, schemaReferences(registry.Bindings)...) {
		compiled, err := compiler.Compile(schemaOrigin + reference)
		if err != nil {
			return nil, fmt.Errorf("cannot compile wire schema %s", reference)
		}
		validator.schemas[reference] = compiled
	}
	sort.SliceStable(validator.bindings, func(first, second int) bool {
		return specificity(validator.bindings[first].Path) > specificity(validator.bindings[second].Path)
	})
	return validator, nil
}

func schemaReferences(bindings []Binding) []string {
	references := []string{}
	for _, binding := range bindings {
		if binding.Schema != "" {
			references = append(references, binding.Schema)
		}
		if binding.DescriptorSchema != "" {
			references = append(references, binding.DescriptorSchema)
		}
	}
	return references
}

func specificity(path string) int {
	count := 0
	for _, segment := range strings.Split(path, "/") {
		if !strings.HasPrefix(segment, "{") {
			count++
		}
	}
	return count
}

func (validator *wireValidator) binding(method, path string) (Binding, bool) {
	path, _, _ = strings.Cut(path, "?")
	path, _, _ = strings.Cut(path, "#")
	segments := strings.Split(path, "/")
	for _, binding := range validator.bindings {
		candidate := strings.Split(binding.Path, "/")
		if binding.Method != method || len(segments) != len(candidate) {
			continue
		}
		matched := true
		for index, segment := range candidate {
			if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
				matched = matched && segments[index] != ""
			} else {
				matched = matched && segments[index] == segment
			}
		}
		if matched {
			return binding, true
		}
	}
	return Binding{}, false
}

func wireMode(method, path string, status int) (WireEvidence, Binding, error) {
	validator, err := loadWireValidator()
	if err != nil {
		return WireEvidence{Version: WireVersion}, Binding{}, err
	}
	binding, _ := validator.binding(method, path)
	descriptor := RequestVariant(method, path) == "access-description"
	evidence := WireEvidence{Version: WireVersion, Schema: "common.v1.schema.json", Mode: "json"}
	if !strings.HasPrefix(path, "/api/") {
		evidence.Schema, evidence.Mode = "", "unbound-raw"
	} else if descriptor && status < 400 {
		if status != http.StatusOK {
			return evidence, binding, fmt.Errorf("wire access-description: expected status 200")
		}
		evidence.Mode = "access-description"
		binding.Schema = binding.DescriptorSchema
	} else if status == http.StatusNotModified {
		evidence.Schema, evidence.Mode = binding.Schema, "not-modified"
	} else if binding.AllowRedirect && status == http.StatusTemporaryRedirect {
		evidence.Schema, evidence.Mode = "", "redirect"
	} else if binding.Binary && status >= 200 && status < 300 {
		evidence.Schema, evidence.Mode = "", "binary"
	} else if binding.Binary && status == http.StatusRequestedRangeNotSatisfiable {
		evidence.Schema, evidence.Mode = "", "range-error"
	} else if status >= 400 {
		evidence.Schema = "common.v1.schema.json#/$defs/error-envelope"
	}
	return evidence, binding, nil
}

func RequestVariant(method, path string) string {
	validator, err := loadWireValidator()
	if err != nil {
		return ""
	}
	binding, matched := validator.binding(method, path)
	parsed, err := url.Parse(path)
	if matched && binding.DescriptorSchema != "" && err == nil && parsed.Query().Get("access") == "1" {
		return "access-description"
	}
	return ""
}

func RequiresJSON(method, path string, status int, contentType string) bool {
	evidence, _, err := wireMode(method, path, status)
	mediaType, _, _ := mime.ParseMediaType(contentType)
	return err == nil && (evidence.Mode == "json" || evidence.Mode == "access-description" || evidence.Mode == "range-error" && mediaType == "application/json")
}

func ValidateResponse(method, path string, status int, contentType string, body []byte, responseHeaders ...http.Header) (WireEvidence, error) {
	evidence, binding, err := wireMode(method, path, status)
	if err != nil {
		return evidence, err
	}
	invalid := func(facet string) (WireEvidence, error) {
		return evidence, fmt.Errorf("wire %s %s: %s", evidence.Mode, evidence.Schema, facet)
	}
	switch evidence.Mode {
	case "not-modified":
		if !(binding.AllowNotModified || binding.Binary) || len(body) != 0 {
			return invalid("unexpected 304 or nonempty body")
		}
	case "binary", "unbound-raw":
	case "redirect":
		if len(body) != 0 || len(responseHeaders) != 1 || len(responseHeaders[0].Values("Location")) != 1 {
			return invalid("redirect requires an empty body and one Location header")
		}
		location := responseHeaders[0].Get("Location")
		parsed, parseErr := url.Parse(location)
		if parseErr != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" || strings.ContainsAny(location, " \t\r\n") {
			return invalid("redirect Location must be an absolute HTTP origin without credentials")
		}
		if !privateAccessHeaders(responseHeaders) || responseHeaders[0].Get("X-Content-Type-Options") != "nosniff" {
			return invalid("redirect privacy headers are missing")
		}
	case "range-error":
		mediaType, _, _ := mime.ParseMediaType(contentType)
		if mediaType == "application/json" {
			return validateJSON(evidence, Binding{}, status, body)
		}
		if mediaType != "text/plain" || len(body) == 0 {
			return invalid("unexpected range-error content type or empty body")
		}
	case "json", "access-description":
		if evidence.Mode == "access-description" && !privateAccessHeaders(responseHeaders) {
			return invalid("access description privacy headers are missing")
		}
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return invalid("expected JSON content type")
		}
		return validateJSON(evidence, binding, status, body)
	}
	evidence.Valid = true
	return evidence, nil
}

func privateAccessHeaders(headers []http.Header) bool {
	if len(headers) != 1 || headers[0].Get("Referrer-Policy") != "no-referrer" {
		return false
	}
	private, noStore := false, false
	for _, value := range headers[0].Values("Cache-Control") {
		for _, directive := range strings.Split(value, ",") {
			switch strings.ToLower(strings.TrimSpace(directive)) {
			case "private":
				private = true
			case "no-store":
				noStore = true
			case "public":
				return false
			}
		}
	}
	return private && noStore
}

func validateJSON(evidence WireEvidence, binding Binding, status int, body []byte) (WireEvidence, error) {
	invalid := func(facet string) (WireEvidence, error) {
		return evidence, fmt.Errorf("wire %s %s: %s", evidence.Mode, evidence.Schema, facet)
	}
	validator, err := loadWireValidator()
	if err != nil {
		return evidence, err
	}
	if evidence.Mode == "range-error" {
		evidence.Mode, evidence.Schema = "json", "common.v1.schema.json#/$defs/error-envelope"
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return invalid("invalid JSON")
	}
	if err := validator.schemas[evidence.Schema].Validate(document); err != nil {
		return invalid("invalid envelope shape at " + schemaFailureLocation(err))
	}
	object := document.(map[string]any)
	code, err := object["code"].(json.Number).Float64()
	if err != nil {
		return invalid("business code is out of range")
	}
	if (status >= 200 && status < 300) != (code == 0) {
		return invalid("HTTP and business status disagree")
	}
	if code == 0 && binding.Schema != "" {
		evidence.Schema = binding.Schema
		if err := validator.schemas[binding.Schema].Validate(object["data"]); err != nil {
			return invalid("invalid data shape at " + schemaFailureLocation(err))
		}
	}
	evidence.Valid = true
	return evidence, nil
}

func schemaFailureLocation(err error) string {
	failure, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return "unknown-schema"
	}
	for len(failure.Causes) != 0 {
		failure = failure.Causes[0]
	}
	if !strings.HasPrefix(failure.SchemaURL, schemaOrigin) {
		return "unknown-schema"
	}
	return strings.TrimPrefix(failure.SchemaURL, schemaOrigin)
}
