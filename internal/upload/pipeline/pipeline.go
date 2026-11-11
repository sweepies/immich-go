// Package pipeline implements the upload run against an Immich server.
// The Runner discovers server assets and albums, streams asset groups from
// the Source through the configured filters and groupers, uploads them to the
// server, and records album and tag membership through batched caches that are
// flushed when the run finishes.
package pipeline

import (
	"log/slog"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
)

// Config holds immutable configuration for the upload pipeline.
type Config struct {
	DryRun         bool
	Overwrite      bool
	ConcurrentTask int
	SessionTag     bool
	Tags           []string
	TimeZone       *time.Location
	OutputFormat   string // "text" or "json"
}

// Context holds the runtime state for a single pipeline execution.
// It is created once per upload run and passed through all stages.
type Context struct {
	// Immutable configuration
	Config Config

	// Runtime dependencies
	Logger    *slog.Logger
	Processor *fileprocessor.FileProcessor
	Media     filetypes.SupportedMedia

	// Server interaction
	Server immich.UploadClient
	Jobs   immich.JobControlService

	// Pipeline state (mutable during execution)
	Index     *Index
	StartTime time.Time

	// Session tag value (set once at start)
	SessionTagValue string
}

// NewContext creates a pipeline context with the provided configuration and dependencies.
// It initializes the asset index and start time, and generates a session tag when enabled.
func NewContext(
	cfg Config,
	log *slog.Logger,
	processor *fileprocessor.FileProcessor,
	media filetypes.SupportedMedia,
	server immich.UploadClient,
	jobs immich.JobControlService,
) *Context {
	ctx := &Context{
		Config:    cfg,
		Logger:    log,
		Processor: processor,
		Media:     media,
		Server:    server,
		Jobs:      jobs,
		Index:     NewIndex(),
		StartTime: time.Now(),
	}

	if cfg.SessionTag {
		ctx.SessionTagValue = "{immich-go}/" + time.Now().Format("2006-01-02 15:04:05")
	}

	return ctx
}

// Source is an alias to adapters.Source for asset sources.
// It provides a stream of asset groups for processing.
type Source = adapters.Source
