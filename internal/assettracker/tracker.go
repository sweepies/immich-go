package assettracker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/journal"
)

// AssetTracker tracks the complete lifecycle of assets (images/videos) from
// discovery through final state (processed/discarded/error).
// Non-asset files are ignored by this tracker.
type AssetTracker struct {
	// Per-asset tracking (ONLY images/videos)
	assets map[string]*AssetRecord // key: file.FullName()
	mu     sync.RWMutex

	// Asset counters (derived from tracked assets)
	pending   int64 // Assets in PENDING state
	processed int64 // Assets in PROCESSED state
	discarded int64 // Assets in DISCARDED state (includes immediate discards)
	errors    int64 // Assets in ERROR state

	// invalidTransitions counts illegal transitions that were attempted and rejected
	invalidTransitions int64

	// Size tracking (assets only)
	assetSize     int64 // Total bytes of assets (all states)
	processedSize int64 // Bytes of processed assets
	discardedSize int64 // Bytes of discarded assets
	errorSize     int64 // Bytes of errored assets

	// Debug mode
	debugMode bool
	log       *slog.Logger
}

// New creates a new AssetTracker
func New() *AssetTracker {
	return &AssetTracker{
		assets:    make(map[string]*AssetRecord),
		debugMode: false,
	}
}

// NewWithLogger creates a new AssetTracker with debug logging
func NewWithLogger(log *slog.Logger, debugMode bool) *AssetTracker {
	return &AssetTracker{
		assets:    make(map[string]*AssetRecord),
		debugMode: debugMode,
		log:       log,
	}
}

// expectedCategory maps a target state to the journal category its event code
// must belong to.
func expectedCategory(to AssetState) journal.Category {
	switch to {
	case StatePending:
		return journal.CategoryAssetDiscovery
	case StateProcessed:
		return journal.CategoryProcessed
	case StateDiscarded:
		return journal.CategoryDiscarded
	case StateError:
		return journal.CategoryError
	default:
		return journal.CategoryNone
	}
}

// legalTransition reports whether moving from one state to another is allowed.
//
//	untracked -> PENDING (discovery), DISCARDED or ERROR (immediate outcomes)
//	PENDING   -> PROCESSED, DISCARDED or ERROR (terminal states)
//
// Everything else is illegal: re-discovering a tracked asset, finalising an
// untracked one, or finalising twice.
func legalTransition(from, to AssetState) bool {
	switch from {
	case StateUntracked:
		return to == StatePending || to == StateDiscarded || to == StateError
	case StatePending:
		return to == StateProcessed || to == StateDiscarded || to == StateError
	default:
		return false
	}
}

// transitionMessage describes the transition for the debug event history.
func transitionMessage(from, to AssetState) string {
	if from == StateUntracked {
		switch to {
		case StatePending:
			return "Asset discovered"
		case StateDiscarded:
			return "Asset discovered and immediately discarded"
		case StateError:
			return "Asset discovered and immediately errored"
		}
	}
	switch to {
	case StateProcessed:
		return "Asset processed"
	case StateDiscarded:
		return "Asset discarded"
	case StateError:
		return "Asset error"
	}
	return "Asset state changed"
}

