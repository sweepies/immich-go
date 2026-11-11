package pipeline

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/assets/cache"
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
		source.processor = pctx.Processor

		albumSaveCalled := false
		tagSaveCalled := false

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			PauseJobs:   false,
			OnError: func(err error) error {
				return err // Propagate errors
			},
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				albumSaveCalled = true
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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

		if counters := pctx.Processor.GetAssetCounters(); !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
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
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
			switch cmd.command {
			case immich.JobCommandPause:
				pauseCount++
			case immich.JobCommandResume:
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
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
		source.processor = pctx.Processor

		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
		source.processor = pctx.Processor

		albumsSaved := []string{}
		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				albumsSaved = append(albumsSaved, album.Title)
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
		source.processor = pctx.Processor

		errorCallCount := 0
		runner := NewRunner(RunnerConfig{
			Source:      source,
			PipelineCtx: pctx,
			OnError: func(err error) error {
				errorCallCount++
				return nil // Suppress error, continue
			},
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
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
		server := newMockServerClient()
		jobs := newMockServerClient()

		cfg := Config{
			SessionTag:     true,
			ConcurrentTask: 4,
		}

		ctx := NewContext(cfg, logger, processor, nil, server, jobs)

		if ctx.SessionTagValue == "" {
			t.Error("expected session tag value to be set")
		}
		if ctx.Index == nil {
			t.Error("expected index to be initialized")
		}
		if ctx.Server != server {
			t.Error("context server does not preserve upload client argument")
		}
		if ctx.Jobs != jobs {
			t.Error("context jobs does not preserve job-control client argument")
		}
	})

	t.Run("creates context without session tag", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		recorder := journal.NewRecorder(logger)
		processor := fileprocessor.New(nil, recorder)
		server := newMockServerClient()
		jobs := newMockServerClient()

		cfg := Config{
			SessionTag: false,
		}

		ctx := NewContext(cfg, logger, processor, nil, server, jobs)

		if ctx.SessionTagValue != "" {
			t.Errorf("expected empty session tag value, got %s", ctx.SessionTagValue)
		}
		if ctx.Server != server {
			t.Error("context server does not preserve upload client argument")
		}
		if ctx.Jobs != jobs {
			t.Error("context jobs does not preserve job-control client argument")
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

// TestRunnerFinalizesPendingAssets checks that a completed run moves every
// leftover PENDING asset to ERROR and reports it (A4).
func TestRunnerFinalizesPendingAssets(t *testing.T) {
	mock := newMockServerClient()
	source := newMockSource()

	pctx := createRunnerTestContext(mock)

	// An asset that was discovered but never reaches the pipeline.
	stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
	if err := pctx.Processor.RecordAssetDiscovered(context.Background(), stuck, 100, journal.DiscoveredImage); err != nil {
		t.Fatalf("RecordAssetDiscovered: %v", err)
	}

	runner := NewRunner(RunnerConfig{
		Source:      source,
		PipelineCtx: pctx,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected the Finalize error, got nil")
	}
	if !strings.Contains(err.Error(), "never reached final state") {
		t.Errorf("expected 'never reached final state' error, got %v", err)
	}

	counters := pctx.Processor.GetAssetCounters()
	if counters.Pending != 0 {
		t.Errorf("expected 0 pending after Finalize, got %d", counters.Pending)
	}
	if counters.Errors != 1 {
		t.Errorf("expected 1 error after Finalize, got %d", counters.Errors)
	}
	if got := pctx.Processor.GetEventCounts()[journal.ErrorIncomplete]; got != 1 {
		t.Errorf("expected 1 ErrorIncomplete event, got %d", got)
	}
}

// TestRunnerJobPauseResume covers the run-scoped job queue tracking: only the
// queues this run paused may be resumed, and a failed pause must leave the
// server exactly as it found it.
func TestRunnerJobPauseResume(t *testing.T) {
	newRunner := func(mock *mockServerClient, noResume bool) (*Runner, *Context) {
		pctx := createRunnerTestContext(mock)
		return NewRunner(RunnerConfig{
			Source:       newMockSource(),
			PipelineCtx:  pctx,
			PauseJobs:    true,
			NoResumeJobs: noResume,
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		}), pctx
	}

	commands := func(mock *mockServerClient) []string {
		seq := make([]string, 0, len(mock.jobCommands))
		for _, c := range mock.jobCommands {
			seq = append(seq, string(c.command)+" "+c.name)
		}
		return seq
	}

	t.Run("already paused queues are left alone", func(t *testing.T) {
		mock := newMockServerClient()
		alreadyPaused := immich.Job{}
		alreadyPaused.QueueStatus.IsPaused = true
		mock.jobs = map[string]immich.Job{"metadataExtraction": alreadyPaused}

		runner, _ := newRunner(mock, false)
		if err := runner.Run(context.Background()); err != nil {
			t.Fatalf("Runner.Run() error = %v", err)
		}

		paused, resumed := 0, 0
		for _, c := range mock.jobCommands {
			if c.name == "metadataExtraction" {
				t.Errorf("no job command expected for the already paused queue, got %s", c.command)
			}
			switch c.command {
			case immich.JobCommandPause:
				paused++
			case immich.JobCommandResume:
				resumed++
			}
		}
		if paused != len(managedQueues)-1 {
			t.Errorf("expected %d pause commands, got %d", len(managedQueues)-1, paused)
		}
		if resumed != len(managedQueues)-1 {
			t.Errorf("expected %d resume commands, got %d", len(managedQueues)-1, resumed)
		}
	})

	t.Run("failed pause resumes only the queues it paused", func(t *testing.T) {
		mock := newMockServerClient()
		// videoConversion is the third queue in managedQueues.
		mock.sendJobCmdErrs = map[string]error{"pause videoConversion": errors.New("pause failed")}

		runner, _ := newRunner(mock, false)
		if err := runner.Run(context.Background()); err == nil {
			t.Fatal("expected error when a pause fails, got nil")
		}

		want := []string{
			"pause thumbnailGeneration",
			"pause metadataExtraction",
			"pause videoConversion",
			"resume thumbnailGeneration",
			"resume metadataExtraction",
		}
		if got := commands(mock); !slices.Equal(got, want) {
			t.Errorf("expected commands %v, got %v", want, got)
		}
	})

	t.Run("GetJobs failure aborts without touching the queues", func(t *testing.T) {
		mock := newMockServerClient()
		mock.getJobsErr = errors.New("get jobs failed")

		runner, _ := newRunner(mock, false)
		if err := runner.Run(context.Background()); err == nil {
			t.Fatal("expected error when GetJobs fails, got nil")
		}
		if len(mock.jobCommands) != 0 {
			t.Errorf("expected no job commands, got %v", commands(mock))
		}
	})

	t.Run("NoResumeJobs skips the final resume", func(t *testing.T) {
		mock := newMockServerClient()

		runner, _ := newRunner(mock, true)
		if err := runner.Run(context.Background()); err != nil {
			t.Fatalf("Runner.Run() error = %v", err)
		}

		paused, resumed := 0, 0
		for _, c := range mock.jobCommands {
			switch c.command {
			case immich.JobCommandPause:
				paused++
			case immich.JobCommandResume:
				resumed++
			}
		}
		if paused != len(managedQueues) {
			t.Errorf("expected %d pause commands, got %d", len(managedQueues), paused)
		}
		if resumed != 0 {
			t.Errorf("expected no resume commands with NoResumeJobs, got %d", resumed)
		}
	})

	t.Run("failed rollback is still attempted for every paused queue", func(t *testing.T) {
		mock := newMockServerClient()
		// Pause fails on the second queue and resume fails on the first: the
		// rollback must still try the queues it managed to pause.
		mock.sendJobCmdErrs = map[string]error{
			"pause metadataExtraction":   errors.New("pause failed"),
			"resume thumbnailGeneration": errors.New("resume failed"),
		}

		runner, _ := newRunner(mock, false)
		if err := runner.Run(context.Background()); err == nil {
			t.Fatal("expected error when a pause fails, got nil")
		}

		got := commands(mock)
		if !slices.Contains(got, "resume thumbnailGeneration") {
			t.Errorf("expected a rollback resume for thumbnailGeneration, got %v", got)
		}
		for _, c := range got {
			if c == "pause metadataExtraction" {
				continue
			}
			if slices.Contains([]string{"pause videoConversion", "pause faceDetection", "pause smartSearch"}, c) {
				t.Errorf("pause must stop at the first failure, got %v", got)
			}
		}
	})
}

// TestRunnerFlushSurvivesCancellation checks that the final album/tag flush
// runs on a fresh context even when the run context was cancelled (B3).
func TestRunnerFlushSurvivesCancellation(t *testing.T) {
	mock := newMockServerClient()
	source := newMockSource()
	pctx := createRunnerTestContext(mock)

	runCtx, cancel := context.WithCancel(context.Background())

	var flushErr error
	flushDelivered := false
	runner := NewRunner(RunnerConfig{
		Source:      source,
		PipelineCtx: pctx,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			flushErr = ctx.Err()
			flushDelivered = len(ids) > 0
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	// Simulate a run that queued an album membership then was cancelled.
	runner.albumsCache = cache.NewCollectionCache(50, runner.config.SaveAlbum)
	runner.tagsCache = cache.NewCollectionCache(50, runner.config.SaveTag)
	runner.albumsCache.AddIDToCollection(runCtx, "album", assets.NewAlbum("", "album", ""), "asset-1")
	cancel()

	runner.finish()

	if flushErr != nil {
		t.Errorf("expected the final flush to run on a live context, got %v", flushErr)
	}
	if !flushDelivered {
		t.Error("expected the queued album membership to be delivered by the final flush")
	}
}

// TestRunnerDoesNotFinalizeAfterAbort checks that an aborted run (a pipeline
// error, not a context cancellation) leaves pending assets alone instead of
// turning every unattempted asset into an ErrorIncomplete error (F4).
func TestRunnerDoesNotFinalizeAfterAbort(t *testing.T) {
	mock := newMockServerClient()
	mock.uploadError = errors.New("upload failed")

	pctx := createRunnerTestContext(mock)

	// An asset that was discovered but never reaches the pipeline.
	stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
	if err := pctx.Processor.RecordAssetDiscovered(context.Background(), stuck, 100, journal.DiscoveredImage); err != nil {
		t.Fatalf("RecordAssetDiscovered: %v", err)
	}

	asset := createTestAsset("photo.jpg", 1024, time.Now())
	source := newMockSource(&assets.Group{Assets: []*assets.Asset{asset}})
	source.processor = pctx.Processor

	runner := NewRunner(RunnerConfig{
		Source:      source,
		PipelineCtx: pctx,
		OnError: func(err error) error {
			return err // Abort the run on the first error
		},
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected the abort error, got nil")
	}

	counters := pctx.Processor.GetAssetCounters()
	if counters.Pending != 1 {
		t.Errorf("expected the unattempted asset to stay pending after an abort, got %+v", counters)
	}
	if got := pctx.Processor.GetEventCounts()[journal.ErrorIncomplete]; got != 0 {
		t.Errorf("expected no ErrorIncomplete events after an abort, got %d", got)
	}
}

// TestRunnerJSONSummaryReportsRunError checks that a run failing before any
// asset exists still writes a JSON summary with status "error" (F6).
func TestRunnerJSONSummaryReportsRunError(t *testing.T) {
	mock := newMockServerClient()
	mock.assetStatsError = errors.New("server unreachable")

	pctx := createRunnerTestContext(mock)
	pctx.Config.OutputFormat = "json"

	runner := NewRunner(RunnerConfig{
		Source:      newMockSource(),
		PipelineCtx: pctx,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	// The JSON summary is written to os.Stdout by a deferred call inside Run.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	runErr := runner.Run(context.Background())
	os.Stdout = old
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}

	if runErr == nil {
		t.Fatal("expected a run error, got nil")
	}

	var summary string
	for line := range strings.Lines(string(out)) {
		if strings.Contains(line, `"type":"summary"`) {
			summary = line
		}
	}
	if summary == "" {
		t.Fatal("no JSON summary written")
	}
	if !strings.Contains(summary, `"status":"error"`) || !strings.Contains(summary, `"exit_code":1`) {
		t.Errorf("expected the summary to report the run error, got %s", summary)
	}
}

// TestRunnerPauseFailureDoesNotStartCaches checks that a failed job pause
// returns before the album/tag caches (and their goroutines) are created (F8).
func TestRunnerPauseFailureDoesNotStartCaches(t *testing.T) {
	mock := newMockServerClient()
	mock.getJobsErr = errors.New("get jobs failed")

	pctx := createRunnerTestContext(mock)
	runner := NewRunner(RunnerConfig{
		Source:      newMockSource(),
		PipelineCtx: pctx,
		PauseJobs:   true,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	if err := runner.Run(context.Background()); err == nil {
		t.Fatal("expected error when pausing jobs fails, got nil")
	}
	if runner.albumsCache != nil || runner.tagsCache != nil {
		t.Error("caches must not be created when pausing jobs fails")
	}
}

// TestRunnerFinishGivesEachFlushItsOwnDeadline checks that a slow album flush
// cannot consume the tag flush's deadline (F7).
func TestRunnerFinishGivesEachFlushItsOwnDeadline(t *testing.T) {
	old := cacheFlushTimeout
	cacheFlushTimeout = 100 * time.Millisecond
	defer func() { cacheFlushTimeout = old }()

	var tagRemaining time.Duration
	var tagCtxErr error

	runner := NewRunner(RunnerConfig{
		PipelineCtx: createRunnerTestContext(newMockServerClient()),
	})
	runner.albumsCache = cache.NewCollectionCache(10, func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
		<-ctx.Done() // Consume the whole album deadline
		return album, ctx.Err()
	})
	runner.tagsCache = cache.NewCollectionCache(10, func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
		tagCtxErr = ctx.Err()
		if d, ok := ctx.Deadline(); ok {
			tagRemaining = time.Until(d)
		}
		return tag, nil
	})

	ctx := context.Background()
	runner.albumsCache.AddIDToCollection(ctx, "album", assets.Album{}, "id1")
	runner.tagsCache.AddIDToCollection(ctx, "tag", assets.Tag{}, "id1")

	runner.finish()

	if tagCtxErr != nil {
		t.Errorf("tag flush ran on an exhausted context: %v", tagCtxErr)
	}
	if tagRemaining <= 0 {
		t.Errorf("tag flush did not get its own deadline, remaining=%v", tagRemaining)
	}
}

// TestRunnerSkipsFinalizeOnCancellation checks that pending assets are left
// alone when the run is cancelled.
func TestRunnerSkipsFinalizeOnCancellation(t *testing.T) {
	mock := newMockServerClient()
	source := newMockSource()

	pctx := createRunnerTestContext(mock)

	stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
	if err := pctx.Processor.RecordAssetDiscovered(context.Background(), stuck, 100, journal.DiscoveredImage); err != nil {
		t.Fatalf("RecordAssetDiscovered: %v", err)
	}

	runner := NewRunner(RunnerConfig{
		Source:      source,
		PipelineCtx: pctx,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = runner.Run(ctx)

	counters := pctx.Processor.GetAssetCounters()
	if counters.Pending != 1 {
		t.Errorf("expected the pending asset to be left alone on cancellation, got %+v", counters)
	}
	if got := pctx.Processor.GetEventCounts()[journal.ErrorIncomplete]; got != 0 {
		t.Errorf("expected no ErrorIncomplete event on cancellation, got %d", got)
	}
}

// progressCancellingHandler cancels the run context when the pipeline logs
// its final progress message, i.e. after the pipeline work has finished but
// before runPipeline returns to Run.
type progressCancellingHandler struct {
	slog.Handler
	cancel context.CancelCauseFunc
	cause  error
}

func (h *progressCancellingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "progress" {
		h.cancel(h.cause)
	}
	return h.Handler.Handle(ctx, r)
}

// G2: a context cancelled after the pipeline finished must surface the
// cancellation cause, not report a clean success while assets are pending.
func TestRunnerReturnsCauseWhenCancelledAfterPipelineEnd(t *testing.T) {
	mock := newMockServerClient()
	pctx := createRunnerTestContext(mock)

	stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
	if err := pctx.Processor.RecordAssetDiscovered(context.Background(), stuck, 100, journal.DiscoveredImage); err != nil {
		t.Fatalf("RecordAssetDiscovered: %v", err)
	}

	interrupt := errors.New("interrupted")
	runCtx, cancel := context.WithCancelCause(context.Background())
	pctx.Logger = slog.New(&progressCancellingHandler{
		Handler: slog.NewTextHandler(io.Discard, nil),
		cancel:  cancel,
		cause:   interrupt,
	})

	runner := NewRunner(RunnerConfig{
		Source:      newMockSource(),
		PipelineCtx: pctx,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	err := runner.Run(runCtx)
	if !errors.Is(err, interrupt) {
		t.Fatalf("expected the cancellation cause, got %v", err)
	}

	counters := pctx.Processor.GetAssetCounters()
	if counters.Pending != 1 {
		t.Errorf("expected the pending asset to be left alone on cancellation, got %+v", counters)
	}
	if got := pctx.Processor.GetEventCounts()[journal.ErrorIncomplete]; got != 0 {
		t.Errorf("expected no ErrorIncomplete event on cancellation, got %d", got)
	}
}

// G4: a source whose browse failed makes the run fail instead of exiting
// cleanly; emitted assets are still processed and Finalize does not run.
func TestRunnerReturnsSourceError(t *testing.T) {
	mock := newMockServerClient()
	sourceErr := errors.New("corrupt takeout")

	pctx := createRunnerTestContext(mock)

	stuck := fshelper.NewFilename(fstest.MapFS{}, "stuck.jpg")
	if err := pctx.Processor.RecordAssetDiscovered(context.Background(), stuck, 100, journal.DiscoveredImage); err != nil {
		t.Fatalf("RecordAssetDiscovered: %v", err)
	}

	asset := createTestAsset("photo.jpg", 1024, time.Now())
	source := newMockSource(&assets.Group{Assets: []*assets.Asset{asset}})
	source.processor = pctx.Processor
	source.err = sourceErr

	runner := NewRunner(RunnerConfig{
		Source:      source,
		PipelineCtx: pctx,
		SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		},
		SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		},
	})

	err := runner.Run(context.Background())
	if !errors.Is(err, sourceErr) {
		t.Fatalf("expected the source error, got %v", err)
	}

	counters := pctx.Processor.GetAssetCounters()
	if counters.Processed != 1 {
		t.Errorf("expected the emitted asset to be processed, got %+v", counters)
	}
	if counters.Pending != 1 {
		t.Errorf("expected the unattempted asset to stay pending after a source failure, got %+v", counters)
	}
	if got := pctx.Processor.GetEventCounts()[journal.ErrorIncomplete]; got != 0 {
		t.Errorf("expected no ErrorIncomplete events after a source failure, got %d", got)
	}
}

// G5: the error reported by the run must be the failure that stopped the
// pipeline, not a secondary context.Canceled produced by a racing goroutine.
// The mask only shows when scheduling lines up, so the run is repeated.
func TestRunnerPipelineErrorIsNotMasked(t *testing.T) {
	sentinel := errors.New("asset statistics unreachable")
	for range 15 {
		mock := newMockServerClient()
		mock.assetStatsError = sentinel

		runner := NewRunner(RunnerConfig{
			Source:      newMockSource(),
			PipelineCtx: createRunnerTestContext(mock),
			SaveAlbum: func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
				return album, nil
			},
			SaveTag: func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
				return tag, nil
			},
		})

		if err := runner.Run(context.Background()); !errors.Is(err, sentinel) {
			t.Fatalf("expected the real pipeline error %v, got %v", sentinel, err)
		}
	}
}
