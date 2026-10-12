package canvas_contract_test

import (
	"net/http"
	"reflect"
	"sort"
	"testing"

	"canvas-contract/runner"
)

type legacyOwnershipFixture struct {
	OwnerAsset    string
	ForeignAsset  string
	OwnerFolder   string
	ForeignFolder string
	Content       []byte
}

type visibleLegacyState struct {
	List      map[string]any
	Folders   map[string]any
	Assets    []map[string]any
	Metadata  []map[string]any
	Resources visibleResourceState
}

func readLegacyState(test *testing.T, client *runner.Client) visibleLegacyState {
	test.Helper()
	_, envelope := requireHTTPSuccess(test, client, http.MethodGet, "/api/assets", nil)
	list := runner.DecodeData[map[string]any](test, envelope)
	identities := runner.DecodeData[struct {
		Assets []struct {
			ID string `json:"id"`
		} `json:"assets"`
	}](test, envelope)
	_, envelope = requireHTTPSuccess(test, client, http.MethodGet, "/api/asset-folders", nil)
	state := visibleLegacyState{List: list, Folders: runner.DecodeData[map[string]any](test, envelope), Resources: resourceState(test, client)}
	for _, identity := range identities.Assets {
		_, envelope = requireHTTPSuccess(test, client, http.MethodGet, "/api/assets/"+identity.ID, nil)
		state.Assets = append(state.Assets, runner.DecodeData[struct {
			Asset map[string]any `json:"asset"`
		}](test, envelope).Asset)
	}
	sort.Slice(state.Assets, func(first, second int) bool {
		return state.Assets[first]["id"].(string) < state.Assets[second]["id"].(string)
	})
	for _, resourceID := range state.Resources.IDs {
		_, envelope = requireHTTPSuccess(test, client, http.MethodGet, "/api/resources/"+resourceID, nil)
		state.Metadata = append(state.Metadata, runner.DecodeData[map[string]any](test, envelope))
	}
	return state
}

func createLegacyOwnershipFixture(test *testing.T, owner, foreign *runner.Client, label string) legacyOwnershipFixture {
	test.Helper()
	content := runner.ImageFixture(test)
	fixture := legacyOwnershipFixture{OwnerAsset: uploadResource(test, owner, label+"-owner.png", "image", content), ForeignAsset: uploadResource(test, foreign, label+"-foreign.png", "image", content), Content: content}
	for _, item := range []struct {
		Client *runner.Client
		Asset  string
		Folder *string
	}{{owner, fixture.OwnerAsset, &fixture.OwnerFolder}, {foreign, fixture.ForeignAsset, &fixture.ForeignFolder}} {
		_, envelope := requireHTTPSuccess(test, item.Client, http.MethodPost, "/api/asset-folders", map[string]any{"name": label})
		*item.Folder = runner.DecodeData[struct {
			Folder struct {
				ID string `json:"id"`
			} `json:"folder"`
		}](test, envelope).Folder.ID
		if *item.Folder == "" {
			test.Fatal("ownership fixture did not return a folder identity")
		}
		asset := legacyAssetPayload(item.Asset, label, len(content))
		asset["folderId"] = *item.Folder
		upsertAsset(test, item.Client, item.Asset, asset)
	}
	if fixture.OwnerFolder == fixture.ForeignFolder {
		test.Fatal("same-name folders were not independently owned")
	}
	return fixture
}

func requireLegacyOwnershipUnchanged(test *testing.T, owner, foreign *runner.Client, beforeOwner, beforeForeign visibleLegacyState, fixture legacyOwnershipFixture) {
	test.Helper()
	if !reflect.DeepEqual(readLegacyState(test, owner), beforeOwner) || !reflect.DeepEqual(readLegacyState(test, foreign), beforeForeign) {
		test.Fatal("failed ownership mutation changed a complete asset, folder, resource metadata or quota state")
	}
	requireResourceBytes(test, owner, fixture.OwnerAsset, fixture.Content)
	requireResourceBytes(test, foreign, fixture.ForeignAsset, fixture.Content)
}

