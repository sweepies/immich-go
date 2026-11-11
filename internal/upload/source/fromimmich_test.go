package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/immichtest"
	"github.com/sweepies/immich-go/internal/journal"
)

func newTestFromImmichSource(server *immichtest.Server, logBuf *bytes.Buffer, cfg *adapters.FromImmichConfig) *FromImmichSource {
	var logger *slog.Logger
	if logBuf != nil {
		logger = slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	cfg.ServerURL = server.URL
	cfg.APIKey = server.Profile().APIKey
	return &FromImmichSource{
		deps: adapters.SourceDependencies{
			Logger:          logger,
			Processor:       fileprocessor.New(assettracker.New(), journal.NewRecorder(logger)),
			SupportedMedia:  filetypes.DefaultSupportedMedia,
			TimeZone:        time.UTC,
			ConcurrentTasks: 1,
		},
		config: cfg,
	}
}

// C4: a --from-immich model filter must reach the server as "model", not as a
// second "make".
func TestFromImmichSourceSendsModelFilter(t *testing.T) {
	server := immichtest.NewServer(t, immichtest.V310())
	server.Handle(http.MethodGet, "/api/search/suggestions", func(r immichtest.Request) immichtest.Response {
		return immichtest.JSONResponse(http.StatusOK, []string{"CoolCam X"})
	})

	var mu sync.Mutex
	var queries []immich.SearchMetadataQuery
	server.Handle(http.MethodPost, "/api/search/metadata", func(r immichtest.Request) immichtest.Response {
		var q immich.SearchMetadataQuery
		if err := json.Unmarshal(r.JSON, &q); err == nil {
			mu.Lock()
			queries = append(queries, q)
			mu.Unlock()
		}
		return immichtest.Response{
			Status: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   []byte(`{"assets":{"total":0,"count":0,"items":[],"nextPage":"0"}}`),
		}
	})

	src := newTestFromImmichSource(server, nil, &adapters.FromImmichConfig{Model: "CoolCam X"})
	_ = collectAssets(src.Browse(context.Background()))

	mu.Lock()
	defer mu.Unlock()
	if len(queries) == 0 {
		t.Fatal("expected at least one search query")
	}
	for _, q := range queries {
		if q.Model != "CoolCam X" {
			t.Errorf("expected model filter \"CoolCam X\", got model=%q make=%q", q.Model, q.Make)
		}
		if q.Make != "" {
			t.Errorf("expected an empty make filter, got %q", q.Make)
		}
	}
}

// C4: when GetAssetInfo fails, the log line must name the asset that failed,
// not the shadowed result variable.
func TestFromImmichSourceLogsAssetIDOnInfoError(t *testing.T) {
	profile := immichtest.V310()
	server := immichtest.NewServer(t, profile)

	server.Handle(http.MethodPost, "/api/search/metadata", func(r immichtest.Request) immichtest.Response {
		body := fmt.Sprintf(`{"assets":{"total":1,"count":1,"items":[`+
			`{"id":"asset-1","ownerId":%q,"type":"IMAGE","originalFileName":"photo.jpg","originalPath":"p","checksum":"x"}`+
			`],"nextPage":"0"}}`, profile.UserID)
		return immichtest.Response{
			Status: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   []byte(body),
		}
	})
	server.Handle(http.MethodGet, "/api/assets/asset-1", func(r immichtest.Request) immichtest.Response {
		return profile.ValidationResponse(http.StatusInternalServerError, "", immichtest.ValidationError{Message: "boom"})
	})

	var buf bytes.Buffer
	src := newTestFromImmichSource(server, &buf, &adapters.FromImmichConfig{})
	if got := collectAssets(src.Browse(context.Background())); len(got) != 0 {
		t.Fatalf("expected no emitted asset, got %d", len(got))
	}

	if !strings.Contains(buf.String(), `"id":"asset-1"`) {
		t.Errorf("expected the failing asset id in the log, got %s", buf.String())
	}
}

// G4: a fatal browse failure (here a rejected API key during client
// initialization) is reported through Err() once the channel closes.
func TestFromImmichSourceErrReportsBrowseFailure(t *testing.T) {
	server := immichtest.NewServer(t, immichtest.V310())
	src := newTestFromImmichSource(server, nil, &adapters.FromImmichConfig{})
	src.config.APIKey = "wrong-key"

	if got := collectAssets(src.Browse(context.Background())); len(got) != 0 {
		t.Fatalf("expected no emitted asset, got %d", len(got))
	}
	if err := src.Err(); err == nil {
		t.Error("expected Err() to report the failed browse")
	}
}