// transition is the single place where an asset's state changes. It checks the
// move is legal, updates the record and the counters, and returns a
// *TransitionError for illegal attempts.
func (at *AssetTracker) transition(file fshelper.Filename, fileSize int64, to AssetState, code journal.Code, reason string) error {
	at.mu.Lock()
	defer at.mu.Unlock()

	key := file.FullName()
	record, tracked := at.assets[key]
	from := StateUntracked
	if tracked {
		from = record.State
	}

	if !legalTransition(from, to) || code.Category() != expectedCategory(to) {
		at.invalidTransitions++
		if at.log != nil {
			at.log.Error("illegal asset transition", "file", key, "from", from, "to", to, "code", code, "reason", reason)
		}
		return &TransitionError{File: key, From: from, To: to, Code: code}
	}

	now := time.Now()
	if !tracked {
		record = &AssetRecord{
			File:         file,
			FileSize:     fileSize,
			DiscoveredAt: now,
		}
		at.assets[key] = record
		at.assetSize += fileSize
	}

	record.State = to
	record.EventCode = code
	if to != StatePending {
		record.Reason = reason
		record.FinalizedAt = now
	}

	if at.debugMode {
		var args map[string]any
		switch to {
		case StateDiscarded:
			args = map[string]any{"reason": reason}
		case StateError:
			args = map[string]any{"error": reason}
		}
		record.EventHistory = append(record.EventHistory, EventRecord{
			Code:      code,
			Timestamp: now,
			Message:   transitionMessage(from, to),
			Args:      args,
		})
	}

	switch to {
	case StatePending:
		at.pending++
	case StateProcessed:
		at.pending--
		at.processed++
		at.processedSize += record.FileSize
	case StateDiscarded:
		if tracked {
			at.pending--
		}
		at.discarded++
		at.discardedSize += record.FileSize
	case StateError:
		if tracked {
			at.pending--
		}
		at.errors++
		at.errorSize += record.FileSize
	}
	return nil
}

// DiscoverAsset registers an asset (image/video) entering the pipeline.
// The event code must be in the CategoryAssetDiscovery category.
func (at *AssetTracker) DiscoverAsset(file fshelper.Filename, fileSize int64, eventCode journal.Code) error {
	return at.transition(file, fileSize, StatePending, eventCode, "")
}

// DiscoverAndDiscard registers an asset that is immediately discarded
// (e.g., banned filename that's still an image type).
// The event code must be in the CategoryDiscarded category.
func (at *AssetTracker) DiscoverAndDiscard(file fshelper.Filename, fileSize int64, eventCode journal.Code, reason string) error {
	return at.transition(file, fileSize, StateDiscarded, eventCode, reason)
}

// DiscoverAndError registers an asset that immediately errored
// (e.g., a file that could not be stat'ed).
// The event code must be in the CategoryError category.
func (at *AssetTracker) DiscoverAndError(file fshelper.Filename, fileSize int64, eventCode journal.Code, err error) error {
	return at.transition(file, fileSize, StateError, eventCode, err.Error())
}

// SetProcessed transitions an asset to the PROCESSED state.
// The event code must be in the CategoryProcessed category.
func (at *AssetTracker) SetProcessed(file fshelper.Filename, eventCode journal.Code) error {
	return at.transition(file, 0, StateProcessed, eventCode, "")
}

// SetDiscarded transitions an asset to the DISCARDED state.
// The event code must be in the CategoryDiscarded category.
func (at *AssetTracker) SetDiscarded(file fshelper.Filename, eventCode journal.Code, reason string) error {
	return at.transition(file, 0, StateDiscarded, eventCode, reason)
}

// SetError transitions an asset to the ERROR state.
// The event code must be in the CategoryError category.
func (at *AssetTracker) SetError(file fshelper.Filename, eventCode journal.Code, err error) error {
	return at.transition(file, 0, StateError, eventCode, err.Error())
}

// RecordInvalidTransition counts an illegal lifecycle transition attempted
// outside the tracker (e.g., a lifecycle event code passed to RecordNonAsset).
func (at *AssetTracker) RecordInvalidTransition() {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.invalidTransitions++
}

// GetCounters returns current asset counters
func (at *AssetTracker) GetCounters() AssetCounters {
	at.mu.RLock()
	defer at.mu.RUnlock()

	pendingSize := at.assetSize - at.processedSize - at.discardedSize - at.errorSize

	return AssetCounters{
		Pending:            at.pending,
		Processed:          at.processed,
		Discarded:          at.discarded,
		Errors:             at.errors,
		InvalidTransitions: at.invalidTransitions,
		AssetSize:          at.assetSize,
		ProcessedSize:      at.processedSize,
		DiscardedSize:      at.discardedSize,
		ErrorSize:          at.errorSize,
		PendingSize:        pendingSize,
	}
}

