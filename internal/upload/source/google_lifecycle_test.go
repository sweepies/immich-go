package source

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assettracker"
	cliflags "github.com/sweepies/immich-go/internal/cliFlags"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/journal"
)

const googleAssetJSON = `{"title":"photo.jpg","photoTakenTime":{"timestamp":"1680000000"}}`

func newTestGoogleSource(fsyss ...fs.FS) *GoogleSource {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return &GoogleSource{
		deps: adapters.SourceDependencies{
			Logger:          logger,
			Processor:       fileprocessor.New(assettracker.New(), journal.NewRecorder(logger)),
			SupportedMedia:  filetypes.DefaultSupportedMedia,
			TimeZone:        time.UTC,
			ConcurrentTasks: 1,
		},
		config: &adapters.GoogleConfig{},
		fsyss:  fsyss,
	}
}

// namedFS wraps an fs.FS with a name, like real takeout file systems have.
type namedFS struct {
	fs.FS
	name string
}

func (n namedFS) Name() string { return n.name }

// A5.3: a file duplicated in the same directory (e.g. across takeout parts)
// must be recorded either discovered or discarded-immediately, never both.
func TestGoogleSourcePassOneDuplicateInDirectory(t *testing.T) {
	part1 := namedFS{fstest.MapFS{
		"Takeout/photo.jpg":      &fstest.MapFile{Data: []byte("image-data")},
		"Takeout/photo.jpg.json": &fstest.MapFile{Data: []byte(googleAssetJSON)},
	}, "takeout-001.zip"}
	// A second takeout part holding the same file in the same directory.
	part2 := namedFS{fstest.MapFS{
		"Takeout/photo.jpg": &fstest.MapFile{Data: []byte("image-data")},
	}, "takeout-002.zip"}

	src := newTestGoogleSource(part1, part2)
	assetsOut := collectAssets(src.Browse(context.Background()))
	if len(assetsOut) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d", len(assetsOut))
	}

	counters := src.deps.Processor.GetAssetCounters()
	if counters.Discarded != 1 {
		t.Errorf("expected 1 discarded duplicate, got %d", counters.Discarded)
	}
	if counters.Pending != 1 {
		t.Errorf("expected the surviving asset to be pending, got %d", counters.Pending)
	}
	if counters.InvalidTransitions != 0 {
		t.Errorf("expected no invalid transitions, got %d", counters.InvalidTransitions)
	}
	if got := src.deps.Processor.GetEventCounts()[journal.DiscardedLocalDuplicate]; got != 1 {
		t.Errorf("expected 1 DiscardedLocalDuplicate event, got %d", got)
	}
}

// A5.4: a media file with no JSON match and KeepJSONLess=false must reach the
// DISCARDED state instead of staying PENDING forever.
func TestGoogleSourceUnmatchedFileDiscarded(t *testing.T) {
	fsys := fstest.MapFS{
		"Takeout/orphan.jpg": &fstest.MapFile{Data: []byte("image-data")},
		"Takeout/side.xmp":   &fstest.MapFile{Data: []byte("<xmp/>")},
	}

	src := newTestGoogleSource(fsys)
	if got := collectAssets(src.Browse(context.Background())); len(got) != 0 {
		t.Fatalf("expected no emitted asset, got %d", len(got))
	}

	counters := src.deps.Processor.GetAssetCounters()
	if counters.Discarded != 1 {
		t.Errorf("expected 1 discarded asset, got %d", counters.Discarded)
	}
	if !counters.RunFinishedCleanly() {
		t.Errorf("expected a clean run, got %+v", counters)
	}
	events := src.deps.Processor.GetEventCounts()
	if got := events[journal.DiscardedFiltered]; got != 1 {
		t.Errorf("expected 1 DiscardedFiltered event, got %d", got)
	}
	// Only the image reaches the puzzle: the xmp sidecar is a non-asset and is
	// not a candidate for JSON matching.
	if got := events[journal.ProcessedMissingMetadata]; got != 1 {
		t.Errorf("expected 1 ProcessedMissingMetadata event, got %d", got)
	}
	if got := events[journal.DiscoveredSidecar]; got != 1 {
		t.Errorf("expected 1 DiscoveredSidecar event, got %d", got)
	}
}

