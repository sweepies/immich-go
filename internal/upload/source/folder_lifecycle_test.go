package source

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assettracker"
	cliflags "github.com/sweepies/immich-go/internal/cliFlags"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/journal"
	"github.com/sweepies/immich-go/internal/namematcher"
)

// A5.1: an asset outside the date range was already discovered; it must be
// discarded (PENDING -> DISCARDED), not silently kept pending.
func TestFolderSourceDiscardsAssetOutsideDateRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old.jpg"), make([]byte, 4096), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	src := newTestFolderSource(root, false, 1)
	t.Cleanup(func() { _ = src.Close() })
	src.config.InclusionFlags.DateRange = cliflags.InitDateRange(time.UTC, "2020-01-01,2020-12-31")

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

// A5.2: assetFromFile fails for a file that was never discovered; it must be
// counted as an error (untracked -> ERROR).
func TestFolderSourceCountsAssetFromFileError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}
	if err := os.Symlink("nonexistent", filepath.Join(root, "broken.jpg")); err != nil {
		t.Logf("creating broken symlink: %v (skipping test if symlinks not supported)", err)
		t.Skip()
	}

	src := newTestFolderSource(root, false, 1)
	t.Cleanup(func() { _ = src.Close() })

	if got := collectAssets(src.Browse(context.Background())); len(got) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d", len(got))
	}

	counters := src.deps.Processor.GetAssetCounters()
	if counters.Errors != 1 {
		t.Errorf("expected 1 errored asset, got %d", counters.Errors)
	}
	if counters.InvalidTransitions != 0 {
		t.Errorf("expected no invalid transitions, got %d", counters.InvalidTransitions)
	}
	if got := src.deps.Processor.GetEventCounts()[journal.ErrorFileAccess]; got != 1 {
		t.Errorf("expected 1 ErrorFileAccess event, got %d", got)
	}
}

// failDirFS lists the root directory but fails to read the directory named
// failing, so parseDir of that directory errors while siblings keep working.
type failDirFS struct {
	fstest.MapFS
	failing string
}

func (f failDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == f.failing {
		return nil, errors.New("unreadable directory")
	}
	return f.MapFS.ReadDir(name)
}

// G4: a directory that fails to be read does not abort the browse of the
// other directories, but is reported through Err() once the channel closes.
func TestFolderSourceErrJoinsDirectoryFailures(t *testing.T) {
	fsys := failDirFS{
		MapFS: fstest.MapFS{
			"good/ok.jpg": &fstest.MapFile{Data: []byte("x")},
			"bad/x.jpg":   &fstest.MapFile{Data: []byte("x")},
		},
		failing: "bad",
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	src := &FolderSource{
		deps: adapters.SourceDependencies{
			Logger:          logger,
			Processor:       fileprocessor.New(assettracker.New(), journal.NewRecorder(nil)),
			SupportedMedia:  filetypes.DefaultSupportedMedia,
			TimeZone:        time.UTC,
			ConcurrentTasks: 2,
		},
		config: &adapters.FolderConfig{
			BannedFiles: namematcher.MustList(),
			Recursive:   true,
		},
		fsyss: []fs.FS{fsys},
	}
	t.Cleanup(func() { _ = src.Close() })

	assetsOut := collectAssets(src.Browse(context.Background()))
	if len(assetsOut) == 0 {
		t.Error("expected the good directory's asset to be emitted")
	}

	err := src.Err()
	if err == nil {
		t.Fatal("expected Err() to report the failed directory")
	}
	if !strings.Contains(err.Error(), "unreadable directory") {
		t.Errorf("expected the joined error to mention the failed directory, got %v", err)
	}
}
