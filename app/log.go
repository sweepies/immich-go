package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/phsym/console-slog"
	"github.com/simulot/immich-go/immich/httptrace"
	"github.com/simulot/immich-go/internal/fshelper/debugfiles"
	"github.com/simulot/immich-go/internal/loghelper"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Log struct {
	Level string `mapstructure:"level" json:"level" toml:"level" yaml:"level"` // Indicate the log level (string)

	*slog.Logger             // Logger
	sLevel        slog.Level // the log level value
	mainWriter    io.Writer  // the log writer (always stderr)
	consoleWriter io.Writer

	apiTracer      *httptrace.Tracer
	apiTraceWriter *os.File
	apiTraceName   string
}

func (log *Log) RegisterFlags(flags *pflag.FlagSet) {
	flags.StringVar(&log.Level, "log-level", "INFO", "Log level (DEBUG|INFO|WARN|ERROR), default INFO")
}

// InitLogger initializes the logger with the appropriate format
// Always logs to stderr (use shell redirection to save logs)
func (log *Log) InitLogger(jsonMode bool) error {
	err := log.sLevel.UnmarshalText([]byte(strings.ToUpper(log.Level)))
	if err != nil {
		return err
	}
	log.setHandlers(os.Stderr, jsonMode)
	loghelper.SetGlobalLogger(log.Logger)
	return nil
}

func (log *Log) Open(ctx context.Context, cmd *cobra.Command, app *Application) error {
	for c := cmd; c != nil; c = c.Parent() {
		// no log, nor banner for those commands
		switch c.Name() {
		case "version", "completion":
			return nil
		}
		if cmd.Flags().Changed("--help") {
			return nil
		}
	}

	// Check if JSON output mode is enabled
	isJSONMode := app.Output == "json"

	// Only print banner in non-JSON mode (banner goes to stdout)
	if !isJSONMode {
		fmt.Println(Banner())
	}

	// Initialize logger (always outputs to stderr)
	err := log.InitLogger(isJSONMode)
	if err != nil {
		return err
	}
	// List flags
	log.Info(GetVersion())

	// Log configuration file if used
	if configFile := app.Config.GetConfigFile(); configFile != "" {
		log.Info("", "Configuration file", configFile)
	} else {
		log.Info("", "Configuration file", "none (using defaults)")
	}

	log.Info("Running environment:", "architecture", runtime.GOARCH, "os", runtime.GOOS)

	cmdStack := []string{cmd.Name()}
	for c := cmd.Parent(); c != nil; c = c.Parent() {
		cmdStack = append([]string{c.Name()}, cmdStack...)
	}

	log.Info(fmt.Sprintf("Command: %s", strings.Join(cmdStack, " ")))
	log.Info("Flags:")
	visitFlags := func(flag *pflag.Flag) {
		origin := app.Config.GetFlagOrigin(cmd, flag)
		val := flag.Value.String()
		if strings.Contains(flag.Name, "api-key") && len(val) > 4 {
			val = strings.Repeat("*", len(val)-4) + val[len(val)-4:]
		}
		log.Info("", "--"+flag.Name, val, "origin", origin)
	}
	cmd.Flags().VisitAll(visitFlags)
	cmd.PersistentFlags().VisitAll(visitFlags)

	// List arguments
	log.Info("Arguments:")
	for _, arg := range cmd.Flags().Args() {
		log.Info(fmt.Sprintf("  %q", arg))
	}
	if log.sLevel == slog.LevelDebug {
		debugfiles.EnableTrackFiles(log.Logger)
	}

	return nil
}

/*
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level)
		a.Value = slog.StringValue(fmt.Sprintf("%-7s", level.String()))
	}
	return a
}
*/

func (log *Log) setHandlers(writer io.Writer, jsonMode bool) {
	log.mainWriter = writer

	var handler slog.Handler
	if jsonMode {
		// JSON format for machine-readable logs
		handler = slog.NewJSONHandler(log.mainWriter, &slog.HandlerOptions{
			Level: log.sLevel,
		})
	} else {
		// Text format for human-readable logs
		handler = console.NewHandler(log.mainWriter, &console.HandlerOptions{
			Level:      log.sLevel,
			TimeFormat: time.DateTime,
			NoColor:    false, // Enable colors for stderr output
			Theme:      console.NewDefaultTheme(),
		})
	}

	log.Logger = slog.New(NewFilteredHandler(handler))
}

// SetLogWriter returns a logger using the provided writer.
// When writer is nil, it returns the main application logger.
func (log *Log) SetLogWriter(writer io.Writer) *slog.Logger {
	if writer == nil {
		return log.Logger
	}

	handler := console.NewHandler(writer, &console.HandlerOptions{
		Level:      log.sLevel,
		TimeFormat: time.DateTime,
		NoColor:    false,
		Theme:      console.NewDefaultTheme(),
	})

	return slog.New(NewFilteredHandler(handler))
}

// Message logs an important message that should always be visible to the user
// In text mode, it appears as a colored log line on stderr
// In JSON mode, it appears as a JSON log line on stderr
func (log *Log) Message(msg string, values ...any) {
	if log.Logger != nil {
		s := fmt.Sprintf(msg, values...)
		log.Info(s)
	}
}

func (log *Log) Close(ctx context.Context, cmd *cobra.Command, app *Application) error {
	if cmd.Name() == "version" {
		// No log for version command
		return nil
	}
	debugfiles.ReportTrackedFiles()

	// Close API trace if open
	if log.apiTraceWriter != nil {
		log.apiTracer.Close()
		log.apiTraceWriter.Close()
	}

	// No need to close stderr
	return nil
}

func (log *Log) GetSLog() *slog.Logger {
	return log.Logger
}

func (log *Log) OpenAPITrace() error {
	if log.apiTraceWriter == nil {
		var err error
		// Create trace file in current directory
		log.apiTraceName = time.Now().Format("immich-go_2006-01-02_15-04-05") + ".trace.log"
		log.apiTraceWriter, err = os.OpenFile(log.apiTraceName, os.O_CREATE|os.O_WRONLY, 0o664)
		if err != nil {
			return err
		}
		log.Info("API trace file created", "file", log.apiTraceName)
		log.apiTracer = httptrace.NewTracer(log.apiTraceWriter)
	}
	return nil
}

func (log *Log) APITracer() *httptrace.Tracer {
	return log.apiTracer
}

// FilteredHandler filterslog messages and filters out context canceled errors
// if err, ok := a.Value.Any().(error); ok {
// if errors.Is(err, context.Canceled) {
type FilteredHandler struct {
	handler slog.Handler
}

var _ slog.Handler = (*FilteredHandler)(nil)

func NewFilteredHandler(handler slog.Handler) slog.Handler {
	return &FilteredHandler{
		handler: handler,
	}
}

func (h *FilteredHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *FilteredHandler) Handle(ctx context.Context, r slog.Record) error {
	// When error level is Error or more serious
	if r.Level >= slog.LevelError {
		keepMe := true
		// parses the attributes
		r.Attrs(func(a slog.Attr) bool {
			if err, ok := a.Value.Any().(error); ok {
				if errors.Is(err, context.Canceled) {
					keepMe = false
					return false
				}
			}
			return true
		})
		if !keepMe {
			return nil
		}
	}
	// Otherwise, log the message
	return h.handler.Handle(ctx, r)
}

func (h *FilteredHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &FilteredHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *FilteredHandler) WithGroup(name string) slog.Handler {
	return &FilteredHandler{handler: h.handler.WithGroup(name)}
}