// E1: a sidecar file is not a candidate asset. Even when a JSON matches its
// name, it must be recorded as a sidecar and never emitted as an asset.
func TestGoogleSourceDoesNotEmitSidecarAsAsset(t *testing.T) {
	fsys := fstest.MapFS{
		"Takeout/photo.xmp":      &fstest.MapFile{Data: []byte("<xmp/>")},
		"Takeout/photo.xmp.json": &fstest.MapFile{Data: []byte(googleAssetJSON)},
	}

	src := newTestGoogleSource(fsys)
	if got := collectAssets(src.Browse(context.Background())); len(got) != 0 {
		t.Fatalf("expected no emitted asset, got %d", len(got))
	}

	counters := src.deps.Processor.GetAssetCounters()
	if counters.Total() != 0 {
		t.Errorf("expected no tracked assets, got %+v", counters)
	}
	if !counters.RunFinishedCleanly() {
		t.Errorf("expected a clean run, got %+v", counters)
	}
	events := src.deps.Processor.GetEventCounts()
	if got := events[journal.DiscoveredSidecar]; got != 2 {
		t.Errorf("expected 2 DiscoveredSidecar events (xmp and its JSON), got %d", got)
	}
}

// C1: --keep-archived=false must discard assets whose Google metadata says
// they are archived.
func TestGoogleSourceKeepArchivedFalseDiscardsArchived(t *testing.T) {
	fsys := fstest.MapFS{
		"Takeout/photo.jpg": &fstest.MapFile{Data: []byte("image-data")},
		"Takeout/photo.jpg.json": &fstest.MapFile{Data: []byte(
			`{"title":"photo.jpg","photoTakenTime":{"timestamp":"1680000000"},"archived":true}`)},
	}

	src := newTestGoogleSource(fsys)
	src.config.KeepArchived = false
	if got := collectAssets(src.Browse(context.Background())); len(got) != 0 {
		t.Fatalf("expected no emitted asset, got %d", len(got))
	}

	counters := src.deps.Processor.GetAssetCounters()
	if counters.Discarded != 1 {
		t.Errorf("expected 1 discarded asset, got %d", counters.Discarded)
	}
	if !counters.RunFinishedCleanly() {
		t.Errorf("expected a clean run, got %+v", counters)
	}
	if got := src.deps.Processor.GetEventCounts()[journal.DiscardedFiltered]; got != 1 {
		t.Errorf("expected 1 DiscardedFiltered event, got %d", got)
	}
}

// errorFS fails every Open, making pass one fail.
type errorFS struct{}

func (errorFS) Open(string) (fs.File, error) { return nil, errors.New("unreadable archive") }

// errorLogCapture collects error-level log records.
type errorLogCapture struct {
	mu   sync.Mutex
	logs []string
}

func (h *errorLogCapture) Enabled(context.Context, slog.Level) bool { return true }
func (h *errorLogCapture) Handle(_ context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		h.mu.Lock()
		h.logs = append(h.logs, r.Message)
		h.mu.Unlock()
	}
	return nil
}
func (h *errorLogCapture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *errorLogCapture) WithGroup(string) slog.Handler      { return h }

// F5: a fatal browse error must be logged at error level; the run otherwise
// ends with only a "N assets never reached final state" report and the real
// cause nowhere.
func TestGoogleSourceLogsBrowseFailure(t *testing.T) {
	lc := &errorLogCapture{}
	src := newTestGoogleSource(errorFS{})
	src.deps.Logger = slog.New(lc)

	for range src.Browse(context.Background()) {
	}

	if len(lc.logs) == 0 {
		t.Error("expected the browse failure to be logged at error level")
	}
}

