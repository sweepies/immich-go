package jsonoutput

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
)

// ProgressUpdate represents a single progress update during processing
type ProgressUpdate struct {
	Type           string    `json:"type"`
	Timestamp      time.Time `json:"timestamp"`
	ImmichReadPct  int       `json:"immich_read_pct"`
	AssetsFound    int64     `json:"assets_found"`
	UploadErrors   int64     `json:"upload_errors"`
	Uploaded       int64     `json:"uploaded"`
}

// EventSummary represents statistics for a specific event type
type EventSummary struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"`
}

// FinalSummary represents the complete summary at the end of processing
type FinalSummary struct {
	Type           string                     `json:"type"`
	Status         string                     `json:"status"`
	ExitCode       int                        `json:"exit_code"`
	Counters       assettracker.AssetCounters `json:"counters"`
	Events         map[string]EventSummary    `json:"events"`
	DurationSeconds float64                   `json:"duration_seconds"`
	Timestamp      time.Time                  `json:"timestamp"`
}

// WriteProgress writes a progress update to stdout as a JSON line
func WriteProgress(immichPct int, totalAssets, uploadErrors, uploaded int64) error {
	progress := ProgressUpdate{
		Type:          "progress",
		Timestamp:     time.Now(),
		ImmichReadPct: immichPct,
		AssetsFound:   totalAssets,
		UploadErrors:  uploadErrors,
		Uploaded:      uploaded,
	}
	return writeJSON(progress)
}

// WriteSummary writes the final summary to stdout as a JSON line
func WriteSummary(
	status string,
	exitCode int,
	counters assettracker.AssetCounters,
	eventCounts map[fileevent.Code]int64,
	eventSizes map[fileevent.Code]int64,
	duration float64,
) error {
	// Convert event codes to human-readable names
	events := make(map[string]EventSummary)
	for code, count := range eventCounts {
		eventName := code.String()
		events[eventName] = EventSummary{
			Count: count,
			Size:  eventSizes[code],
		}
	}

	summary := FinalSummary{
		Type:            "summary",
		Status:          status,
		ExitCode:        exitCode,
		Counters:        counters,
		Events:          events,
		DurationSeconds: duration,
		Timestamp:       time.Now(),
	}
	return writeJSON(summary)
}

// writeJSON marshals data and writes it to stdout as a JSON line
func writeJSON(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	// Append newline and write directly to avoid string conversion
	jsonData = append(jsonData, '\n')
	_, err = os.Stdout.Write(jsonData)
	return err
}