// GetPending returns all assets that haven't reached a final state
func (at *AssetTracker) GetPending() []AssetRecord {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var pending []AssetRecord
	for _, record := range at.assets {
		if record.State == StatePending {
			pending = append(pending, *record)
		}
	}
	return pending
}

// GetAllAssets returns all tracked assets
func (at *AssetTracker) GetAllAssets() []AssetRecord {
	at.mu.RLock()
	defer at.mu.RUnlock()

	assets := make([]AssetRecord, 0, len(at.assets))
	for _, record := range at.assets {
		assets = append(assets, *record)
	}
	return assets
}

// IsComplete returns true if all assets have reached a final state
func (at *AssetTracker) IsComplete() bool {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.pending == 0
}

// GenerateReport creates a summary report of asset processing
func (at *AssetTracker) GenerateReport() string {
	counters := at.GetCounters()

	report := "\nAsset Tracking Report:\n"
	report += "=====================\n"
	report += fmt.Sprintf("Total Assets:    %7d  (%s)\n", counters.Total(), formatBytes(counters.AssetSize))
	report += fmt.Sprintf("  Processed:     %7d  (%s)\n", counters.Processed, formatBytes(counters.ProcessedSize))
	report += fmt.Sprintf("  Discarded:     %7d  (%s)\n", counters.Discarded, formatBytes(counters.DiscardedSize))
	report += fmt.Sprintf("  Errors:        %7d  (%s)\n", counters.Errors, formatBytes(counters.ErrorSize))
	report += fmt.Sprintf("  Pending:       %7d  (%s)\n", counters.Pending, formatBytes(counters.PendingSize))

	if !counters.IsComplete() {
		report += fmt.Sprintf("\n⚠️  WARNING: %d assets did not reach a final state!\n", counters.Pending)
	}
	if counters.InvalidTransitions > 0 {
		report += fmt.Sprintf("\n⚠️  WARNING: %d inconsistent asset events were ignored (see the log); counts may be incomplete.\n", counters.InvalidTransitions)
	}

	return report
}

// GenerateDetailedReport creates a detailed CSV report of all assets (debug mode)
func (at *AssetTracker) GenerateDetailedReport(ctx context.Context) string {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("FilePath,Size,State,EventCode,Reason,DiscoveredAt,FinalizedAt\n")

	for _, record := range at.assets {
		finalizedAt := ""
		if !record.FinalizedAt.IsZero() {
			finalizedAt = record.FinalizedAt.Format(time.RFC3339)
		}

		fmt.Fprintf(&sb, "%s,%d,%s,%s,%q,%s,%s\n",
			record.File.FullName(),
			record.FileSize,
			record.State,
			record.EventCode,
			record.Reason,
			record.DiscoveredAt.Format(time.RFC3339),
			finalizedAt,
		)
	}

	return sb.String()
}

// GetPendingCount returns the count of pending assets
func (at *AssetTracker) GetPendingCount() int {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return int(at.pending)
}

// GetPendingSize returns the total size of pending assets
func (at *AssetTracker) GetPendingSize() int64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.assetSize - at.processedSize - at.discardedSize - at.errorSize
}

// GetProcessedCount returns the count of processed assets
func (at *AssetTracker) GetProcessedCount() int {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return int(at.processed)
}

// GetProcessedSize returns the total size of processed assets
func (at *AssetTracker) GetProcessedSize() int64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.processedSize
}

// GetDiscardedCount returns the count of discarded assets
func (at *AssetTracker) GetDiscardedCount() int {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return int(at.discarded)
}

// GetDiscardedSize returns the total size of discarded assets
func (at *AssetTracker) GetDiscardedSize() int64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.discardedSize
}

// GetErrorCount returns the count of errored assets
func (at *AssetTracker) GetErrorCount() int {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return int(at.errors)
}

// GetErrorSize returns the total size of errored assets
func (at *AssetTracker) GetErrorSize() int64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.errorSize
}

// formatBytes formats byte count as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
