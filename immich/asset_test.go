package immich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/immichtest"
)

func Test_AssetJSON(t *testing.T) {
	js := `{
 "id": "9a2fff7a-f226-48e8-a888-fdac199f3d56",
 "deviceAssetId": "IMG_20180811_173822_1.jpg-2082855",
 "ownerId": "13e05729-8933-494e-982e-5910a0c4420f",
 "deviceId": "DESKTOP-ILBKKE7",
 "type": "IMAGE",
 "originalPath": "upload/upload/13e05729-8933-494e-982e-5910a0c4420f/17/6c/176c335a-fbc0-412f-a46f-c187351a55bd.jpg",
 "originalFileName": "IMG_20180811_173822_1.jpg",
 "resized": true,
 "thumbhash": "WRgGDQTZeaiYNz6FCUQXZg4BtAAV",
 "fileCreatedAt": "\"2018-08-11T19:38:22+02:00\"",
 "fileModifiedAt": "\"2024-07-07T17:29:15+02:00\"",
 "updatedAt": "\"2024-11-17T18:57:15+01:00\"",
 "isFavorite": false,
 "isArchived": false,
 "isTrashed": false,
 "duration": "0:00:00.00000",
 "rating": 0,
 "exifInfo": {
  "make": "HUAWEI",
  "model": "CLT-L09",
  "exifImageWidth": 2736,
  "exifImageHeight": 3648,
  "fileSizeInByte": 2082855,
  "orientation": "0",
  "dateTimeOriginal": "\"2018-08-11T19:38:22+02:00\"",
  "timeZone": "Europe/Paris",
  "latitude": 48.8413085936111,
  "longitude": 2.4199056625,
  "description": "oznor"
 },
 "livePhotoVideoId": "",
 "checksum": "fDpZUcgYJjZnzLAHfIddp8BLzjE=",
 "stackParentId": "",
 "tags": [
  {
   "id": "e6745272-71d2-4a61-976e-d4ac6b7de3b8",
   "name": "tag2",
   "value": "tag1/tag2"
  },
  {
   "id": "bbfd950a-f1b5-4e2d-acc9-e000a27d41e5",
   "name": "activities",
   "value": "activities"
  }
 ]
}`

	asset := Asset{}
	dec := json.NewDecoder(strings.NewReader(js))
	err := dec.Decode(&asset)
	if err != nil {
		t.Error(err)
	}
	if len(asset.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(asset.Tags))
	}
	expectedTags := []struct {
		ID    string
		Name  string
		Value string
	}{
		{"e6745272-71d2-4a61-976e-d4ac6b7de3b8", "tag2", "tag1/tag2"},
		{"bbfd950a-f1b5-4e2d-acc9-e000a27d41e5", "activities", "activities"},
	}

	for i, tag := range asset.Tags {
		if tag.ID.String() != expectedTags[i].ID || tag.Name != expectedTags[i].Name || tag.Value != expectedTags[i].Value {
			t.Errorf("expected tag %v, got %v", expectedTags[i], tag)
		}
	}
}

func TestAssetResponseDurationGenerations(t *testing.T) {
	tests := []struct {
		name     string
		profile  immichtest.Profile
		duration string
	}{
		{name: "v2 string", profile: immichtest.V275(), duration: `"00:00:01.500000"`},
		{name: "v3 integer", profile: immichtest.V310(), duration: `1500`},
		{name: "v3 null", profile: immichtest.V310(), duration: `null`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := immichtest.NewServer(t, test.profile)
			server.Handle(http.MethodGet, "/api/assets/asset-id", func(request immichtest.Request) immichtest.Response {
				return immichtest.Response{
					Status: http.StatusOK,
					Header: http.Header{"Content-Type": {"application/json"}},
					Body:   []byte(assetResponseJSON(test.duration)),
				}
			})
			client, err := NewImmichClient(server.URL, test.profile.APIKey)
			if err != nil {
				t.Fatal(err)
			}

			asset, err := client.GetAssetInfo(context.Background(), "asset-id")
			if err != nil {
				t.Fatal(err)
			}
			assertDecodedAsset(t, asset)
			assertConvertedAsset(t, asset.AsAsset())
		})
	}
}

