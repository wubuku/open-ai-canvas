package canvas_contract_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"canvas-contract/runner"
)

type canvasPage struct {
	Projects []map[string]any `json:"projects"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int              `json:"total"`
	HasMore  bool             `json:"hasMore"`
}

func requireHTTPSuccess(test *testing.T, client *runner.Client, method, path string, payload any) (runner.Response, runner.Envelope) {
	test.Helper()
	response, envelope, err := client.Do(runner.Context(test), method, path, payload, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	return response, envelope
}

func requireHTTPError(test *testing.T, client *runner.Client, method, path string, payload any, status int) {
	test.Helper()
	response, envelope, err := client.Do(runner.Context(test), method, path, payload, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, status, "")
}

func readCanvasPage(test *testing.T, client *runner.Client, query string) canvasPage {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, client, http.MethodGet, "/api/canvas-projects?"+query, nil)
	return runner.DecodeData[canvasPage](test, envelope)
}

func runCanvasPageMatrix(test *testing.T, owner, foreign *runner.Client, runID string) {
	prefix := "Page-Matrix-" + runID
	identities := []string{"page-a-" + runID, "page-b-" + runID, "page-c-" + runID}
	for index, identity := range identities {
		project := runner.FixtureProject(identity, prefix+fmt.Sprintf("-%d", index), 0, false)
		nodes := make([]any, 0, index+1)
		for nodeIndex := 0; nodeIndex <= index; nodeIndex++ {
			nodes = append(nodes, map[string]any{
				"id": fmt.Sprintf("page-node-%d", nodeIndex), "type": "text", "title": "page node",
				"position": map[string]int{"x": nodeIndex * 100, "y": 0}, "metadata": map[string]any{},
			})
		}
		project["nodes"] = nodes
		createCanvas(test, owner, identity, project)
	}
	query := "q=" + url.QueryEscape(strings.ToUpper(prefix)) + "&sort=name&projectId=independent"
	for pageNumber := 1; pageNumber <= 4; pageNumber++ {
		page := readCanvasPage(test, owner, query+fmt.Sprintf("&page=%d&pageSize=1", pageNumber))
		if page.Page != pageNumber || page.PageSize != 1 || page.Total != 3 || page.HasMore != (pageNumber < 3) {
			test.Fatal("pagination metadata is inconsistent with the filtered set")
		}
		if pageNumber <= 3 {
			if len(page.Projects) != 1 || page.Projects[0]["id"] != identities[pageNumber-1] {
				test.Fatal("name-sorted page has the wrong identity")
			}
		} else if len(page.Projects) != 0 {
			test.Fatal("page after the last result is not empty")
		}
	}
	nodePage := readCanvasPage(test, owner, "page=1&pageSize=3&sort=nodes&q="+url.QueryEscape(prefix))
	if len(nodePage.Projects) != 3 {
		test.Fatal("node-count page lost results")
	}
	for index, project := range nodePage.Projects {
		if project["id"] != identities[2-index] {
			test.Fatal("node-count sort is not descending")
		}
		runner.RequireNumber(test, project["nodeCount"], float64(3-index), "node count")
	}
	clamped := readCanvasPage(test, owner, "page=1&pageSize=1000&q="+url.QueryEscape(prefix))
	if clamped.PageSize != 50 || clamped.Total != 3 || len(clamped.Projects) != 3 {
		test.Fatal("page size cap is not represented in response metadata")
	}
	for _, client := range []*runner.Client{owner, foreign} {
		emptyQuery := "page=1&pageSize=3&projectId=missing-project&q=" + url.QueryEscape(prefix)
		if client == foreign {
			emptyQuery = "page=1&pageSize=3&q=" + url.QueryEscape(prefix)
		}
		page := readCanvasPage(test, client, emptyQuery)
		if page.Total != 0 || len(page.Projects) != 0 || page.HasMore {
			test.Fatal("filtered or foreign list exposed an unexpected canvas")
		}
	}
	for _, invalidQuery := range []string{"page=0", "page=-1", "page=abc", "page=1000001", "page=1&pageSize=0", "page=1&pageSize=-1", "page=1&pageSize=abc"} {
		requireHTTPError(test, owner, http.MethodGet, "/api/canvas-projects?"+invalidQuery, nil, http.StatusBadRequest)
	}
}

func runCanvasInvalidWriteMatrix(test *testing.T, owner *runner.Client, runID string) {
	identity := "invalid-write-" + runID
	path := "/api/canvas-projects/" + identity
	createCanvas(test, owner, identity, runner.FixtureProject(identity, "Unchanged after invalid input", 0, false))
	missingRevision := runner.FixtureProject(identity, "Missing revision", 1, false)
	delete(missingRevision, "revision")
	for _, request := range []struct {
		project map[string]any
		status  int
	}{
		{runner.FixtureProject("wrong-identity", "Wrong ID", 1, false), http.StatusBadRequest},
		{missingRevision, http.StatusPreconditionRequired},
		{runner.FixtureProject(identity, "Negative revision", -1, false), http.StatusBadRequest},
		{runner.FixtureProject(identity, "Unsafe integer revision", 9007199254740991, false), http.StatusBadRequest},
	} {
		requireHTTPError(test, owner, http.MethodPut, path, map[string]any{"project": request.project}, request.status)
		getCanvasAndAssert(test, owner, identity, 1, "Unchanged after invalid input")
	}
	for _, body := range [][]byte{[]byte(`{"project":`), []byte(strings.Repeat(" ", (5<<20)+1) + `{}`)} {
		response, envelope, err := owner.DoBytes(runner.Context(test), http.MethodPut, path, "application/json", body, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		getCanvasAndAssert(test, owner, identity, 1, "Unchanged after invalid input")
	}
}

type uploadSession struct {
	UploadID   string `json:"uploadId"`
	ChunkSize  int    `json:"chunkSize"`
	ChunkCount int    `json:"chunkCount"`
}

func startChunkUpload(test *testing.T, owner *runner.Client, size int) uploadSession {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/resources/uploads", map[string]any{"fileName": "matrix.txt", "kind": "file", "size": size})
	session := runner.DecodeData[uploadSession](test, envelope)
	if session.UploadID == "" || session.ChunkSize <= 0 || session.ChunkCount <= 0 {
		test.Fatal("upload session is missing its identity or chunk boundaries")
	}
	return session
}

func writeChunk(test *testing.T, owner *runner.Client, path string, content []byte, status int) {
	test.Helper()
	response, envelope, err := owner.DoBytes(runner.Context(test), http.MethodPut, path, "application/octet-stream", content, nil)
	if err != nil {
		test.Fatal(err)
	}
	if status == http.StatusOK {
		runner.RequireSuccess(test, response, envelope)
	} else {
		runner.RequireError(test, response, envelope, status, "")
	}
}

func completeChunkUpload(test *testing.T, owner *runner.Client, session uploadSession, content []byte) string {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, owner, http.MethodPost, "/api/resources/uploads/"+session.UploadID+"/complete", nil)
	resource := runner.DecodeData[struct {
		Resource struct {
			ID string `json:"id"`
		} `json:"resource"`
	}](test, envelope)
	if resource.Resource.ID == "" {
		test.Fatal("completed upload has no resource identity")
	}
	response, err := owner.DoRaw(runner.Context(test), http.MethodGet, "/api/resources/"+resource.Resource.ID+"/file", nil, "", nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusOK)
	if !bytes.Equal(response.Body, content) {
		test.Fatal("completed resource bytes differ from the supplied chunks")
	}
	return resource.Resource.ID
}

func runChunkUploadFailureMatrix(test *testing.T, owner, foreign *runner.Client) {
	for _, payload := range []map[string]any{
		{"fileName": "", "size": 1}, {"fileName": strings.Repeat("x", 256), "size": 1},
		{"fileName": "invalid.txt", "size": 0}, {"fileName": "invalid.txt", "size": -1},
	} {
		requireHTTPError(test, owner, http.MethodPost, "/api/resources/uploads", payload, http.StatusBadRequest)
	}
	content := []byte("exact-sized upload fixture")
	session := startChunkUpload(test, owner, len(content))
	basePath := "/api/resources/uploads/" + session.UploadID
	for _, index := range []string{"-1", "1", "invalid"} {
		writeChunk(test, owner, basePath+"/chunks/"+index, content, http.StatusBadRequest)
	}
	writeChunk(test, foreign, basePath+"/chunks/0", content, http.StatusNotFound)
	requireHTTPError(test, foreign, http.MethodPost, basePath+"/complete", nil, http.StatusNotFound)
	requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusBadRequest)
	for _, malformed := range [][]byte{nil, content[:len(content)-1], append(append([]byte{}, content...), 'x')} {
		writeChunk(test, owner, basePath+"/chunks/0", malformed, http.StatusBadRequest)
		requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusBadRequest)
	}
	writeChunk(test, owner, basePath+"/chunks/0", content, http.StatusOK)
	writeChunk(test, owner, basePath+"/chunks/0", content, http.StatusOK)
	completeChunkUpload(test, owner, session, content)
	requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusNotFound)
	writeChunk(test, owner, basePath+"/chunks/0", content, http.StatusNotFound)
}

func runChunkUploadBoundaryMatrix(test *testing.T, owner *runner.Client) {
	content := bytes.Repeat([]byte("b"), (8<<20)+17)
	session := startChunkUpload(test, owner, len(content))
	if session.ChunkSize != 8<<20 || session.ChunkCount != 2 {
		test.Fatal("Go baseline chunk boundary differs from the frozen 8 MiB contract")
	}
	basePath := "/api/resources/uploads/" + session.UploadID
	writeChunk(test, owner, basePath+"/chunks/1", content[session.ChunkSize:], http.StatusOK)
	requireHTTPError(test, owner, http.MethodPost, basePath+"/complete", nil, http.StatusBadRequest)
	writeChunk(test, owner, basePath+"/chunks/0", content[:session.ChunkSize], http.StatusOK)
	writeChunk(test, owner, basePath+"/chunks/1", content[session.ChunkSize:], http.StatusOK)
	completeChunkUpload(test, owner, session, content)
}

func runProjectCanvasValidationMatrix(test *testing.T, owner, foreign *runner.Client, projectID, unitID, canvasID string) {
	before := readWorkbenchState(test, owner, projectID)
	anonymous, err := runner.NewClient("anonymous", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	for _, request := range []struct {
		method  string
		path    string
		payload any
	}{
		{http.MethodGet, "/api/projects/" + projectID + "/canvases", nil},
		{http.MethodPost, "/api/projects/" + projectID + "/canvas-links", map[string]any{"canvasId": canvasID, "unitId": unitID}},
		{http.MethodDelete, "/api/projects/" + projectID + "/canvas-links/" + canvasID + "/units/" + unitID, nil},
		{http.MethodDelete, "/api/projects/" + projectID + "/canvases/" + canvasID, nil},
	} {
		requireHTTPError(test, anonymous, request.method, request.path, request.payload, http.StatusUnauthorized)
		if request.method != http.MethodGet {
			requireHTTPError(test, foreign, request.method, request.path, request.payload, http.StatusNotFound)
		}
	}
	for _, payload := range []map[string]string{
		{"canvasId": "", "unitId": unitID}, {"canvasId": canvasID, "unitId": ""},
	} {
		requireHTTPError(test, owner, http.MethodPost, "/api/projects/"+projectID+"/canvas-links", payload, http.StatusBadRequest)
	}
	for _, query := range []string{"page=0", "page=1&pageSize=0", "page=invalid"} {
		requireHTTPError(test, owner, http.MethodGet, "/api/projects/"+projectID+"/canvases?"+query, nil, http.StatusBadRequest)
	}
	if !reflect.DeepEqual(before, readWorkbenchState(test, owner, projectID)) {
		test.Fatal("rejected linking requests mutated project or unit state")
	}
	getCanvasAndAssert(test, owner, canvasID, 1, "Project canvas")
}
