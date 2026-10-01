package immichtest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"testing"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/immichtest"
)

func TestProfilesModelConnectionEndpoints(t *testing.T) {
	profiles := []immichtest.Profile{immichtest.V275(), immichtest.V310()}
	for _, profile := range profiles {
		t.Run(profile.Version, func(t *testing.T) {
			server := immichtest.NewServer(t, profile)
			client, err := immich.NewImmichClient(server.URL, profile.APIKey)
			if err != nil {
				t.Fatal(err)
			}

			if err := client.PingServer(context.Background()); err != nil {
				t.Fatal(err)
			}
			user, err := client.ValidateConnection(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if user.ID.String() != profile.UserID || user.Email != profile.UserEmail {
				t.Errorf("user = %#v, want ID %q and email %q", user, profile.UserID, profile.UserEmail)
			}
			about, err := client.GetAboutInfo(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if about.Version != profile.Version || client.ServerVersion().String() != profile.Version {
				t.Errorf("about version = %q, retained = %q, want %q", about.Version, client.ServerVersion(), profile.Version)
			}
			if client.SupportedMedia()[".jpg"] != filetypes.TypeImage || client.SupportedMedia()[".mp4"] != filetypes.TypeVideo {
				t.Errorf("supported media = %#v", client.SupportedMedia())
			}

			requests := server.Requests()
			paths := make([]string, len(requests))
			for i, request := range requests {
				paths[i] = request.Path
				if request.Method != http.MethodGet {
					t.Errorf("method = %q, want GET", request.Method)
				}
				if request.Header.Get("x-api-key") != profile.APIKey {
					t.Errorf("API key = %q", request.Header.Get("x-api-key"))
				}
			}
			wantPaths := []string{"/api/server/ping", "/api/users/me", "/api/server/media-types", "/api/server/about"}
			if !reflect.DeepEqual(paths, wantPaths) {
				t.Errorf("paths = %v, want %v", paths, wantPaths)
			}
		})
	}
}

func TestServerCapturesExactRequestContracts(t *testing.T) {
	server := immichtest.NewServer(t, immichtest.V310())
	server.Handle(http.MethodPost, "/api/json", func(request immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusCreated, map[string]bool{"ok": true})
	})
	server.Handle(http.MethodPost, "/api/multipart", func(request immichtest.Request) immichtest.Response {
		return immichtest.Response{Status: http.StatusNoContent}
	})

	jsonBody := []byte(`{"name":"album","ids":["one","two"]}`)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/json?shared=true&id=one&id=two", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Contract", "json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	if err := writer.WriteField("name", "photo"); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("assetData", "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("jpeg-data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err = http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/multipart", &multipartBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}

	requests := server.Requests()
	if len(requests) != 2 {
		t.Fatalf("received %d requests, want 2", len(requests))
	}
	jsonRequest := requests[0]
	if jsonRequest.Method != http.MethodPost || jsonRequest.Path != "/api/json" {
		t.Errorf("JSON request = %s %s", jsonRequest.Method, jsonRequest.Path)
	}
	if !reflect.DeepEqual(jsonRequest.Query["id"], []string{"one", "two"}) || jsonRequest.Query.Get("shared") != "true" {
		t.Errorf("query = %v", jsonRequest.Query)
	}
	if jsonRequest.Header.Get("X-Contract") != "json" {
		t.Errorf("header = %q", jsonRequest.Header.Get("X-Contract"))
	}
	if !bytes.Equal(jsonRequest.JSON, jsonBody) {
		t.Errorf("JSON = %s, want %s", jsonRequest.JSON, jsonBody)
	}

	multipartRequest := requests[1]
	if fields := multipartRequest.Multipart["name"]; len(fields) != 1 || string(fields[0].Value) != "photo" || fields[0].Filename != "" {
		t.Errorf("name fields = %#v", fields)
	}
	if fields := multipartRequest.Multipart["assetData"]; len(fields) != 1 || string(fields[0].Value) != "jpeg-data" || fields[0].Filename != "photo.jpg" {
		t.Errorf("asset fields = %#v", fields)
	}
}

func TestProfileValidationResponses(t *testing.T) {
	tests := []struct {
		name              string
		profile           immichtest.Profile
		wantHeader        string
		wantBodyCID       string
		wantMessageIsList bool
	}{
		{name: "v2", profile: immichtest.V275(), wantBodyCID: "contract-correlation", wantMessageIsList: true},
		{name: "v3", profile: immichtest.V310(), wantHeader: "contract-correlation"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := test.profile.ValidationResponse(http.StatusBadRequest, "contract-correlation", immichtest.ValidationError{
				Path:    []any{"asset", "duration"},
				Message: "Expected number",
			})
			if response.Header.Get("X-Correlation-ID") != test.wantHeader {
				t.Errorf("correlation header = %q, want %q", response.Header.Get("X-Correlation-ID"), test.wantHeader)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body, &body); err != nil {
				t.Fatal(err)
			}
			if body["correlationId"] != test.wantBodyCID && test.wantBodyCID != "" {
				t.Errorf("body correlation = %#v, want %q", body["correlationId"], test.wantBodyCID)
			}
			_, messageIsList := body["message"].([]any)
			if messageIsList != test.wantMessageIsList {
				t.Errorf("message list = %t, want %t; body = %#v", messageIsList, test.wantMessageIsList, body)
			}
			if test.name == "v3" {
				errors, ok := body["errors"].([]any)
				if !ok || len(errors) != 1 {
					t.Errorf("errors = %#v", body["errors"])
				}
			}
		})
	}
}

func TestPaginateUsesProfilePageSize(t *testing.T) {
	profile := immichtest.V310()
	profile.PageSize = 2
	items := []string{"one", "two", "three", "four", "five"}

	page, hasNext := immichtest.Paginate(profile, items, 2)
	if !reflect.DeepEqual(page, []string{"three", "four"}) || !hasNext {
		t.Errorf("page 2 = %v, hasNext = %t", page, hasNext)
	}
	page, hasNext = immichtest.Paginate(profile, items, 3)
	if !reflect.DeepEqual(page, []string{"five"}) || hasNext {
		t.Errorf("page 3 = %v, hasNext = %t", page, hasNext)
	}
}
