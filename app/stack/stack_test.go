package stack

import (
	"context"
	"testing"

	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/filters"
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

type mockFilter struct{}

func (f mockFilter) Filter(g *assets.Group) *assets.Group {
	if len(g.Assets) > 1 {
		// Mark the second asset for removal
		assetToRemove := g.Assets[1]
		g.RemoveAsset(assetToRemove, "test removal")
	}
	return g
}

func TestProcessAssets_DeleteBatching(t *testing.T) {
	mi := &mockImmich{}
	s := &StackCmd{
		client: app.Client{
			Immich: mi,
		},
		assets: []*assets.Asset{
			{ID: "1", Radical: "file", OriginalFileName: "file.jpg"},
			{ID: "2", Radical: "file", OriginalFileName: "file.heic"},
			{ID: "3", Radical: "other", OriginalFileName: "other.jpg"},
			{ID: "4", Radical: "other", OriginalFileName: "other.heic"},
		},
		filters: []filters.Filter{mockFilter{}},
	}

	// We need a grouper that groups by radical
	s.groupers = append(s.groupers, func(ctx context.Context, out chan<- *assets.Group, in <-chan *assets.Asset) {
		groupsMap := make(map[string]*assets.Group)
		for a := range in {
			g, ok := groupsMap[a.Radical]
			if !ok {
				g = assets.NewGroup(assets.GroupByOther)
				groupsMap[a.Radical] = g
			}
			g.AddAsset(a)
		}
		for _, g := range groupsMap {
			out <- g
		}
	})

	ctx := context.Background()
	a := app.NewApplication(app.OptionNoLogFile())

	err := s.ProcessAssets(ctx, a)
	if err != nil {
		t.Fatalf("ProcessAssets failed: %v", err)
	}

	// Baseline expectation (current code):
	// 2 groups (file, other), each having 1 asset removed.
	// Total 2 assets removed.
	// Current code calls DeleteAssets for each removed asset.
	// So 2 calls.
	//
	// After optimization, if we batch per group, it would still be 2 calls if each group has 1 removed asset.
	// To see the benefit, we need a group with multiple removed assets.

	mi.deleteCalls = 0
	mi.deletedIDs = nil

	s.assets = []*assets.Asset{
		{ID: "1", Radical: "group1", OriginalFileName: "g1-1.jpg"},
		{ID: "2", Radical: "group1", OriginalFileName: "g1-2.jpg"},
		{ID: "3", Radical: "group1", OriginalFileName: "g1-3.jpg"},
	}
	// mockFilter2 removes all but the first asset
	mockFilter2 := filters.FilterFn(func(g *assets.Group) *assets.Group {
		for len(g.Assets) > 1 {
			g.RemoveAsset(g.Assets[1], "test removal")
		}
		return g
	})
	s.filters = []filters.Filter{mockFilter2}

	err = s.ProcessAssets(ctx, a)
	if err != nil {
		t.Fatalf("ProcessAssets failed: %v", err)
	}

	// After optimization, for 2 removed assets in 1 group, it should call DeleteAssets once.
	t.Logf("DeleteAssets calls: %d", mi.deleteCalls)
	if mi.deleteCalls != 1 {
		t.Errorf("Expected 1 DeleteAssets call (optimized), got %d", mi.deleteCalls)
	}

	if len(mi.deletedIDs) != 1 || len(mi.deletedIDs[0]) != 2 {
		t.Errorf("Expected 1 call with 2 IDs, got %d calls, 1st call with %d IDs", len(mi.deletedIDs), len(mi.deletedIDs[0]))
	}
}
