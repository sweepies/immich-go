package assettracker

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/sweepies/immich-go/internal/journal"
)

// mockFS implements a simple fs.FS for testing
type mockFS struct{}

func (m mockFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (m mockFS) Name() string {
	return "test.zip"
}

func TestNew(t *testing.T) {
	tracker := New()
	if tracker == nil {
		t.Fatal("New() returned nil")
	}
	if tracker.assets == nil {
		t.Error("assets map not initialized")
	}
	if tracker.IsComplete() != true {
		t.Error("new tracker should be complete (no assets)")
	}
}

func TestDiscoverAsset(t *testing.T) {
	tracker := New()
	file := journal.NewFilename(mockFS{}, "test.jpg")

	tracker.DiscoverAsset(file, 1024, journal.DiscoveredImage)

	counters := tracker.GetCounters()
	if counters.Pending != 1 {
		t.Errorf("expected 1 pending asset, got %d", counters.Pending)
	}
	if counters.AssetSize != 1024 {
		t.Errorf("expected asset size 1024, got %d", counters.AssetSize)
	}
	if counters.Total() != 1 {
		t.Errorf("expected total 1, got %d", counters.Total())
	}
	if tracker.IsComplete() {
		t.Error("tracker should not be complete with pending assets")
	}
}

func TestDiscoverAndDiscard(t *testing.T) {
	tracker := New()
	file := journal.NewFilename(mockFS{}, "banned.jpg")

	tracker.DiscoverAndDiscard(file, 2048, journal.DiscardedBanned, "banned filename")

	counters := tracker.GetCounters()
	if counters.Discarded != 1 {
		t.Errorf("expected 1 discarded asset, got %d", counters.Discarded)
	}
	if counters.DiscardedSize != 2048 {
		t.Errorf("expected discarded size 2048, got %d", counters.DiscardedSize)
	}
	if counters.Pending != 0 {
		t.Errorf("expected 0 pending assets, got %d", counters.Pending)
	}
	if !tracker.IsComplete() {
		t.Error("tracker should be complete (asset immediately discarded)")
	}
}

func TestSetProcessed(t *testing.T) {
	tracker := New()
	file := journal.NewFilename(mockFS{}, "photo.jpg")

	// Discover asset
	tracker.DiscoverAsset(file, 1024, journal.DiscoveredImage)

	// Process it
	tracker.SetProcessed(file, journal.ProcessedUploadSuccess)

	counters := tracker.GetCounters()
	if counters.Processed != 1 {
		t.Errorf("expected 1 processed asset, got %d", counters.Processed)
	}
	if counters.Pending != 0 {
		t.Errorf("expected 0 pending assets, got %d", counters.Pending)
	}
	if counters.ProcessedSize != 1024 {
		t.Errorf("expected processed size 1024, got %d", counters.ProcessedSize)
	}
	if !tracker.IsComplete() {
		t.Error("tracker should be complete")
	}
}

func TestSetDiscarded(t *testing.T) {
	tracker := New()
	file := journal.NewFilename(mockFS{}, "duplicate.jpg")

	// Discover asset
	tracker.DiscoverAsset(file, 512, journal.DiscoveredImage)

	// Discard it
	tracker.SetDiscarded(file, journal.DiscardedLocalDuplicate, "duplicate in input")

	counters := tracker.GetCounters()
	if counters.Discarded != 1 {
		t.Errorf("expected 1 discarded asset, got %d", counters.Discarded)
	}
	if counters.Pending != 0 {
		t.Errorf("expected 0 pending assets, got %d", counters.Pending)
	}
	if counters.DiscardedSize != 512 {
		t.Errorf("expected discarded size 512, got %d", counters.DiscardedSize)
	}
}

func TestSetError(t *testing.T) {
	tracker := New()
	file := journal.NewFilename(mockFS{}, "failed.jpg")

	// Discover asset
	tracker.DiscoverAsset(file, 2048, journal.DiscoveredImage)

	// Error it
	tracker.SetError(file, journal.ErrorUploadFailed, fs.ErrPermission)

	counters := tracker.GetCounters()
	if counters.Errors != 1 {
		t.Errorf("expected 1 error asset, got %d", counters.Errors)
	}
	if counters.Pending != 0 {
		t.Errorf("expected 0 pending assets, got %d", counters.Pending)
	}
	if counters.ErrorSize != 2048 {
		t.Errorf("expected error size 2048, got %d", counters.ErrorSize)
	}
}

func TestMultipleAssets(t *testing.T) {
	tracker := New()

	// Discover multiple assets
	files := []struct {
		name string
		size int64
	}{
		{"photo1.jpg", 1024},
		{"photo2.jpg", 2048},
		{"photo3.jpg", 4096},
		{"video1.mp4", 8192},
	}

	for _, f := range files {
		file := journal.NewFilename(mockFS{}, f.name)
		tracker.DiscoverAsset(file, f.size, journal.DiscoveredImage)
	}

	counters := tracker.GetCounters()
	if counters.Pending != 4 {
		t.Errorf("expected 4 pending assets, got %d", counters.Pending)
	}
	if counters.AssetSize != 15360 {
		t.Errorf("expected asset size 15360, got %d", counters.AssetSize)
	}

	// Process some
	tracker.SetProcessed(journal.NewFilename(mockFS{}, "photo1.jpg"), journal.ProcessedUploadSuccess)
	tracker.SetProcessed(journal.NewFilename(mockFS{}, "photo2.jpg"), journal.ProcessedUploadSuccess)

	// Discard some
	tracker.SetDiscarded(journal.NewFilename(mockFS{}, "photo3.jpg"), journal.DiscardedLocalDuplicate, "duplicate")

	// Error some
	tracker.SetError(journal.NewFilename(mockFS{}, "video1.mp4"), journal.ErrorUploadFailed, fs.ErrPermission)

	counters = tracker.GetCounters()
	if counters.Processed != 2 {
		t.Errorf("expected 2 processed assets, got %d", counters.Processed)
	}
	if counters.Discarded != 1 {
		t.Errorf("expected 1 discarded asset, got %d", counters.Discarded)
	}
	if counters.Errors != 1 {
		t.Errorf("expected 1 error asset, got %d", counters.Errors)
	}
	if counters.Pending != 0 {
		t.Errorf("expected 0 pending assets, got %d", counters.Pending)
	}
	if !tracker.IsComplete() {
		t.Error("tracker should be complete")
	}
}

func TestGetPending(t *testing.T) {
	tracker := New()

	// Add some assets
	tracker.DiscoverAsset(journal.NewFilename(mockFS{}, "pending1.jpg"), 1024, journal.DiscoveredImage)
	tracker.DiscoverAsset(journal.NewFilename(mockFS{}, "pending2.jpg"), 2048, journal.DiscoveredImage)
	tracker.DiscoverAsset(journal.NewFilename(mockFS{}, "processed.jpg"), 4096, journal.DiscoveredImage)

	// Process one
	tracker.SetProcessed(journal.NewFilename(mockFS{}, "processed.jpg"), journal.ProcessedUploadSuccess)

	pending := tracker.GetPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending assets, got %d", len(pending))
	}
}

