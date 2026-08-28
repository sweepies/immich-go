package immich

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/sweepies/immich-go/internal/immichtest"
)

func TestGetAlbumAssetIDsPaginationContracts(t *testing.T) {
	for _, baseProfile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		t.Run(baseProfile.Version, func(t *testing.T) {
			profile := baseProfile
			profile.PageSize = 2
			server := immichtest.NewServer(t, profile)
			allIDs := []AssetID{"asset-1", "asset-2", "asset-2", "asset-3", "asset-4"}
			server.Handle(http.MethodPost, "/api/search/metadata", func(request immichtest.Request) immichtest.Response {
				var query SearchMetadataQuery
				if err := json.Unmarshal(request.JSON, &query); err != nil {
					t.Error(err)
				}
				pageIDs, hasNext := immichtest.Paginate(profile, allIDs, query.Page)
				items := make([]map[string]AssetID, len(pageIDs))
				for i, id := range pageIDs {
					items[i] = map[string]AssetID{"id": id}
				}
				nextPage := 0
				if hasNext {
					nextPage = query.Page + 1
				}
				return immichtest.JSONResponse(http.StatusOK, map[string]any{
					"assets": map[string]any{
						"total":    len(allIDs),
						"count":    len(items),
						"items":    items,
						"nextPage": strconv.Itoa(nextPage),
					},
				})
			})
			client, err := NewImmichClient(server.URL, profile.APIKey)
			if err != nil {
				t.Fatal(err)
			}

			ids, err := client.GetAlbumAssetIDs(context.Background(), "album-id")
			if err != nil {
				t.Fatal(err)
			}
			wantIDs := []AssetID{"asset-1", "asset-2", "asset-3", "asset-4"}
			if !reflect.DeepEqual(ids, wantIDs) {
				t.Errorf("asset IDs = %v, want %v", ids, wantIDs)
			}

			requests := server.Requests()
			if len(requests) != 3 {
				t.Fatalf("requests = %d, want 3", len(requests))
			}
			for i, request := range requests {
				if request.Method != http.MethodPost || request.Path != "/api/search/metadata" {
					t.Errorf("request %d = %s %s", i, request.Method, request.Path)
				}
				if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Accept") != "application/json" {
					t.Errorf("request %d headers = %v", i, request.Header)
				}
				var query SearchMetadataQuery
				if err := json.Unmarshal(request.JSON, &query); err != nil {
					t.Fatal(err)
				}
				if query.Page != i+1 || query.Size != 1000 || !reflect.DeepEqual(query.AlbumIds, []string{"album-id"}) {
					t.Errorf("request %d query = %#v", i, query)
				}
			}
		})
	}
}

func TestAlbumReconciliationAndMutationContracts(t *testing.T) {
	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		t.Run(profile.Version, func(t *testing.T) {
			server := immichtest.NewServer(t, profile)
			server.Handle(http.MethodGet, "/api/albums", func(request immichtest.Request) immichtest.Response {
				return immichtest.JSONResponse(http.StatusOK, []map[string]any{{
					"id":          "existing-id",
					"albumName":   "Existing",
					"description": "existing description",
				}})
			})
			server.Handle(http.MethodPost, "/api/search/metadata", func(request immichtest.Request) immichtest.Response {
				return immichtest.JSONResponse(http.StatusOK, map[string]any{
					"assets": map[string]any{
						"total":    1,
						"count":    1,
						"items":    []map[string]AssetID{{"id": "asset-1"}},
						"nextPage": "0",
					},
				})
			})
			server.Handle(http.MethodPost, "/api/albums", func(request immichtest.Request) immichtest.Response {
				var body struct {
					AlbumName   string    `json:"albumName"`
					Description string    `json:"description"`
					AssetIDs    []AssetID `json:"assetIds"`
				}
				if err := json.Unmarshal(request.JSON, &body); err != nil {
					t.Error(err)
				}
				if body.AlbumName != "New" || body.Description != "new description" ||
					!reflect.DeepEqual(body.AssetIDs, []AssetID{"asset-2"}) {
					t.Errorf("create album body = %#v", body)
				}
				return immichtest.JSONResponse(http.StatusCreated, map[string]any{
					"id":          "new-id",
					"albumName":   body.AlbumName,
					"description": body.Description,
				})
			})
			server.Handle(http.MethodPut, "/api/albums/existing-id/assets", func(request immichtest.Request) immichtest.Response {
				var body struct {
					IDs []AssetID `json:"ids"`
				}
				if err := json.Unmarshal(request.JSON, &body); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(body.IDs, []AssetID{"asset-2"}) {
					t.Errorf("add assets body = %#v", body)
				}
				return immichtest.JSONResponse(http.StatusOK, []UpdateAlbumResult{{
					ID:      "asset-2",
					Success: true,
				}})
			})
			client, err := NewImmichClient(server.URL, profile.APIKey)
			if err != nil {
				t.Fatal(err)
			}

			albums, err := client.GetAllAlbums(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(albums) != 1 || albums[0].ID != "existing-id" || albums[0].AlbumName != "Existing" {
				t.Fatalf("existing albums = %#v", albums)
			}
			membership, err := client.GetAlbumAssetIDs(context.Background(), albums[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(membership, []AssetID{"asset-1"}) {
				t.Errorf("existing membership = %v", membership)
			}

			created, err := client.CreateAlbum(context.Background(), "New", "new description", []AssetID{"asset-2"})
			if err != nil {
				t.Fatal(err)
			}
			if created.ID != "new-id" || created.Title != "New" || created.Description != "new description" {
				t.Errorf("created album = %#v", created)
			}

			results, err := client.AddAssetToAlbum(context.Background(), albums[0].ID, []AssetID{"asset-2"})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].ID != "asset-2" || !results[0].Success {
				t.Errorf("add results = %#v", results)
			}

			for _, request := range server.Requests() {
				if request.Path == "/api/albums/existing-id" {
					t.Error("album reconciliation requested embedded album assets")
				}
			}
		})
	}
}
