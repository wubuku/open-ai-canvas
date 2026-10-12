package canvas_contract_test

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"canvas-contract/runner"
)

type visibleShare struct {
	Enabled   bool       `json:"enabled"`
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

func readShare(test *testing.T, owner *runner.Client, canvasID string) visibleShare {
	test.Helper()
	response, envelope, err := owner.Do(runner.Context(test), http.MethodGet, "/api/canvas-projects/"+canvasID+"/share", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	return runner.DecodeData[struct {
		Share visibleShare `json:"share"`
	}](test, envelope).Share
}

func createShare(test *testing.T, owner *runner.Client, canvasID string, days int, rotate bool) visibleShare {
	test.Helper()
	response, envelope, err := owner.Do(runner.Context(test), http.MethodPost, "/api/canvas-projects/"+canvasID+"/share", map[string]any{"expiresDays": days, "rotate": rotate}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	share := runner.DecodeData[struct {
		Share visibleShare `json:"share"`
	}](test, envelope).Share
	if !share.Enabled || share.Token == "" || days > 0 && share.ExpiresAt == nil {
		test.Fatal("share creation did not return an enabled share and token")
	}
	return share
}

func runShareInvalidInputMatrix(test *testing.T, owner, anonymous *runner.Client, runID string) {
	canvasID := "share-validation-" + runID
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Share validation", 0, false))
	share := createShare(test, owner, canvasID, 1, false)
	path := "/api/canvas-projects/" + canvasID + "/share"
	for _, days := range []int{-1, 366} {
		requireHTTPError(test, owner, http.MethodPost, path, map[string]any{"expiresDays": days, "rotate": true}, http.StatusBadRequest)
		current := readShare(test, owner, canvasID)
		if !current.Enabled || current.Token != share.Token || current.ExpiresAt == nil || !current.ExpiresAt.Equal(*share.ExpiresAt) {
			test.Fatal("invalid expiry mutated a valid share")
		}
	}
	for _, body := range []string{`{"expiresDays":`, `{"expiresDays":1,"padding":"` + strings.Repeat("x", 8<<10) + `"}`} {
		response, envelope, err := owner.DoBytes(runner.Context(test), http.MethodPost, path, "application/json", []byte(body), nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireError(test, response, envelope, http.StatusBadRequest, "")
		if current := readShare(test, owner, canvasID); !current.Enabled || current.Token != share.Token {
			test.Fatal("bad/oversized JSON mutated share state")
		}
	}
	for _, days := range []int{0, 365} {
		started := time.Now()
		replayed := createShare(test, owner, canvasID, days, false)
		if replayed.Token != share.Token {
			test.Fatal("non-rotating replay changed token")
		}
		if days == 0 && replayed.ExpiresAt != nil {
			test.Fatal("zero expiry must be represented as null/absent expiry, not zero timestamp")
		}
		if days != 0 && (replayed.ExpiresAt == nil || replayed.ExpiresAt.Before(started.Add(365*24*time.Hour-time.Minute)) || replayed.ExpiresAt.After(time.Now().Add(365*24*time.Hour+time.Minute))) {
			test.Fatal("maximum share expiry is outside the configured lifetime")
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		response, envelope, err := owner.Do(runner.Context(test), http.MethodDelete, path, nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		runner.RequireSuccess(test, response, envelope)
		if current := readShare(test, owner, canvasID); current.Enabled || current.Token != "" {
			test.Fatal("share revoke/replay left management enabled")
		}
		requireHTTPError(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+share.Token, nil, http.StatusNotFound)
	}
	getCanvasAndAssert(test, owner, canvasID, 1, "Share validation")
}

func runShareExpiryRecovery(test *testing.T, owner, foreign, anonymous *runner.Client, config runner.Config, runID string) {
	canvasID := "share-expiry-" + runID
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Share expiry", 0, false))
	share := createShare(test, owner, canvasID, 1, false)
	beforeDocument := readCanvasDocument(test, owner, canvasID)
	beforeHistory := readCanvasHistory(test, owner, canvasID)
	requireHTTPSuccess(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+share.Token, nil)
	if err := runner.ExpireShare(runner.Context(test), config, canvasID); err != nil {
		test.Fatal(err)
	}
	requireHTTPError(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+share.Token, nil, http.StatusNotFound)
	status := readShare(test, owner, canvasID)
	if status.Enabled || status.Token != "" || status.ExpiresAt != nil {
		test.Fatal("expired share remained enabled or exposed its token to the owner")
	}
	requireHTTPError(test, foreign, http.MethodGet, "/api/canvas-projects/"+canvasID+"/share", nil, http.StatusNotFound)
	if !reflect.DeepEqual(beforeDocument, readCanvasDocument(test, owner, canvasID)) || !reflect.DeepEqual(beforeHistory, readCanvasHistory(test, owner, canvasID)) {
		test.Fatal("share expiration changed the source canvas or history")
	}
	restored := createShare(test, owner, canvasID, 0, false)
	if restored.Token != share.Token || restored.ExpiresAt != nil {
		test.Fatal("share recovery did not re-enable the existing token without an expiry")
	}
	requireHTTPSuccess(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+restored.Token, nil)
}

func runShareResourceMatrix(test *testing.T, owner, foreign, anonymous *runner.Client, runID string) {
	canvasID := "share-resource-" + runID
	content := runner.ImageFixture(test)
	allowed := uploadResource(test, owner, "shared-image.png", "image", content)
	unlisted := uploadResource(test, owner, "unlisted-image.png", "image", content)
	foreignID := uploadResource(test, foreign, "foreign-image.png", "image", content)
	createImageAsset(test, owner, allowed, len(content))
	project := runner.FixtureProject(canvasID, "Shared resource", 0, false)
	attachResource(project, allowed)
	createCanvas(test, owner, canvasID, project)
	share := createShare(test, owner, canvasID, 0, false)
	resourcePath := "/api/public/canvas-shares/" + share.Token + "/resources/"
	for _, rejected := range []string{unlisted, foreignID, "missing-resource"} {
		requireHTTPError(test, anonymous, http.MethodGet, resourcePath+rejected+"/file", nil, http.StatusNotFound)
	}
	headers := make(http.Header)
	headers.Set("Range", "bytes=0-7")
	response, err := anonymous.DoRaw(runner.Context(test), http.MethodGet, resourcePath+allowed+"/file", nil, "", headers)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusPartialContent)
	if !bytes.Equal(response.Body, content[:8]) {
		test.Fatal("shared resource range bytes differ")
	}
	runner.RequireHeader(test, response.Header, "Content-Range", fmt.Sprintf("bytes 0-7/%d", len(content)))
	runner.RequireHeader(test, response.Header, "Cache-Control", "no-store")
	headers.Set("Range", "bytes=999999-")
	response, err = anonymous.DoRaw(runner.Context(test), http.MethodGet, resourcePath+allowed+"/file", nil, "", headers)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireStatus(test, response, http.StatusRequestedRangeNotSatisfiable)
	updated := runner.FixtureProject(canvasID, "Resource removed", 1, false)
	saveResponse, saveEnvelope, err := owner.Do(runner.Context(test), http.MethodPut, "/api/canvas-projects/"+canvasID, map[string]any{"project": updated}, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, saveResponse, saveEnvelope)
	requireHTTPError(test, anonymous, http.MethodGet, resourcePath+allowed+"/file", nil, http.StatusNotFound)
	publicResponse, envelope, err := anonymous.Do(runner.Context(test), http.MethodGet, "/api/public/canvas-shares/"+share.Token, nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, publicResponse, envelope)
	public := runner.DecodeData[struct {
		Project map[string]any `json:"project"`
	}](test, envelope)
	if public.Project["title"] != "Resource removed" {
		test.Fatal("share must render the current live canvas after save")
	}
	requireResourceBytes(test, owner, allowed, content)
	response, envelope, err = owner.Do(runner.Context(test), http.MethodDelete, "/api/canvas-projects/"+canvasID, nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	requireHTTPError(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+share.Token, nil, http.StatusNotFound)
	requireHTTPError(test, anonymous, http.MethodGet, resourcePath+allowed+"/file", nil, http.StatusNotFound)
}

func runShareConcurrentMatrix(test *testing.T, owner, anonymous *runner.Client, runID string) {
	canvasID := "share-concurrent-" + runID
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Concurrent share", 0, false))
	initial := createShare(test, owner, canvasID, 0, false)
	path := "/api/canvas-projects/" + canvasID + "/share"
	for _, revoke := range []bool{false, true} {
		start := make(chan struct{})
		var workers sync.WaitGroup
		responses := make([]runner.Response, 2)
		envelopes := make([]runner.Envelope, 2)
		errors := make([]error, 2)
		for index := range responses {
			workers.Add(1)
			go func(index int) {
				defer workers.Done()
				<-start
				method := http.MethodPost
				var payload any = map[string]any{"expiresDays": 0, "rotate": true}
				if index == 1 && revoke {
					method = http.MethodDelete
					payload = nil
				}
				responses[index], envelopes[index], errors[index] = owner.Do(runner.Context(test), method, path, payload, nil)
			}(index)
		}
		close(start)
		workers.Wait()
		current := readShare(test, owner, canvasID)
		tokens := []string{initial.Token}
		for index, response := range responses {
			if errors[index] != nil {
				test.Fatal(errors[index])
			}
			runner.RequireSuccess(test, response, envelopes[index])
			if index == 0 || !revoke {
				share := runner.DecodeData[struct {
					Share visibleShare `json:"share"`
				}](test, envelopes[index]).Share
				if !share.Enabled || share.Token == "" {
					test.Fatal("concurrent rotate returned no share token")
				}
				tokens = append(tokens, share.Token)
			}
		}
		validTokens := 0
		for _, token := range tokens {
			response, envelope, err := anonymous.Do(runner.Context(test), http.MethodGet, "/api/public/canvas-shares/"+token, nil, nil)
			if err != nil {
				test.Fatal(err)
			}
			if current.Enabled && token == current.Token {
				runner.RequireSuccess(test, response, envelope)
				validTokens++
			} else {
				runner.RequireError(test, response, envelope, http.StatusNotFound, "")
			}
		}
		if !revoke && (!current.Enabled || validTokens != 1) || revoke && (current.Enabled && validTokens != 1 || !current.Enabled && validTokens != 0) {
			test.Fatal("settled share status and public token validity disagree")
		}
		if current.Enabled {
			initial = current
		}
	}
	getCanvasAndAssert(test, owner, canvasID, 1, "Concurrent share")
}

func runShareFirstCreateConcurrentMatrix(test *testing.T, owner, foreign, anonymous *runner.Client, runID string) {
	test.Helper()
	canvasID := "share-first-create-concurrent-" + runID
	createCanvas(test, owner, canvasID, runner.FixtureProject(canvasID, "Concurrent first share", 0, false))
	path := "/api/canvas-projects/" + canvasID + "/share"
	start := make(chan struct{})
	responses := make([]runner.Response, 2)
	envelopes := make([]runner.Envelope, 2)
	errors := make([]error, 2)
	var workers sync.WaitGroup
	for index := range responses {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			responses[index], envelopes[index], errors[index] = owner.Do(runner.Context(test), http.MethodPost, path, map[string]any{"expiresDays": 0, "rotate": false}, nil)
		}(index)
	}
	close(start)
	workers.Wait()
	successes := 0
	tokens := make([]string, 0, 2)
	for index, response := range responses {
		if errors[index] != nil {
			test.Fatal(errors[index])
		}
		switch response.StatusCode {
		case http.StatusOK:
			runner.RequireSuccess(test, response, envelopes[index])
			share := runner.DecodeData[struct {
				Share visibleShare `json:"share"`
			}](test, envelopes[index]).Share
			if !share.Enabled || share.Token == "" || share.ExpiresAt != nil {
				test.Fatal("first-create race returned an invalid non-expiring share")
			}
			tokens = append(tokens, share.Token)
			successes++
		case http.StatusInternalServerError:
			runner.RequireError(test, response, envelopes[index], http.StatusInternalServerError, "internal")
		default:
			test.Fatalf("first-create race returned unexpected status=%d", response.StatusCode)
		}
	}
	if successes == 0 {
		test.Fatal("first-create race did not publish a management result")
	}
	current := readShare(test, owner, canvasID)
	if !current.Enabled || current.Token == "" {
		test.Fatal("first-create race left no enabled share")
	}
	validTokens := 0
	for _, token := range tokens {
		response, envelope, err := anonymous.Do(runner.Context(test), http.MethodGet, "/api/public/canvas-shares/"+token, nil, nil)
		if err != nil {
			test.Fatal(err)
		}
		if token == current.Token {
			runner.RequireSuccess(test, response, envelope)
			validTokens++
		} else {
			runner.RequireError(test, response, envelope, http.StatusNotFound, "")
		}
	}
	if validTokens != 1 || len(tokens) != successes {
		test.Fatalf("first-create race published %d valid tokens from %d successful responses", validTokens, successes)
	}
	if foreignShare := readShareError(test, foreign, canvasID); foreignShare != http.StatusNotFound {
		test.Fatalf("foreign share status returned %d", foreignShare)
	}
	currentToken := current.Token
	response, envelope, err := owner.Do(runner.Context(test), http.MethodDelete, path, nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireSuccess(test, response, envelope)
	if current := readShare(test, owner, canvasID); current.Enabled || current.Token != "" {
		test.Fatal("first-create race revoke did not clear management state")
	}
	requireHTTPError(test, anonymous, http.MethodGet, "/api/public/canvas-shares/"+currentToken, nil, http.StatusNotFound)
	test.Logf("first_create_share_race=successes:%d current_token_only:%t; a concurrent loser may expose Go internal error, not a Java contract", successes, validTokens == 1)
}

func readShareError(test *testing.T, client *runner.Client, canvasID string) int {
	test.Helper()
	response, envelope, err := client.Do(runner.Context(test), http.MethodGet, "/api/canvas-projects/"+canvasID+"/share", nil, nil)
	if err != nil {
		test.Fatal(err)
	}
	runner.RequireError(test, response, envelope, http.StatusNotFound, "")
	return response.StatusCode
}
