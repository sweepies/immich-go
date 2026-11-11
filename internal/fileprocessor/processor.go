package fileprocessor

import (
	"context"
	"errors"
	"fmt"

	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/journal"
)

// FileProcessor coordinates AssetTracker and EventLogger to provide
// a unified interface for tracking file processing lifecycle.
type FileProcessor struct {
	tracker *assettracker.AssetTracker
	logger  *journal.Recorder
}

// New creates a new FileProcessor with the given tracker and logger
func New(tracker *assettracker.AssetTracker, logger *journal.Recorder) *FileProcessor {
	return &FileProcessor{
		tracker: tracker,
		logger:  logger,
	}
}

// Tracker returns the underlying AssetTracker
func (fp *FileProcessor) Tracker() *assettracker.AssetTracker {
	return fp.tracker
}

// Logger returns the underlying EventLogger
func (fp *FileProcessor) Logger() *journal.Recorder {
	return fp.logger
}

// RecordAssetDiscovered records an asset (image/video) entering the pipeline.
// The asset is tracked and the discovery event is logged.
// The journal event is recorded only if the tracker accepts the transition;
// the tracker's error is returned otherwise.
func (fp *FileProcessor) RecordAssetDiscovered(ctx context.Context, file fshelper.Filename, size int64, code journal.Code) error {
	if err := fp.tracker.DiscoverAsset(file, size, code); err != nil {
		return err
	}
	fp.logger.RecordWithSize(ctx, code, file, size)
	return nil
}

// RecordAssetDiscardedImmediately records an asset that is immediately discarded
// upon discovery (e.g., banned filename that's still an image type).
// The asset is tracked in DISCARDED state and the event is logged.
// The journal event is recorded only if the tracker accepts the transition.
func (fp *FileProcessor) RecordAssetDiscardedImmediately(ctx context.Context, file fshelper.Filename, size int64, code journal.Code, reason string) error {
	if err := fp.tracker.DiscoverAndDiscard(file, size, code, reason); err != nil {
		return err
	}
	fp.logger.RecordWithSize(ctx, code, file, size, "reason", reason)
	return nil
}

// RecordAssetErrorImmediately records an asset that immediately errored
// upon discovery (e.g., a file that could not be stat'ed).
// The asset is tracked in ERROR state and the event is logged.
// The journal event is recorded only if the tracker accepts the transition.
func (fp *FileProcessor) RecordAssetErrorImmediately(ctx context.Context, file fshelper.Filename, size int64, code journal.Code, err error) error {
	if err := fp.tracker.DiscoverAndError(file, size, code, err); err != nil {
		return err
	}
	fp.logger.RecordWithSize(ctx, code, file, size, "error", err.Error())
	return nil
}

// RecordAssetProcessed transitions an asset to PROCESSED state.
// The state change is tracked and the event is logged.
// The journal event is recorded only if the tracker accepts the transition.
func (fp *FileProcessor) RecordAssetProcessed(ctx context.Context, file fshelper.Filename, size int64, code journal.Code) error {
	if err := fp.tracker.SetProcessed(file, code); err != nil {
		return err
	}
	fp.logger.RecordWithSize(ctx, code, file, size)
	return nil
}

// RecordAssetDiscarded transitions an asset to DISCARDED state.
// The state change is tracked and the event is logged with the reason.
// The journal event is recorded only if the tracker accepts the transition.
func (fp *FileProcessor) RecordAssetDiscarded(ctx context.Context, file fshelper.Filename, size int64, code journal.Code, reason string) error {
	if err := fp.tracker.SetDiscarded(file, code, reason); err != nil {
		return err
	}
	fp.logger.RecordWithSize(ctx, code, file, size, "reason", reason)
	return nil
}

