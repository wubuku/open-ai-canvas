package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func encodedSuccess(test *testing.T, data any) []byte {
	test.Helper()
	body, err := json.Marshal(map[string]any{"code": 0, "data": data, "msg": "ok"})
	if err != nil {
		test.Fatal(err)
	}
	return body
}

func documentFixture() map[string]any {
	return map[string]any{
		"id": "fixture", "title": "Synthetic", "revision": 1,
		"nodes":       []any{map[string]any{"id": "first"}, map[string]any{"id": "second"}},
		"connections": []any{map[string]any{"id": "edge", "fromNodeId": "first", "toNodeId": "second"}},
		"createdAt":   "2026-10-12T00:00:00Z", "updatedAt": "2026-10-12T00:01:00+08:00",
	}
}

func TestEmbeddedBindingsCompileOfflineAndMatchLiteralRoutes(test *testing.T) {
	validator, err := loadWireValidator()
	if err != nil {
		test.Fatal(err)
	}
	if len(validator.bindings) != 71 {
		test.Fatal("required route registry changed without updating its audit")
	}
	binaryCount := 0
	for _, binding := range validator.bindings {
		matched, found := validator.binding(binding.Method, binding.Path)
		if !found || matched != binding {
			test.Fatal("binding does not match its own route")
		}
		if binding.Binary {
			binaryCount++
		}
	}
	if binaryCount != 4 {
		test.Fatal("binary bindings changed without an explicit contract update")
	}
	for _, request := range []struct{ method, path, schema string }{
		{"GET", "/api/resources/storage-usage", "resource.v1.schema.json#/$defs/usage"},
		{"PATCH", "/api/assets/folder", "legacy.v1.schema.json#/$defs/identities"},
		{"PATCH", "/api/projects/fixture/units/reorder", "project.v1.schema.json#/$defs/reorder"},
		{"GET", "/api/canvas-projects/fixture?token=secret-query", "canvas.v1.schema.json#/$defs/read"},
	} {
		binding, found := validator.binding(request.method, request.path)
		if !found || binding.Schema != request.schema {
			test.Fatal("literal specificity or query-independent routing failed")
		}
	}
	if _, found := validator.binding("GET", "/api/canvas-projects/"); found {
		test.Fatal("empty identity matched a required binding")
	}
}

func TestSSEBindingRequiresEventStreamAndDoesNotUseJSONDecoder(test *testing.T) {
	body := []byte("event: progress\ndata: {\"progress\":100}\n\nevent: terminal\ndata: {}\n\n")
	evidence, err := ValidateResponse("GET", "/api/tasks/task-1/text-events?after=0", http.StatusOK, "text/event-stream; charset=utf-8", body)
	if err != nil || !evidence.Valid || evidence.Mode != "stream" || evidence.Schema != "" || RequiresJSON("GET", "/api/tasks/task-1/text-events", http.StatusOK, "text/event-stream") {
		test.Fatalf("valid SSE response was not classified as a stream: evidence=%+v err=%v", evidence, err)
	}
	for _, contentType := range []string{"application/json", "text/plain"} {
		evidence, err = ValidateResponse("GET", "/api/tasks/task-1/text-events", http.StatusOK, contentType, body)
		if err == nil || evidence.Valid {
			test.Fatalf("content type %q incorrectly passed SSE validation: evidence=%+v", contentType, evidence)
		}
	}
	evidence, err = ValidateResponse("GET", "/api/tasks/task-1/text-events", http.StatusOK, "text/event-stream", nil)
	if err == nil || evidence.Valid {
		test.Fatalf("empty SSE body incorrectly passed validation: evidence=%+v", evidence)
	}
}

