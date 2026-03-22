package stack

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/groups"
	"github.com/sweepies/immich-go/internal/groups/series"
)

type mockImmich struct {
	immich.ImmichInterface
	deleteCalls int
	deletedIDs  [][]string
}

func (m *mockImmich) DeleteAssets(ctx context.Context, IDs []string, force bool) error {
	m.deleteCalls++
	m.deletedIDs = append(m.deletedIDs, IDs)
	return nil
}

func (m *mockImmich) CreateStack(ctx context.Context, ids []string) (string, error) {
	return "stack-id", nil
}

func newTestApp() *app.Application {
	a := app.New(context.Background(), &cobra.Command{Use: "test"})
	a.SetLog(&app.Log{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return a
}

func newImageAsset(id, radical, name string, taken time.Time) *assets.Asset {
	return &assets.Asset{
		ID:               id,
		OriginalFileName: name,
		CaptureDate:      taken,
		NameInfo: assets.NameInfo{
			Radical: radical,
			Type:    filetypes.TypeImage,
		},
	}
}

func newStackCmd(mi *mockImmich, as []*assets.Asset, fs ...filters.Filter) *StackCmd {
	return &StackCmd{
		client: app.Client{
			Immich: mi,
		},
		assets:   as,
		filters:  fs,
		groupers: []groups.Grouper{series.Group},
	}
}

func removeSecondAsset() filters.Filter {
	return func(g *assets.Group) *assets.Group {
		if len(g.Assets) > 1 {
			g.RemoveAsset(g.Assets[1], "test removal")
		}
		return g
	}
}

func keepOnlyFirstAsset() filters.Filter {
	return func(g *assets.Group) *assets.Group {
		for len(g.Assets) > 1 {
			g.RemoveAsset(g.Assets[1], "test removal")
		}
		return g
	}
}

func TestProcessAssetsDeleteBatching(t *testing.T) {
	ctx := context.Background()
	a := newTestApp()
	baseTime := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	t.Run("deletes once per group", func(t *testing.T) {
		mi := &mockImmich{}
		s := newStackCmd(
			mi,
			[]*assets.Asset{
				newImageAsset("1", "file", "file-1.jpg", baseTime),
				newImageAsset("2", "file", "file-2.heic", baseTime.Add(100*time.Millisecond)),
				newImageAsset("3", "other", "other-1.jpg", baseTime.Add(2*time.Second)),
				newImageAsset("4", "other", "other-2.heic", baseTime.Add(2100*time.Millisecond)),
			},
			removeSecondAsset(),
		)

		if err := s.ProcessAssets(ctx, a); err != nil {
			t.Fatalf("ProcessAssets failed: %v", err)
		}

		if mi.deleteCalls != 2 {
			t.Fatalf("expected 2 DeleteAssets calls, got %d", mi.deleteCalls)
		}

		want := [][]string{{"2"}, {"4"}}
		if !reflect.DeepEqual(mi.deletedIDs, want) {
			t.Fatalf("expected deleted IDs %v, got %v", want, mi.deletedIDs)
		}
	})

	t.Run("batches multiple removals from one group", func(t *testing.T) {
		mi := &mockImmich{}
		s := newStackCmd(
			mi,
			[]*assets.Asset{
				newImageAsset("1", "group1", "g1-1.jpg", baseTime),
				newImageAsset("2", "group1", "g1-2.jpg", baseTime.Add(100*time.Millisecond)),
				newImageAsset("3", "group1", "g1-3.jpg", baseTime.Add(200*time.Millisecond)),
			},
			keepOnlyFirstAsset(),
		)

		if err := s.ProcessAssets(ctx, a); err != nil {
			t.Fatalf("ProcessAssets failed: %v", err)
		}

		if mi.deleteCalls != 1 {
			t.Fatalf("expected 1 DeleteAssets call, got %d", mi.deleteCalls)
		}

		want := [][]string{{"2", "3"}}
		if !reflect.DeepEqual(mi.deletedIDs, want) {
			t.Fatalf("expected deleted IDs %v, got %v", want, mi.deletedIDs)
		}
	})
}
