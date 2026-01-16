package root

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/app/archive"
	"github.com/simulot/immich-go/app/stack"
	"github.com/simulot/immich-go/app/upload"
	"github.com/simulot/immich-go/app/version"
	"github.com/simulot/immich-go/internal/jsonoutput"
	"github.com/spf13/cobra"
)

// RootImmichGoCommand creates and returns the root Cobra command for immich-go.
// It sets up the CLI structure, configuration handling, and adds all subcommands.
// Returns the root command and the application instance.
func RootImmichGoCommand(ctx context.Context) (*cobra.Command, *app.Application) {
	// Enable traverse run hooks to ensure PersistentPreRunE runs for all commands
	cobra.EnableTraverseRunHooks = true // doc: cobra/site/content/user_guide.md

	// Initialize the root Cobra command with basic metadata
	cmd := &cobra.Command{
		Use:     "immich-go",
		Short:   "Immich-go is a command line application to interact with the Immich application using its API",
		Long:    `An alternative to the immich-CLI command that doesn't depend on nodejs installation. It tries its best for importing google photos takeout archives.`,
		Version: app.Version,
	}

	// Create the application context
	a := app.New(ctx, cmd)

	// Track start time for duration calculation
	var startTime time.Time

	flags := cmd.PersistentFlags()
	_ = a.OnErrors.Set("stop")
	a.RegisterFlags(flags)
	a.Log().RegisterFlags(flags)

	// Add all subcommands to the root command
	cmd.AddCommand(
		version.NewVersionCommand(ctx, a), // Version command to display app version
		upload.NewUploadCommand(ctx, a),   // Upload command for uploading assets
		archive.NewArchiveCommand(ctx, a), // Archive command for archiving assets
		stack.NewStackCommand(ctx, a),     // Stack command for managing stacks
	)

	// PersistentPreRunE is executed before any command runs, used for initialization
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error { //nolint:contextcheck
		// Track start time for duration calculation
		startTime = time.Now()

		// Validate --output flag
		if a.Output != "text" && a.Output != "json" {
			return fmt.Errorf("invalid output format: %q (must be 'text' or 'json')", a.Output)
		}

		// Auto-detect non-interactive mode if not explicitly set
		if !a.NonInteractive && !cmd.Flags().Changed("non-interactive") {
			// Check if stdout is a terminal
			if fileInfo, err := os.Stdout.Stat(); err == nil {
				// If stdout is not a character device (not a TTY), enable non-interactive mode
				if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
					a.NonInteractive = true
				}
			}
		}

		// Initialize configuration from the specified config file
		err := a.Config.Init(a.CfgFile)
		if err != nil {
			return err
		}

		// Process command-specific configuration
		err = a.Config.ProcessCommand(cmd)
		if err != nil {
			return err
		}

		// clip the number of concurrent tasks
		a.ConcurrentTask = min(max(a.ConcurrentTask, 1), 20)

		// Save configuration if the --save-config flag is set
		if save, _ := cmd.Flags().GetBool("save-config"); save {
			if err := a.Config.Save("immich-go.yaml"); err != nil {
				fmt.Fprintln(os.Stderr, "Can't save the configuration: ", err.Error())
				return err
			}
		}

		// Start the log
		err = a.Log().Open(cmd.Context(), cmd, a)

		return err
	}

	// PersistentPostRunE is executed after any command completes, used for cleanup and final reporting
	cmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error { //nolint:contextcheck
		// Close logs
		err := a.Log().Close(cmd.Context(), cmd, a)
		if err != nil {
			return err
		}

		// Output JSON summary for archive and stack commands (upload handles its own)
		// Only output if we have a FileProcessor and we're in JSON mode
		if a.Output == "json" && a.FileProcessor() != nil && cmd.Name() != "upload" {
			duration := time.Since(startTime).Seconds()
			counters := a.FileProcessor().GetAssetCounters()
			eventCounts := a.FileProcessor().GetEventCounts()
			eventSizes := a.FileProcessor().GetEventSizes()

			status := "success"
			exitCode := 0
			if counters.Errors > 0 {
				status = "error"
				exitCode = 1
			}

			if err := jsonoutput.WriteSummary(status, exitCode, counters, eventCounts, eventSizes, duration); err != nil {
				a.Log().Error("failed to write JSON summary", "err", err)
			}
		}

		return nil
	}

	return cmd, a
}
