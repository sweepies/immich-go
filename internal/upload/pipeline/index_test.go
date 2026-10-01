package pipeline

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/fshelper"
)

// mockFilename creates a mock Filename for testing
func mockFilename(name string) fshelper.Filename {
	fsys := fstest.MapFS{
		name: &fstest.MapFile{
			Data:    []byte("test content"),
			ModTime: time.Now(),
		},
	}
	return fshelper.NewFilename(fsys, name)
}

func TestNewIndex(t *testing.T) {
	idx := NewIndex()
	if idx == nil {
		t.Fatal("NewIndex returned nil")
	}
	if idx.Len() != 0 {
		t.Errorf("expected empty index, got %d", idx.Len())
	}
}

func TestIndex_AddImmichAsset(t *testing.T) {
	idx := NewIndex()

	ia := &immich.Asset{
		ID:               "asset-1",
		Checksum:         "checksum-1",
		OriginalFileName: "photo.jpg",
		ExifInfo: immich.ExifInfo{
			DateTimeOriginal: immich.ImmichExifTime{Time: time.Now()},
			FileSizeInByte:   1024,
		},
	}

	added, ok := idx.AddImmichAsset(ia)
	if !ok {
		t.Error("expected asset to be added")
	}
	if added == nil {
		t.Fatal("expected non-nil asset")
	}
	if idx.Len() != 1 {
		t.Errorf("expected index length 1, got %d", idx.Len())
	}

	// Adding the same asset should return false
	_, ok = idx.AddImmichAsset(ia)
	if ok {
		t.Error("expected asset not to be added again")
	}
	if idx.Len() != 1 {
		t.Errorf("expected index length 1 after duplicate, got %d", idx.Len())
	}
}

func TestIndex_GetByID(t *testing.T) {
	idx := NewIndex()

	ia := &immich.Asset{
		ID:               "asset-1",
		Checksum:         "checksum-1",
		OriginalFileName: "photo.jpg",
		ExifInfo: immich.ExifInfo{
			DateTimeOriginal: immich.ImmichExifTime{Time: time.Now()},
			FileSizeInByte:   1024,
		},
	}

	idx.AddImmichAsset(ia)

	found := idx.GetByID("asset-1")
	if found == nil {
		t.Fatal("expected to find asset")
	}
	if found.ID != "asset-1" {
		t.Errorf("expected ID 'asset-1', got '%s'", found.ID)
	}

	notFound := idx.GetByID("nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent asset")
	}
}

func TestIndex_ShouldUpload_NotOnServer(t *testing.T) {
	idx := NewIndex()

	// Create a local asset with a unique checksum
	la := &assets.Asset{
		File:             mockFilename("new_photo.jpg"),
		OriginalFileName: "new_photo.jpg",
		FileSize:         2048,
		CaptureDate:      time.Now(),
		Checksum:         "unique-checksum",
	}

	advice, err := idx.ShouldUpload(la, false)
	if err != nil {
		t.Fatalf("ShouldUpload failed: %v", err)
	}
	if advice.Advice != NotOnServer {
		t.Errorf("expected NotOnServer, got %v", advice.Advice)
	}
}

func TestIndex_ShouldUpload_SameOnServer(t *testing.T) {
	idx := NewIndex()

	captureDate := time.Now()

	// Add an asset to the server
	ia := &immich.Asset{
		ID:               "server-asset-1",
		Checksum:         "checksum-same",
		OriginalFileName: "photo.jpg",
		ExifInfo: immich.ExifInfo{
			DateTimeOriginal: immich.ImmichExifTime{Time: captureDate},
			FileSizeInByte:   1024,
		},
	}
	idx.AddImmichAsset(ia)

	// Create a local asset with the same checksum
	la := &assets.Asset{
		File:             mockFilename("photo.jpg"),
		OriginalFileName: "photo.jpg",
		FileSize:         1024,
		CaptureDate:      captureDate,
		Checksum:         "checksum-same",
	}

	advice, err := idx.ShouldUpload(la, false)
	if err != nil {
		t.Fatalf("ShouldUpload failed: %v", err)
	}
	if advice.Advice != SameOnServer {
		t.Errorf("expected SameOnServer, got %v", advice.Advice)
	}
}

