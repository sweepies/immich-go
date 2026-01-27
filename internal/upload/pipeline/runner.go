package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/assets/cache"
	"github.com/sweepies/immich-go/internal/fileevent"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/groups"
	"github.com/sweepies/immich-go/internal/jsonoutput"
	"golang.org/x/sync/errgroup"
)

// RunnerConfig holds configuration for the pipeline runner.
type RunnerConfig struct {
	Source       Source
	Server       ServerClient
	PipelineCtx  *Context
	Groupers     []groups.Grouper
	Filters      []filters.Filter
	PauseJobs    bool
	NoResumeJobs bool
	OnError      func(err error) error
	SaveAlbum    func(album assets.Album, ids []string) (assets.Album, error)
	SaveTag      func(tag assets.Tag, ids []string) (assets.Tag, error)
}

// Runner orchestrates the upload pipeline execution.
type Runner struct {
	config      RunnerConfig
	pctx        *Context
	albumsCache *cache.CollectionCache[assets.Album]
	tagsCache   *cache.CollectionCache[assets.Tag]
	finished    bool
	finishMu    sync.Mutex
}

// NewRunner creates a new pipeline runner.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{
		config: cfg,
	}
}

// Run executes the upload pipeline.
func (r *Runner) Run(ctx context.Context) error {
	startTime := time.Now()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Use the provided pipeline context
	r.pctx = r.config.PipelineCtx
	r.pctx.StartTime = startTime

	// Initialize caches
	r.albumsCache = cache.NewCollectionCache(50, r.config.SaveAlbum)
	r.tagsCache = cache.NewCollectionCache(50, r.config.SaveTag)

	// Pause jobs if requested
	if r.config.PauseJobs {
		pauseStage := &JobControlStage{Pause: true}
		if err := pauseStage.Run(ctx, r.pctx); err != nil {
			return fmt.Errorf("can't pause immich background jobs: pass an administrator key with the flag --admin-api-key or disable the jobs pausing with the flag --pause-immich-jobs=FALSE\n%w", err)
		}
	}
	defer func() { _ = r.finish(ctx) }()

	// Report generation deferred
	defer func() {
		r.generateReport(startTime)
	}()

	// Run the pipeline
	return r.runPipeline(ctx)
}

func (r *Runner) runPipeline(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	stopProgress := make(chan any)
	var maxImmich, currImmich int
	var lock sync.RWMutex

	immichUpdate := func(value, total int) {
		lock.Lock()
		currImmich, maxImmich = value, total
		lock.Unlock()
	}

	getImmichPct := func() int {
		lock.Lock()
		defer lock.Unlock()
		if maxImmich > 0 {
			return 100 * currImmich / maxImmich
		}
		return 100
	}

	isJSONMode := r.pctx.Config.OutputFormat == "json"

	logProgress := func() {
		counts := r.pctx.Processor.Logger().GetCounts()
		immichPct := getImmichPct()
		r.pctx.Logger.Info("progress",
			"immich_read_pct", immichPct,
			"assets_found", r.pctx.Processor.Logger().TotalAssets(),
			"upload_errors", counts[fileevent.ErrorServerError],
			"uploaded", counts[fileevent.ProcessedUploadSuccess],
		)
	}

	outputJSONProgress := func() {
		counts := r.pctx.Processor.Logger().GetCounts()
		immichPct := getImmichPct()

		if err := jsonoutput.WriteProgress(
			immichPct,
			r.pctx.Processor.Logger().TotalAssets(),
			counts[fileevent.ErrorServerError],
			counts[fileevent.ProcessedUploadSuccess],
		); err != nil {
			r.pctx.Logger.Error("failed to write JSON progress", "err", err)
		}
	}

	uiGrp := errgroup.Group{}

	// Progress reporting goroutine
	uiGrp.Go(func() error {
		tickInterval := 5 * time.Second
		if isJSONMode {
			tickInterval = 500 * time.Millisecond
		}
		ticker := time.NewTicker(tickInterval)
		defer func() {
			ticker.Stop()
			if isJSONMode {
				outputJSONProgress()
			} else {
				logProgress()
			}
		}()
		for {
			select {
			case <-stopProgress:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if isJSONMode {
					outputJSONProgress()
				} else {
					logProgress()
				}
			}
		}
	})

	// Main processing goroutine
	uiGrp.Go(func() error {
		defer close(stopProgress)

		assetsReady := make(chan struct{})

		processGrp := errgroup.Group{}

		// Discovery stage
		processGrp.Go(func() error {
			defer close(assetsReady)
			discoveryStage := &DiscoveryStage{
				ProgressUpdate: immichUpdate,
			}
			err := discoveryStage.Run(ctx, r.pctx)
			if err != nil {
				cancel(err)
			}
			return err
		})

		// Album discovery stage
		processGrp.Go(func() error {
			albumStage := &AlbumDiscoveryStage{
				AlbumsCache: r.albumsCache,
				AssetsReady: assetsReady,
			}
			return albumStage.Run(ctx, r.pctx)
		})

		err := processGrp.Wait()
		if err != nil {
			return err
		}

		// Upload stage
		uploadStage := &UploadStage{
			Source:      r.config.Source,
			AlbumsCache: r.albumsCache,
			TagsCache:   r.tagsCache,
			Groupers:    r.config.Groupers,
			Filters:     r.config.Filters,
			Tags:        r.pctx.Config.Tags,
			SessionTag:  r.pctx.SessionTagValue,
			Overwrite:   r.pctx.Config.Overwrite,
			Concurrency: r.pctx.Config.ConcurrentTask,
			OnError:     r.config.OnError,
		}

		err = uploadStage.Run(ctx, r.pctx)
		if err != nil {
			cancel(err)
		}

		return err
	})

	return uiGrp.Wait()
}

func (r *Runner) finish(ctx context.Context) error {
	r.finishMu.Lock()
	defer r.finishMu.Unlock()

	if r.finished {
		return nil
	}
	r.finished = true

	// Close caches
	if r.albumsCache != nil {
		r.albumsCache.Close()
	}
	if r.tagsCache != nil {
		r.tagsCache.Close()
	}

	// Resume jobs if they were paused
	if r.config.PauseJobs && !r.config.NoResumeJobs {
		resumeStage := &JobControlStage{Pause: false}
		return resumeStage.Run(ctx, r.pctx)
	}

	return nil
}

func (r *Runner) generateReport(startTime time.Time) {
	if r.pctx == nil || r.pctx.Processor == nil {
		return
	}

	if r.pctx.Config.OutputFormat == "json" {
		duration := time.Since(startTime).Seconds()
		counters := r.pctx.Processor.GetAssetCounters()
		eventCounts := r.pctx.Processor.GetEventCounts()
		eventSizes := r.pctx.Processor.GetEventSizes()

		status := "success"
		exitCode := 0
		if counters.Errors > 0 {
			status = "error"
			exitCode = 1
		}

		if summaryErr := jsonoutput.WriteSummary(status, exitCode, counters, eventCounts, eventSizes, duration); summaryErr != nil {
			r.pctx.Logger.Error("failed to write JSON summary", "err", summaryErr)
		}
	} else {
		fmt.Println(r.pctx.Processor.GenerateReport())
	}
}