func runLegacyForeignWriteMatrix(test *testing.T, owner, foreign *runner.Client) {
	fixture := createLegacyOwnershipFixture(test, owner, foreign, "Legacy foreign write matrix")
	beforeOwner, beforeForeign := readLegacyState(test, owner), readLegacyState(test, foreign)
	anonymous, err := runner.NewClient("anonymous", owner.Config)
	if err != nil {
		test.Fatal(err)
	}
	defer anonymous.HTTP.CloseIdleConnections()
	for _, request := range []struct {
		Method, Path string
		Payload      any
	}{
		{http.MethodPost, "/api/asset-folders", map[string]any{"name": "Anonymous forbidden"}},
		{http.MethodPatch, "/api/asset-folders/" + fixture.OwnerFolder, map[string]any{"name": "Anonymous forbidden"}},
		{http.MethodDelete, "/api/asset-folders/" + fixture.OwnerFolder, nil},
		{http.MethodPatch, "/api/assets/folder", map[string]any{"assetIds": []string{fixture.OwnerAsset}, "folderId": fixture.OwnerFolder}},
		{http.MethodPut, "/api/assets/" + fixture.OwnerAsset, map[string]any{"asset": legacyAssetPayload(fixture.OwnerAsset, "Anonymous forbidden", len(fixture.Content))}},
		{http.MethodDelete, "/api/assets/" + fixture.OwnerAsset, nil},
		{http.MethodPost, "/api/assets/batch-delete", []string{fixture.OwnerAsset}},
	} {
		requireHTTPError(test, anonymous, request.Method, request.Path, request.Payload, http.StatusUnauthorized)
		requireLegacyOwnershipUnchanged(test, owner, foreign, beforeOwner, beforeForeign, fixture)
	}
	for _, side := range []struct {
		Client                                 *runner.Client
		Asset, Folder, OtherAsset, OtherFolder string
	}{
		{foreign, fixture.ForeignAsset, fixture.ForeignFolder, fixture.OwnerAsset, fixture.OwnerFolder},
		{owner, fixture.OwnerAsset, fixture.OwnerFolder, fixture.ForeignAsset, fixture.ForeignFolder},
	} {
		foreignFolderPayload := legacyAssetPayload(side.Asset, "Foreign folder forbidden", len(fixture.Content))
		foreignFolderPayload["folderId"] = side.OtherFolder
		for _, request := range []struct {
			Method, Path string
			Payload      any
			Status       int
		}{
			{http.MethodPatch, "/api/asset-folders/" + side.OtherFolder, map[string]any{"name": "Foreign forbidden"}, http.StatusBadRequest},
			{http.MethodDelete, "/api/asset-folders/" + side.OtherFolder, nil, http.StatusBadRequest},
			{http.MethodPatch, "/api/assets/folder", map[string]any{"assetIds": []string{side.Asset}, "folderId": side.OtherFolder}, http.StatusBadRequest},
			{http.MethodPatch, "/api/assets/folder", map[string]any{"assetIds": []string{side.OtherAsset}, "folderId": side.Folder}, http.StatusBadRequest},
			{http.MethodPut, "/api/assets/" + side.Asset, map[string]any{"asset": foreignFolderPayload}, http.StatusBadRequest},
			{http.MethodPut, "/api/assets/" + side.OtherAsset, map[string]any{"asset": legacyAssetPayload(side.OtherAsset, "Foreign forbidden", len(fixture.Content))}, http.StatusInternalServerError},
			{http.MethodDelete, "/api/assets/" + side.OtherAsset, nil, http.StatusNotFound},
			{http.MethodPost, "/api/assets/batch-delete", []string{side.OtherAsset}, http.StatusNotFound},
		} {
			requireHTTPError(test, side.Client, request.Method, request.Path, request.Payload, request.Status)
			requireLegacyOwnershipUnchanged(test, owner, foreign, beforeOwner, beforeForeign, fixture)
		}
	}
	test.Log("legacy_foreign_write: anonymous=401 folder/move=400 foreign-upsert=500 foreign-purge=404; complete two-owner state and bytes unchanged")
}

func runLegacyMixedOwnerRollback(test *testing.T, owner, foreign *runner.Client) {
	fixture := createLegacyOwnershipFixture(test, owner, foreign, "Legacy mixed owner rollback")
	beforeOwner, beforeForeign := readLegacyState(test, owner), readLegacyState(test, foreign)
	for _, side := range []struct {
		Client                    *runner.Client
		Asset, Folder, OtherAsset string
	}{
		{owner, fixture.OwnerAsset, fixture.OwnerFolder, fixture.ForeignAsset},
		{foreign, fixture.ForeignAsset, fixture.ForeignFolder, fixture.OwnerAsset},
	} {
		for _, identities := range [][]string{{side.Asset, side.OtherAsset}, {side.OtherAsset, side.Asset}, {side.Asset, "contract-missing-legacy-asset"}, {"contract-missing-legacy-asset", side.Asset}} {
			for _, destination := range []string{"", side.Folder} {
				requireHTTPError(test, side.Client, http.MethodPatch, "/api/assets/folder", map[string]any{"assetIds": identities, "folderId": destination}, http.StatusBadRequest)
				requireLegacyOwnershipUnchanged(test, owner, foreign, beforeOwner, beforeForeign, fixture)
			}
			requireHTTPError(test, side.Client, http.MethodPost, "/api/assets/batch-delete", identities, http.StatusNotFound)
			requireLegacyOwnershipUnchanged(test, owner, foreign, beforeOwner, beforeForeign, fixture)
		}
	}
}
