// Package reporting provides report generation and JSON output for upload pipelines.
// It consolidates the report generation logic from the upload orchestration.
package reporting

import (
	"fmt"
	"time"

	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/journal"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/jsonoutput"
)

// Reporter handles generating reports and summaries for upload operations.
type Reporter struct {
	processor *fileprocessor.FileProcessor
	startTime time.Time
	output    string // "text" or "json"
}

// New creates a new Reporter.
func New(processor *fileprocessor.FileProcessor, output string) *Reporter {
	return &Reporter{
		processor: processor,
		startTime: time.Now(),
		output:    output,
	}
}

// SetStartTime sets the start time for duration calculation.
func (r *Reporter) SetStartTime(t time.Time) {
	r.startTime = t
}

// IsJSONOutput returns true if output format is JSON.
func (r *Reporter) IsJSONOutput() bool {
	return r.output == "json"
}

// GenerateFinalReport generates the final report based on output format.
// For JSON output, it writes the summary to stdout.
// For text output, it returns the report string.
func (r *Reporter) GenerateFinalReport(err error) string {
	if r.processor == nil {
		return ""
	}

	if r.IsJSONOutput() {
		r.writeJSONSummary(err)
		return ""
	}

	return r.processor.GenerateReport()
}

func (r *Reporter) writeJSONSummary(err error) {
	duration := time.Since(r.startTime).Seconds()
	counters := r.processor.GetAssetCounters()
	eventCounts := r.processor.GetEventCounts()
	eventSizes := r.processor.GetEventSizes()

	status := "success"
	exitCode := 0
	if counters.Errors > 0 || err != nil {
		status = "error"
		exitCode = 1
	}

	if summaryErr := jsonoutput.WriteSummary(status, exitCode, counters, eventCounts, eventSizes, duration); summaryErr != nil {
		fmt.Printf("failed to write JSON summary: %v\n", summaryErr)
	}
}

// GetProgressData returns current progress data for reporting.
func (r *Reporter) GetProgressData() ProgressData {
	if r.processor == nil {
		return ProgressData{}
	}

	counts := r.processor.Logger().GetCounts()
	return ProgressData{
		TotalAssets:  r.processor.Logger().TotalAssets(),
		UploadErrors: counts[journal.ErrorServerError],
		Uploaded:     counts[journal.ProcessedUploadSuccess],
	}
}

// ProgressData holds progress information for reporting.
type ProgressData struct {
	TotalAssets  int64
	UploadErrors int64
	Uploaded     int64
}

// WriteJSONProgress writes progress as JSON output.
func (r *Reporter) WriteJSONProgress(immichPct int) error {
	if r.processor == nil {
		return nil
	}

	counts := r.processor.Logger().GetCounts()
	return jsonoutput.WriteProgress(
		immichPct,
		r.processor.Logger().TotalAssets(),
		counts[journal.ErrorServerError],
		counts[journal.ProcessedUploadSuccess],
	)
}

// GetAssetCounters returns the current asset counters.
func (r *Reporter) GetAssetCounters() assettracker.AssetCounters {
	if r.processor == nil {
		return assettracker.AssetCounters{}
	}
	return r.processor.GetAssetCounters()
}

// GetEventCounts returns the current event counts.
func (r *Reporter) GetEventCounts() map[journal.Code]int64 {
	if r.processor == nil {
		return nil
	}
	return r.processor.GetEventCounts()
}

// HasErrors returns true if any errors were recorded.
func (r *Reporter) HasErrors() bool {
	if r.processor == nil {
		return false
	}
	counts := r.processor.Logger().GetCounts()
	return counts[journal.ErrorUploadFailed]+
		counts[journal.ErrorServerError]+
		counts[journal.ErrorFileAccess]+
		counts[journal.ErrorIncomplete] > 0
}
