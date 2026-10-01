package immich

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/immichtest"
)

func TestMetadataSearchVisibilityContracts(t *testing.T) {
	searches := []struct {
		name         string
		options      func() *searchOptions
		visibilities []assets.Visibility
		trashed      bool
	}{
		{
			name:         "archive",
			options:      func() *searchOptions { return SearchOptions().WithOnlyArchived() },
			visibilities: []assets.Visibility{assets.VisibilityArchive},
		},
		{
			name:         "hidden",
			options:      func() *searchOptions { return SearchOptions().WithVisibility(assets.VisibilityHidden) },
			visibilities: []assets.Visibility{assets.VisibilityHidden},
		},
		{
			name:         "timeline",
			options:      func() *searchOptions { return SearchOptions().WithVisibility(assets.VisibilityTimeline) },
			visibilities: []assets.Visibility{assets.VisibilityTimeline},
		},
		{
			name:    "trashed",
			options: func() *searchOptions { return SearchOptions().WithOnlyTrashed() },
			visibilities: []assets.Visibility{
				assets.VisibilityArchive,
				assets.VisibilityHidden,
				assets.VisibilityTimeline,
			},
			trashed: true,
		},
	}

	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		for _, search := range searches {
			t.Run(profile.Version+"/"+search.name, func(t *testing.T) {
				server := immichtest.NewServer(t, profile)
				duration := `null`
				if profile.Version == "v2.7.5" {
					duration = `"00:00:00.000000"`
				}
				server.Handle(http.MethodPost, "/api/search/metadata", func(request immichtest.Request) immichtest.Response {
					return immichtest.Response{
						Status: http.StatusOK,
						Header: http.Header{"Content-Type": {"application/json"}},
						Body: []byte(`{"assets":{"total":1,"count":1,"items":[` +
							assetResponseJSON(duration) + `],"nextPage":"0"}}`),
					}
				})
				client, err := NewImmichClient(server.URL, profile.APIKey)
				if err != nil {
					t.Fatal(err)
				}

				var decoded atomic.Int64
				err = client.GetFilteredAssetsFn(context.Background(), search.options(), func(asset *Asset) error {
					assertDecodedAsset(t, asset)
					decoded.Add(1)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Load() != int64(len(search.visibilities)) {
					t.Errorf("decoded assets = %d, want %d", decoded.Load(), len(search.visibilities))
				}

				requests := server.Requests()
				if len(requests) != len(search.visibilities) {
					t.Fatalf("search requests = %d, want %d", len(requests), len(search.visibilities))
				}
				gotVisibilities := make([]string, 0, len(requests))
				for _, request := range requests {
					var query SearchMetadataQuery
					if err := json.Unmarshal(request.JSON, &query); err != nil {
						t.Fatal(err)
					}
					gotVisibilities = append(gotVisibilities, string(query.Visibility))
					if (query.TrashedAfter != "") != search.trashed {
						t.Errorf("query %#v trashed filter mismatch", query)
					}
				}
				wantVisibilities := make([]string, len(search.visibilities))
				for i, visibility := range search.visibilities {
					wantVisibilities[i] = string(visibility)
				}
				sort.Strings(gotVisibilities)
				sort.Strings(wantVisibilities)
				if !reflect.DeepEqual(gotVisibilities, wantVisibilities) {
					t.Errorf("visibilities = %v, want %v", gotVisibilities, wantVisibilities)
				}
			})
		}
	}
}
