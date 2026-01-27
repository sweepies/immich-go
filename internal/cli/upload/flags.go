// Package upload provides CLI flag registration and configuration mapping
// for the upload command. It separates CLI concerns from the upload pipeline.
package upload

import (
	"errors"
	"os"
	"time"

	"github.com/sweepies/immich-go/internal/adapters"
	uploadcfg "github.com/sweepies/immich-go/internal/upload"
	"github.com/spf13/pflag"
)

// Flags holds all CLI flags for the upload command.
// This struct captures command-line values and maps them to configuration.
type Flags struct {
	// Server connection flags
	Server        string
	APIKey        string
	AdminAPIKey   string
	APITrace      bool
	SkipSSL       bool
	ClientTimeout time.Duration
	DeviceUUID    string
	TimeZone      string
	DryRun        bool
	PauseJobs     bool
	NoResumeJobs  bool

	// Upload behavior flags
	Overwrite  bool
	Tags       []string
	SessionTag bool

	// Source mode flags (mutually exclusive)
	GoogleTakeout bool
	ICloudTakeout bool
	PicasaMode    bool
	FromImmich    bool

	// Stack options
	ManageHEICJPG       string
	ManageRawJPG        string
	ManageBurst         string
	ManageEpsonFastFoto bool

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
	TakeoutTag         bool
	TakeoutName        string
	PeopleTag          bool

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
	Make            string
	Model           string
	Country         string
	State           string
	City            string

	// Date range flags
	DateAfter  string
	DateBefore string
}

// NewFlags creates a new Flags with default values.
func NewFlags() *Flags {
	hostname, _ := os.Hostname()
	return &Flags{
		ClientTimeout: 20 * time.Minute,
		DeviceUUID:    hostname,
		PauseJobs:     true,
		Recursive:     true,
	}
}

// RegisterFlags adds all upload-related flags to the flag set.
func (f *Flags) RegisterFlags(flags *pflag.FlagSet) {
	// Server connection flags
	flags.StringVarP(&f.Server, "server", "s", f.Server, "Immich server address (example http://your-ip:2283 or https://your-domain)")
	flags.StringVarP(&f.APIKey, "api-key", "k", "", "API Key")
	flags.StringVar(&f.AdminAPIKey, "admin-api-key", "", "Admin's API Key for managing server's jobs")
	flags.BoolVar(&f.APITrace, "api-trace", false, "Enable trace of api calls")
	flags.BoolVar(&f.SkipSSL, "skip-verify-ssl", false, "Skip SSL verification")
	flags.DurationVar(&f.ClientTimeout, "client-timeout", f.ClientTimeout, "Set server calls timeout")
	flags.StringVar(&f.DeviceUUID, "device-uuid", f.DeviceUUID, "Set a device UUID")
	flags.StringVar(&f.TimeZone, "time-zone", f.TimeZone, "Override the system time zone")
	flags.BoolVar(&f.DryRun, "dry-run", false, "Simulate all actions")
	flags.BoolVar(&f.PauseJobs, "pause-immich-jobs", f.PauseJobs, "Pause Immich background jobs during upload operations")
	flags.BoolVar(&f.NoResumeJobs, "no-resume-jobs", false, "Do not resume Immich background jobs after upload (for testing)")

	// Upload behavior flags
	flags.BoolVar(&f.Overwrite, "overwrite", false, "Always overwrite files on the server with local versions")
	flags.StringSliceVar(&f.Tags, "tag", nil, "Add tags to the imported assets. Can be specified multiple times. Hierarchy is supported using a / separator (e.g. 'tag1/subtag1')")
	flags.BoolVar(&f.SessionTag, "session-tag", false, "Tag uploaded photos with a tag \"{immich-go}/YYYY-MM-DD HH-MM-SS\"")

	// Source mode flags (mutually exclusive)
	flags.BoolVar(&f.GoogleTakeout, "google", false, "Import from Google Photos takeout")
	flags.BoolVar(&f.ICloudTakeout, "icloud", false, "Import from iCloud takeout")
	flags.BoolVar(&f.PicasaMode, "picasa", false, "Enable Picasa album parsing")
	flags.BoolVar(&f.FromImmich, "from-immich", false, "Transfer from another Immich server")

	// Stack options
	flags.StringVar(&f.ManageHEICJPG, "manage-heic-jpeg", "", "Manage coupled HEIC and JPEG files. Possible values: NoStack, KeepHeic, KeepJPG, StackCoverHeic, StackCoverJPG")
	flags.StringVar(&f.ManageRawJPG, "manage-raw-jpeg", "", "Manage coupled RAW and JPEG files. Possible values: NoStack, KeepRaw, KeepJPG, StackCoverRaw, StackCoverJPG")
	flags.StringVar(&f.ManageBurst, "manage-burst", "", "Manage burst photos. Possible values: NoStack, Stack, StackKeepRaw, StackKeepJPEG")
	flags.BoolVar(&f.ManageEpsonFastFoto, "manage-epson-fastfoto", false, "Manage Epson FastFoto file (default: false)")

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
	flags.BoolVar(&f.TakeoutTag, "takeout-tag", false, "Tag with takeout name")
	flags.StringVar(&f.TakeoutName, "takeout-name", "", "Name for takeout tag")
	flags.BoolVar(&f.PeopleTag, "people-tag", false, "Create tags from people")

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
	flags.StringVar(&f.Make, "make", "", "Filter by camera make")
	flags.StringVar(&f.Model, "model", "", "Filter by camera model")
	flags.StringVar(&f.Country, "country", "", "Filter by country")
	flags.StringVar(&f.State, "state", "", "Filter by state")
	flags.StringVar(&f.City, "city", "", "Filter by city")

	// Date range flags
	flags.StringVar(&f.DateAfter, "date-after", "", "Only process files after this date (YYYY-MM-DD)")
	flags.StringVar(&f.DateBefore, "date-before", "", "Only process files before this date (YYYY-MM-DD)")
}

