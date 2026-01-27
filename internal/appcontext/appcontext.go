// Package appcontext provides immutable application context with explicit dependencies.
// It replaces the service-locator pattern of app.Application with a more testable
// and explicit dependency injection approach.
package appcontext

import (
	"log/slog"
	"time"

	cliflags "github.com/sweepies/immich-go/internal/cliFlags"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
)

// Context holds immutable configuration and runtime dependencies for commands.
// It is designed to be created once at startup and passed to all commands,
// replacing the mutable app.Application pattern.
type Context struct {
	// Configuration (immutable after creation)
	dryRun         bool
	onErrors       cliflags.OnErrorsFlag
	concurrentTask int
	output         string // "text" or "json"
	tz             *time.Location

	// Dependencies (set once, used by all commands)
	log            *slog.Logger
	processor      *fileprocessor.FileProcessor
	supportedMedia filetypes.SupportedMedia
}

// Option is a functional option for configuring Context.
type Option func(*Context)

// New creates a new immutable Context with the provided options.
func New(opts ...Option) *Context {
	ctx := &Context{
		tz:             time.Local,
		concurrentTask: 1,
		output:         "text",
		supportedMedia: filetypes.DefaultSupportedMedia,
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

// WithDryRun sets the dry-run mode.
func WithDryRun(dryRun bool) Option {
	return func(c *Context) {
		c.dryRun = dryRun
	}
}

// WithOnErrors sets the error handling policy.
func WithOnErrors(onErrors cliflags.OnErrorsFlag) Option {
	return func(c *Context) {
		c.onErrors = onErrors
	}
}

// WithConcurrentTasks sets the number of concurrent tasks.
func WithConcurrentTasks(n int) Option {
	return func(c *Context) {
		c.concurrentTask = n
	}
}

// WithOutput sets the output format ("text" or "json").
func WithOutput(output string) Option {
	return func(c *Context) {
		c.output = output
	}
}

// WithTimeZone sets the timezone.
func WithTimeZone(tz *time.Location) Option {
	return func(c *Context) {
		if tz != nil {
			c.tz = tz
		}
	}
}

// WithLogger sets the logger.
func WithLogger(log *slog.Logger) Option {
	return func(c *Context) {
		c.log = log
	}
}

// WithFileProcessor sets the file processor.
func WithFileProcessor(processor *fileprocessor.FileProcessor) Option {
	return func(c *Context) {
		c.processor = processor
	}
}

// WithSupportedMedia sets the supported media types.
func WithSupportedMedia(sm filetypes.SupportedMedia) Option {
	return func(c *Context) {
		if sm != nil {
			c.supportedMedia = sm
		}
	}
}

// DryRun returns whether dry-run mode is enabled.
func (c *Context) DryRun() bool {
	return c.dryRun
}

// OnErrors returns the error handling policy.
func (c *Context) OnErrors() cliflags.OnErrorsFlag {
	return c.onErrors
}

// ConcurrentTasks returns the number of concurrent tasks.
func (c *Context) ConcurrentTasks() int {
	return c.concurrentTask
}

// Output returns the output format.
func (c *Context) Output() string {
	return c.output
}

// IsJSONOutput returns true if output format is JSON.
func (c *Context) IsJSONOutput() bool {
	return c.output == "json"
}

// TimeZone returns the configured timezone.
func (c *Context) TimeZone() *time.Location {
	if c.tz == nil {
		return time.Local
	}
	return c.tz
}

// Logger returns the logger.
func (c *Context) Logger() *slog.Logger {
	return c.log
}

// FileProcessor returns the file processor.
func (c *Context) FileProcessor() *fileprocessor.FileProcessor {
	return c.processor
}

// SupportedMedia returns the supported media types.
func (c *Context) SupportedMedia() filetypes.SupportedMedia {
	if c.supportedMedia == nil {
		return filetypes.DefaultSupportedMedia
	}
	return c.supportedMedia
}

// Derive creates a new Context with updated options, preserving immutability.
// This is useful for command-specific overrides.
func (c *Context) Derive(opts ...Option) *Context {
	derived := &Context{
		dryRun:         c.dryRun,
		onErrors:       c.onErrors,
		concurrentTask: c.concurrentTask,
		output:         c.output,
		tz:             c.tz,
		log:            c.log,
		processor:      c.processor,
		supportedMedia: c.supportedMedia,
	}
	for _, opt := range opts {
		opt(derived)
	}
	return derived
}
