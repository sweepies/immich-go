package assets

import (
	"fmt"
	"testing"
)

func BenchmarkMergeAlbums(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			albums1 := make([]Album, n)
			for i := 0; i < n; i++ {
				albums1[i] = Album{Title: fmt.Sprintf("Album %d", i)}
			}
			albums2 := make([]Album, n)
			for i := 0; i < n; i++ {
				albums2[i] = Album{Title: fmt.Sprintf("Album %d", i+n/2)}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				asset := &Asset{Albums: albums1}
				asset.MergeAlbums(albums2)
			}
		})
	}
}

func TestMergeAlbums(t *testing.T) {
	asset := &Asset{
		Albums: []Album{
			{Title: "Album 1"},
			{Title: "Album 2"},
		},
	}
	newAlbums := []Album{
		{Title: "Album 2"},
		{Title: "Album 3"},
	}

	asset.MergeAlbums(newAlbums)

	if len(asset.Albums) != 3 {
		t.Errorf("Expected 3 albums, got %d", len(asset.Albums))
	}

	expected := []string{"Album 1", "Album 2", "Album 3"}
	for i, alb := range asset.Albums {
		if alb.Title != expected[i] {
			t.Errorf("Expected album %d to be %s, got %s", i, expected[i], alb.Title)
		}
	}
}

func BenchmarkMergeTags(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			tags1 := make([]Tag, n)
			for i := 0; i < n; i++ {
				tags1[i] = Tag{Name: fmt.Sprintf("Tag %d", i)}
			}
			tags2 := make([]Tag, n)
			for i := 0; i < n; i++ {
				tags2[i] = Tag{Name: fmt.Sprintf("Tag %d", i+n/2)}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				asset := &Asset{Tags: tags1}
				asset.MergeTags(tags2)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	asset := &Asset{
		Tags: []Tag{
			{Name: "Tag 1"},
			{Name: "Tag 2"},
		},
	}
	newTags := []Tag{
		{Name: "Tag 2"},
		{Name: "Tag 3"},
	}

	asset.MergeTags(newTags)

	if len(asset.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(asset.Tags))
	}

	expected := []string{"Tag 1", "Tag 2", "Tag 3"}
	for i, tag := range asset.Tags {
		if tag.Name != expected[i] {
			t.Errorf("Expected tag %d to be %s, got %s", i, expected[i], tag.Name)
		}
	}
}
