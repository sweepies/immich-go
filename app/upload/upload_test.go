package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
)

type emptyTagUpsertClient struct {
	immich.Client
}

func (c *emptyTagUpsertClient) UpsertTags(context.Context, []string) ([]immich.TagSimplified, error) {
	return []immich.TagSimplified{}, nil
}

func TestSaveTagRejectsEmptyUpsertResponse(t *testing.T) {
	client := &emptyTagUpsertClient{}
	command := &UpCmd{client: app.Client{Immich: client}}
	tag := assets.Tag{Name: "vacation", Value: "vacation"}

	got, err := command.saveTag(t.Context(), tag, []string{"asset-id"})
	if err == nil {
		t.Fatal("saveTag() error = nil, want empty-response error")
	}
	if !strings.Contains(err.Error(), "returned no tags") {
		t.Errorf("saveTag() error = %q, want empty-response context", err)
	}
	if got.ID != "" {
		t.Errorf("saveTag() tag ID = %q, want empty", got.ID)
	}
}

// albumFlakyClient simulates a server on which the first CreateAlbum call for
// a run succeeded but the client saw an error, so the album exists although
// the run kept no ID for it.
type albumFlakyClient struct {
	immich.Client

	mu        sync.Mutex
	creates   int
	albums    []immich.AlbumSimplified
	addedTo   []immich.AlbumID
	createErr error // reported by the first CreateAlbum, which still creates the album server-side when createLeftBehind is set
}

func (c *albumFlakyClient) CreateAlbum(_ context.Context, title, _ string, _ []immich.AssetID) (assets.Album, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.creates++
	if err := c.createErr; err != nil {
		c.createErr = nil
		// The create reached the server but the response was lost.
		c.albums = append(c.albums, immich.AlbumSimplified{ID: immich.AlbumID(fmt.Sprintf("server-%d", c.creates)), AlbumName: title})
		return assets.Album{}, err
	}
	id := fmt.Sprintf("album-%d", c.creates)
	c.albums = append(c.albums, immich.AlbumSimplified{ID: immich.AlbumID(id), AlbumName: title})
	return assets.NewAlbum(id, title, ""), nil
}

func (c *albumFlakyClient) GetAllAlbums(context.Context) ([]immich.AlbumSimplified, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]immich.AlbumSimplified(nil), c.albums...), nil
}

func (c *albumFlakyClient) AddAssetToAlbum(_ context.Context, id immich.AlbumID, ids []immich.AssetID) ([]immich.UpdateAlbumResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addedTo = append(c.addedTo, id)
	results := make([]immich.UpdateAlbumResult, len(ids))
	for i, aid := range ids {
		results[i] = immich.UpdateAlbumResult{ID: aid, Success: true}
	}
	return results, nil
}

func newTestUpCmd(client immich.Client) *UpCmd {
	a := app.New(context.Background(), nil)
	a.SetLog(&app.Log{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return &UpCmd{client: app.Client{Immich: client}, app: a}
}

// G1: a retried album save must not create a second album when the first
// CreateAlbum failed after reaching the server.
func TestSaveAlbumAdoptsAlbumLeftByFailedCreate(t *testing.T) {
	client := &albumFlakyClient{createErr: errors.New("connection reset by peer")}
	uc := newTestUpCmd(client)
	album := assets.NewAlbum("", "Holiday", "")

	// First save: the album was created on the server but the response was
	// lost, so the collection still has no ID.
	if _, err := uc.saveAlbum(context.Background(), album, []string{"asset-1"}); err == nil {
		t.Fatal("expected the first save to report the create error")
	}
	if client.creates != 1 {
		t.Fatalf("expected 1 create call, got %d", client.creates)
	}

	// The cache retries the save with the same ID-less album: the run must
	// adopt the album the first attempt left behind, not create a second.
	got, err := uc.saveAlbum(context.Background(), album, []string{"asset-1", "asset-2"})
	if err != nil {
		t.Fatalf("saveAlbum() error = %v", err)
	}
	if client.creates != 1 {
		t.Errorf("a duplicate album was created, creates = %d", client.creates)
	}
	if got.ID != "server-1" {
		t.Errorf("expected the adopted album ID %q, got %q", "server-1", got.ID)
	}
	if len(client.addedTo) != 1 || client.addedTo[0] != "server-1" {
		t.Errorf("expected the ids to be added to the adopted album, got %v", client.addedTo)
	}
}

// G1: when the failed create really did not reach the server, the retry
// creates the album as usual.
func TestSaveAlbumRetriesCreateWhenNothingWasLeftBehind(t *testing.T) {
	client := &albumFlakyClient{}
	uc := newTestUpCmd(client)
	album := assets.NewAlbum("", "Holiday", "")

	// Simulate a first save that failed before reaching the server: the
	// title is remembered as a failed create and nothing exists server-side.
	uc.failedAlbumCreates.Store("Holiday", struct{}{})

	got, err := uc.saveAlbum(context.Background(), album, []string{"asset-1"})
	if err != nil {
		t.Fatalf("saveAlbum() error = %v", err)
	}
	if client.creates != 1 {
		t.Errorf("expected the retry to create the album once, got %d creates", client.creates)
	}
	if got.ID == "" {
		t.Error("expected the retried save to return an album ID")
	}
}
