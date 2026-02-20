package source

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fileevent"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/namematcher"
)

func BenchmarkFolderSourceICloudBrowse(b *testing.B) {
	root, err := os.MkdirTemp("", "bench-icloud")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(root)

	// Create a large number of directories and files to make the double scan noticeable
	numDirs := 500
	numFilesPerDir := 100

	for i := 0; i < numDirs; i++ {
		dir := filepath.Join(root, fmt.Sprintf("dir-%03d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("failed to create dir: %v", err)
		}
		for j := 0; j < numFilesPerDir; j++ {
			filename := filepath.Join(dir, fmt.Sprintf("photo-%03d.jpg", j))
			if err := os.WriteFile(filename, []byte("test"), 0o644); err != nil {
				b.Fatalf("failed to write file: %v", err)
			}
		}
		// Add an iCloud metadata file in some directories
		if i%10 == 0 {
			csvName := filepath.Join(dir, "Photo Details.csv")
			csvContent := "imgName,fileChecksum,favorite,hidden,deleted,originalCreationDate,viewCount,importDate\n"
			for j := 0; j < numFilesPerDir; j++ {
				csvContent += fmt.Sprintf("photo-%03d.jpg,hash,no,no,no,\"Saturday June 4,2022 12:11 PM GMT\",1,date\n", j)
			}
			if err := os.WriteFile(csvName, []byte(csvContent), 0o644); err != nil {
				b.Fatalf("failed to write csv: %v", err)
			}
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		src := newBenchFolderSource(root, true, 8)
		src.config.ICloudTakeout = true

		gOut := src.Browse(context.Background())
		count := 0
		for g := range gOut {
			count += len(g.Assets)
		}
		src.Close()

		if count != numDirs*numFilesPerDir {
			b.Errorf("expected %d assets, got %d", numDirs*numFilesPerDir, count)
		}
	}
}

func newBenchFolderSource(root string, recursive bool, workers int) *FolderSource {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	processor := fileprocessor.New(assettracker.New(), fileevent.NewRecorder(nil))

	return &FolderSource{
		deps: adapters.SourceDependencies{
			Logger:          logger,
			Processor:       processor,
			SupportedMedia:  filetypes.DefaultSupportedMedia,
			TimeZone:        time.UTC,
			ConcurrentTasks: workers,
		},
		config: &adapters.FolderConfig{
			BannedFiles: namematcher.MustList(),
			Recursive:   recursive,
		},
		fsyss: []fs.FS{os.DirFS(root)},
	}
}
