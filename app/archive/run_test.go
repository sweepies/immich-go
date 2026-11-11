package archive

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spf13/cobra"
	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/internal/assets"
	cliarchive "github.com/sweepies/immich-go/internal/cli/archive"
	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/journal"
)

type mockSource struct {
	groups []*assets.Group
	before func(ctx context.Context)
	err    error // when set, Err() reports a failed browse
}

func (m *mockSource) Browse(ctx context.Context) <-chan *assets.Group {
	ch := make(chan *assets.Group)
	go func() {
		defer close(ch)
		if m.before != nil {
			m.before(ctx)
		}
		for _, g := range m.groups {
			select {
			case <-ctx.Done():
				return
			case ch <- g:
			}
		}
	}()
	return ch
}

func (m *mockSource) Close() error { return nil }

func (m *mockSource) Err() error { return m.err }

func newTestApp() *app.Application {
	a := app.New(context.Background(), nil)
	a.SetLog(&app.Log{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	a.EnsureFileProcessor()
	return a
}

func newTestCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	return cmd
}

// A4: a completed archive run finalizes the asset lifecycle.
func TestArchiveRunFinalizes(t *testing.T) {
	t.Run("clean run finalizes processed assets", func(t *testing.T) {
		a := newTestApp()
		ac := &ArchiveCmd{app: a, config: &cliarchive.Config{ArchivePath: t.TempDir()}}

		fsys := fstest.MapFS{"photo.jpg": &fstest.MapFile{Data: []byte("data")}}
		asset := &assets.Asset{
			File:             fshelper.NewFilename(fsys, "photo.jpg"),
			OriginalFileName: "photo.jpg",
			NameInfo:         assets.NameInfo{Base: "photo.jpg"},
			FileSize:         4,
		}
		src := &mockSource{
			groups: []*assets.Group{{Assets: []*assets.Asset{asset}}},
			before: func(ctx context.Context) {
				_ = a.FileProcessor().RecordAssetDiscovered(ctx, asset.File, 4, journal.DiscoveredImage)
			},
		}

		if err := ac.Run(newTestCommand(context.Background()), src); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		counters := a.FileProcessor().GetAssetCounters()
		if counters.Processed != 1 || !counters.RunFinishedCleanly() {
			t.Errorf("expected 1 processed asset and a clean run, got %+v", counters)
		}
	})

	t.Run("pending assets become errors", func(t *testing.T) {
		a := newTestApp()
		ac := &ArchiveCmd{app: a, config: &cliarchive.Config{ArchivePath: t.TempDir()}}

		src := &mockSource{
			before: func(ctx context.Context) {
				stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
				_ = a.FileProcessor().RecordAssetDiscovered(ctx, stuck, 10, journal.DiscoveredImage)
			},
		}

		err := ac.Run(newTestCommand(context.Background()), src)
		if err == nil {
			t.Fatal("expected the Finalize error, got nil")
		}
		if !strings.Contains(err.Error(), "never reached final state") {
			t.Errorf("expected 'never reached final state' error, got %v", err)
		}
		counters := a.FileProcessor().GetAssetCounters()
		if counters.Pending != 0 || counters.Errors != 1 {
			t.Errorf("expected 0 pending, 1 error, got %+v", counters)
		}
	})
}

// G4: a source whose browse failed makes the archive run fail; the
// already-emitted assets are still written and Finalize does not run.
func TestArchiveRunReturnsSourceError(t *testing.T) {
	a := newTestApp()
	ac := &ArchiveCmd{app: a, config: &cliarchive.Config{ArchivePath: t.TempDir()}}
	srcErr := errors.New("corrupt source")

	src := &mockSource{
		err: srcErr,
		before: func(ctx context.Context) {
			stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
			_ = a.FileProcessor().RecordAssetDiscovered(ctx, stuck, 10, journal.DiscoveredImage)
		},
	}

	err := ac.Run(newTestCommand(context.Background()), src)
	if !errors.Is(err, srcErr) {
		t.Fatalf("expected the source error, got %v", err)
	}
	counters := a.FileProcessor().GetAssetCounters()
	if counters.Pending != 1 {
		t.Errorf("expected the unattempted asset to stay pending after a source failure, got %+v", counters)
	}
}
