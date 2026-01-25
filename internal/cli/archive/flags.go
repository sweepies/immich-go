// Package archive provides CLI flag registration and configuration mapping
// for the archive command. It separates CLI concerns from the archive execution.
package archive

import (
	"errors"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/spf13/pflag"
)

// SourceMode indicates which adapter to use for reading assets.
type SourceMode = adapters.SourceMode

// Re-export source mode constants for convenience.
const (
	SourceModeFolder     = adapters.SourceModeFolder
	SourceModeICloud     = adapters.SourceModeICloud
	SourceModePicasa     = adapters.SourceModePicasa
	SourceModeGoogle     = adapters.SourceModeGoogle
	SourceModeFromImmich = adapters.SourceModeFromImmich
)

// Config holds all configuration for an archive operation.
type Config struct {
	// Output path
	ArchivePath string

	// Source selection
	SourceMode SourceMode

	// Adapter-specific configuration
	FolderConfig     *adapters.FolderConfig
	GoogleConfig     *adapters.GoogleConfig
	FromImmichConfig *adapters.FromImmichConfig

	// Paths from command line arguments
	Paths []string
}

// Flags holds all CLI flags for the archive command.
type Flags struct {
	// Required output path
	ArchivePath string

	// Source mode flags (mutually exclusive)
	GoogleTakeout bool
	ICloudTakeout bool
	PicasaMode    bool
	FromImmich    bool

	// Folder adapter flags
	UsePathAsAlbumName     string
	AlbumNamePathSeparator string
	ImportIntoAlbum        string
	BannedFiles            []string
	Recursive              bool
	IgnoreSideCarFiles     bool
	FolderAsTags           bool
	TakeDateFromFilename   bool

	// Google adapter flags
	CreateAlbums       bool
	ImportFromAlbum    string
	PartnerSharedAlbum string
	KeepTrashed        bool
	KeepPartner        bool
	KeepUntitled       bool
	KeepArchived       bool
	KeepJSONLess       bool

	// From-immich adapter flags
	SourceServer    string
	SourceAPIKey    string
	Albums          []string
	FilterTags      []string
	People          []string
	IncludePartners bool
	OnlyArchived    bool
	OnlyTrashed     bool
	OnlyFavorite    bool
	OnlyNoAlbum     bool
	MinimalRating   int

	// Date range flags
	DateAfter  string
	DateBefore string
}

// NewFlags creates a new Flags with default values.
func NewFlags() *Flags {
	return &Flags{
		Recursive: true,
	}
}

// RegisterFlags adds all archive-related flags to the flag set.
func (f *Flags) RegisterFlags(flags *pflag.FlagSet) {
	// Required output path
	flags.StringVarP(&f.ArchivePath, "write-to-folder", "w", "", "Path where to write the archive")

	// Source mode flags (mutually exclusive)
	flags.BoolVar(&f.GoogleTakeout, "google", false, "Import from Google Photos takeout")
	flags.BoolVar(&f.ICloudTakeout, "icloud", false, "Import from iCloud takeout")
	flags.BoolVar(&f.PicasaMode, "picasa", false, "Enable Picasa album parsing")
	flags.BoolVar(&f.FromImmich, "from-immich", false, "Transfer from another Immich server")

	// Folder adapter flags
	flags.StringVar(&f.UsePathAsAlbumName, "album-from-folder", "", "Use folder name as album name (NONE, FOLDER, PATH)")
	flags.StringVar(&f.AlbumNamePathSeparator, "album-path-separator", "/", "Separator for album path")
	flags.StringVar(&f.ImportIntoAlbum, "album", "", "Import all assets into specified album")
	flags.StringSliceVar(&f.BannedFiles, "exclude-files", nil, "Exclude files matching pattern")
	flags.BoolVar(&f.Recursive, "recursive", f.Recursive, "Recursively scan folders")
	flags.BoolVar(&f.IgnoreSideCarFiles, "ignore-sidecar", false, "Ignore sidecar files")
	flags.BoolVar(&f.FolderAsTags, "folder-as-tags", false, "Use folder path as tags")
	flags.BoolVar(&f.TakeDateFromFilename, "date-from-filename", false, "Try to get date from filename")

	// Google adapter flags
	flags.BoolVar(&f.CreateAlbums, "create-albums", true, "Create albums from takeout")
	flags.StringVar(&f.ImportFromAlbum, "import-from-album", "", "Only import from specific album")
	flags.StringVar(&f.PartnerSharedAlbum, "partner-shared-album", "", "Partner shared album name")
	flags.BoolVar(&f.KeepTrashed, "keep-trashed", false, "Keep trashed photos")
	flags.BoolVar(&f.KeepPartner, "keep-partner", true, "Keep partner shared photos")
	flags.BoolVar(&f.KeepUntitled, "keep-untitled", false, "Keep untitled albums")
	flags.BoolVar(&f.KeepArchived, "keep-archived", true, "Keep archived photos")
	flags.BoolVar(&f.KeepJSONLess, "keep-json-less", false, "Keep photos without JSON metadata")

	// From-immich adapter flags
	flags.StringVar(&f.SourceServer, "source-server", "", "Source Immich server URL")
	flags.StringVar(&f.SourceAPIKey, "source-api-key", "", "Source Immich API key")
	flags.StringSliceVar(&f.Albums, "from-album", nil, "Import from specific albums")
	flags.StringSliceVar(&f.FilterTags, "from-tag", nil, "Import assets with specific tags")
	flags.StringSliceVar(&f.People, "from-person", nil, "Import assets with specific people")
	flags.BoolVar(&f.IncludePartners, "include-partners", false, "Include partner's assets")
	flags.BoolVar(&f.OnlyArchived, "only-archived", false, "Only archived assets")
	flags.BoolVar(&f.OnlyTrashed, "only-trashed", false, "Only trashed assets")
	flags.BoolVar(&f.OnlyFavorite, "only-favorite", false, "Only favorite assets")
	flags.BoolVar(&f.OnlyNoAlbum, "only-no-album", false, "Only assets without album")
	flags.IntVar(&f.MinimalRating, "min-rating", 0, "Minimum rating (0-5)")

	// Date range flags
	flags.StringVar(&f.DateAfter, "date-after", "", "Only process files after this date (YYYY-MM-DD)")
	flags.StringVar(&f.DateBefore, "date-before", "", "Only process files before this date (YYYY-MM-DD)")
}