func TestMetadataSearchDecodesV3Assets(t *testing.T) {
	profile := immichtest.V310()
	server := immichtest.NewServer(t, profile)
	server.Handle(http.MethodPost, "/api/search/metadata", func(request immichtest.Request) immichtest.Response {
		body := fmt.Sprintf(`{"assets":{"total":1,"count":1,"items":[%s],"nextPage":"0"}}`, assetResponseJSON(`null`))
		return immichtest.Response{
			Status: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   []byte(body),
		}
	})
	client, err := NewImmichClient(server.URL, profile.APIKey)
	if err != nil {
		t.Fatal(err)
	}

	var decoded *Asset
	err = client.callSearchMetadata(context.Background(), &SearchMetadataQuery{Size: 2}, func(asset *Asset) error {
		decoded = asset
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDecodedAsset(t, decoded)
	requests := server.Requests()
	search := requests[len(requests)-1]
	if search.Method != http.MethodPost || search.Path != "/api/search/metadata" {
		t.Errorf("search request = %s %s", search.Method, search.Path)
	}
}

func TestCopyAssetContract(t *testing.T) {
	profile := immichtest.V310()
	server := immichtest.NewServer(t, profile)
	server.Handle(http.MethodPut, "/api/assets/copy", func(request immichtest.Request) immichtest.Response {
		var body struct {
			SourceID    AssetID `json:"sourceId"`
			TargetID    AssetID `json:"targetId"`
			Albums      bool    `json:"albums"`
			Favorite    bool    `json:"favorite"`
			SharedLinks bool    `json:"sharedLinks"`
			Sidecar     bool    `json:"sidecar"`
			Stack       bool    `json:"stack"`
		}
		if err := json.Unmarshal(request.JSON, &body); err != nil {
			t.Error(err)
		}
		if body.SourceID != "source-id" || body.TargetID != "target-id" ||
			!body.Albums || !body.Favorite || !body.SharedLinks || !body.Sidecar || !body.Stack {
			t.Errorf("copy request = %#v", body)
		}
		return immichtest.Response{Status: http.StatusNoContent}
	})
	client, err := NewImmichClient(server.URL, profile.APIKey)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.CopyAsset(context.Background(), "source-id", "target-id"); err != nil {
		t.Fatal(err)
	}
	requests := server.Requests()
	copyRequest := requests[len(requests)-1]
	if copyRequest.Method != http.MethodPut || copyRequest.Path != "/api/assets/copy" {
		t.Errorf("copy request = %s %s", copyRequest.Method, copyRequest.Path)
	}
}

func assetResponseJSON(duration string) string {
	return fmt.Sprintf(`{
		"id":"asset-id",
		"checksum":"checksum",
		"deviceAssetId":"legacy-device-asset",
		"deviceId":"legacy-device",
		"duration":%s,
		"type":"IMAGE",
		"originalFileName":"photo.jpg",
		"originalPath":"upload/photo.jpg",
		"ownerId":"owner-id",
		"fileCreatedAt":"2025-12-01T02:03:04.000Z",
		"fileModifiedAt":"2026-01-02T03:04:05.000Z",
		"localDateTime":"2025-12-01T02:03:04.000Z",
		"updatedAt":"2026-01-02T03:04:05.000Z",
		"isFavorite":true,
		"isArchived":true,
		"isTrashed":false,
		"rating":4,
		"visibility":"archive",
		"exifInfo":{
			"fileSizeInByte":9,
			"dateTimeOriginal":"2025-12-01T02:03:04+00:00",
			"latitude":1.5,
			"longitude":2.5,
			"description":"description"
		},
		"tags":[{"id":"tag-id","name":"tag","value":"tag"}]
	}`, duration)
}

func assertDecodedAsset(t *testing.T, asset *Asset) {
	t.Helper()
	if asset == nil {
		t.Fatal("asset is nil")
	}
	if asset.ID != "asset-id" || asset.OriginalFileName != "photo.jpg" || asset.Checksum != "checksum" {
		t.Errorf("asset identity = %#v", asset)
	}
	if asset.FileModifiedAt.IsZero() || asset.ExifInfo.DateTimeOriginal.IsZero() {
		t.Errorf("asset timestamps were not decoded: %#v", asset)
	}
	if !asset.IsArchived || !asset.IsFavorite || asset.Rating != 4 || len(asset.Tags) != 1 {
		t.Errorf("asset metadata = %#v", asset)
	}
}

func assertConvertedAsset(t *testing.T, converted *assets.Asset) {
	t.Helper()
	if converted.ID != "asset-id" || converted.OriginalFileName != "photo.jpg" ||
		converted.Checksum != "checksum" || !converted.Archived || !converted.Favorite ||
		converted.Rating != 4 || converted.Description != "description" ||
		converted.Latitude != 1.5 || converted.Longitude != 2.5 || len(converted.Tags) != 1 {
		t.Errorf("converted asset = %#v", converted)
	}
}
