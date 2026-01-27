package source

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/filetypes"
)

func TestFactory_Dependencies(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	tz := time.UTC

	factory := NewFactory(log, nil, filetypes.DefaultSupportedMedia, tz, 4)

	deps := factory.Dependencies()

	if deps.Logger != log {
		t.Error("expected logger to be set")
	}
	if deps.TimeZone != tz {
		t.Error("expected timezone to be set")
	}
	if deps.ConcurrentTasks != 4 {
		t.Errorf("expected concurrent tasks to be 4, got %d", deps.ConcurrentTasks)
	}
}

func TestFactory_CreateFromConfig_InvalidMode(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	factory := NewFactory(log, nil, filetypes.DefaultSupportedMedia, time.UTC, 4)

	_, err := factory.CreateFromConfig(context.Background(), adapters.SourceMode("invalid"), nil)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestFactory_CreateFromConfig_InvalidConfig(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	factory := NewFactory(log, nil, filetypes.DefaultSupportedMedia, time.UTC, 4)

	tests := []struct {
		name string
		mode adapters.SourceMode
		cfg  any
	}{
		{"folder with wrong config", adapters.SourceModeFolder, "wrong"},
		{"icloud with wrong config", adapters.SourceModeICloud, 123},
		{"picasa with wrong config", adapters.SourceModePicasa, nil},
		{"google with wrong config", adapters.SourceModeGoogle, &adapters.FolderConfig{}},
		{"fromimmich with wrong config", adapters.SourceModeFromImmich, &adapters.FolderConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.CreateFromConfig(context.Background(), tt.mode, tt.cfg)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

// mockLegacyReader is a mock that implements the legacy adapters.Reader interface
// with a bidirectional channel as required by the old interface.
type mockLegacyReader struct {
	groups []*assets.Group
}

func (m *mockLegacyReader) Browse(ctx context.Context) chan *assets.Group {
	out := make(chan *assets.Group)
	go func() {
		defer close(out)
		for _, g := range m.groups {
			select {
			case out <- g:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func TestLegacyReaderAdapter(t *testing.T) {
	expectedGroups := []*assets.Group{
		assets.NewGroup(assets.GroupByNone),
	}

	mock := &mockLegacyReader{groups: expectedGroups}
	adapter := NewLegacyReaderAdapter(mock, nil)

	ctx := context.Background()
	groups := adapter.Browse(ctx)

	var received []*assets.Group
	for g := range groups {
		received = append(received, g)
	}

	if len(received) != len(expectedGroups) {
		t.Errorf("expected %d groups, got %d", len(expectedGroups), len(received))
	}

	if err := adapter.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestCloseFSs(t *testing.T) {
	// Test with nil slice
	err := CloseFSs(nil)
	if err != nil {
		t.Errorf("unexpected error closing nil slice: %v", err)
	}

	// Test with empty slice
	err = CloseFSs([]fs.FS{})
	if err != nil {
		t.Errorf("unexpected error closing empty slice: %v", err)
	}
}

func TestParsePaths_Empty(t *testing.T) {
	_, err := ParsePaths(nil)
	if err == nil {
		t.Error("expected error for nil paths")
	}

	_, err = ParsePaths([]string{})
	if err == nil {
		t.Error("expected error for empty paths")
	}
}
