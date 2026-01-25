package upload

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/adapters/folder"
	"github.com/simulot/immich-go/adapters/fromimmich"
	gp "github.com/simulot/immich-go/adapters/googlePhotos"
	"github.com/simulot/immich-go/adapters/shared"
	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/immich"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/assets/cache"
	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/gen/syncset"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type UpLoadMode int

const (
	UpModeGoogleTakeout UpLoadMode = iota
	UpModeFolder
	UpModeICloud
	UpModePicasa
)

func (m UpLoadMode) String() string {
	switch m {
	case UpModeGoogleTakeout:
		return "Google Takeout"
	case UpModeFolder:
		return "Folder"
	case UpModeICloud:
		return "iCloud"
	case UpModePicasa:
		return "Picasa"
	default:
		return "Unknown"
	}
}

type UpCmd struct {
	// Cli flags
	shared.StackOptions
	client     app.Client
	Overwrite  bool // Always overwrite files on the server with local versions
	Tags       []string
	SessionTag bool
	session    string // Session tag value

	// Source mode flags (mutually exclusive)
	GoogleTakeout bool // --google flag
	ICloudTakeout bool // --icloud flag
	PicasaMode    bool // --picasa flag
	FromImmich    bool // --from-immich flag

	// Adapter configurations (embedded for flag registration)
	folderCmd     folder.ImportFolderCmd
	googleCmd     gp.TakeoutCmd
	fromImmichCmd fromimmich.FromImmichCmd

	// Upload command state
	tz                *time.Location
	Mode              UpLoadMode
	app               *app.Application
	assetIndex        *immichIndex                         // List of assets present on the server
	localAssets       *syncset.Set[string]                 // List of assets present on the local input by name+size
	immichAssetsReady chan struct{}                        // Signal that the asset index is ready
	deleteServerList  []*immich.Asset                      // List of server assets to remove
	adapter           adapters.Reader                      // the source of assets
	DebugCounters     bool                                 // Enable CSV action counters per file
	albumsCache       *cache.CollectionCache[assets.Album] // List of albums present on the server
	tagsCache         *cache.CollectionCache[assets.Tag]   // List of tags present on the server
	finished          bool                                 // the finish task has been run
	infoCollector     *filenames.InfoCollector             // Collects information about the files being processed
}

func (uc *UpCmd) RegisterFlags(flags *pflag.FlagSet) {
	uc.client.RegisterFlags(flags, "")
	flags.BoolVar(&uc.Overwrite, "overwrite", false, "Always overwrite files on the server with local versions")
	flags.StringSliceVar(&uc.Tags, "tag", nil, "Add tags to the imported assets. Can be specified multiple times. Hierarchy is supported using a / separator (e.g. 'tag1/subtag1')")
	flags.BoolVar(&uc.SessionTag, "session-tag", false, "Tag uploaded photos with a tag \"{immich-go}/YYYY-MM-DD HH-MM-SS\"")

	// Source mode flags (mutually exclusive)
	flags.BoolVar(&uc.GoogleTakeout, "google", false, "Import from Google Photos takeout")
	flags.BoolVar(&uc.ICloudTakeout, "icloud", false, "Import from iCloud takeout")
	flags.BoolVar(&uc.PicasaMode, "picasa", false, "Enable Picasa album parsing")
	flags.BoolVar(&uc.FromImmich, "from-immich", false, "Transfer from another Immich server")

	// Register adapter-specific flags
	uc.folderCmd.RegisterFlagsFlat(flags, true)
	uc.googleCmd.RegisterFlagsFlat(flags, true)
	uc.fromImmichCmd.RegisterFlagsFlat(flags)
}