func TestWireDocumentMutationsAndDiagnostics(test *testing.T) {
	mutations := map[string]func(map[string]any){
		"missing-title":       func(document map[string]any) { delete(document, "title") },
		"missing-revision":    func(document map[string]any) { delete(document, "revision") },
		"zero-revision":       func(document map[string]any) { document["revision"] = 0 },
		"fractional-revision": func(document map[string]any) { document["revision"] = 1.5 },
		"unsafe-revision":     func(document map[string]any) { document["revision"] = json.Number("9007199254740992") },
		"string-revision":     func(document map[string]any) { document["revision"] = "secret-token" },
		"invalid-time":        func(document map[string]any) { document["createdAt"] = "secret-time" },
		"impossible-date":     func(document map[string]any) { document["updatedAt"] = "2026-02-30T00:00:00Z" },
		"local-time":          func(document map[string]any) { document["updatedAt"] = "2026-10-12T00:00:00" },
		"null-nodes":          func(document map[string]any) { document["nodes"] = nil },
		"node-without-id":     func(document map[string]any) { document["nodes"] = []any{map[string]any{"prompt": "secret-prompt"}} },
		"edge-without-target": func(document map[string]any) {
			document["connections"] = []any{map[string]any{"id": "edge", "fromNodeId": "first"}}
		},
	}
	for name, mutate := range mutations {
		test.Run(name, func(test *testing.T) {
			document := documentFixture()
			mutate(document)
			evidence, err := ValidateResponse("GET", "/api/canvas-projects/secret-id?signature=secret-query", 200, "application/json", encodedSuccess(test, map[string]any{"project": document}))
			if err == nil || evidence.Valid || strings.Contains(err.Error(), "secret") {
				test.Fatal("schema mutation accepted or diagnostics disclosed an instance value")
			}
		})
	}
	document := documentFixture()
	document["futureExtension"] = map[string]any{"prompt": "secret-prompt"}
	document["revision"] = json.Number("9007199254740991")
	evidence, err := ValidateResponse("GET", "/api/canvas-projects/fixture", 200, "application/json; charset=utf-8", encodedSuccess(test, map[string]any{"project": document}))
	if err != nil || !evidence.Valid || evidence.Version != WireVersion || evidence.Schema != "canvas.v1.schema.json#/$defs/read" {
		test.Fatal("valid additive document rejected")
	}
}

func TestWireEnvelopeMutations(test *testing.T) {
	for _, response := range []struct {
		status int
		body   string
	}{
		{200, `{}`},
		{200, `{"code":0,"data":{},"msg":null}`},
		{200, `{"code":"secret-code","data":{},"msg":"ok"}`},
		{200, `{"code":0,"data":{},"msg":"ok"} {"prompt":"secret-extra"}`},
		{200, `{"code":1,"data":null,"msg":"secret-error","reason":"internal"}`},
		{400, `{"code":0,"data":null,"msg":"ok"}`},
		{400, `{"code":1,"data":null,"msg":"secret-error"}`},
		{400, `{"code":1,"data":null,"msg":"secret-error","reason":""}`},
		{400, `{"code":1,"data":null,"msg":"secret-error","reason":1}`},
	} {
		evidence, err := ValidateResponse("POST", "/api/auth/login", response.status, "application/json", []byte(response.body))
		if err == nil || evidence.Valid || strings.Contains(err.Error(), "secret") {
			test.Fatal("invalid envelope accepted or sensitive diagnostic emitted")
		}
	}
	for _, response := range []struct {
		status int
		body   string
	}{
		{200, `{"code":0,"data":{},"msg":"ok"}`},
		{400, `{"code":1,"data":null,"msg":"synthetic","reason":"bad_request"}`},
	} {
		if evidence, err := ValidateResponse("POST", "/api/auth/login", response.status, "application/json", []byte(response.body)); err != nil || !evidence.Valid {
			test.Fatal("valid setup envelope rejected")
		}
	}
}

func TestWirePaginationMustBeAtomic(test *testing.T) {
	for _, data := range []map[string]any{
		{"projects": []any{}},
		{"projects": []any{}, "page": 1, "pageSize": 10, "total": 0, "hasMore": false},
	} {
		if evidence, err := ValidateResponse("GET", "/api/canvas-projects", 200, "application/json", encodedSuccess(test, data)); err != nil || !evidence.Valid {
			test.Fatal("valid pagination rejected")
		}
	}
	for _, data := range []map[string]any{
		{"projects": []any{}, "page": 1},
		{"projects": []any{}, "hasMore": false},
		{"projects": []any{}, "page": 0, "pageSize": 10, "total": 0, "hasMore": false},
		{"projects": []any{}, "page": 1, "pageSize": 10, "total": -1, "hasMore": false},
	} {
		if evidence, err := ValidateResponse("GET", "/api/canvas-projects", 200, "application/json", encodedSuccess(test, data)); err == nil || evidence.Valid {
			test.Fatal("partial or invalid pagination accepted")
		}
	}
}