func TestIndex_ShouldUpload_SmallerOnServer(t *testing.T) {
	idx := NewIndex()

	captureDate := time.Now()

	// Add a smaller asset to the server
	ia := &immich.Asset{
		ID:               "server-asset-1",
		Checksum:         "checksum-server",
		OriginalFileName: "photo.jpg",
		ExifInfo: immich.ExifInfo{
			DateTimeOriginal: immich.ImmichExifTime{Time: captureDate},
			FileSizeInByte:   512, // Smaller than local
		},
	}
	idx.AddImmichAsset(ia)

	// Create a local asset that's larger with same name and date
	la := &assets.Asset{
		File:             mockFilename("photo.jpg"),
		OriginalFileName: "photo.jpg",
		FileSize:         1024, // Larger
		CaptureDate:      captureDate,
		Checksum:         "checksum-local", // Different checksum
	}

	advice, err := idx.ShouldUpload(la, false)
	if err != nil {
		t.Fatalf("ShouldUpload failed: %v", err)
	}
	if advice.Advice != SmallerOnServer {
		t.Errorf("expected SmallerOnServer, got %v", advice.Advice)
	}
}

func TestIndex_ShouldUpload_BetterOnServer(t *testing.T) {
	idx := NewIndex()

	captureDate := time.Now()

	// Add a larger asset to the server
	ia := &immich.Asset{
		ID:               "server-asset-1",
		Checksum:         "checksum-server",
		OriginalFileName: "photo.jpg",
		ExifInfo: immich.ExifInfo{
			DateTimeOriginal: immich.ImmichExifTime{Time: captureDate},
			FileSizeInByte:   2048, // Larger than local
		},
	}
	idx.AddImmichAsset(ia)

	// Create a local asset that's smaller with same name and date
	la := &assets.Asset{
		File:             mockFilename("photo.jpg"),
		OriginalFileName: "photo.jpg",
		FileSize:         1024, // Smaller
		CaptureDate:      captureDate,
		Checksum:         "checksum-local", // Different checksum
	}

	advice, err := idx.ShouldUpload(la, false)
	if err != nil {
		t.Fatalf("ShouldUpload failed: %v", err)
	}
	if advice.Advice != BetterOnServer {
		t.Errorf("expected BetterOnServer, got %v", advice.Advice)
	}
}

func TestIndex_ShouldUpload_ForceUpload(t *testing.T) {
	idx := NewIndex()

	captureDate := time.Now()

	// Add an asset to the server
	ia := &immich.Asset{
		ID:               "server-asset-1",
		Checksum:         "checksum-server",
		OriginalFileName: "photo.jpg",
		ExifInfo: immich.ExifInfo{
			DateTimeOriginal: immich.ImmichExifTime{Time: captureDate},
			FileSizeInByte:   1024,
		},
	}
	idx.AddImmichAsset(ia)

	// Create a local asset with same name and date, overwrite=true
	la := &assets.Asset{
		File:             mockFilename("photo.jpg"),
		OriginalFileName: "photo.jpg",
		FileSize:         1024,
		CaptureDate:      captureDate,
		Checksum:         "checksum-local", // Different checksum
	}

	advice, err := idx.ShouldUpload(la, true) // overwrite=true
	if err != nil {
		t.Fatalf("ShouldUpload failed: %v", err)
	}
	if advice.Advice != ForceUpload {
		t.Errorf("expected ForceUpload, got %v", advice.Advice)
	}
}

func TestAdviceCode_String(t *testing.T) {
	tests := []struct {
		code     AdviceCode
		expected string
	}{
		{IDontKnow, "IDontKnow"},
		{SmallerOnServer, "SmallerOnServer"},
		{BetterOnServer, "BetterOnServer"},
		{SameOnServer, "SameOnServer"},
		{NotOnServer, "NotOnServer"},
		{AlreadyProcessed, "AlreadyProcessed"},
		{ForceUpload, "ForceUpload"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("AdviceCode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := formatBytes(tt.input); got != tt.expected {
				t.Errorf("formatBytes(%d) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
