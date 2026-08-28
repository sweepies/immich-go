package immich

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/immichtest"
)

func TestAssetUploadContracts(t *testing.T) {
	tests := []struct {
		name           string
		profile        immichtest.Profile
		wantDuration   string
		wantDeviceData bool
	}{
		{name: "v2.7.5", profile: immichtest.V275(), wantDuration: "00:00:00.000000", wantDeviceData: true},
		{name: "v3.1.0", profile: immichtest.V310(), wantDuration: "0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := test.profile
			server := immichtest.NewServer(t, profile)
			server.Handle(http.MethodPost, "/api/assets", func(request immichtest.Request) immichtest.Response {
				return immichtest.JSONResponse(http.StatusCreated, AssetResponse{ID: "asset-id", Status: UploadCreated})
			})
			client, err := NewImmichClient(server.URL, profile.APIKey)
			if err != nil {
				t.Fatal(err)
			}
			client.SetDeviceUUID("contract-device")
			if _, err := client.ValidateConnection(context.Background()); err != nil {
				t.Fatal(err)
			}

			assetFS := fstest.MapFS{
				"photo.jpg":     &fstest.MapFile{Data: []byte("jpeg-data"), ModTime: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)},
				"photo.jpg.xmp": &fstest.MapFile{Data: []byte("xmp-data")},
			}
			asset := &assets.Asset{
				File:             fshelper.NewFilename(assetFS, "photo.jpg"),
				OriginalFileName: "photo.jpg",
				CaptureDate:      time.Date(2025, time.December, 1, 2, 3, 4, 0, time.UTC),
				Archived:         true,
				Favorite:         true,
				Checksum:         "contract-checksum",
				FromSideCar: &assets.Metadata{
					File: fshelper.NewFilename(assetFS, "photo.jpg.xmp"),
				},
			}
			t.Cleanup(func() { _ = asset.Close() })

			response, err := client.AssetUpload(context.Background(), asset)
			if err != nil {
				t.Fatal(err)
			}
			if response.ID != "asset-id" || response.Status != UploadCreated {
				t.Errorf("upload response = %#v", response)
			}
			if client.ServerVersion().String() != profile.Version {
				t.Errorf("lazy server version = %q, want %q", client.ServerVersion(), profile.Version)
			}

			requests := server.Requests()
			uploadRequest := requests[len(requests)-1]
			if uploadRequest.Method != http.MethodPost || uploadRequest.Path != "/api/assets" {
				t.Errorf("upload request = %s %s", uploadRequest.Method, uploadRequest.Path)
			}
			if uploadRequest.Header.Get("x-api-key") != profile.APIKey {
				t.Errorf("API key header = %q", uploadRequest.Header.Get("x-api-key"))
			}
			if uploadRequest.Header.Get("x-immich-checksum") != asset.Checksum {
				t.Errorf("checksum header = %q", uploadRequest.Header.Get("x-immich-checksum"))
			}
			if uploadRequest.Header.Get("Accept") != "application/json" {
				t.Errorf("accept header = %q", uploadRequest.Header.Get("Accept"))
			}

			fields := uploadRequest.Multipart
			wantFieldNames := []string{"assetData", "duration", "fileCreatedAt", "fileModifiedAt", "isFavorite", "sidecarData", "visibility"}
			if test.wantDeviceData {
				wantFieldNames = append(wantFieldNames, "deviceAssetId", "deviceId")
			}
			gotFieldNames := make([]string, 0, len(fields))
			for name := range fields {
				gotFieldNames = append(gotFieldNames, name)
			}
			sort.Strings(gotFieldNames)
			sort.Strings(wantFieldNames)
			if !reflect.DeepEqual(gotFieldNames, wantFieldNames) {
				t.Errorf("multipart fields = %v, want %v", gotFieldNames, wantFieldNames)
			}
			assertMultipartValue(t, fields, "duration", test.wantDuration)
			assertMultipartValue(t, fields, "fileCreatedAt", "2025-12-01T02:03:04.000Z")
			assertMultipartTime(t, fields, "fileModifiedAt")
			assertMultipartValue(t, fields, "isFavorite", "true")
			assertMultipartValue(t, fields, "visibility", "archive")
			if test.wantDeviceData {
				assertMultipartValue(t, fields, "deviceId", "contract-device")
				assertMultipartValue(t, fields, "deviceAssetId", "photo.jpg-9")
			}
			if field := fields["assetData"]; len(field) != 1 || field[0].Filename != "photo.jpg" || string(field[0].Value) != "jpeg-data" {
				t.Errorf("assetData = %#v", field)
			}
			if field := fields["sidecarData"]; len(field) != 1 || field[0].Filename != "photo.jpg.xmp" || string(field[0].Value) != "xmp-data" {
				t.Errorf("sidecarData = %#v", field)
			}
		})
	}
}

func TestFormatUploadDuration(t *testing.T) {
	duration := 1500 * time.Millisecond
	if got, ok := formatUploadDuration(ServerVersion{value: "v2.7.5", major: 2}, &duration); !ok || got != "00:00:01.000500" {
		t.Errorf("v2 duration = %q, %t", got, ok)
	}
	if got, ok := formatUploadDuration(ServerVersion{value: "v3.1.0", major: 3}, &duration); !ok || got != "1500" {
		t.Errorf("v3 duration = %q, %t", got, ok)
	}
	if got, ok := formatUploadDuration(ServerVersion{value: "v3.1.0", major: 3}, nil); ok || got != "" {
		t.Errorf("nil duration = %q, %t", got, ok)
	}
}

