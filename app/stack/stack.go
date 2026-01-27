package stack

import (
	"context"
	"sort"
	"time"

	"github.com/simulot/immich-go/adapters/shared"
	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/immich"
	"github.com/simulot/immich-go/internal/assets"
	clistack "github.com/simulot/immich-go/internal/cli/stack"
	cliflags "github.com/simulot/immich-go/internal/cliFlags"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/groups"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
	"github.com/spf13/cobra"
)

// StackCmd holds the state for stack command execution.
// CLI flags are managed by the internal/cli/stack package.
type StackCmd struct {
	// Stack options (from config)
	StackOptions shared.StackOptions
	DateRange    cliflags.DateRange

	// internal state
	SupportedMedia filetypes.SupportedMedia
	InfoCollector  *filenames.InfoCollector
	TZ             *time.Location
	assets         []*assets.Asset
	client         app.Client
	groupers       []groups.Grouper // groups are used to group assets
	filters        []filters.Filter // filters are used to filter assets in groups

	// Configuration (built from CLI flags)
	config *clistack.Config
}

// NewStackCommandFromCLI creates a stack command using the CLI package.
func NewStackCommandFromCLI(ctx context.Context, a *app.Application) *cobra.Command {
	builder := clistack.NewCommandBuilder()

	return builder.Build(ctx, func(cmd *cobra.Command, args []string, flags *clistack.Flags) error {
		// Build configuration from CLI flags
		cfg, err := flags.ToConfig()
		if err != nil {
			return err
		}

		// Create StackCmd with configuration
		o := &StackCmd{
			config: cfg,
		}

		// Copy stack options from config
		o.StackOptions.ManageHEICJPG = cfg.StackOptions.ManageHEICJPG
		o.StackOptions.ManageRawJPG = cfg.StackOptions.ManageRawJPG
		o.StackOptions.ManageBurst = cfg.StackOptions.ManageBurst
		o.StackOptions.ManageEpsonFastFoto = cfg.StackOptions.ManageEpsonFastFoto

		// Copy server config to client
		o.client.Server = cfg.Server.Server
		o.client.APIKey = cfg.Server.APIKey
		o.client.AdminAPIKey = cfg.Server.AdminAPIKey
		o.client.APITrace = cfg.Server.APITrace
		o.client.SkipSSL = cfg.Server.SkipSSL
		o.client.ClientTimeout = cfg.Server.ClientTimeout
		o.client.DeviceUUID = cfg.Server.DeviceUUID
		o.client.TimeZone = cfg.Server.TimeZone
		o.client.DryRun = cfg.Server.DryRun

		// Open client connection
		if err := o.client.Open(cmd.Context(), a); err != nil {
			return err
		}

		o.TZ = a.GetTZ()
		o.DateRange.SetTZ(a.GetTZ())

		o.InfoCollector = filenames.NewInfoCollector(o.TZ, o.client.Immich.SupportedMedia())
		o.filters = append(o.filters,
			o.StackOptions.ManageBurst.GroupFilter(),
			o.StackOptions.ManageRawJPG.GroupFilter(),
			o.StackOptions.ManageHEICJPG.GroupFilter())

		if o.StackOptions.ManageEpsonFastFoto {
			o.groupers = append(o.groupers, epsonfastfoto.Group{}.Group)
		}
		if o.StackOptions.ManageBurst != filters.BurstNothing {
			o.groupers = append(o.groupers, burst.Group)
		}
		o.groupers = append(o.groupers, series.Group)

		so := immich.SearchOptions().WithExif().WithDateRange(o.DateRange)

		err = o.client.Immich.GetFilteredAssetsFn(cmd.Context(), so,
			func(asset *immich.Asset) error {
				if asset.IsTrashed {
					return nil
				}

				assetData := asset.AsAsset()
				assetData.SetNameInfo(o.InfoCollector.GetInfo(assetData.OriginalFileName))
				assetData.FromApplication = &assets.Metadata{
					FileName:    asset.OriginalFileName,
					Latitude:    asset.ExifInfo.Latitude,
					Longitude:   asset.ExifInfo.Longitude,
					Description: asset.ExifInfo.Description,
					DateTaken:   asset.ExifInfo.DateTimeOriginal.Time,
					Trashed:     asset.IsTrashed,
					Archived:    asset.IsArchived,
					Favorited:   asset.IsFavorite,
					Rating:      byte(asset.Rating),
					Tags:        assetData.Tags,
				}

				o.assets = append(o.assets, assetData)
				return nil
			})
		if err != nil {
			return err
		}
		return o.ProcessAssets(cmd.Context(), a)
	})
}

func (s *StackCmd) ProcessAssets(ctx context.Context, app *app.Application) error {
	log := app.Log()

	in := make(chan *assets.Asset)

	go func() {
		defer close(in)
		// Sort assets by radical, then date
		sort.Slice(s.assets, func(i, j int) bool {
			r1, r2 := s.assets[i].Radical, s.assets[j].Radical
			if r1 != r2 {
				return r1 < r2
			}
			return s.assets[i].CaptureDate.Before(s.assets[j].CaptureDate)
		})
		for _, a := range s.assets {
			select {
			case in <- a:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Group assets
	gChan := groups.NewGrouperPipeline(ctx, s.groupers...).PipeGrouper(ctx, in)

	for g := range gChan {
		g = filters.ApplyFilters(g, s.filters...)
		// Delete filtered assets
		if len(g.Removed) > 0 {
			for _, r := range g.Removed {
				if err := s.client.Immich.DeleteAssets(ctx, []string{r.Asset.ID}, false); err != nil {
					log.Error("can't delete asset %s: %s", r.Asset.OriginalFileName, err)
				} else {
					log.Info("Asset %s deleted: %s", r.Asset.OriginalFileName, r.Reason)
				}
			}
		}

		if len(g.Assets) > 1 && g.Grouping != assets.GroupByNone {
			client := s.client.Immich.(immich.ImmichStackInterface)
			ids := []string{g.Assets[g.CoverIndex].ID}
			for _, a := range g.Assets {
				log.Info("Stacking", "file", a.OriginalFileName)
				if a.ID != ids[0] {
					ids = append(ids, a.ID)
				}
			}
			if len(ids) > 1 {
				if _, err := client.CreateStack(ctx, ids); err != nil {
					log.Error("Can't create stack", "error", err)
				}
			}
		}
	}
	return nil
}

// 	gChan := make(chan *assets.Group)
// 	go func() {
// 		defer close(gChan)
// 		g := assets.NewGroup()
// 		for _, a := range s.assets {
// 			if !g.Add(a) {
// 				gChan <- g
// 				g = assets.NewGroup()
// 				g.Add(a)
// 			}
// 		}
// 		gChan <- g
// 	}
// 	gs := groups.NewGrouperPipeline(ctx, la.groupers...).PipeGrouper(ctx, in)
// 	g = filters.ApplyFilters(g, upCmd.UploadOptions.Filters...)

// filters := 	append( []filters.Filter,)
