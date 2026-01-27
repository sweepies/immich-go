package adapters

import (
	cliflags "github.com/sweepies/immich-go/internal/cliFlags"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/namematcher"
)

// FolderConfig holds configuration for folder-based adapters.
// This includes folder, iCloud, and Picasa sources.
type FolderConfig struct {
	// Album handling
	UsePathAsAlbumName     AlbumFolderMode
	AlbumNamePathSeparator string
	ImportIntoAlbum        string

	// File filtering
	BannedFiles        namematcher.List
	Recursive          bool
	IgnoreSideCarFiles bool
	InclusionFlags     cliflags.InclusionFlags

	// Metadata handling
	FolderAsTags         bool
	TakeDateFromFilename bool

	// Source-specific options
	PicasaAlbum            bool // Picasa mode
	ICloudTakeout          bool // iCloud mode
	ICloudMemoriesAsAlbums bool // iCloud memories as albums

	// Grouping and stacking
	ManageBurst         filters.BurstFlag
	ManageRawJPG        filters.RawJPGFlag
	ManageHEICJPG       filters.HeicJpgFlag
	ManageEpsonFastFoto bool

	// Source paths (files or directories)
	Paths []string
}

// AlbumFolderMode represents the mode in which album folders are organized.
type AlbumFolderMode string

const (
	FolderModeNone   AlbumFolderMode = "NONE"
	FolderModeFolder AlbumFolderMode = "FOLDER"
	FolderModePath   AlbumFolderMode = "PATH"
)

// GoogleConfig holds configuration for Google Photos takeout adapter.
type GoogleConfig struct {
	// Album handling
	CreateAlbums       bool
	ImportFromAlbum    string
	ImportIntoAlbum    string
	PartnerSharedAlbum string

	// Filtering
	KeepTrashed    bool
	KeepPartner    bool
	KeepUntitled   bool
	KeepArchived   bool
	KeepJSONLess   bool
	InclusionFlags cliflags.InclusionFlags
	BannedFiles    namematcher.List

	// Tagging
	TakeoutTag  bool
	TakeoutName string
	PeopleTag   bool

	// Grouping and stacking
	ManageBurst         filters.BurstFlag
	ManageRawJPG        filters.RawJPGFlag
	ManageHEICJPG       filters.HeicJpgFlag
	ManageEpsonFastFoto bool

	// Source paths (files or directories)
	Paths []string
}

// FromImmichConfig holds configuration for the from-immich adapter.
type FromImmichConfig struct {
	// Server connection
	ServerURL string
	APIKey    string

	// Filtering
	Albums          []string
	Tags            []string
	People          []string
	IncludePartners bool
	OnlyArchived    bool
	OnlyTrashed     bool
	OnlyFavorite    bool
	OnlyNoAlbum     bool
	MinimalRating   int

	// Location filtering
	Make    string
	Model   string
	Country string
	State   string
	City    string

	// Date filtering
	InclusionFlags cliflags.InclusionFlags
}

// StackOptions holds common options for stacking/grouping photos.
type StackOptions struct {
	ManageBurst         filters.BurstFlag
	ManageRawJPG        filters.RawJPGFlag
	ManageHEICJPG       filters.HeicJpgFlag
	ManageEpsonFastFoto bool
}
