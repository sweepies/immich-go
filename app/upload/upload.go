package upload

import (
	"context"
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
	cliupload "github.com/simulot/immich-go/internal/cli/upload"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/gen/syncset"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
	uploadcfg "github.com/simulot/immich-go/internal/upload"
	"github.com/spf13/cobra"
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

// UpCmd holds the state for upload command execution.
// CLI flags are managed by the internal/cli/upload package.
// This struct focuses on runtime state and adapter coordination.
type UpCmd struct {
	// Stack options (from config)
	shared.StackOptions

	// Server client
	client app.Client

	// Adapter configurations (for creating adapters)
	folderCmd     folder.ImportFolderCmd
	googleCmd     gp.TakeoutCmd
	fromImmichCmd fromimmich.FromImmichCmd

	// Upload command state (runtime)
	tz                *time.Location
	Mode              UpLoadMode
	app               *app.Application
	assetIndex        *immichIndex                         // List of assets present on the server
	localAssets       *syncset.Set[string]                 // List of assets present on the local input by name+size
	immichAssetsReady chan struct{}                        // Signal that the asset index is ready
	deleteServerList  []*immich.Asset                      // List of server assets to remove
	adapter           adapters.Reader                      // the source of assets
	albumsCache       *cache.CollectionCache[assets.Album] // List of albums present on the server
	tagsCache         *cache.CollectionCache[assets.Tag]   // List of tags present on the server
	finished          bool                                 // the finish task has been run
	infoCollector     *filenames.InfoCollector             // Collects information about the files being processed
	session           string                               // Session tag value (computed at runtime)

	// Configuration (built from CLI flags)
	config *uploadcfg.Config
}

// NewUploadCommandFromCLI creates an upload command using the CLI package.
func NewUploadCommandFromCLI(ctx context.Context, a *app.Application) *cobra.Command {
	builder := cliupload.NewCommandBuilder()

	return builder.Build(ctx, func(cmd *cobra.Command, args []string, flags *cliupload.Flags) error {
		// Build configuration from CLI flags
		cfg, err := flags.ToConfig(args)
		if err != nil {
			return err
		}

		// Create UpCmd with configuration
		uc := &UpCmd{
			app:               a,
			localAssets:       syncset.New[string](),
			immichAssetsReady: make(chan struct{}),
			config:            cfg,
		}

		// Copy stack options
		uc.ManageHEICJPG = cfg.StackOptions.ManageHEICJPG
		uc.ManageRawJPG = cfg.StackOptions.ManageRawJPG
		uc.ManageBurst = cfg.StackOptions.ManageBurst
		uc.ManageEpsonFastFoto = cfg.StackOptions.ManageEpsonFastFoto

		// Copy server config to client
		uc.client.Server = cfg.Server.Server
		uc.client.APIKey = cfg.Server.APIKey
		uc.client.AdminAPIKey = cfg.Server.AdminAPIKey
		uc.client.APITrace = cfg.Server.APITrace
		uc.client.SkipSSL = cfg.Server.SkipSSL
		uc.client.ClientTimeout = cfg.Server.ClientTimeout
		uc.client.DeviceUUID = cfg.Server.DeviceUUID
		uc.client.TimeZone = cfg.Server.TimeZone
		uc.client.DryRun = cfg.Server.DryRun
		uc.client.PauseImmichBackgroundJobs = cfg.Server.PauseJobs

		// Initialize application
		a.EnsureFileProcessor()
		if tz, err := cliupload.ParseTimeZone(cfg.Server.TimeZone); err == nil {
			a.SetTZ(tz)
		}

		return uc.run(cmd, args)
	})
}

// run creates the appropriate adapter based on config source mode and executes the upload.
func (uc *UpCmd) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var adapter adapters.Reader
	var err error

	// Select and create the appropriate adapter based on config source mode
	switch uc.config.SourceMode {
	case uploadcfg.SourceModeFromImmich:
		uc.Mode = UpModeFolder
		adapter, err = uc.fromImmichCmd.NewAdapter(ctx, uc.app)
		if err != nil {
			return fmt.Errorf("failed to create immich adapter: %w", err)
		}

	case uploadcfg.SourceModeGoogle:
		uc.Mode = UpModeGoogleTakeout
		adapter, err = uc.googleCmd.NewAdapter(uc.app, args)
		if err != nil {
			return fmt.Errorf("failed to create google photos adapter: %w", err)
		}
		defer uc.googleCmd.Close()

	case uploadcfg.SourceModeICloud:
		uc.Mode = UpModeICloud
		adapter, err = uc.folderCmd.NewAdapter(cmd, uc.app, args, folder.SourceModeICloud)
		if err != nil {
			return fmt.Errorf("failed to create icloud adapter: %w", err)
		}
		defer uc.folderCmd.Close()

	case uploadcfg.SourceModePicasa:
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

// Run executes the upload with the given adapter.
func (uc *UpCmd) Run(cmd *cobra.Command, adapter adapters.Reader) error {
	ctx := cmd.Context()
	err := uc.client.Open(ctx, uc.app)
	if err != nil {
		return err
	}
	uc.tz = uc.app.GetTZ()
	uc.app.SetSupportedMedia(uc.client.Immich.SupportedMedia())

	// Initialize the FileProcessor if not already done
	uc.app.EnsureFileProcessor()

	// Set session tag from config
	if uc.config.SessionTag {
		uc.session = fmt.Sprintf("{immich-go}/%s", time.Now().Format("2006-01-02 15:04:05"))
	}

	// Set up groupers from config stack options
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
