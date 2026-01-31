package immich_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/fshelper"
)

func TestAssetUpload_MetadataFields(t *testing.T) {
	// Create a temporary file for upload
	tmpFile, err := os.CreateTemp("", "test-asset-*.jpg")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	_, _ = tmpFile.Write([]byte("dummy content"))

	dir := filepath.Dir(tmpFile.Name())
	base := filepath.Base(tmpFile.Name())
	fsys := os.DirFS(dir)

	t.Run("sends metadata when present", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/users/me" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"1"}`))
				return
			}
			if r.URL.Path == "/api/server/media-types" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"image":[".jpg"]}`))
				return
			}

			if r.URL.Path == "/api/assets" {
				err := r.ParseMultipartForm(10 << 20)
				if err != nil {
					t.Fatalf("failed to parse multipart form: %v", err)
				}

				if got := r.FormValue("description"); got != "My Description" {
					t.Errorf("expected description 'My Description', got '%s'", got)
				}
				if got := r.FormValue("rating"); got != "5" {
					t.Errorf("expected rating '5', got '%s'", got)
				}
				if got := r.FormValue("latitude"); !strings.HasPrefix(got, "12.34") {
					t.Errorf("expected latitude starting with '12.34', got '%s'", got)
				}
				if got := r.FormValue("longitude"); !strings.HasPrefix(got, "56.78") {
					t.Errorf("expected longitude starting with '56.78', got '%s'", got)
				}
				if got := r.FormValue("dateTimeOriginal"); got == "" {
					t.Error("expected dateTimeOriginal to be present")
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"123","status":"created"}`))
				return
			}
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}))
		defer server.Close()

		client, _ := immich.NewImmichClient(server.URL, "test-key")

		_, err := client.ValidateConnection(context.Background())
		if err != nil {
			t.Fatalf("ValidateConnection failed: %v", err)
		}

		asset := &assets.Asset{
			File:             fshelper.FSName(fsys, base),
			OriginalFileName: "test.jpg",
			FileSize:         13,
			Description:      "My Description",
			Rating:           5,
			Latitude:         12.34,
			Longitude:        56.78,
			CaptureDate:      time.Now(),
		}

		_, err = client.AssetUpload(context.Background(), asset)
		if err != nil {
			t.Fatalf("AssetUpload failed: %v", err)
		}
	})

	t.Run("omits metadata when empty/zero", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/users/me" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"1"}`))
				return
			}
			if r.URL.Path == "/api/server/media-types" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"image":[".jpg"]}`))
				return
			}

			if r.URL.Path == "/api/assets" {
				err := r.ParseMultipartForm(10 << 20)
				if err != nil {
					t.Fatalf("failed to parse multipart form: %v", err)
				}

				if got := r.FormValue("description"); got != "" {
					t.Errorf("expected no description, got '%s'", got)
				}
				if got := r.FormValue("rating"); got != "" {
					t.Errorf("expected no rating, got '%s'", got)
				}
				if got := r.FormValue("latitude"); got != "" {
					t.Errorf("expected no latitude, got '%s'", got)
				}
				if got := r.FormValue("longitude"); got != "" {
					t.Errorf("expected no longitude, got '%s'", got)
				}
				if got := r.FormValue("dateTimeOriginal"); got != "" {
					t.Errorf("expected no dateTimeOriginal, got '%s'", got)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"123","status":"created"}`))
				return
			}
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}))
		defer server.Close()

		client, _ := immich.NewImmichClient(server.URL, "test-key")

		_, err := client.ValidateConnection(context.Background())
		if err != nil {
			t.Fatalf("ValidateConnection failed: %v", err)
		}

		asset := &assets.Asset{
			File:             fshelper.FSName(fsys, base),
			OriginalFileName: "test.jpg",
			FileSize:         13,
			// Zero values
			Description: "",
			Rating:      0,
			Latitude:    0,
			Longitude:   0,
			CaptureDate: time.Time{},
		}

		_, err = client.AssetUpload(context.Background(), asset)
		if err != nil {
			t.Fatalf("AssetUpload failed: %v", err)
		}
	})
}
