package upload

import (
	"context"
	"fmt"
	"time"

	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assets"
	cliupload "github.com/sweepies/immich-go/internal/cli/upload"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/groups"
	"github.com/sweepies/immich-go/internal/groups/burst"
	"github.com/sweepies/immich-go/internal/groups/epsonfastfoto"
	"github.com/sweepies/immich-go/internal/groups/series"
	uploadcfg "github.com/sweepies/immich-go/internal/upload"
	"github.com/sweepies/immich-go/internal/upload/pipeline"
	"github.com/sweepies/immich-go/internal/upload/source"
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
	adapters.StackOptions

	// Server client
	client app.Client

	// Upload command state (runtime)
	tz   *time.Location
	Mode UpLoadMode
	app  *app.Application

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
			app:    a,
			config: cfg,
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

// run creates the appropriate source based on config and executes the upload.
func (uc *UpCmd) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Set mode for display purposes
	switch uc.config.SourceMode {
	case uploadcfg.SourceModeFromImmich:
		uc.Mode = UpModeFolder
	case uploadcfg.SourceModeGoogle:
		uc.Mode = UpModeGoogleTakeout
	case uploadcfg.SourceModeICloud:
		uc.Mode = UpModeICloud
	case uploadcfg.SourceModePicasa:
		uc.Mode = UpModePicasa
	default:
		uc.Mode = UpModeFolder
	}

	return uc.Run(ctx, cmd)
}

// Run executes the upload using the new source factory and pipeline.
func (uc *UpCmd) Run(ctx context.Context, cmd *cobra.Command) error {
	err := uc.client.Open(ctx, uc.app)
	if err != nil {
		return err
	}
	uc.tz = uc.app.GetTZ()
	uc.app.SetSupportedMedia(uc.client.Immich.SupportedMedia())

	// Initialize the FileProcessor if not already done
	uc.app.EnsureFileProcessor()

	// Create source factory
	factory := source.NewFactory(
		uc.app.Log().Logger,
		uc.app.FileProcessor(),
		uc.app.GetSupportedMedia(),
		uc.tz,
		uc.app.ConcurrentTask,
	)

	// Create source from config
	var cfg any
	switch uc.config.SourceMode {
	case uploadcfg.SourceModeFromImmich:
		cfg = uc.config.FromImmichConfig
	case uploadcfg.SourceModeGoogle:
		cfg = uc.config.GoogleConfig
	case uploadcfg.SourceModeICloud, uploadcfg.SourceModePicasa, uploadcfg.SourceModeFolder:
		cfg = uc.config.FolderConfig
	default:
		cfg = uc.config.FolderConfig
	}

	adapterSource, err := factory.CreateFromConfig(ctx, uc.config.SourceMode, cfg)
	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}
	defer adapterSource.Close()

	// Build groupers from config stack options
	var groupers []groups.Grouper
	if uc.ManageEpsonFastFoto {
		g := epsonfastfoto.Group{}
		groupers = append(groupers, g.Group)
	}
	if uc.ManageBurst != filters.BurstNothing {
		groupers = append(groupers, burst.Group)
	}
	groupers = append(groupers, series.Group)

	// Build filters
	filterList := []filters.Filter{
		uc.ManageBurst.GroupFilter(),
		uc.ManageRawJPG.GroupFilter(),
		uc.ManageHEICJPG.GroupFilter(),
	}

	// Create the server client adapter
	serverClient := &pipeline.ServerClientAdapter{
		Immich:      uc.client.Immich,
		AdminImmich: uc.client.AdminImmich,
		User:        uc.client.User,
	}

	// Create pipeline context
	pipelineCfg := pipeline.Config{
		DryRun:         uc.config.Server.DryRun,
		Overwrite:      uc.config.Overwrite,
		ConcurrentTask: uc.app.ConcurrentTask,
		SessionTag:     uc.config.SessionTag,
		Tags:           uc.config.Tags,
		TimeZone:       uc.tz,
		OutputFormat:   uc.app.Output,
	}

	pctx := pipeline.NewContext(
		pipelineCfg,
		uc.app.Log().Logger,
		uc.app.FileProcessor(),
		uc.app.GetSupportedMedia(),
		serverClient,
	)

	// Create save callbacks for albums and tags
	saveAlbum := func(album assets.Album, ids []string) (assets.Album, error) {
		return uc.saveAlbum(ctx, album, ids)
	}
	saveTag := func(tag assets.Tag, ids []string) (assets.Tag, error) {
		return uc.saveTag(ctx, tag, ids)
	}

	// Create and run the pipeline runner
	runner := pipeline.NewRunner(pipeline.RunnerConfig{
		Source:       adapterSource,
		Server:       serverClient,
		PipelineCtx:  pctx,
		Groupers:     groupers,
		Filters:      filterList,
		PauseJobs:    uc.client.PauseImmichBackgroundJobs,
		NoResumeJobs: uc.config.Server.NoResumeJobs,
		OnError:      uc.app.ProcessError,
		SaveAlbum:    saveAlbum,
		SaveTag:      saveTag,
	})

	return runner.Run(ctx)
}

// saveAlbum creates or updates an album on the server.
func (uc *UpCmd) saveAlbum(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
	if len(ids) == 0 {
		return album, nil
	}
	if album.ID == "" {
		r, err := uc.client.Immich.CreateAlbum(ctx, album.Title, album.Description, ids)
		if err != nil {
			uc.app.Log().Error("failed to create album", "err", err, "album", album.Title)
			return album, err
		}
		uc.app.Log().Info("created album", "album", album.Title, "assets", len(ids))
		album.ID = r.ID
		return album, nil
	}
	_, err := uc.client.Immich.AddAssetToAlbum(ctx, album.ID, ids)
	if err != nil {
		uc.app.Log().Error("failed to add assets to album", "err", err, "album", album.Title, "assets", len(ids))
		return album, err
	}
	uc.app.Log().Info("updated album", "album", album.Title, "assets", len(ids))
	return album, err
}

// saveTag creates or updates a tag on the server.
func (uc *UpCmd) saveTag(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error) {
	if len(ids) == 0 {
		return tag, nil
	}
	if tag.ID == "" {
		r, err := uc.client.Immich.UpsertTags(ctx, []string{tag.Value})
		if err != nil {
			uc.app.Log().Error("failed to create tag", "err", err, "tag", tag.Name)
			return tag, err
		}
		uc.app.Log().Info("created tag", "tag", tag.Value)
		tag.ID = r[0].ID
	}
	_, err := uc.client.Immich.TagAssets(ctx, tag.ID, ids)
	if err != nil {
		uc.app.Log().Error("failed to add assets to tag", "err", err, "tag", tag.Value, "assets", len(ids))
		return tag, err
	}
	uc.app.Log().Info("updated tag", "tag", tag.Value, "assets", len(ids))
	return tag, err
}
