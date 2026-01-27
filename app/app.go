package app

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/sweepies/immich-go/internal/appcontext"
	cliflags "github.com/sweepies/immich-go/internal/cliFlags"
	"github.com/sweepies/immich-go/internal/config"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Application holds configuration used by all commands
// It manages global settings like:
// - the log and the log-level
// - application counters
// - the concurrency
// - configuration values

type Application struct {
	// CLI flags
	DryRun         bool
	OnErrors       cliflags.OnErrorsFlag
	ConcurrentTask int
	Output         string // Output format: text or json

	// Internal state
	log       *Log
	processor *fileprocessor.FileProcessor // Unified file processing tracker
	tz        *time.Location
	Config    *config.ConfigurationManager

	sm filetypes.SupportedMedia

	numErrors atomic.Int64 // count the errors occurred during the run
}

func (app *Application) RegisterFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&app.DryRun, "dry-run", false, "dry run")
	flags.Var(&app.OnErrors, "on-errors", "What to do when an error occurs (stop, continue, accept N errors at max)")
	flags.IntVar(&app.ConcurrentTask, "concurrent-tasks", runtime.NumCPU(), "Number of concurrent tasks (1-20)")
	flags.StringVarP(&app.Output, "output", "o", "text", "Output format (text|json) - json outputs JSONL to stdout, logs to stderr")
}

func New(ctx context.Context, cmd *cobra.Command) *Application {
	// application's context
	a := &Application{
		log:    &Log{},
		tz:     time.Local,
		Config: config.New(),
	}
	return a
}

func (app *Application) Log() *Log {
	return app.log
}

func (app *Application) GetTZ() *time.Location {
	if app.tz == nil {
		app.tz = time.Local
	}
	return app.tz
}

func (app *Application) SetTZ(tz *time.Location) {
	app.tz = tz
}

// FileProcessor returns the file processor for coordinated asset tracking and event logging
func (app *Application) FileProcessor() *fileprocessor.FileProcessor {
	return app.processor
}

// SetFileProcessor sets the file processor
func (app *Application) SetFileProcessor(processor *fileprocessor.FileProcessor) {
	app.processor = processor
}

func (app *Application) SetLog(log *Log) {
	app.log = log
}

func (app *Application) GetSupportedMedia() filetypes.SupportedMedia {
	if app.sm == nil {
		return filetypes.DefaultSupportedMedia
	}
	return app.sm
}

func (app *Application) SetSupportedMedia(sm filetypes.SupportedMedia) {
	app.sm = sm
}

func (app *Application) ProcessError(err error) error {
	if err == nil {
		return nil
	}
	// we don't count context.Canceled as an error
	// but we want to return it to the caller
	if errors.Is(err, context.Canceled) {
		return err
	}

	nErr := app.numErrors.Add(1)
	if app.OnErrors == cliflags.OnErrorsStop {
		app.Log().Error("Error", "err", err.Error())
		return err
	} else if app.OnErrors == cliflags.OnErrorsNeverStop {
		app.Log().Error("Error", "err", err.Error())
		return nil
	} else if nErr > int64(app.OnErrors) {
		app.Log().Error("Too many errors, stopping", "err", err.Error())
		return err
	}
	return nil
}

// Context creates an immutable appcontext.Context from the current Application state.
// This serves as a bridge during the transition from Application to appcontext.Context,
// allowing commands to gradually adopt the new pattern.
func (app *Application) Context() *appcontext.Context {
	return appcontext.New(
		appcontext.WithDryRun(app.DryRun),
		appcontext.WithOnErrors(app.OnErrors),
		appcontext.WithConcurrentTasks(app.ConcurrentTask),
		appcontext.WithOutput(app.Output),
		appcontext.WithTimeZone(app.tz),
		appcontext.WithLogger(app.log.Logger),
		appcontext.WithFileProcessor(app.processor),
		appcontext.WithSupportedMedia(app.sm),
	)
}

// EnsureFileProcessor ensures the FileProcessor is initialized.
// If not already set, it creates one using the appcontext factory.
func (app *Application) EnsureFileProcessor() {
	if app.processor == nil {
		app.processor = appcontext.NewFileProcessor(app.log.Logger, app.DryRun)
	}
}
