package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
)

func TestFolderSourceICloudTakeoutAssignsAlbumsFromMetadata(t *testing.T) {
	root := t.TempDir()
	albumsDir := filepath.Join(root, "Albums")
	if err := os.MkdirAll(albumsDir, 0o755); err != nil {
		t.Fatalf("creating albums directory: %v", err)
	}

	assetName := "IMG_0001.JPG"
	if err := os.WriteFile(filepath.Join(root, assetName), []byte("x"), 0o644); err != nil {
		t.Fatalf("creating test asset: %v", err)
	}

	albumCSV := "imgName\n" + assetName + "\n"
	if err := os.WriteFile(filepath.Join(albumsDir, "Trip.csv"), []byte(albumCSV), 0o644); err != nil {
		t.Fatalf("creating album metadata csv: %v", err)
	}

	src := newTestFolderSource(root, true, 1)
	t.Cleanup(func() {
		_ = src.Close()
	})
	src.config.ICloudTakeout = true

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
}

func TestFolderSourceICloudTakeoutUsesPhotoDetailsForCaptureDate(t *testing.T) {
	root := t.TempDir()

	assetName := "IMG_0002.JPG"
	if err := os.WriteFile(filepath.Join(root, assetName), []byte("x"), 0o644); err != nil {
		t.Fatalf("creating test asset: %v", err)
	}

	originalDate := "Saturday June 4,2022 12:11 PM GMT"
	photoDetailsCSV := "imgName,fileChecksum,favorite,hidden,deleted,originalCreationDate,viewCount,importDate\n" +
		assetName + ",checksum,no,no,no,\"" + originalDate + "\",10,\"" + originalDate + "\"\n"
	if err := os.WriteFile(filepath.Join(root, "Photo Details.csv"), []byte(photoDetailsCSV), 0o644); err != nil {
		t.Fatalf("creating photo details csv: %v", err)
	}

	wantDate, err := time.Parse(iCloudOriginalCreationDateLayout, originalDate)
	if err != nil {
		t.Fatalf("parsing expected date: %v", err)
	}

	src := newTestFolderSource(root, false, 1)
	t.Cleanup(func() {
		_ = src.Close()
	})
	src.config.ICloudTakeout = true
	src.config.TakeDateFromFilename = true

	assetsOut := collectAssets(src.Browse(context.Background()))
	if len(assetsOut) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assetsOut))
	}

	if !assetsOut[0].CaptureDate.Equal(wantDate) {
		t.Fatalf("expected capture date %s, got %s", wantDate, assetsOut[0].CaptureDate)
	}
}

func collectAssets(groups <-chan *assets.Group) []*assets.Asset {
	assetsOut := make([]*assets.Asset, 0)
	for g := range groups {
		if g == nil {
			continue
		}
		assetsOut = append(assetsOut, g.Assets...)
	}
	return assetsOut
}
