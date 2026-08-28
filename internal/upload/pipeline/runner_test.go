package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/journal"
)

// TestRunnerIntegration provides end-to-end tests for the upload pipeline runner.
func TestRunnerIntegration(t *testing.T) {
	t.Run("successful upload flow", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 1, Images: 1}
		mock.assets = []*immich.Asset{
			{ID: "existing-1", Checksum: "existing-cs", OriginalFileName: "existing.jpg", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}},
		}

		// Create source with new assets to upload
		asset1 := createTestAsset("new_photo1.jpg", 2048, time.Now())
		asset2 := createTestAsset("new_photo2.jpg", 4096, time.Now())

		source := newMockSource(
			&assets.Group{Assets: []*assets.Asset{asset1}},
			&assets.Group{Assets: []*assets.Asset{asset2}},
		)

		pctx := createRunnerTestContext(mock)

		albumSaveCalled := false
		tagSaveCalled := false

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			PauseJobs:   false,
			OnError: func(err error) error {
				return err // Propagate errors
			},
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				albumSaveCalled = true
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				tagSaveCalled = true
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("Runner.Run() error = %v", err)
		}

		// Verify assets were indexed
		if pctx.Index.Len() < 1 {
			t.Errorf("expected at least 1 asset in index, got %d", pctx.Index.Len())
		}

		// Note: albumSaveCalled and tagSaveCalled may not be true if assets don't have albums/tags
		_ = albumSaveCalled
		_ = tagSaveCalled
	})

	t.Run("upload with job pausing", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 0}

		source := newMockSource()

		pctx := createRunnerTestContext(mock)

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			PauseJobs:   true,
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("Runner.Run() error = %v", err)
		}

		// Verify jobs were paused and resumed
		pauseCount := 0
		resumeCount := 0
		for _, cmd := range mock.jobCommands {
			if cmd.command == immich.JobCommandPause {
				pauseCount++
			} else if cmd.command == immich.JobCommandResume {
				resumeCount++
			}
		}

		if pauseCount != 5 {
			t.Errorf("expected 5 pause commands, got %d", pauseCount)
		}
		if resumeCount != 5 {
			t.Errorf("expected 5 resume commands, got %d", resumeCount)
		}
	})

	t.Run("pause jobs error", func(t *testing.T) {
		mock := newMockServerClient()
		mock.sendJobCmdErr = errors.New("pause failed")

		source := newMockSource()
		pctx := createRunnerTestContext(mock)

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			PauseJobs:   true,
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err == nil {
			t.Fatal("expected error when pause fails, got nil")
		}
	})

	t.Run("discovery error", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStatsError = errors.New("stats error")

		source := newMockSource()
		pctx := createRunnerTestContext(mock)

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err == nil {
			t.Fatal("expected error from discovery, got nil")
		}
	})

	t.Run("upload with duplicate detection", func(t *testing.T) {
		captureDate := time.Now()

		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 1}
		mock.assets = []*immich.Asset{
			{
				ID:               "server-asset-1",
				Checksum:         "same-checksum",
				OriginalFileName: "photo.jpg",
				OwnerID:          "test-user-id",
				ExifInfo: immich.ExifInfo{
					FileSizeInByte:   2048,
					DateTimeOriginal: immich.ImmichExifTime{Time: captureDate},
				},
			},
		}

		// Create a local asset with the same checksum
		localAsset := createTestAssetWithChecksum("photo.jpg", 2048, captureDate, "same-checksum")

		source := newMockSource(
			&assets.Group{Assets: []*assets.Asset{localAsset}},
		)

		pctx := createRunnerTestContext(mock)

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("Runner.Run() error = %v", err)
		}

		// With same checksum, the asset should be detected as duplicate and not uploaded
		// The upload index should be 0 (no uploads made)
		if mock.uploadIndex != 0 {
			t.Errorf("expected 0 uploads for duplicate asset, got %d", mock.uploadIndex)
		}
	})

	t.Run("upload with albums", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 0}
		mock.albums = []immich.AlbumSimplified{
			{ID: "album-1", AlbumName: "Existing Album"},
		}
		mock.albumAssetIDs["album-1"] = []immich.AssetID{}

		asset := createTestAsset("photo.jpg", 2048, time.Now())
		asset.Albums = []assets.Album{
			assets.NewAlbum("", "New Album", ""),
		}

		source := newMockSource(
			&assets.Group{Assets: []*assets.Asset{asset}},
		)

		pctx := createRunnerTestContext(mock)

		albumsSaved := []string{}
		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				albumsSaved = append(albumsSaved, album.Title)
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("Runner.Run() error = %v", err)
		}

		// Album save should have been called during cache close
		foundNewAlbum := slices.Contains(albumsSaved, "New Album")
		if !foundNewAlbum {
			t.Errorf("expected 'New Album' to be in saved albums, got %v", albumsSaved)
		}
	})

	t.Run("context cancellation during upload", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 0}

		// Create many assets to upload
		groups := make([]*assets.Group, 100)
		for i := range 100 {
			asset := createTestAsset("photo"+string(rune(i))+".jpg", 1024, time.Now())
			groups[i] = &assets.Group{Assets: []*assets.Asset{asset}}
		}

		source := newMockSource(groups...)
		pctx := createRunnerTestContext(mock)

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_ = runner.Run(ctx)
		// We don't check for specific error since cancellation timing is non-deterministic
	})

	t.Run("error callback controls abort", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 0}
		mock.uploadError = errors.New("upload failed")

		asset := createTestAsset("photo.jpg", 1024, time.Now())
		source := newMockSource(
			&assets.Group{Assets: []*assets.Asset{asset}},
		)

		pctx := createRunnerTestContext(mock)

		errorCallCount := 0
		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			OnError: func(err error) error {
				errorCallCount++
				return nil // Suppress error, continue
			},
			SaveAlbum: func(album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		err := runner.Run(context.Background())
		if err != nil {
			t.Logf("Runner.Run() completed with error = %v (may be expected)", err)
		}

		if errorCallCount == 0 {
			t.Error("expected OnError to be called at least once")
		}
	})
}

