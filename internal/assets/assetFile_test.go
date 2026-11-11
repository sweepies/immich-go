package assets

import (
	"io"
	"testing"
	"testing/fstest"

	"github.com/sweepies/immich-go/internal/fshelper"
)

// Closing an asset clears its file handle: a second Close is a no-op and
// OpenFile works again afterwards.
func TestAssetFileOpenCloseReopen(t *testing.T) {
	t.Setenv("IMMICHGO_TEMPDIR", t.TempDir())

	a := &Asset{
		File: fshelper.NewFilename(fstest.MapFS{
			"photo.jpg": &fstest.MapFile{Data: []byte("data")},
		}, "photo.jpg"),
	}

	f, err := a.OpenFile()
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing file handle: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The temporary file is gone; OpenFile must rebuild the cache reader.
	f2, err := a.OpenFile()
	if err != nil {
		t.Fatalf("OpenFile after Close: %v", err)
	}
	content, err := io.ReadAll(f2)
	if err != nil {
		t.Fatalf("reading reopened file: %v", err)
	}
	if string(content) != "data" {
		t.Errorf("expected %q, got %q", "data", content)
	}
	if err := f2.Close(); err != nil {
		t.Fatalf("closing reopened file handle: %v", err)
	}

	// Closing twice is safe.
	if err := a.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("third Close: %v", err)
	}
}
