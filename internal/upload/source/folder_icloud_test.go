package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/gen"
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

func TestUseICloudAlbumCSVConcurrentUpdatesPreserveAllAlbums(t *testing.T) {
	const (
		assetName   = "IMG_0003.JPG"
		albumCount  = 64
		csvTemplate = "imgName\n%s\n"
	)

	fsys := fstest.MapFS{}
	for i := range albumCount {
		name := fmt.Sprintf("Albums/Album_%03d.csv", i)
		fsys[name] = &fstest.MapFile{Data: []byte(fmt.Sprintf(csvTemplate, assetName))}
	}

	meta := gen.NewSyncMap[string, icloudMeta]()
	start := make(chan struct{})
	errCh := make(chan error, albumCount)

	var wg sync.WaitGroup
	for i := range albumCount {
		filename := fmt.Sprintf("Albums/Album_%03d.csv", i)
		albumName := fmt.Sprintf("Album_%03d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := useICloudAlbumCSV(meta, fsys, filename, albumName); err != nil {
				errCh <- err
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("useICloudAlbumCSV returned error: %v", err)
	}

	got, ok := meta.Load(assetName)
	if !ok {
		t.Fatalf("expected metadata for %s", assetName)
	}
	if len(got.albums) != albumCount {
		t.Fatalf("expected %d albums, got %d", albumCount, len(got.albums))
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
