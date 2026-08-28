package root

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/immichtest"
)

func TestUploadCommandEntryPointsAgainstProfiles(t *testing.T) {
	modes := []struct {
		name              string
		flag              string
		fixture           func(*testing.T) string
		modeRequestMethod string
		modeRequestPath   string
	}{
		{name: "folder", fixture: folderCommandFixture},
		{name: "google-takeout", flag: "--google", fixture: googleCommandFixture, modeRequestMethod: http.MethodPut, modeRequestPath: "/api/assets/uploaded-id"},
		{name: "icloud", flag: "--icloud", fixture: iCloudCommandFixture, modeRequestMethod: http.MethodPost, modeRequestPath: "/api/albums"},
		{name: "picasa", flag: "--picasa", fixture: picasaCommandFixture, modeRequestMethod: http.MethodPost, modeRequestPath: "/api/albums"},
	}

	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		for _, mode := range modes {
			t.Run(profile.Version+"/"+mode.name, func(t *testing.T) {
				target := newCommandContractServer(t, profile)
				args := []string{
					"--output=json",
					"--concurrent-tasks=1",
					"upload",
					"--server=" + target.URL,
					"--api-key=" + profile.APIKey,
					"--pause-immich-jobs=false",
				}
				if mode.flag != "" {
					args = append(args, mode.flag)
				}
				args = append(args, mode.fixture(t))

				output, err := executeRootCommand(t, args...)
				if err != nil {
					t.Fatal(err)
				}
				assertJSONSummary(t, output)
				assertRequested(t, target.Requests(), http.MethodGet, "/api/assets/statistics")
				assertRequested(t, target.Requests(), http.MethodPost, "/api/search/metadata")
				assertRequested(t, target.Requests(), http.MethodPost, "/api/assets")
				if mode.modeRequestPath != "" {
					assertRequested(t, target.Requests(), mode.modeRequestMethod, mode.modeRequestPath)
				}
			})
		}
	}
}

func TestFromImmichUploadCommandEntryPointAgainstProfiles(t *testing.T) {
	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		t.Run(profile.Version, func(t *testing.T) {
			source := newFromImmichCommandServer(t, profile)
			target := newCommandContractServer(t, profile)
			output, err := executeRootCommand(t,
				"--output=json",
				"--concurrent-tasks=1",
				"upload",
				"--from-immich",
				"--source-server="+source.URL,
				"--source-api-key="+profile.APIKey,
				"--server="+target.URL,
				"--api-key="+profile.APIKey,
				"--pause-immich-jobs=false",
			)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONSummary(t, output)
			assertRequested(t, source.Requests(), http.MethodGet, "/api/users/me")
			assertRequested(t, source.Requests(), http.MethodPost, "/api/search/metadata")
			assertRequested(t, target.Requests(), http.MethodGet, "/api/assets/statistics")
			assertRequested(t, source.Requests(), http.MethodGet, "/api/assets/source-id/original")
			assertRequested(t, target.Requests(), http.MethodPost, "/api/assets")
		})
	}
}

func TestArchiveFromImmichAndStackCommandEntryPointsAgainstProfiles(t *testing.T) {
	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		t.Run(profile.Version+"/archive-from-immich", func(t *testing.T) {
			server := newFromImmichCommandServer(t, profile)
			output, err := executeRootCommand(t,
				"--output=json",
				"archive",
				"--from-immich",
				"--source-server="+server.URL,
				"--source-api-key="+profile.APIKey,
				"--write-to-folder="+filepath.Join(t.TempDir(), "archive"),
			)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONSummary(t, output)
			assertRequested(t, server.Requests(), http.MethodPost, "/api/search/metadata")
			assertRequested(t, server.Requests(), http.MethodGet, "/api/assets/source-id/original")
		})

		t.Run(profile.Version+"/stack", func(t *testing.T) {
			server := newCommandContractServer(t, profile)
			_, err := executeRootCommand(t,
				"stack",
				"--server="+server.URL,
				"--api-key="+profile.APIKey,
			)
			if err != nil {
				t.Fatal(err)
			}
			assertRequested(t, server.Requests(), http.MethodPost, "/api/search/metadata")
		})
	}
}

