package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/assets/cache"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/groups"
	"github.com/sweepies/immich-go/internal/journal"
	"github.com/sweepies/immich-go/internal/jsonoutput"
	"golang.org/x/sync/errgroup"
)

// RunnerConfig holds configuration for the pipeline runner.
type RunnerConfig struct {
	Source       Source
	PipelineCtx  *Context
	Groupers     []groups.Grouper
	Filters      []filters.Filter
	PauseJobs    bool
	NoResumeJobs bool
	OnError      func(err error) error
	SaveAlbum    func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error)
	SaveTag      func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error)
}

// Runner orchestrates the upload pipeline execution.
type Runner struct {
	config      RunnerConfig
	pctx        *Context
	albumsCache *cache.CollectionCache[assets.Album]
	tagsCache   *cache.CollectionCache[assets.Tag]
	pauser      *jobPauser
}

// NewRunner creates a new pipeline runner.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{
		config: cfg,
	}
}

// Run executes the upload pipeline.
func (r *Runner) Run(ctx context.Context) (runErr error) {
	startTime := time.Now()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Use the provided pipeline context
	r.pctx = r.config.PipelineCtx
	r.pctx.StartTime = startTime

	// Pause jobs if requested. This must happen before the caches are created:
	// a pause failure returns before the deferred finish and must not leak
	// the caches' goroutines.
	if r.config.PauseJobs {
		r.pauser = &jobPauser{jobs: r.pctx.Jobs, logger: r.pctx.Logger}
		if err := r.pauser.Pause(ctx); err != nil {
			return fmt.Errorf("can't pause immich background jobs: pass an administrator key with the flag --admin-api-key or disable the jobs pausing with the flag --pause-immich-jobs=FALSE\n%w", err)
		}
	}

	// Initialize caches
	r.albumsCache = cache.NewCollectionCache(50, r.config.SaveAlbum)
	r.tagsCache = cache.NewCollectionCache(50, r.config.SaveTag)

	// Deferred in reverse order of execution: finish flushes the caches and
	// resumes the paused queues, then the report is generated so it reflects
	// the run's final error and counters.
	defer func() {
		r.generateReport(startTime, runErr)
	}()
	defer r.finish()

	// Run the pipeline
	err := r.runPipeline(ctx)

	// Finalize the asset lifecycle only after a run that completed without
	// error. On cancellation or abort, pending assets are expected and
	// reported as pending rather than turned into fake errors.
	if err == nil {
		if cause := context.Cause(ctx); cause != nil {
			// The context was cancelled between the end of the pipeline and
			// this check: report the interruption, not a success.
			err = cause
		} else if ferr := r.pctx.Processor.Finalize(ctx); ferr != nil {
			err = ferr
		}
	}
	return err
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
			"upload_errors", counts[journal.ErrorServerError],
			"uploaded", counts[journal.ProcessedUploadSuccess],
		)
	}

	outputJSONProgress := func() {
		counts := r.pctx.Processor.Logger().GetCounts()
		immichPct := getImmichPct()

		if err := jsonoutput.WriteProgress(
			immichPct,
			r.pctx.Processor.Logger().TotalAssets(),
			counts[journal.ErrorServerError],
			counts[journal.ProcessedUploadSuccess],
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
				// Reporting progress never fails the run: contributing
				// ctx.Err() here could mask the real pipeline error.
				return nil
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
			err := albumStage.Run(ctx, r.pctx)
			if err != nil {
				cancel(err)
			}
			return err
		})

		err := processGrp.Wait()
		if err != nil {
			// errgroup returns the first error a goroutine reported, which
			// may be a secondary context.Canceled; the cancel cause is the
			// failure that actually stopped the pipeline.
			if cause := context.Cause(ctx); cause != nil {
				return cause
			}
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
			if cause := context.Cause(ctx); cause != nil {
				return cause
			}
		}

		return err
	})

	return uiGrp.Wait()
}

// cacheFlushTimeout bounds each final album/tag flush so it survives a
// cancelled run context but cannot hang forever on a stopped server. It is a
// variable so tests can shorten it.
var cacheFlushTimeout = time.Minute

// finish is deferred once from Run. It flushes the album/tag caches and
// resumes the job queues this run paused. Each flush gets its own context
// bounded by a timeout: the run context is already cancelled when the user
// interrupts the upload, the queued memberships must still be delivered, and
// a slow album flush must not consume the tag flush's budget.
func (r *Runner) finish() {
	if r.albumsCache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), cacheFlushTimeout)
		r.albumsCache.Close(ctx)
		cancel()
	}
	if r.tagsCache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), cacheFlushTimeout)
		r.tagsCache.Close(ctx)
		cancel()
	}

	// Resume jobs if they were paused
	if r.pauser != nil && !r.config.NoResumeJobs {
		r.pauser.Resume()
	}
}

func (r *Runner) generateReport(startTime time.Time, runErr error) {
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
		if runErr != nil || counters.Errors > 0 {
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
