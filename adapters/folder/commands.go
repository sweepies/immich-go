package folder

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/adapters/shared"
	"github.com/simulot/immich-go/app"
	cliflags "github.com/simulot/immich-go/internal/cliFlags"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/fshelper"
	"github.com/simulot/immich-go/internal/gen"
	"github.com/simulot/immich-go/internal/groups"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
	"github.com/simulot/immich-go/internal/namematcher"
	"github.com/simulot/immich-go/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SourceMode represents the folder source mode
type SourceMode int

const (
	SourceModeFolder SourceMode = iota
	SourceModeICloud
	SourceModePicasa
)

// ImportFolderCmd represents the flags used for importing assets from a file system.
type ImportFolderCmd struct {
	// CLI flags
	UsePathAsAlbumName     AlbumFolderMode
	AlbumNamePathSeparator string
	ImportIntoAlbum        string
	BannedFiles            namematcher.List
	Recursive              bool
	InclusionFlags         cliflags.InclusionFlags
	IgnoreSideCarFiles     bool
	FolderAsTags           bool
	TakeDateFromFilename   bool
	PicasaAlbum            bool
	ICloudTakeout          bool
	ICloudMemoriesAsAlbums bool
	shared.StackOptions

	// Internal fields
	app                     *app.Application
	processor               *fileprocessor.FileProcessor
	fsyss                   []fs.FS
	tz                      *time.Location
	supportedMedia          filetypes.SupportedMedia
	infoCollector           *filenames.InfoCollector
	pool                    *worker.Pool
	wg                      sync.WaitGroup
	groupers                []groups.Grouper
	requiresDateInformation bool                              // true if we need to read the date from the file for the options
	picasaAlbums            *gen.SyncMap[string, PicasaAlbum] // ap[string]PicasaAlbum
	icloudMetas             *gen.SyncMap[string, iCloudMeta]
	icloudMetaPass          bool
}

func (ifc *ImportFolderCmd) RegisterFlags(flags *pflag.FlagSet, cmd *cobra.Command) {
	ifc.Recursive = true
	ifc.supportedMedia = filetypes.DefaultSupportedMedia
	ifc.UsePathAsAlbumName = FolderModeNone
	ifc.BannedFiles, _ = namematcher.New(shared.DefaultBannedFiles...)

	flags.Var(&ifc.BannedFiles, "ban-file", "Exclude a file based on a pattern (case-insensitive). Can be specified multiple times.")
	flags.StringVar(&ifc.ImportIntoAlbum, "into-album", "", "Specify an album to import all files into")
	flags.Var(&ifc.UsePathAsAlbumName, "folder-as-album", "Import all files in albums defined by the folder structure. Can be set to 'FOLDER' to use the folder name as the album name, or 'PATH' to use the full path as the album name")
	flags.StringVar(&ifc.AlbumNamePathSeparator, "album-path-joiner", " / ", "Specify a string to use when joining multiple folder names to create an album name (e.g. ' ',' - ')")
	flags.BoolVar(&ifc.Recursive, "recursive", true, "Explore the folder and all its sub-folders")
	flags.BoolVar(&ifc.IgnoreSideCarFiles, "ignore-sidecar-files", false, "Don't upload sidecar with the photo.")
	flags.BoolVar(&ifc.FolderAsTags, "folder-as-tags", false, "Use the folder structure as tags, (ex: the file  holiday/summer 2024/file.jpg will have the tag holiday/summer 2024)")
	flags.BoolVar(&ifc.TakeDateFromFilename, "date-from-name", true, "Use the date from the filename if the date isn't available in the metadata (Only for jpg, mp4, heic, dng, cr2, cr3, arw, raf, nef, mov)")

	if cmd.Parent() != nil && cmd.Parent().Name() == "upload" {
		ifc.StackOptions.RegisterFlags(flags)
	}

	ifc.InclusionFlags.RegisterFlags(flags, "") // selection per extension
	ifc.ICloudTakeout = false
	ifc.PicasaAlbum = false
	switch cmd.Name() {
	case "from-picasa":
		flags.BoolVar(&ifc.PicasaAlbum, "album-picasa", true, "Use Picasa album name found in .picasa.ini file")
	case "from-icloud":
		ifc.ICloudTakeout = true
		ifc.PicasaAlbum = false
		cmd.Flags().BoolVar(&ifc.ICloudMemoriesAsAlbums, "memories", false, "Import icloud memories as albums")
	}
}

