package assettracker

import (
	"fmt"
	"time"

	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/journal"
)

// AssetState represents the lifecycle state of an asset
type AssetState int

const (
	// StateUntracked is the state of an asset the tracker has never seen.
	StateUntracked AssetState = -1
	StatePending   AssetState = iota // Asset found, entering pipeline
	StateProcessed                   // Asset successfully handled
	StateDiscarded                   // Asset rejected/skipped
	StateError                       // Asset failed to process
)

// String returns the string representation of the asset state
func (s AssetState) String() string {
	switch s {
	case StateUntracked:
		return "UNTRACKED"
	case StatePending:
		return "PENDING"
	case StateProcessed:
		return "PROCESSED"
	case StateDiscarded:
		return "DISCARDED"
	case StateError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// TransitionError is returned when an illegal asset lifecycle transition is attempted.
type TransitionError struct {
	File string       // Asset identifier (file.FullName())
	From AssetState   // State the asset is currently in (StateUntracked if never discovered)
	To   AssetState   // State the transition attempted to reach
	Code journal.Code // Event code used for the transition
}

// Error implements the error interface.
func (e *TransitionError) Error() string {
	return fmt.Sprintf("illegal asset transition %s -> %s for %q (code %q)", e.From, e.To, e.File, e.Code)
}

// AssetRecord tracks an individual asset through its lifecycle
type AssetRecord struct {
	File         fshelper.Filename // File identity
	FileSize     int64             // File size in bytes
	State        AssetState        // Current state
	EventCode    journal.Code      // Most recent event code
	Reason       string            // Why discarded/errored
	EventHistory []EventRecord     // Complete timeline (only in debug mode)
	DiscoveredAt time.Time         // When asset was discovered
	FinalizedAt  time.Time         // When asset reached final state
}

// EventRecord represents a single event in an asset's history
type EventRecord struct {
	Code      journal.Code   // Event code
	Timestamp time.Time      // When event occurred
	Message   string         // Event message
	Args      map[string]any // Additional arguments
}

// AssetCounters provides summary statistics for tracked assets
type AssetCounters struct {
	// Asset counts (images/videos tracked through lifecycle)
	Pending   int64 // Assets not yet finalized
	Processed int64 // Assets successfully handled
	Discarded int64 // Assets skipped (immediate or during processing)
	Errors    int64 // Assets that failed

	// InvalidTransitions counts illegal lifecycle transitions that were attempted
	// and rejected by the tracker. It must be zero at the end of a clean run.
	InvalidTransitions int64

	// Asset size tracking
	AssetSize     int64 // Total asset bytes (all states)
	ProcessedSize int64 // Processed asset bytes
	DiscardedSize int64 // Discarded asset bytes
	ErrorSize     int64 // Errored asset bytes
	PendingSize   int64 // Bytes pending
}

// Total returns the total number of assets tracked
func (ac *AssetCounters) Total() int64 {
	return ac.Pending + ac.Processed + ac.Discarded + ac.Errors
}

// IsComplete returns true if all assets have reached a final state
func (ac *AssetCounters) IsComplete() bool {
	return ac.Pending == 0
}

// RunFinishedCleanly reports whether a completed run left every asset in a
// final state and attempted no illegal transition. Tests use it as a single
// invariant check.
func (ac *AssetCounters) RunFinishedCleanly() bool {
	return ac.Pending == 0 && ac.InvalidTransitions == 0
}
