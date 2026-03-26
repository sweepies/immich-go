package assets

import (
	"fmt"
	"testing"
)

func BenchmarkRemoveAsset(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				assetsList := make([]*Asset, size)
				for j := 0; j < size; j++ {
					assetsList[j] = &Asset{}
				}
				g := &Group{
					Assets: assetsList,
				}
				target := g.Assets[size/2]
				b.StartTimer()
				g.RemoveAsset(target, "test")
			}
		})
	}
}

func TestRemoveAssetCorrectness(t *testing.T) {
	t.Run("remove middle asset", func(t *testing.T) {
		a1 := &Asset{ID: "1"}
		a2 := &Asset{ID: "2"}
		a3 := &Asset{ID: "3"}
		g := NewGroup(GroupByNone, a1, a2, a3)
		g.CoverIndex = 2 // a3

		g.RemoveAsset(a2, "reason")

		if len(g.Assets) != 2 {
			t.Errorf("expected 2 assets, got %d", len(g.Assets))
		}
		// a2 is removed, a3 is swapped into its place
		if g.Assets[1] != a3 {
			t.Errorf("expected a3 at index 1, got %v", g.Assets[1])
		}
		if g.CoverIndex != 1 {
			t.Errorf("expected CoverIndex to be 1, got %d", g.CoverIndex)
		}
	})

	t.Run("remove cover asset", func(t *testing.T) {
		a1 := &Asset{ID: "1"}
		a2 := &Asset{ID: "2"}
		a3 := &Asset{ID: "3"}
		g := NewGroup(GroupByNone, a1, a2, a3)
		g.CoverIndex = 1 // a2

		g.RemoveAsset(a2, "reason")

		if len(g.Assets) != 2 {
			t.Errorf("expected 2 assets, got %d", len(g.Assets))
		}
		// a2 is removed, a3 is swapped into its place. a2 was cover.
		// New cover should be 0 because cover was removed.
		if g.CoverIndex != 0 {
			t.Errorf("expected CoverIndex to be 0, got %d", g.CoverIndex)
		}
	})

	t.Run("remove last asset", func(t *testing.T) {
		a1 := &Asset{ID: "1"}
		a2 := &Asset{ID: "2"}
		a3 := &Asset{ID: "3"}
		g := NewGroup(GroupByNone, a1, a2, a3)
		g.CoverIndex = 1 // a2

		g.RemoveAsset(a3, "reason")

		if len(g.Assets) != 2 {
			t.Errorf("expected 2 assets, got %d", len(g.Assets))
		}
		if g.Assets[0] != a1 || g.Assets[1] != a2 {
			t.Errorf("unexpected assets: %v, %v", g.Assets[0], g.Assets[1])
		}
		if g.CoverIndex != 1 {
			t.Errorf("expected CoverIndex to be 1, got %d", g.CoverIndex)
		}
	})

	t.Run("remove asset from single element group", func(t *testing.T) {
		a1 := &Asset{ID: "1"}
		g := NewGroup(GroupByNone, a1)
		g.CoverIndex = 0

		g.RemoveAsset(a1, "reason")

		if len(g.Assets) != 0 {
			t.Errorf("expected 0 assets, got %d", len(g.Assets))
		}
		if g.CoverIndex != 0 {
			t.Errorf("expected CoverIndex to be 0, got %d", g.CoverIndex)
		}
	})
}
