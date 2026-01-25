package archive

import (
	"context"
	"fmt"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/adapters/folder"
	"github.com/simulot/immich-go/adapters/fromimmich"
	gp "github.com/simulot/immich-go/adapters/googlePhotos"
	"github.com/simulot/immich-go/app"
	cliarchive "github.com/simulot/immich-go/internal/cli/archive"
	"github.com/spf13/cobra"
)

// ArchiveCmd holds the state for archive command execution.
// CLI flags are managed by the internal/cli/archive package.
type ArchiveCmd struct {
	// Adapter configurations (for creating adapters)
	folderCmd     folder.ImportFolderCmd
	googleCmd     gp.TakeoutCmd
	fromImmichCmd fromimmich.FromImmichCmd

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

// run creates the appropriate adapter based on config source mode and executes the archive.
func (ac *ArchiveCmd) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var adapter adapters.Reader
	var err error

	// Select and create the appropriate adapter based on config source mode
	switch ac.config.SourceMode {
	case cliarchive.SourceModeFromImmich:
		adapter, err = ac.fromImmichCmd.NewAdapter(ctx, ac.app)
		if err != nil {
			return fmt.Errorf("failed to create immich adapter: %w", err)
		}

	case cliarchive.SourceModeGoogle:
		adapter, err = ac.googleCmd.NewAdapter(ac.app, args)
		if err != nil {
			return fmt.Errorf("failed to create google photos adapter: %w", err)
		}
		defer ac.googleCmd.Close()

	case cliarchive.SourceModeICloud:
		adapter, err = ac.folderCmd.NewAdapter(cmd, ac.app, args, folder.SourceModeICloud)
		if err != nil {
			return fmt.Errorf("failed to create icloud adapter: %w", err)
		}
		defer ac.folderCmd.Close()

	case cliarchive.SourceModePicasa:
		adapter, err = ac.folderCmd.NewAdapter(cmd, ac.app, args, folder.SourceModePicasa)
		if err != nil {
			return fmt.Errorf("failed to create picasa adapter: %w", err)
		}
		defer ac.folderCmd.Close()

	default:
		// Default: folder mode
		adapter, err = ac.folderCmd.NewAdapter(cmd, ac.app, args, folder.SourceModeFolder)
		if err != nil {
			return fmt.Errorf("failed to create folder adapter: %w", err)
		}
		defer ac.folderCmd.Close()
	}

	return ac.Run(cmd, adapter)
}
