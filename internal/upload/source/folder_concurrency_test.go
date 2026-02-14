package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fileevent"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
)

func TestFolderSourceBrowseCancellationAndShutdown(t *testing.T) {
	root := t.TempDir()
	for d := range 24 {
		dir := filepath.Join(root, fmt.Sprintf("set-%02d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating test directory %q: %v", dir, err)
		}
		for f := range 12 {
			name := filepath.Join(dir, fmt.Sprintf("asset-%03d.jpg", f))
			if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
				t.Fatalf("creating test file %q: %v", name, err)
			}
		}
	}

	src := newTestFolderSource(root, true, 4)
	t.Cleanup(func() {
		_ = src.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelTimer := time.AfterFunc(10*time.Millisecond, cancel)
	defer cancelTimer.Stop()

	gOut := src.Browse(ctx)
	countCh := make(chan int, 1)
	go func() {
		count := 0
		for range gOut {
			count++
			if count == 1 {
				cancelTimer.Stop()
				cancel()
			}
		}
		countCh <- count
	}()

	select {
	case <-countCh:
	case <-time.After(5 * time.Second):
		t.Fatal("browse output did not close after cancellation")
	}

	if err := src.Close(); err != nil {
		t.Fatalf("closing source: %v", err)
	}

	requireNoPipelineGoroutineLeaks(t)
}

func TestFolderSourceBrowseOrderingIsStable(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"zeta.jpg", "alpha.jpg", "middle.jpg"} {
		file := filepath.Join(root, name)
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatalf("creating test file %q: %v", file, err)
		}
	}

	src := newTestFolderSource(root, false, 1)
	t.Cleanup(func() {
		_ = src.Close()
	})

	got := make([]string, 0, 3)
	for g := range src.Browse(context.Background()) {
		if g == nil || len(g.Assets) == 0 {
			continue
		}
		got = append(got, g.Assets[0].OriginalFileName)
	}

	if err := src.Close(); err != nil {
		t.Fatalf("closing source: %v", err)
	}

	want := []string{"alpha.jpg", "middle.jpg", "zeta.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected output order: got %v, want %v", got, want)
	}

	requireNoPipelineGoroutineLeaks(t)
}

func TestFolderSourceBrowseStopsWorkerPoolWithoutClose(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one.jpg", "two.jpg"} {
		file := filepath.Join(root, name)
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatalf("creating test file %q: %v", file, err)
		}
	}

	src := newTestFolderSource(root, false, 2)
	t.Cleanup(func() {
		_ = src.Close()
	})

	for range src.Browse(context.Background()) {
	}

	requireNoPipelineGoroutineLeaks(t)
}

func newTestFolderSource(root string, recursive bool, workers int) *FolderSource {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	processor := fileprocessor.New(assettracker.New(), fileevent.NewRecorder(nil))

	return &FolderSource{
		deps: adapters.SourceDependencies{
			Logger:          logger,
			Processor:       processor,
			SupportedMedia:  filetypes.DefaultSupportedMedia,
			TimeZone:        time.UTC,
			ConcurrentTasks: workers,
		},
		config: &adapters.FolderConfig{
			Recursive: recursive,
		},
		fsyss: []fs.FS{os.DirFS(root)},
	}
}

func requireNoPipelineGoroutineLeaks(t *testing.T) {
	t.Helper()

	markers := []string{
		"github.com/sweepies/immich-go/internal/worker.(*Pool).worker",
		"github.com/sweepies/immich-go/internal/groups.",
		"github.com/sweepies/immich-go/internal/upload/source.(*FolderSource).parseDir",
		"github.com/sweepies/immich-go/internal/upload/source.(*FolderSource).Browse.func1",
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		profile := goroutineProfile()
		leaked := matchingGoroutines(profile, markers)
		if len(leaked) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("detected leaked pipeline goroutines:\n%s", strings.Join(leaked, "\n\n"))
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func goroutineProfile() string {
	var b bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&b, 2)
	return b.String()
}

func matchingGoroutines(profile string, markers []string) []string {
	blocks := strings.Split(profile, "\n\n")
	matches := make([]string, 0)
	for _, block := range blocks {
		for _, marker := range markers {
			if strings.Contains(block, marker) {
				matches = append(matches, block)
				break
			}
		}
	}
	return matches
}