// RegisterFlagsFlat registers flags for the flattened CLI (without subcommands).
// This is used by the new upload/archive commands that use source mode flags.
func (ifc *ImportFolderCmd) RegisterFlagsFlat(flags *pflag.FlagSet, forUpload bool) {
	ifc.Recursive = true
	ifc.supportedMedia = filetypes.DefaultSupportedMedia
	ifc.UsePathAsAlbumName = FolderModeNone
	ifc.BannedFiles, _ = namematcher.New(shared.DefaultBannedFiles...)

	// Use safe registration to avoid duplicate flag errors when multiple adapters register the same flags
	shared.SafeVar(flags, &ifc.BannedFiles, "ban-file", "Exclude a file based on a pattern (case-insensitive). Can be specified multiple times.")
	shared.SafeStringVar(flags, &ifc.ImportIntoAlbum, "into-album", "", "Specify an album to import all files into")
	shared.SafeVar(flags, &ifc.UsePathAsAlbumName, "folder-as-album", "Import all files in albums defined by the folder structure. Can be set to 'FOLDER' to use the folder name as the album name, or 'PATH' to use the full path as the album name")
	shared.SafeStringVar(flags, &ifc.AlbumNamePathSeparator, "album-path-joiner", " / ", "Specify a string to use when joining multiple folder names to create an album name (e.g. ' ',' - ')")
	shared.SafeBoolVar(flags, &ifc.Recursive, "recursive", true, "Explore the folder and all its sub-folders")
	shared.SafeBoolVar(flags, &ifc.IgnoreSideCarFiles, "ignore-sidecar-files", false, "Don't upload sidecar with the photo.")
	shared.SafeBoolVar(flags, &ifc.FolderAsTags, "folder-as-tags", false, "Use the folder structure as tags, (ex: the file  holiday/summer 2024/file.jpg will have the tag holiday/summer 2024)")
	shared.SafeBoolVar(flags, &ifc.TakeDateFromFilename, "date-from-name", true, "Use the date from the filename if the date isn't available in the metadata (Only for jpg, mp4, heic, dng, cr2, cr3, arw, raf, nef, mov)")

	if forUpload {
		ifc.StackOptions.RegisterFlagsSafe(flags)
	}

	ifc.InclusionFlags.RegisterFlagsSafe(flags, "")

	// Picasa-specific flag
	shared.SafeBoolVar(flags, &ifc.PicasaAlbum, "album-picasa", false, "Use Picasa album name found in .picasa.ini file")

	// iCloud-specific flag
	shared.SafeBoolVar(flags, &ifc.ICloudMemoriesAsAlbums, "memories", false, "Import iCloud memories as albums")
}