// NewUploadCommand creates the "upload" command with flag-based source selection.
// It registers flags and initializes the UpCmd struct, which holds the state for uploads.
func NewUploadCommand(ctx context.Context, app *app.Application) *cobra.Command {
	uc := &UpCmd{
		app:               app,
		localAssets:       syncset.New[string](),
		immichAssetsReady: make(chan struct{}),
	}

	cmd := &cobra.Command{
		Use:   "upload [flags] <paths>...",
		Short: "Upload photos to an Immich server from various sources",
		Long: `Upload photos to an Immich server from various sources.

By default, uploads from local folders. Use source flags to change the source:
  --google      Import from Google Photos takeout
  --icloud      Import from iCloud takeout  
  --picasa      Enable Picasa album parsing
  --from-immich Transfer from another Immich server (no paths required)`,
		Args: func(cmd *cobra.Command, args []string) error {
			// --from-immich doesn't require paths, others do
			if uc.FromImmich {
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
			if uc.GoogleTakeout {
				count++
			}
			if uc.ICloudTakeout {
				count++
			}
			if uc.FromImmich {
				count++
			}
			if uc.PicasaMode && (uc.GoogleTakeout || uc.ICloudTakeout || uc.FromImmich) {
				return errors.New("--picasa can only be used with folder uploads")
			}
			// Note: --picasa can be combined with folder mode, so not counted
			if count > 1 {
				return errors.New("--google, --icloud, and --from-immich are mutually exclusive")
			}

			// Initialize the FileProcessor (tracker + logger)
			if app.FileProcessor() == nil {
				recorder := fileevent.NewRecorder(app.Log().Logger)
				tracker := assettracker.NewWithLogger(app.Log().Logger, app.DryRun)
				processor := fileprocessor.New(tracker, recorder)
				app.SetFileProcessor(processor)
			}

			app.SetTZ(time.Local)
			if tz, err := cmd.Flags().GetString("time-zone"); err == nil && tz != "" {
				if loc, err := time.LoadLocation(tz); err == nil {
					app.SetTZ(loc)
				}
			} else if err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return uc.run(cmd, args)
		},
	}

	// Register CLI flags for the upload command
	uc.RegisterFlags(cmd.Flags())

	return cmd
}

// run creates the appropriate adapter based on source flags and executes the upload.
func (uc *UpCmd) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var adapter adapters.Reader
	var err error

	// Select and create the appropriate adapter based on source flags
	switch {
	case uc.FromImmich:
		uc.Mode = UpModeFolder // Will be overridden, but set a default
		adapter, err = uc.fromImmichCmd.NewAdapter(ctx, uc.app)
		if err != nil {
			return fmt.Errorf("failed to create immich adapter: %w", err)
		}

	case uc.GoogleTakeout:
		uc.Mode = UpModeGoogleTakeout
		adapter, err = uc.googleCmd.NewAdapter(uc.app, args)
		if err != nil {
			return fmt.Errorf("failed to create google photos adapter: %w", err)
		}
		defer uc.googleCmd.Close()

	case uc.ICloudTakeout:
		uc.Mode = UpModeICloud
		adapter, err = uc.folderCmd.NewAdapter(cmd, uc.app, args, folder.SourceModeICloud)
		if err != nil {
			return fmt.Errorf("failed to create icloud adapter: %w", err)
		}
		defer uc.folderCmd.Close()

	case uc.PicasaMode:
		uc.Mode = UpModePicasa
		adapter, err = uc.folderCmd.NewAdapter(cmd, uc.app, args, folder.SourceModePicasa)
		if err != nil {
			return fmt.Errorf("failed to create picasa adapter: %w", err)
		}
		defer uc.folderCmd.Close()

	default:
		// Default: folder mode
		uc.Mode = UpModeFolder
		adapter, err = uc.folderCmd.NewAdapter(cmd, uc.app, args, folder.SourceModeFolder)
		if err != nil {
			return fmt.Errorf("failed to create folder adapter: %w", err)
		}
		defer uc.folderCmd.Close()
	}

	return uc.Run(cmd, adapter)
}

// Run is called back by the actual asset reader
func (uc *UpCmd) Run(cmd *cobra.Command, adapter adapters.Reader) error {
	uc.Mode = UpModeFolder // TODO

	// ready to run
	ctx := cmd.Context()
	err := uc.client.Open(ctx, uc.app)
	if err != nil {
		return err
	}
	uc.tz = uc.app.GetTZ()
	uc.app.SetSupportedMedia(uc.client.Immich.SupportedMedia())

	// Initialize the FileProcessor if not already done
	if uc.app.FileProcessor() == nil {
		recorder := fileevent.NewRecorder(uc.app.Log().Logger)
		tracker := assettracker.NewWithLogger(uc.app.Log().Logger, uc.app.DryRun)
		processor := fileprocessor.New(tracker, recorder)
		uc.app.SetFileProcessor(processor)
	}

	if uc.SessionTag {
		uc.session = fmt.Sprintf("{immich-go}/%s", time.Now().Format("2006-01-02 15:04:05"))
	}

	if uc.ManageEpsonFastFoto {
		g := epsonfastfoto.Group{}
		uc.Groupers = append(uc.Groupers, g.Group)
	}
	if uc.ManageBurst != filters.BurstNothing {
		uc.Groupers = append(uc.Groupers, burst.Group)
	}
	uc.Groupers = append(uc.Groupers, series.Group)
	uc.Filters = append(uc.Filters, uc.ManageBurst.GroupFilter(), uc.ManageRawJPG.GroupFilter(), uc.ManageHEICJPG.GroupFilter())
	uc.infoCollector = filenames.NewInfoCollector(uc.tz, uc.app.GetSupportedMedia())

	return uc.upload(ctx, adapter)
}
