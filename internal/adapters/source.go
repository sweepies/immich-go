// Package adapters provides interfaces and types for asset sources.
// It defines a clean Source interface that returns a stream of assets
// and common configuration types for adapters.
package adapters

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/filetypes"
)

// Source is the interface for asset sources (adapters).
// It provides a stream of asset groups for processing by the upload pipeline.
type Source interface {
	// Browse returns a channel of asset groups from the source.
	// The channel is closed when browsing is complete or context is cancelled.
	Browse(ctx context.Context) <-chan *assets.Group

	// Close releases any resources held by the source.
	io.Closer
}

// SourceDependencies provides the minimal dependencies that adapters need.
// This replaces the service-locator pattern of passing *app.Application.
type SourceDependencies struct {
	// Logger for adapter logging
	Logger *slog.Logger

	// Processor for recording file events
	Processor *fileprocessor.FileProcessor

	// SupportedMedia defines which file types are supported
	SupportedMedia filetypes.SupportedMedia

	// TimeZone for date/time handling
	TimeZone *time.Location

	// ConcurrentTasks for parallel processing
	ConcurrentTasks int
}

// SourceMode represents the type of source being used.
type SourceMode string

const (
	SourceModeFolder     SourceMode = "folder"
	SourceModeICloud     SourceMode = "icloud"
	SourceModePicasa     SourceMode = "picasa"
	SourceModeGoogle     SourceMode = "google"
	SourceModeFromImmich SourceMode = "from-immich"
)

// SourceInfo provides metadata about a source for logging and debugging.
type SourceInfo struct {
	Mode  SourceMode
	Paths []string
}