// F5: a browse interrupted by context cancellation is expected and must not
// be reported as an error.
func TestGoogleSourceSilentOnCancellation(t *testing.T) {
	lc := &errorLogCapture{}
	src := newTestGoogleSource(fstest.MapFS{
		"Takeout/photo.jpg": &fstest.MapFile{Data: []byte("image-data")},
	})
	src.deps.Logger = slog.New(lc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range src.Browse(ctx) {
	}

	if len(lc.logs) != 0 {
		t.Errorf("expected no error log on cancellation, got %v", lc.logs)
	}
}

// C1: --keep-archived=true keeps archived assets.
func TestGoogleSourceKeepArchivedTrueKeepsArchived(t *testing.T) {
	fsys := fstest.MapFS{
		"Takeout/photo.jpg": &fstest.MapFile{Data: []byte("image-data")},
		"Takeout/photo.jpg.json": &fstest.MapFile{Data: []byte(
			`{"title":"photo.jpg","photoTakenTime":{"timestamp":"1680000000"},"archived":true}`)},
	}

	src := newTestGoogleSource(fsys)
	src.config.KeepArchived = true
	got := collectAssets(src.Browse(context.Background()))
	if len(got) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d", len(got))
	}
	if !got[0].Archived {
		t.Error("expected the emitted asset to be marked archived")
	}
}

// G4: a fatal browse failure is reported through Err() once the channel
// closes.
func TestGoogleSourceErrReportsBrowseFailure(t *testing.T) {
	src := newTestGoogleSource(errorFS{})

	for range src.Browse(context.Background()) {
	}

	if err := src.Err(); err == nil {
		t.Error("expected Err() to report the failed browse")
	}
}

// G4: a browse ended by context cancellation reports no error through Err().
func TestGoogleSourceErrIsNilOnCancellation(t *testing.T) {
	src := newTestGoogleSource(fstest.MapFS{
		"Takeout/photo.jpg": &fstest.MapFile{Data: []byte("image-data")},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range src.Browse(ctx) {
	}

	if err := src.Err(); err != nil {
		t.Errorf("expected nil Err() on cancellation, got %v", err)
	}
}

// G4: a clean browse reports no error through Err().
func TestGoogleSourceErrIsNilOnCleanBrowse(t *testing.T) {
	src := newTestGoogleSource(fstest.MapFS{
		"Takeout/photo.jpg":      &fstest.MapFile{Data: []byte("image-data")},
		"Takeout/photo.jpg.json": &fstest.MapFile{Data: []byte(googleAssetJSON)},
	})

	if got := collectAssets(src.Browse(context.Background())); len(got) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d", len(got))
	}
	if err := src.Err(); err != nil {
		t.Errorf("expected nil Err() on a clean browse, got %v", err)
	}
}

// G6: extension filters apply to media files only. JSON metadata and
// sidecars are classified first and never filtered by extension.
func TestGoogleSourceExtensionFiltersApplyOnlyToMedia(t *testing.T) {
	fsys := fstest.MapFS{
		"Takeout/photo.jpg":      &fstest.MapFile{Data: []byte("image-data")},
		"Takeout/photo.jpg.json": &fstest.MapFile{Data: []byte(googleAssetJSON)},
		"Takeout/note.xmp":       &fstest.MapFile{Data: []byte("<xmp/>")},
		"Takeout/clip.png":       &fstest.MapFile{Data: []byte("png-data")},
	}

	src := newTestGoogleSource(fsys)
	src.config.InclusionFlags.IncludedExtensions = cliflags.ExtensionList{".jpg"}

	assetsOut := collectAssets(src.Browse(context.Background()))
	if len(assetsOut) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d", len(assetsOut))
	}
	if assetsOut[0].OriginalFileName != "photo.jpg" {
		t.Errorf("expected the .jpg asset to be kept with its JSON metadata, got %q", assetsOut[0].OriginalFileName)
	}

	events := src.deps.Processor.GetEventCounts()
	// The JSON metadata and the xmp are sidecars, not filtered assets.
	if got := events[journal.DiscoveredSidecar]; got != 2 {
		t.Errorf("expected 2 DiscoveredSidecar events, got %d", got)
	}
	// The .png is a media file: it is filtered as a discarded asset.
	if got := events[journal.DiscardedFiltered]; got != 1 {
		t.Errorf("expected 1 DiscardedFiltered event, got %d", got)
	}
	counters := src.deps.Processor.GetAssetCounters()
	if counters.Discarded != 1 || counters.Pending != 1 {
		t.Errorf("expected 1 discarded, 1 pending, got %+v", counters)
	}
}