// TestNewContext tests the NewContext constructor.
func TestNewContext(t *testing.T) {
	t.Run("creates context with session tag", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		recorder := journal.NewRecorder(logger)
		processor := fileprocessor.New(nil, recorder)
		mock := newMockServerClient()

		cfg := Config{
			SessionTag:     true,
			ConcurrentTask: 4,
		}

		ctx := NewContext(cfg, logger, processor, nil, mock, mock)

		if ctx.SessionTagValue == "" {
			t.Error("expected session tag value to be set")
		}
		if ctx.Index == nil {
			t.Error("expected index to be initialized")
		}
	})

	t.Run("creates context without session tag", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		recorder := journal.NewRecorder(logger)
		processor := fileprocessor.New(nil, recorder)
		mock := newMockServerClient()

		cfg := Config{
			SessionTag: false,
		}

		ctx := NewContext(cfg, logger, processor, nil, mock, mock)

		if ctx.SessionTagValue != "" {
			t.Errorf("expected empty session tag value, got %s", ctx.SessionTagValue)
		}
	})
}

// Helper to create test context for runner tests.
func createRunnerTestContext(server immich.UploadClient) *Context {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	recorder := journal.NewRecorder(logger)
	tracker := assettracker.New()
	processor := fileprocessor.New(tracker, recorder)
	jobs, _ := server.(immich.JobControlService)
	return &Context{
		Config: Config{
			ConcurrentTask: 2,
			OutputFormat:   "text",
		},
		Logger:    logger,
		Processor: processor,
		Server:    server,
		Jobs:      jobs,
		Index:     NewIndex(),
		StartTime: time.Now(),
	}
}

// Helper to create test asset with file.
func createTestAsset(name string, size int, captureDate time.Time) *assets.Asset {
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	fsys := fstest.MapFS{
		name: &fstest.MapFile{
			Data:    content,
			ModTime: captureDate,
		},
	}
	return &assets.Asset{
		File:             fshelper.NewFilename(fsys, name),
		OriginalFileName: name,
		FileSize:         size,
		CaptureDate:      captureDate,
		Checksum:         "checksum-" + name + "-" + captureDate.String(),
	}
}

// Helper to create test asset with specific checksum.
func createTestAssetWithChecksum(name string, size int, captureDate time.Time, checksum string) *assets.Asset {
	content := make([]byte, size)

	fsys := fstest.MapFS{
		name: &fstest.MapFile{
			Data:    content,
			ModTime: captureDate,
		},
	}
	return &assets.Asset{
		File:             fshelper.NewFilename(fsys, name),
		OriginalFileName: name,
		FileSize:         size,
		CaptureDate:      captureDate,
		Checksum:         checksum,
	}
}