// Validate checks flags for errors before building configuration.
func (f *Flags) Validate(args []string) error {
	// Validate required output path
	if f.ArchivePath == "" {
		return errors.New("--write-to-folder is required")
	}

	// Validate mutual exclusivity of source flags
	count := 0
	if f.GoogleTakeout {
		count++
	}
	if f.ICloudTakeout {
		count++
	}
	if f.FromImmich {
		count++
	}
	if f.PicasaMode && (f.GoogleTakeout || f.ICloudTakeout || f.FromImmich) {
		return errors.New("--picasa can only be used with folder archives")
	}
	if count > 1 {
		return errors.New("--google, --icloud, and --from-immich are mutually exclusive")
	}

	// Validate path requirements
	if f.FromImmich {
		if len(args) > 0 {
			return errors.New("--from-immich does not accept path arguments")
		}
	} else {
		if len(args) < 1 {
			return errors.New("requires at least one path argument")
		}
	}

	return nil
}

// ToConfig converts CLI flags to an archive configuration.
func (f *Flags) ToConfig(args []string) (*Config, error) {
	cfg := &Config{
		ArchivePath: f.ArchivePath,
		SourceMode:  f.determineSourceMode(),
		Paths:       args,
	}

	// Build adapter-specific configuration
	cfg.FolderConfig = f.buildFolderConfig(args)
	cfg.GoogleConfig = f.buildGoogleConfig(args)
	cfg.FromImmichConfig = f.buildFromImmichConfig()

	return cfg, nil
}

func (f *Flags) determineSourceMode() SourceMode {
	switch {
	case f.FromImmich:
		return SourceModeFromImmich
	case f.GoogleTakeout:
		return SourceModeGoogle
	case f.ICloudTakeout:
		return SourceModeICloud
	case f.PicasaMode:
		return SourceModePicasa
	default:
		return SourceModeFolder
	}
}

func (f *Flags) buildFolderConfig(args []string) *adapters.FolderConfig {
	return &adapters.FolderConfig{
		UsePathAsAlbumName:     adapters.AlbumFolderMode(f.UsePathAsAlbumName),
		AlbumNamePathSeparator: f.AlbumNamePathSeparator,
		ImportIntoAlbum:        f.ImportIntoAlbum,
		Recursive:              f.Recursive,
		IgnoreSideCarFiles:     f.IgnoreSideCarFiles,
		FolderAsTags:           f.FolderAsTags,
		TakeDateFromFilename:   f.TakeDateFromFilename,
		PicasaAlbum:            f.PicasaMode,
		ICloudTakeout:          f.ICloudTakeout,
		Paths:                  args,
	}
}

func (f *Flags) buildGoogleConfig(args []string) *adapters.GoogleConfig {
	return &adapters.GoogleConfig{
		CreateAlbums:       f.CreateAlbums,
		ImportFromAlbum:    f.ImportFromAlbum,
		ImportIntoAlbum:    f.ImportIntoAlbum,
		PartnerSharedAlbum: f.PartnerSharedAlbum,
		KeepTrashed:        f.KeepTrashed,
		KeepPartner:        f.KeepPartner,
		KeepUntitled:       f.KeepUntitled,
		KeepArchived:       f.KeepArchived,
		KeepJSONLess:       f.KeepJSONLess,
		Paths:              args,
	}
}

func (f *Flags) buildFromImmichConfig() *adapters.FromImmichConfig {
	return &adapters.FromImmichConfig{
		ServerURL:       f.SourceServer,
		APIKey:          f.SourceAPIKey,
		Albums:          f.Albums,
		Tags:            f.FilterTags,
		People:          f.People,
		IncludePartners: f.IncludePartners,
		OnlyArchived:    f.OnlyArchived,
		OnlyTrashed:     f.OnlyTrashed,
		OnlyFavorite:    f.OnlyFavorite,
		OnlyNoAlbum:     f.OnlyNoAlbum,
		MinimalRating:   f.MinimalRating,
	}
}