// NewAdapter creates a folder adapter with the given configuration.
// This is the factory function for the flattened CLI approach.
func (ifc *ImportFolderCmd) NewAdapter(app *app.Application, args []string, mode SourceMode) (adapters.Reader, error) {
	var err error

	if ifc.ImportIntoAlbum != "" && ifc.UsePathAsAlbumName != FolderModeNone {
		return nil, errors.New("cannot use both --into-album and --folder-as-album flags")
	}

	ifc.app = app
	ifc.processor = app.FileProcessor()
	ifc.tz = app.GetTZ()

	// Set mode-specific options
	switch mode {
	case SourceModeICloud:
		ifc.ICloudTakeout = true
		ifc.PicasaAlbum = false
	case SourceModePicasa:
		ifc.ICloudTakeout = false
		// PicasaAlbum is set via --album-picasa flag, default to true for picasa mode
		if !ifc.PicasaAlbum {
			ifc.PicasaAlbum = true
		}
	default:
		ifc.ICloudTakeout = false
	}

	// Parse arguments and generate a fs.FS per argument
	ifc.fsyss, err = fshelper.ParsePath(args)
	if err != nil {
		return nil, err
	}
	if len(ifc.fsyss) == 0 {
		app.Log().Message("No file found matching the pattern: %s", strings.Join(args, ","))
		return nil, errors.New("no file found matching the pattern: " + strings.Join(args, ","))
	}

	// Start the workers
	ifc.pool = worker.NewPool(ifc.app.ConcurrentTask)

	// Create the adapter for folders
	ifc.supportedMedia = ifc.app.GetSupportedMedia()

	ifc.requiresDateInformation = ifc.InclusionFlags.DateRange.IsSet() ||
		ifc.TakeDateFromFilename || ifc.ManageBurst != filters.BurstNothing ||
		ifc.ManageHEICJPG != filters.HeicJpgNothing || ifc.ManageRawJPG != filters.RawJPGNothing

	if ifc.PicasaAlbum {
		ifc.picasaAlbums = gen.NewSyncMap[string, PicasaAlbum]()
	}
	if ifc.ICloudTakeout {
		ifc.icloudMetas = gen.NewSyncMap[string, iCloudMeta]()
		ifc.icloudMetaPass = true
	}

	if ifc.infoCollector == nil {
		ifc.infoCollector = filenames.NewInfoCollector(ifc.tz, ifc.supportedMedia)
	}

	if ifc.InclusionFlags.DateRange.IsSet() {
		ifc.InclusionFlags.DateRange.SetTZ(ifc.tz)
	}

	if ifc.ManageEpsonFastFoto {
		ifc.groupers = append(ifc.groupers, epsonfastfoto.Group{}.Group)
	}
	if ifc.ManageBurst != filters.BurstNothing {
		ifc.groupers = append(ifc.groupers, burst.Group)
	}
	ifc.groupers = append(ifc.groupers, series.Group)

	return ifc, nil
}

// Close closes any resources held by the adapter
func (ifc *ImportFolderCmd) Close() error {
	if ifc.pool != nil {
		ifc.pool.Stop()
	}
	return fshelper.CloseFSs(ifc.fsyss)
}

func NewFromFolderCommand(ctx context.Context, parent *cobra.Command, app *app.Application, runner adapters.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-folder [flags] <path>...",
		Short: "Upload photos from a folder",
		Args:  cobra.MinimumNArgs(1),
	}
	cmd.SetContext(ctx)
	flags := cmd.Flags()
	o := ImportFolderCmd{}
	o.RegisterFlags(flags, cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return o.run(cmd, args, app, runner)
	}

	return cmd
}

func NewFromICloudCommand(ctx context.Context, parent *cobra.Command, app *app.Application, runner adapters.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-icloud [flags] <path>...",
		Short: "Upload photos from an iCloud takeout folder or zip file",
		Args:  cobra.MinimumNArgs(1),
	}
	cmd.SetContext(ctx)
	flags := cmd.Flags()
	o := ImportFolderCmd{}
	o.RegisterFlags(flags, cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return o.run(cmd, args, app, runner)
	}
	return cmd
}

func NewFromPicasaCommand(ctx context.Context, parent *cobra.Command, app *app.Application, runner adapters.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-picasa [flags] <path>...",
		Short: "Upload photos from a Picasa folder or zip file",
		Args:  cobra.MinimumNArgs(1),
	}
	cmd.SetContext(ctx)
	flags := cmd.Flags()
	o := ImportFolderCmd{}
	o.RegisterFlags(flags, cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return o.run(cmd, args, app, runner)
	}
	return cmd
}
