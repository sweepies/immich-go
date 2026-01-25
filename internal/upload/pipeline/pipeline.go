// Package pipeline provides a staged upload pipeline for processing assets.
// It separates the upload orchestration into distinct stages with clear contracts:
// - Source discovery
// - Metadata normalization
// - De-duplication and grouping
// - Upload and server sync
// - Finalize (albums, tags, summary)
package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/filetypes"
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
	Server ServerClient

	// Pipeline state (mutable during execution)
	Index     *Index
	StartTime time.Time

	// Session tag value (set once at start)
	SessionTagValue string
}

// NewContext creates a new pipeline context with the given configuration.
func NewContext(cfg Config, log *slog.Logger, processor *fileprocessor.FileProcessor, media filetypes.SupportedMedia, server ServerClient) *Context {
	ctx := &Context{
		Config:    cfg,
		Logger:    log,
		Processor: processor,
		Media:     media,
		Server:    server,
		Index:     NewIndex(),
		StartTime: time.Now(),
	}

	if cfg.SessionTag {
		ctx.SessionTagValue = "{immich-go}/" + time.Now().Format("2006-01-02 15:04:05")
	}

	return ctx
}

// Source is an interface for asset sources (adapters).
// It provides a stream of asset groups for processing.
type Source interface {
	// Browse returns a channel of asset groups from the source.
	Browse(ctx context.Context) <-chan *assets.Group
}

// Stage represents a processing stage in the pipeline.
type Stage interface {
	// Name returns the stage name for logging/debugging.
	Name() string

	// Run executes the stage with the given pipeline context.
	Run(ctx context.Context, pctx *Context) error
}

// Pipeline orchestrates the execution of upload stages.
type Pipeline struct {
	stages []Stage
	source Source
	log    *slog.Logger
}

// New creates a new pipeline with the given source.
func New(source Source, log *slog.Logger) *Pipeline {
	return &Pipeline{
		source: source,
		log:    log,
	}
}

// AddStage adds a stage to the pipeline.
func (p *Pipeline) AddStage(stage Stage) {
	p.stages = append(p.stages, stage)
}

// Run executes all stages in order.
func (p *Pipeline) Run(ctx context.Context, pctx *Context) error {
	for _, stage := range p.stages {
		p.log.Debug("starting pipeline stage", "stage", stage.Name())
		if err := stage.Run(ctx, pctx); err != nil {
			p.log.Error("pipeline stage failed", "stage", stage.Name(), "error", err)
			return err
		}
		p.log.Debug("completed pipeline stage", "stage", stage.Name())
	}
	return nil
}

// Source returns the pipeline source.
func (p *Pipeline) Source() Source {
	return p.source
}