// RecordAssetError transitions an asset to ERROR state.
// The state change is tracked and the error event is logged.
// The journal event is recorded only if the tracker accepts the transition.
func (fp *FileProcessor) RecordAssetError(ctx context.Context, file fshelper.Filename, size int64, code journal.Code, err error) error {
	if err := fp.tracker.SetError(file, code, err); err != nil {
		return err
	}
	fp.logger.RecordWithSize(ctx, code, file, size, "error", err.Error())
	return nil
}

// RecordNonAsset records a non-asset file (sidecar, metadata, etc.).
// Only logged, not tracked in AssetTracker.
// The event code must be in CategoryNonAsset, CategoryInfo or CategoryError
// (errors on sidecars and other non-asset files are legitimate). Any other
// category is a programming error: it is logged, counted as an invalid
// transition, not recorded, and returned as an error.
func (fp *FileProcessor) RecordNonAsset(ctx context.Context, file fshelper.Filename, size int64, code journal.Code, args ...any) error {
	switch code.Category() {
	case journal.CategoryNonAsset, journal.CategoryInfo, journal.CategoryError:
		fp.logger.RecordWithSize(ctx, code, file, size, args...)
		return nil
	default:
		fp.tracker.RecordInvalidTransition()
		err := fmt.Errorf("RecordNonAsset called with asset lifecycle code %q (file %s)", code, file.FullName())
		if l := fp.logger.Log(); l != nil {
			l.Error("asset lifecycle code passed to RecordNonAsset", "file", file.FullName(), "code", code)
		}
		return err
	}
}

// Finalize moves every asset still PENDING to the ERROR state with code
// ErrorIncomplete, through the normal transition path so journal and counters
// agree. It returns an error naming the count of incomplete assets, nil when
// there are none. After Finalize, Pending is always zero.
func (fp *FileProcessor) Finalize(ctx context.Context) error {
	pending := fp.tracker.GetPending()
	for _, asset := range pending {
		_ = fp.RecordAssetError(ctx, asset.File, asset.FileSize, journal.ErrorIncomplete,
			errors.New("asset never reached final state"))
	}
	if len(pending) > 0 {
		return fmt.Errorf("%d assets never reached final state", len(pending))
	}
	return nil
}

// GenerateReport generates a comprehensive report combining asset tracking
// and event logging information.
func (fp *FileProcessor) GenerateReport() string {
	report := ""

	// Asset tracking report
	assetReport := fp.tracker.GenerateReport()
	if assetReport != "" {
		report += assetReport
	}

	// Event logging report
	eventReport := fp.logger.GenerateEventReport()
	if eventReport != "" {
		report += "\n" + eventReport
	}

	return report
}

// GetAssetCounters returns current asset counters from the tracker
func (fp *FileProcessor) GetAssetCounters() assettracker.AssetCounters {
	return fp.tracker.GetCounters()
}

// GetEventCounts returns event counts from the logger
func (fp *FileProcessor) GetEventCounts() map[journal.Code]int64 {
	return fp.logger.GetEventCounts()
}

// GetEventSizes returns event sizes from the logger
func (fp *FileProcessor) GetEventSizes() map[journal.Code]int64 {
	return fp.logger.GetEventSizes()
}

// IsComplete returns true if all assets have reached a final state
func (fp *FileProcessor) IsComplete() bool {
	return fp.tracker.IsComplete()
}

// GetPendingAssets returns all assets that haven't reached a final state
func (fp *FileProcessor) GetPendingAssets() []assettracker.AssetRecord {
	return fp.tracker.GetPending()
}

// GenerateDetailedReport creates a detailed CSV report of all assets (debug mode)
func (fp *FileProcessor) GenerateDetailedReport(ctx context.Context) string {
	return fp.tracker.GenerateDetailedReport(ctx)
}

// Summary provides a quick overview of processing status
func (fp *FileProcessor) Summary() string {
	counters := fp.tracker.GetCounters()
	return fmt.Sprintf("Assets: %d processed, %d discarded, %d errors, %d pending",
		counters.Processed, counters.Discarded, counters.Errors, counters.Pending)
}
