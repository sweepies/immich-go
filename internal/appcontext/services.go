package appcontext

import (
	"log/slog"

	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/fileprocessor"
)

// NewFileProcessor creates a new FileProcessor with the given logger and dry-run setting.
// This centralizes the creation of FileProcessor instances that was previously scattered
// across command handlers (upload, archive, etc.).
func NewFileProcessor(log *slog.Logger, dryRun bool) *fileprocessor.FileProcessor {
	recorder := fileevent.NewRecorder(log)
	tracker := assettracker.NewWithLogger(log, dryRun)
	return fileprocessor.New(tracker, recorder)
}

// EnsureFileProcessor returns the provided processor if non-nil, otherwise creates a new one.
// This pattern allows commands to use a pre-configured processor or create one on demand.
func EnsureFileProcessor(existing *fileprocessor.FileProcessor, log *slog.Logger, dryRun bool) *fileprocessor.FileProcessor {
	if existing != nil {
		return existing
	}
	return NewFileProcessor(log, dryRun)
}
