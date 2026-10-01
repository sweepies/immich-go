package stack

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/groups"
)

type mockImmich struct {
	immich.Client
	deleteCalls int
	deletedIDs  [][]immich.AssetID
}

func (m *mockImmich) DeleteAssets(ctx context.Context, ids []immich.AssetID, force bool) error {
	m.deleteCalls++
	m.deletedIDs = append(m.deletedIDs, ids)
	return nil
}

func (m *mockImmich) CreateStack(ctx context.Context, ids []immich.AssetID) (immich.StackID, error) {
	return "stack-id", nil
}

func TestProcessAssets_DeleteBatching(t *testing.T) {
	mi := &mockImmich{}
	ctx := context.Background()
	a := app.New(ctx, nil)
	a.SetLog(&app.Log{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})

	groupAll := func(ctx context.Context, in <-chan *assets.Asset, _ chan<- *assets.Asset, out chan<- *assets.Group) {
		group := assets.NewGroup(assets.GroupByOther)
		for asset := range in {
			group.AddAsset(asset)
		}
		select {
		case out <- group:
		case <-ctx.Done():
		}
	}
	removeAllButFirst := func(group *assets.Group) *assets.Group {
		for len(group.Assets) > 1 {
			group.RemoveAsset(group.Assets[1], "test removal")
		}
		return group
	}

	s := &StackCmd{
		client: app.Client{
			Immich: mi,
		},
		assets: []*assets.Asset{
			{ID: "1", OriginalFileName: "g1-1.jpg", NameInfo: assets.NameInfo{Radical: "1"}},
			{ID: "2", OriginalFileName: "g1-2.jpg", NameInfo: assets.NameInfo{Radical: "2"}},
			{ID: "3", OriginalFileName: "g1-3.jpg", NameInfo: assets.NameInfo{Radical: "3"}},
		},
		groupers: []groups.Grouper{groupAll},
		filters:  []filters.Filter{removeAllButFirst},
	}

	if err := s.ProcessAssets(ctx, a); err != nil {
		t.Fatalf("ProcessAssets failed: %v", err)
	}
	if mi.deleteCalls != 1 {
		t.Fatalf("DeleteAssets calls = %d, want 1", mi.deleteCalls)
	}
	if len(mi.deletedIDs) != 1 {
		t.Fatalf("DeleteAssets batches = %d, want 1", len(mi.deletedIDs))
	}
	wantIDs := []immich.AssetID{"2", "3"}
	if !slices.Equal(mi.deletedIDs[0], wantIDs) {
		t.Errorf("DeleteAssets IDs = %v, want %v", mi.deletedIDs[0], wantIDs)
	}
}
