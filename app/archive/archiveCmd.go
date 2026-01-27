package archive

import (
	"context"
	"fmt"

	"github.com/sweepies/immich-go/adapters/folder"
	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/internal/adapters"
	cliarchive "github.com/sweepies/immich-go/internal/cli/archive"
	"github.com/sweepies/immich-go/internal/upload/source"
	"github.com/spf13/cobra"
)

// ArchiveCmd holds the state for archive command execution.
// CLI flags are managed by the internal/cli/archive package.
type ArchiveCmd struct {
	app    *app.Application
	dest   *folder.LocalAssetWriter
	config *cliarchive.Config
}

// NewArchiveCommandFromCLI creates an archive command using the CLI package.
func NewArchiveCommandFromCLI(ctx context.Context, a *app.Application) *cobra.Command {
	builder := cliarchive.NewCommandBuilder()

	return builder.Build(ctx, func(cmd *cobra.Command, args []string, flags *cliarchive.Flags) error {
		// Build configuration from CLI flags
		cfg, err := flags.ToConfig(args)
		if err != nil {
			return err
		}

		// Create ArchiveCmd with configuration
		ac := &ArchiveCmd{
			app:    a,
			config: cfg,
		}

		// Initialize application
		a.EnsureFileProcessor()

		return ac.run(cmd, args)
	})
}

// run creates the appropriate source based on config and executes the archive.
func (ac *ArchiveCmd) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Create source factory
	factory := source.NewFactory(
		ac.app.Log().Logger,
		ac.app.FileProcessor(),
		ac.app.GetSupportedMedia(),
		ac.app.GetTZ(),
		ac.app.ConcurrentTask,
	)

	// Map CLI source mode to adapter source mode and get config
	var sourceMode adapters.SourceMode
	var cfg any

	switch ac.config.SourceMode {
	case cliarchive.SourceModeFromImmich:
		sourceMode = adapters.SourceModeFromImmich
		cfg = ac.config.FromImmichConfig
	case cliarchive.SourceModeGoogle:
		sourceMode = adapters.SourceModeGoogle
		cfg = ac.config.GoogleConfig
	case cliarchive.SourceModeICloud:
		sourceMode = adapters.SourceModeICloud
		cfg = ac.config.FolderConfig
	case cliarchive.SourceModePicasa:
		sourceMode = adapters.SourceModePicasa
		cfg = ac.config.FolderConfig
	default:
		sourceMode = adapters.SourceModeFolder
		cfg = ac.config.FolderConfig
	}

	adapterSource, err := factory.CreateFromConfig(ctx, sourceMode, cfg)
	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}
	defer adapterSource.Close()

	return ac.Run(cmd, adapterSource)
}