func TestObservedProjectPageRevisionDoesNotRelaxDocumentRevision(test *testing.T) {
	pageSummary := documentFixture()
	pageSummary["revision"] = 0
	page := map[string]any{"canvases": []any{pageSummary}, "canvasUnitLinks": []any{}, "page": 1, "pageSize": 10, "total": 1, "hasMore": false}
	if _, err := ValidateResponse("GET", "/api/projects/fixture/canvases", 200, "application/json", encodedSuccess(test, page)); err != nil {
		test.Fatal("observed incomplete page projection was not distinguished from the authoritative document")
	}
	if _, err := ValidateResponse("GET", "/api/canvas-projects/fixture", 200, "application/json", encodedSuccess(test, map[string]any{"project": pageSummary})); err == nil {
		test.Fatal("observed page bug weakened the document revision contract")
	}
	pageSummary["revision"] = 1
	if _, err := ValidateResponse("GET", "/api/projects/fixture/canvases", 200, "application/json", encodedSuccess(test, page)); err == nil {
		test.Fatal("changed page behavior requires an explicit D-11 decision and schema update")
	}
}

func TestWireResourceAccessMustContainExactlyOneOutcome(test *testing.T) {
	failure := map[string]any{"code": 1, "reason": "not_found", "msg": "synthetic"}
	for _, item := range []map[string]any{
		{"resourceId": "fixture"},
		{"resourceId": "fixture", "access": map[string]any{}, "error": failure},
		{"resourceId": "fixture", "access": map[string]any{}},
	} {
		if _, err := ValidateResponse("POST", "/api/resources/access", 200, "application/json", encodedSuccess(test, map[string]any{"items": []any{item}})); err == nil {
			test.Fatal("ambiguous or incomplete access outcome accepted")
		}
	}
	if _, err := ValidateResponse("POST", "/api/resources/access", 200, "application/json", encodedSuccess(test, map[string]any{"items": []any{map[string]any{"resourceId": "fixture", "error": failure}}})); err != nil {
		test.Fatal(err)
	}
}

func TestWireBinaryAndConditionalResponsesStayDistinct(test *testing.T) {
	for _, response := range []struct {
		path                    string
		status                  int
		contentType, body, mode string
	}{
		{"/api/resources/fixture/file", 200, "application/json", `{"arbitrary":"secret-file"}`, "binary"},
		{"/api/public/resources/fixture/file/name", 206, "image/png", "synthetic-bytes", "binary"},
		{"/api/public/canvas-shares/fixture/resources/fixture/file", 416, "text/plain; charset=utf-8", "invalid range", "range-error"},
		{"/api/resources/fixture/file", 416, "application/json", `{"code":1,"data":null,"msg":"range","reason":"bad_request"}`, "json"},
		{"/api/canvas-projects/fixture", 304, "", "", "not-modified"},
		{"/api/resources/fixture/file", 304, "", "", "not-modified"},
		{"/restart", 200, "application/json", `{"restarted":true}`, "unbound-raw"},
	} {
		evidence, err := ValidateResponse("GET", response.path, response.status, response.contentType, []byte(response.body))
		if err != nil || !evidence.Valid || evidence.Mode != response.mode {
			test.Fatal("binary, conditional or private-control response misclassified")
		}
	}
	for _, response := range []struct {
		path              string
		status            int
		contentType, body string
	}{
		{"/api/projects/fixture", 304, "", ""},
		{"/api/canvas-projects/fixture", 304, "", "unexpected"},
		{"/api/resources/fixture/file", 416, "text/html", "secret-file"},
		{"/api/resources/fixture/file", 403, "application/json", `{"code":0,"data":{},"msg":"ok"}`},
		{"/api/canvas-projects", 200, "text/plain", `{"code":0,"data":{"projects":[]},"msg":"ok"}`},
	} {
		if _, err := ValidateResponse("GET", response.path, response.status, response.contentType, []byte(response.body)); err == nil {
			test.Fatal("unexpected response accepted")
		}
	}
}

func TestWireOfflineLoaderRejectsExternalReferences(test *testing.T) {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(offlineLoader{})
	document, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(`{"$ref":"https://127.0.0.1:1/secret-schema"}`))
	if err != nil || compiler.AddResource(schemaOrigin+"offline-test", document) != nil {
		test.Fatal("cannot create synthetic schema")
	}
	if _, err := compiler.Compile(schemaOrigin + "offline-test"); err == nil {
		test.Fatal("external reference was allowed")
	}
}

func TestWireValidationIsConcurrentSafe(test *testing.T) {
	var group sync.WaitGroup
	body := encodedSuccess(test, map[string]any{"projects": []any{}})
	for count := 0; count < 20; count++ {
		group.Go(func() {
			if evidence, err := ValidateResponse("GET", "/api/canvas-projects", 200, "application/json", body); err != nil || !evidence.Valid {
				test.Error("concurrent validation rejected a valid response")
			}
		})
	}
	group.Wait()
}