func TestCommandTextOutputAndErrorExits(t *testing.T) {
	profile := immichtest.V310()
	server := newCommandContractServer(t, profile)
	output, err := executeRootCommand(t,
		"--output=text",
		"--concurrent-tasks=1",
		"upload",
		"--server="+server.URL,
		"--api-key="+profile.APIKey,
		"--pause-immich-jobs=false",
		folderCommandFixture(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Asset Tracking Report:") || !strings.Contains(output, "Total Assets:") {
		t.Errorf("text report is not parseable: %q", output)
	}

	invalidArguments := [][]string{
		{"upload"},
		{"upload", "--google", "--icloud", t.TempDir()},
		{"upload", "--from-immich", t.TempDir()},
		{"archive", t.TempDir()},
	}
	for _, args := range invalidArguments {
		if _, err := executeRootCommand(t, args...); err == nil {
			t.Errorf("arguments %q unexpectedly succeeded", args)
		}
	}

	unauthorized := immichtest.NewServer(t, profile)
	if _, err := executeRootCommand(t,
		"stack",
		"--server="+unauthorized.URL,
		"--api-key=wrong-key",
	); err == nil || !strings.Contains(err.Error(), "Validation failed") {
		t.Errorf("unauthorized exit error = %v", err)
	}
}

func newCommandContractServer(t *testing.T, profile immichtest.Profile) *immichtest.Server {
	t.Helper()
	server := immichtest.NewServer(t, profile)
	server.Handle(http.MethodGet, "/api/assets/statistics", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusOK, immich.AssetStatistics{})
	})
	server.Handle(http.MethodPost, "/api/search/metadata", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusOK, map[string]any{
			"assets": map[string]any{
				"total":    0,
				"count":    0,
				"items":    []any{},
				"nextPage": "0",
			},
		})
	})
	server.Handle(http.MethodGet, "/api/albums", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusOK, []any{})
	})
	server.Handle(http.MethodPost, "/api/assets", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusCreated, immich.AssetResponse{
			ID:     "uploaded-id",
			Status: immich.UploadCreated,
		})
	})
	server.Handle(http.MethodPut, "/api/assets/uploaded-id", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusOK, map[string]any{"id": "uploaded-id"})
	})
	server.Handle(http.MethodPost, "/api/albums", func(request immichtest.Request) immichtest.Response {
		var body struct {
			AlbumName   string `json:"albumName"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(request.JSON, &body); err != nil {
			t.Error(err)
		}
		return immichtest.JSONResponse(http.StatusCreated, map[string]any{
			"id":          "created-album-id",
			"albumName":   body.AlbumName,
			"description": body.Description,
		})
	})
	return server
}

func newFromImmichCommandServer(t *testing.T, profile immichtest.Profile) *immichtest.Server {
	t.Helper()
	server := newCommandContractServer(t, profile)
	asset := map[string]any{
		"id":               "source-id",
		"checksum":         "c291cmNlLWltYWdl",
		"ownerId":          profile.UserID,
		"type":             "IMAGE",
		"originalFileName": "source.jpg",
		"originalPath":     "upload/source.jpg",
		"visibility":       "timeline",
		"fileCreatedAt":    "2023-11-14T22:13:20Z",
		"fileModifiedAt":   "2023-11-14T22:13:20Z",
		"localDateTime":    "2023-11-14T22:13:20Z",
		"updatedAt":        "2023-11-14T22:13:20Z",
		"exifInfo": map[string]any{
			"dateTimeOriginal": "2023-11-14T22:13:20Z",
			"fileSizeInByte":   12,
			"description":      "source contract asset",
		},
	}
	server.Handle(http.MethodPost, "/api/search/metadata", func(request immichtest.Request) immichtest.Response {
		var query struct {
			Visibility string `json:"visibility"`
		}
		if err := json.Unmarshal(request.JSON, &query); err != nil {
			t.Error(err)
		}
		items := []any{}
		if query.Visibility == "timeline" {
			items = append(items, asset)
		}
		return immichtest.JSONResponse(http.StatusOK, map[string]any{
			"assets": map[string]any{
				"total":    len(items),
				"count":    len(items),
				"items":    items,
				"nextPage": "0",
			},
		})
	})
	server.Handle(http.MethodGet, "/api/assets/source-id", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusOK, asset)
	})
	server.Handle(http.MethodGet, "/api/assets/source-id/original", func(request immichtest.Request) immichtest.Response {
		return immichtest.Response{
			Status: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/octet-stream"}},
			Body:   []byte("source-image"),
		}
	})
	return server
}

func executeRootCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	capture, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = capture
	defer func() {
		os.Stdout = originalStdout
		_ = capture.Close()
	}()

	ctx := context.Background()
	command, _ := RootImmichGoCommand(ctx)
	command.SetArgs(args)
	executeErr := command.ExecuteContext(ctx)
	if _, err := capture.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(capture)
	if err != nil {
		t.Fatal(err)
	}
	return string(output), executeErr
}

func assertJSONSummary(t *testing.T, output string) {
	t.Helper()
	var summary map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", line, err)
		}
		if value["type"] == "summary" {
			summary = value
		}
	}
	if summary == nil {
		t.Fatalf("missing JSON summary in %q", output)
	}
	for _, field := range []string{"status", "exit_code", "counters", "events", "duration_seconds", "timestamp"} {
		if _, ok := summary[field]; !ok {
			t.Errorf("summary missing %q: %#v", field, summary)
		}
	}
	if summary["status"] != "success" || summary["exit_code"] != float64(0) {
		t.Errorf("summary status = %#v", summary)
	}
}

func assertRequested(t *testing.T, requests []immichtest.Request, method, path string) {
	t.Helper()
	for _, request := range requests {
		if request.Method == method && request.Path == path {
			return
		}
	}
	t.Errorf("missing %s %s in requests %#v", method, path, requests)
}

func folderCommandFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "folder.jpg"), "folder-image")
	return root
}

func googleCommandFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "google.jpg"), "google-image")
	writeFixtureFile(t, filepath.Join(root, "google.jpg.json"), `{
		"title":"google.jpg",
		"description":"contract fixture",
		"photoTakenTime":{"timestamp":"1700000000","formatted":"Nov 14, 2023, 10:13:20 PM UTC"},
		"geoData":{"latitude":0,"longitude":0,"altitude":0}
	}`)
	return root
}

func iCloudCommandFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "IMG_0001.JPG"), "icloud-image")
	writeFixtureFile(t, filepath.Join(root, "Albums", "Trip.csv"), "imgName\nIMG_0001.JPG\n")
	return root
}

func picasaCommandFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "IMG_0001.JPG"), "picasa-image")
	writeFixtureFile(t, filepath.Join(root, ".picasa.ini"), "[Picasa]\nname=Trip\ndescription=Contract fixture\n")
	return root
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