func TestValidate(t *testing.T) {
	tracker := New()

	// Initially valid (no assets)
	if err := tracker.Validate(); err != nil {
		t.Errorf("empty tracker should be valid: %v", err)
	}

	// Add asset
	tracker.DiscoverAsset(journal.NewFilename(mockFS{}, "test.jpg"), 1024, journal.DiscoveredImage)

	// Should be invalid (pending asset)
	if err := tracker.Validate(); err == nil {
		t.Error("tracker with pending assets should be invalid")
	}

	// Process asset
	tracker.SetProcessed(journal.NewFilename(mockFS{}, "test.jpg"), journal.ProcessedUploadSuccess)

	// Should be valid again
	if err := tracker.Validate(); err != nil {
		t.Errorf("tracker should be valid after processing: %v", err)
	}
}

func TestDebugMode(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracker := NewWithLogger(log, true)

	file := journal.NewFilename(mockFS{}, "test.jpg")

	// Discover asset
	tracker.DiscoverAsset(file, 1024, journal.DiscoveredImage)

	// Get the record
	assets := tracker.GetAllAssets()
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	// Check event history exists in debug mode
	if assets[0].EventHistory == nil {
		t.Error("event history should be populated in debug mode")
	}
	if len(assets[0].EventHistory) != 1 {
		t.Errorf("expected 1 event in history, got %d", len(assets[0].EventHistory))
	}

	// Process and check history grows
	tracker.SetProcessed(file, journal.ProcessedUploadSuccess)
	assets = tracker.GetAllAssets()
	if len(assets[0].EventHistory) != 2 {
		t.Errorf("expected 2 events in history, got %d", len(assets[0].EventHistory))
	}
}