// Validate checks flags for errors before building configuration.
func (f *Flags) Validate(args []string) error {
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
		return errors.New("--picasa can only be used with folder uploads")
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

// ToConfig converts CLI flags to an upload configuration.
func (f *Flags) ToConfig(args []string) (*uploadcfg.Config, error) {
	cfg := uploadcfg.NewConfig()

	// Server configuration
	cfg.Server = uploadcfg.ServerConfig{
		Server:        f.Server,
		APIKey:        f.APIKey,
		AdminAPIKey:   f.AdminAPIKey,
		APITrace:      f.APITrace,
		SkipSSL:       f.SkipSSL,
		ClientTimeout: f.ClientTimeout,
		DeviceUUID:    f.DeviceUUID,
		TimeZone:      f.TimeZone,
		DryRun:        f.DryRun,
		PauseJobs:     f.PauseJobs,
		NoResumeJobs:  f.NoResumeJobs,
	}

	// Upload behavior
	cfg.Overwrite = f.Overwrite
	cfg.Tags = f.Tags
	cfg.SessionTag = f.SessionTag
	cfg.Paths = args

	// Determine source mode
	cfg.SourceMode = f.determineSourceMode()

	// Build adapter-specific configuration
	cfg.FolderConfig = f.buildFolderConfig(args)
	cfg.GoogleConfig = f.buildGoogleConfig(args)
	cfg.FromImmichConfig = f.buildFromImmichConfig()

	// Stack options
	cfg.StackOptions = f.buildStackOptions()

	return cfg, nil
}

func (f *Flags) determineSourceMode() uploadcfg.SourceMode {
	switch {
	case f.FromImmich:
		return uploadcfg.SourceModeFromImmich
	case f.GoogleTakeout:
		return uploadcfg.SourceModeGoogle
	case f.ICloudTakeout:
		return uploadcfg.SourceModeICloud
	case f.PicasaMode:
		return uploadcfg.SourceModePicasa
	default:
		return uploadcfg.SourceModeFolder
	}
}

func (f *Flags) buildFolderConfig(args []string) *adapters.FolderConfig {
	cfg := &adapters.FolderConfig{
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
	return cfg
}

func (f *Flags) buildGoogleConfig(args []string) *adapters.GoogleConfig {
	cfg := &adapters.GoogleConfig{
		CreateAlbums:       f.CreateAlbums,
		ImportFromAlbum:    f.ImportFromAlbum,
		ImportIntoAlbum:    f.ImportIntoAlbum,
		PartnerSharedAlbum: f.PartnerSharedAlbum,
		KeepTrashed:        f.KeepTrashed,
		KeepPartner:        f.KeepPartner,
		KeepUntitled:       f.KeepUntitled,
		KeepArchived:       f.KeepArchived,
		KeepJSONLess:       f.KeepJSONLess,
		TakeoutTag:         f.TakeoutTag,
		TakeoutName:        f.TakeoutName,
		PeopleTag:          f.PeopleTag,
		Paths:              args,
	}
	return cfg
}

func (f *Flags) buildFromImmichConfig() *adapters.FromImmichConfig {
	cfg := &adapters.FromImmichConfig{
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
		Make:            f.Make,
		Model:           f.Model,
		Country:         f.Country,
		State:           f.State,
		City:            f.City,
	}
	return cfg
}

func (f *Flags) buildStackOptions() uploadcfg.StackOptions {
	opts := uploadcfg.StackOptions{
		ManageEpsonFastFoto: f.ManageEpsonFastFoto,
	}

	// Parse string flags into typed values
	if f.ManageHEICJPG != "" {
		_ = opts.ManageHEICJPG.Set(f.ManageHEICJPG)
	}
	if f.ManageRawJPG != "" {
		_ = opts.ManageRawJPG.Set(f.ManageRawJPG)
	}
	if f.ManageBurst != "" {
		_ = opts.ManageBurst.Set(f.ManageBurst)
	}

	return opts
}
