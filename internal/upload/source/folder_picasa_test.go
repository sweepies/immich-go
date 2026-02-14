package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFolderSourcePicasaAssignsAlbumsFromIni(t *testing.T) {
	tests := []struct {
		name    string
		iniName string
	}{
		{name: "dot prefixed ini", iniName: ".picasa.ini"},
		{name: "plain ini", iniName: "picasa.ini"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			assetName := "IMG_0001.JPG"

			if err := os.WriteFile(filepath.Join(root, assetName), []byte("x"), 0o644); err != nil {
				t.Fatalf("creating test asset: %v", err)
			}

			picasaIni := "[Picasa]\nname=Trip\ndescription=Summer memories\n"
			if err := os.WriteFile(filepath.Join(root, tt.iniName), []byte(picasaIni), 0o644); err != nil {
				t.Fatalf("creating picasa ini file: %v", err)
			}

			src := newTestFolderSource(root, false, 1)
			t.Cleanup(func() {
				_ = src.Close()
			})
			src.config.PicasaAlbum = true

			assetsOut := collectAssets(src.Browse(context.Background()))
			if len(assetsOut) != 1 {
				t.Fatalf("expected 1 asset, got %d", len(assetsOut))
			}

			if len(assetsOut[0].Albums) != 1 {
				t.Fatalf("expected 1 album, got %d", len(assetsOut[0].Albums))
			}

			if assetsOut[0].Albums[0].Title != "Trip" {
				t.Fatalf("expected album title Trip, got %q", assetsOut[0].Albums[0].Title)
			}

			if assetsOut[0].Albums[0].Description != "Summer memories" {
				t.Fatalf("expected album description Summer memories, got %q", assetsOut[0].Albums[0].Description)
			}
		})
	}
}