func TestAssetUploadDryRunDoesNotDiscoverVersion(t *testing.T) {
	server := immichtest.NewServer(t, immichtest.V310())
	client, err := NewImmichClient(server.URL, "contract-api-key", OptionDryRun(true))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.AssetUpload(context.Background(), &assets.Asset{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != UploadCreated || response.ID == "" {
		t.Errorf("dry-run response = %#v", response)
	}
	if requests := server.Requests(); len(requests) != 0 {
		t.Errorf("dry-run made %d requests", len(requests))
	}
}

func TestAssetUploadVideoContract(t *testing.T) {
	profile := immichtest.V310()
	server, client := newUploadContractClient(t, profile, func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusCreated, AssetResponse{ID: "video-id", Status: UploadCreated})
	})
	asset := newUploadContractAsset(t, "clip.mp4", []byte("video-data"), false)

	response, err := client.AssetUpload(context.Background(), asset)
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "video-id" {
		t.Errorf("video response = %#v", response)
	}
	requests := server.Requests()
	upload := requests[len(requests)-1]
	assertMultipartValue(t, upload.Multipart, "duration", "0")
	if field := upload.Multipart["assetData"]; len(field) != 1 || field[0].Filename != "clip.mp4" || string(field[0].Value) != "video-data" {
		t.Errorf("video assetData = %#v", field)
	}
	if _, ok := upload.Multipart["deviceId"]; ok {
		t.Error("v3 video request contains deviceId")
	}
	if _, ok := upload.Multipart["deviceAssetId"]; ok {
		t.Error("v3 video request contains deviceAssetId")
	}
}

func TestAssetUploadDuplicateResponses(t *testing.T) {
	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		t.Run(profile.Version, func(t *testing.T) {
			_, client := newUploadContractClient(t, profile, func(request immichtest.Request) immichtest.Response {
				return immichtest.JSONResponse(http.StatusOK, AssetResponse{ID: "existing-id", Status: UploadDuplicate})
			})
			asset := newUploadContractAsset(t, "duplicate.jpg", []byte("duplicate"), false)

			response, err := client.AssetUpload(context.Background(), asset)
			if err != nil {
				t.Fatal(err)
			}
			if response.ID != "existing-id" || response.Status != UploadDuplicate {
				t.Errorf("duplicate response = %#v", response)
			}
		})
	}
}

func TestAssetUploadServerErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		message string
	}{
		{name: "retryable", status: http.StatusInternalServerError, message: "Temporary server failure"},
		{name: "terminal", status: http.StatusBadRequest, message: "Invalid upload"},
	}
	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		for _, test := range tests {
			t.Run(profile.Version+"/"+test.name, func(t *testing.T) {
				server, client := newUploadContractClient(t, profile, func(request immichtest.Request) immichtest.Response {
					return profile.ValidationResponse(test.status, "upload-correlation", immichtest.ValidationError{
						Path:    []any{"assetData"},
						Message: test.message,
					})
				})
				asset := newUploadContractAsset(t, "failure.jpg", []byte("failure"), false)

				_, err := client.AssetUpload(context.Background(), asset)
				if err == nil {
					t.Fatal("expected upload error")
				}
				for _, expected := range []string{
					fmt.Sprintf("%d %s", test.status, http.StatusText(test.status)),
					http.MethodPost,
					"/api/assets",
					test.message,
					"Correlation ID: upload-correlation",
				} {
					if !strings.Contains(err.Error(), expected) {
						t.Errorf("error %q does not contain %q", err, expected)
					}
				}
				uploadCalls := 0
				for _, request := range server.Requests() {
					if request.Path == "/api/assets" {
						uploadCalls++
					}
				}
				if uploadCalls != 1 {
					t.Errorf("upload calls = %d, want 1", uploadCalls)
				}
			})
		}
	}
}

func newUploadContractClient(t *testing.T, profile immichtest.Profile, handler immichtest.Handler) (*immichtest.Server, *ImmichClient) {
	t.Helper()
	server := immichtest.NewServer(t, profile)
	server.Handle(http.MethodPost, "/api/assets", handler)
	client, err := NewImmichClient(server.URL, profile.APIKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ValidateConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
	return server, client
}

func newUploadContractAsset(t *testing.T, filename string, data []byte, withSidecar bool) *assets.Asset {
	t.Helper()
	assetFS := fstest.MapFS{
		filename: &fstest.MapFile{Data: append([]byte(nil), data...)},
	}
	asset := &assets.Asset{
		File:             fshelper.NewFilename(assetFS, filename),
		OriginalFileName: filename,
		Checksum:         "contract-checksum",
	}
	if withSidecar {
		sidecarName := filename + ".xmp"
		assetFS[sidecarName] = &fstest.MapFile{Data: []byte("xmp-data")}
		asset.FromSideCar = &assets.Metadata{File: fshelper.NewFilename(assetFS, sidecarName)}
	}
	t.Cleanup(func() { _ = asset.Close() })
	return asset
}

func assertMultipartValue(t *testing.T, fields map[string][]immichtest.MultipartField, name, want string) {
	t.Helper()
	field := fields[name]
	if len(field) != 1 || field[0].Filename != "" || string(field[0].Value) != want {
		t.Errorf("%s = %#v, want %q", name, field, want)
	}
}

func assertMultipartTime(t *testing.T, fields map[string][]immichtest.MultipartField, name string) {
	t.Helper()
	field := fields[name]
	if len(field) != 1 || field[0].Filename != "" {
		t.Errorf("%s = %#v", name, field)
		return
	}
	if _, err := time.Parse(TimeFormat, string(field[0].Value)); err != nil {
		t.Errorf("%s = %q: %v", name, field[0].Value, err)
	}
}