func TestGenerateReport(t *testing.T) {
	tracker := New()

	// Add some assets
	tracker.DiscoverAsset(journal.NewFilename(mockFS{}, "photo1.jpg"), 1024, journal.DiscoveredImage)
	tracker.DiscoverAsset(journal.NewFilename(mockFS{}, "photo2.jpg"), 2048, journal.DiscoveredImage)

	tracker.SetProcessed(journal.NewFilename(mockFS{}, "photo1.jpg"), journal.ProcessedUploadSuccess)
	tracker.SetDiscarded(journal.NewFilename(mockFS{}, "photo2.jpg"), journal.DiscardedLocalDuplicate, "duplicate")

	report := tracker.GenerateReport()
	if report == "" {
		t.Error("report should not be empty")
	}
	// Report should contain key information
	if len(report) < 100 {
		t.Errorf("report seems too short: %d characters", len(report))
	}
}

func TestGenerateDetailedReport(t *testing.T) {
	tracker := New()

	file := journal.NewFilename(mockFS{}, "photo.jpg")
	tracker.DiscoverAsset(file, 1024, journal.DiscoveredImage)
	tracker.SetProcessed(file, journal.ProcessedUploadSuccess)

	report := tracker.GenerateDetailedReport(context.Background())
	if report == "" {
		t.Error("detailed report should not be empty")
	}
	// Should be CSV format
	if report[:8] != "FilePath" {
		t.Error("detailed report should start with CSV header")
	}
}

func TestStateTransitionErrors(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := NewWithLogger(log, false)
	file := journal.NewFilename(mockFS{}, "test.jpg")

	// Try to transition non-existent asset - should log error but not fail
	tracker.SetProcessed(file, journal.ProcessedUploadSuccess)

	// Discover asset
	tracker.DiscoverAsset(file, 1024, journal.DiscoveredImage)

	// Process it
	tracker.SetProcessed(file, journal.ProcessedUploadSuccess)

	// Try to transition already-processed asset - should log error but not fail
	tracker.SetDiscarded(file, journal.DiscardedLocalDuplicate, "duplicate")
}

func TestConcurrency(t *testing.T) {
	tracker := New()
	done := make(chan bool)

	// Concurrently add assets
	for i := range 10 {
		go func(n int) {
			file := journal.NewFilename(mockFS{}, "photo"+string(rune(n))+".jpg")
			tracker.DiscoverAsset(file, 1024, journal.DiscoveredImage)
			time.Sleep(time.Millisecond)
			tracker.SetProcessed(file, journal.ProcessedUploadSuccess)
			done <- true
		}(i)
	}

	// Wait for all
	for range 10 {
		<-done
	}

	counters := tracker.GetCounters()
	if counters.Processed != 10 {
		t.Errorf("expected 10 processed assets, got %d", counters.Processed)
	}
}

