package immich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAboutInfoRetainsServerVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/server/about" {
			t.Errorf("path = %q, want /api/server/about", request.URL.Path)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"version":"v3.2.1-beta.1+build.7","licensed":true}`))
	}))
	defer server.Close()

	client, err := NewImmichClient(server.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	about, err := client.GetAboutInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if about.Version != "v3.2.1-beta.1+build.7" {
		t.Errorf("about version = %q", about.Version)
	}
	if client.ServerVersion().String() != about.Version {
		t.Errorf("retained version = %q, want %q", client.ServerVersion(), about.Version)
	}
	if client.ServerVersion().Major() != 3 {
		t.Errorf("major version = %d, want 3", client.ServerVersion().Major())
	}
}

func TestGetAboutInfoRejectsInvalidServerVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"version":"development"}`))
	}))
	defer server.Close()

	client, err := NewImmichClient(server.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetAboutInfo(context.Background()); err == nil {
		t.Fatal("expected invalid version error")
	}
	if client.ServerVersion().String() != "" {
		t.Errorf("retained invalid version %q", client.ServerVersion())
	}
}
