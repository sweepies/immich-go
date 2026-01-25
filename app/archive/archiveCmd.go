package archive

import (
	"context"
	"errors"
	"fmt"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/adapters/folder"
	"github.com/simulot/immich-go/adapters/fromimmich"
	gp "github.com/simulot/immich-go/adapters/googlePhotos"
	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/spf13/cobra"
)

type ArchiveCmd struct {
	ArchivePath string

	// Source mode flags (mutually exclusive)
	GoogleTakeout bool // --google flag
	ICloudTakeout bool // --icloud flag
	PicasaMode    bool // --picasa flag
	FromImmich    bool // --from-immich flag

	// Adapter configurations (embedded for flag registration)
	folderCmd     folder.ImportFolderCmd
	googleCmd     gp.TakeoutCmd
	fromImmichCmd fromimmich.FromImmichCmd

	app  *app.Application
	dest *folder.LocalAssetWriter
}

func NewArchiveCommand(ctx context.Context, app *app.Application) *cobra.Command {
	ac := &ArchiveCmd{
		app: app,
	}

	cmd := &cobra.Command{
		Use:   "archive [flags] <paths>...",
		Short: "Archive various sources of photos to a file system",
		Long: `Archive photos from various sources to a local file system.

By default, archives from local folders. Use source flags to change the source:
  --google      Import from Google Photos takeout
  --icloud      Import from iCloud takeout  
  --picasa      Enable Picasa album parsing
  --from-immich Transfer from another Immich server (no paths required)`,
		Args: func(cmd *cobra.Command, args []string) error {
			// --from-immich doesn't require paths, others do
			if ac.FromImmich {
				if len(args) > 0 {
					return errors.New("--from-immich does not accept path arguments")
				}
				return nil
			}
			if len(args) < 1 {
				return errors.New("requires at least one path argument")
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate mutual exclusivity of source flags
			count := 0
			if ac.GoogleTakeout {
				count++
			}
			if ac.ICloudTakeout {
				count++
			}
			if ac.FromImmich {
				count++
			}
			if ac.PicasaMode && (ac.GoogleTakeout || ac.ICloudTakeout || ac.FromImmich) {
				return errors.New("--picasa can only be used with folder archives")
			}
			// Note: --picasa can be combined with folder mode, so not counted
			if count > 1 {
				return errors.New("--google, --icloud, and --from-immich are mutually exclusive")
			}

			// Initialize the FileProcessor (tracker + logger)
			if app.FileProcessor() == nil {
				logger := fileevent.NewRecorder(app.Log().Logger)
				tracker := assettracker.NewWithLogger(app.Log().Logger, app.DryRun)
				processor := fileprocessor.New(tracker, logger)
				app.SetFileProcessor(processor)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.run(cmd, args)
		},
	}

	// Required flag
	cmd.Flags().StringVarP(&ac.ArchivePath, "write-to-folder", "w", "", "Path where to write the archive")
	_ = cmd.MarkFlagRequired("write-to-folder")

	// Source mode flags (mutually exclusive)
	cmd.Flags().BoolVar(&ac.GoogleTakeout, "google", false, "Import from Google Photos takeout")
	cmd.Flags().BoolVar(&ac.ICloudTakeout, "icloud", false, "Import from iCloud takeout")
	cmd.Flags().BoolVar(&ac.PicasaMode, "picasa", false, "Enable Picasa album parsing")
	cmd.Flags().BoolVar(&ac.FromImmich, "from-immich", false, "Transfer from another Immich server")

	// Register adapter-specific flags
	ac.folderCmd.RegisterFlagsFlat(cmd.Flags(), false)
	ac.googleCmd.RegisterFlagsFlat(cmd.Flags(), false)
	ac.fromImmichCmd.RegisterFlagsFlat(cmd.Flags())

	return cmd
}

// run creates the appropriate adapter based on source flags and executes the archive.
func (ac *ArchiveCmd) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var adapter adapters.Reader
	var err error

	// Select and create the appropriate adapter based on source flags
	switch {
	case ac.FromImmich:
		adapter, err = ac.fromImmichCmd.NewAdapter(ctx, ac.app)
		if err != nil {
			return fmt.Errorf("failed to create immich adapter: %w", err)
		}

	case ac.GoogleTakeout:
		adapter, err = ac.googleCmd.NewAdapter(ac.app, args)
		if err != nil {
			return fmt.Errorf("failed to create google photos adapter: %w", err)
		}
		defer ac.googleCmd.Close()

	case ac.ICloudTakeout:
		adapter, err = ac.folderCmd.NewAdapter(ac.app, args, folder.SourceModeICloud)
		if err != nil {
			return fmt.Errorf("failed to create icloud adapter: %w", err)
		}
		defer ac.folderCmd.Close()

	case ac.PicasaMode:
		adapter, err = ac.folderCmd.NewAdapter(ac.app, args, folder.SourceModePicasa)
		if err != nil {
			return fmt.Errorf("failed to create picasa adapter: %w", err)
		}
		defer ac.folderCmd.Close()

	default:
		// Default: folder mode
		adapter, err = ac.folderCmd.NewAdapter(ac.app, args, folder.SourceModeFolder)
		if err != nil {
			return fmt.Errorf("failed to create folder adapter: %w", err)
		}
		defer ac.folderCmd.Close()
	}

	return ac.Run(cmd, adapter)
}