func TestStatusMethods(t *testing.T) {
	tracker := New()
	file1 := journal.NewFilename(mockFS{}, "test1.jpg")
	file2 := journal.NewFilename(mockFS{}, "test2.jpg")
	file3 := journal.NewFilename(mockFS{}, "test3.jpg")
	file4 := journal.NewFilename(mockFS{}, "test4.jpg")

	// Discover assets
	tracker.DiscoverAsset(file1, 1024, journal.DiscoveredImage) // pending: 1, size: 1024
	tracker.DiscoverAsset(file2, 2048, journal.DiscoveredImage) // pending: 2, size: 3072
	tracker.DiscoverAsset(file3, 512, journal.DiscoveredImage)  // pending: 3, size: 3584
	tracker.DiscoverAsset(file4, 256, journal.DiscoveredImage)  // pending: 4, size: 3840

	// Test initial pending state
	if count := tracker.GetPendingCount(); count != 4 {
		t.Errorf("expected 4 pending assets, got %d", count)
	}
	if size := tracker.GetPendingSize(); size != 3840 {
		t.Errorf("expected pending size 3840, got %d", size)
	}
	if count := tracker.GetProcessedCount(); count != 0 {
		t.Errorf("expected 0 processed assets, got %d", count)
	}
	if size := tracker.GetProcessedSize(); size != 0 {
		t.Errorf("expected processed size 0, got %d", size)
	}

	// Discover assets
	tracker.DiscoverAsset(file1, 1024, journal.DiscoveredImage)
	tracker.DiscoverAsset(file2, 2048, journal.DiscoveredImage)
	tracker.DiscoverAsset(file3, 512, journal.DiscoveredImage)
	tracker.DiscoverAsset(file4, 256, journal.DiscoveredImage)

	// Test initial state
	if count := tracker.GetPendingCount(); count != 4 {
		t.Errorf("expected 4 pending assets, got %d", count)
	}
	if size := tracker.GetPendingSize(); size != 3840 { // 1024 + 2048 + 512 + 256
		t.Errorf("expected pending size 3840, got %d", size)
	}
	if count := tracker.GetProcessedCount(); count != 0 {
		t.Errorf("expected 0 processed assets, got %d", count)
	}
	if size := tracker.GetProcessedSize(); size != 0 {
		t.Errorf("expected processed size 0, got %d", size)
	}

	// Process some assets
	tracker.SetProcessed(file1, journal.ProcessedUploadSuccess)
	tracker.SetProcessed(file2, journal.ProcessedUploadSuccess)

	// Test after processing
	if count := tracker.GetPendingCount(); count != 2 {
		t.Errorf("expected 2 pending assets, got %d", count)
	}
	if size := tracker.GetPendingSize(); size != 768 { // 512 + 256
		t.Errorf("expected pending size 768, got %d", size)
	}
	if count := tracker.GetProcessedCount(); count != 2 {
		t.Errorf("expected 2 processed assets, got %d", count)
	}
	if size := tracker.GetProcessedSize(); size != 3072 { // 1024 + 2048
		t.Errorf("expected processed size 3072, got %d", size)
	}

	// Discard an asset
	tracker.SetDiscarded(file3, journal.DiscardedLocalDuplicate, "duplicate")

	// Test after discarding
	if count := tracker.GetPendingCount(); count != 1 {
		t.Errorf("expected 1 pending asset, got %d", count)
	}
	if size := tracker.GetPendingSize(); size != 256 {
		t.Errorf("expected pending size 256, got %d", size)
	}
	if count := tracker.GetDiscardedCount(); count != 1 {
		t.Errorf("expected 1 discarded asset, got %d", count)
	}
	if size := tracker.GetDiscardedSize(); size != 512 {
		t.Errorf("expected discarded size 512, got %d", size)
	}

	// Error on an asset
	tracker.SetError(file4, journal.ErrorUploadFailed, fmt.Errorf("read error"))

	// Test after error
	if count := tracker.GetPendingCount(); count != 0 {
		t.Errorf("expected 0 pending assets, got %d", count)
	}
	if size := tracker.GetPendingSize(); size != 0 {
		t.Errorf("expected pending size 0, got %d", size)
	}
	if count := tracker.GetErrorCount(); count != 1 {
		t.Errorf("expected 1 error asset, got %d", count)
	}
	if size := tracker.GetErrorSize(); size != 256 {
		t.Errorf("expected error size 256, got %d", size)
	}
}
