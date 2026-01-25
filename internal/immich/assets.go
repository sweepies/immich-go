package immich

import (
	"context"
	"io"
	"time"

	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/fshelper"
)

// AssetsService provides asset-related server operations.
type AssetsService interface {
	// GetAssetInfo returns detailed information about an asset.
	GetAssetInfo(ctx context.Context, id AssetID) (*Asset, error)

	// DownloadAsset downloads the original asset file.
	DownloadAsset(ctx context.Context, id AssetID) (io.ReadCloser, error)

	// UpdateAsset updates asset metadata.
	UpdateAsset(ctx context.Context, id AssetID, fields UpdateAssetRequest) (*Asset, error)

	// CopyAsset copies metadata from one asset to another.
	CopyAsset(ctx context.Context, sourceID, targetID AssetID) error

	// GetAllAssets iterates over all assets on the server.
	GetAllAssets(ctx context.Context, fn func(*Asset) error) error

	// GetAssetStatistics returns statistics about the user's assets.
	GetAssetStatistics(ctx context.Context) (AssetStatistics, error)

	// GetAssetsByHash returns assets matching the given checksum.
	GetAssetsByHash(ctx context.Context, hash string) ([]*Asset, error)

	// GetAssetsByImageName returns assets matching the filename.
	GetAssetsByImageName(ctx context.Context, name string) ([]*Asset, error)

	// AssetUpload uploads an asset to the server.
	AssetUpload(ctx context.Context, a *assets.Asset) (AssetResponse, error)

	// DeleteAssets deletes assets from the server.
	DeleteAssets(ctx context.Context, ids []AssetID, forceDelete bool) error
}

// Asset represents an Immich server asset.
type Asset struct {
	ID               AssetID         `json:"id"`
	Checksum         string          `json:"checksum"`
	DeviceAssetID    string          `json:"deviceAssetId"`
	DeviceID         string          `json:"deviceId"`
	Duration         string          `json:"duration"`
	ExifInfo         ExifInfo        `json:"exifInfo"`
	FileCreatedAt    ImmichTime      `json:"fileCreatedAt"`
	FileModifiedAt   ImmichTime      `json:"fileModifiedAt"`
	IsArchived       bool            `json:"isArchived"`
	IsFavorite       bool            `json:"isFavorite"`
	IsTrashed        bool            `json:"isTrashed"`
	LibraryID        string          `json:"libraryId,omitempty"`
	LivePhotoVideoID AssetID         `json:"livePhotoVideoId"`
	LocalDateTime    ImmichTime      `json:"localDateTime"`
	OriginalFileName string          `json:"originalFileName"`
	OriginalPath     string          `json:"originalPath"`
	OwnerID          UserID          `json:"ownerId"`
	Resized          bool            `json:"resized"`
	Tags             []TagSimplified `json:"tags"`
	Thumbhash        string          `json:"thumbhash"`
	Type             string          `json:"type"`
	UpdatedAt        ImmichTime      `json:"updatedAt"`
	Rating           int             `json:"rating"`
	Visibility       string          `json:"visibility"`

	Albums []AlbumSimplified `json:"-"`
}

// AsAsset converts an Immich Asset to the internal assets.Asset type.
func (ia Asset) AsAsset() *assets.Asset {
	a := &assets.Asset{
		FileDate:         ia.FileModifiedAt.Time,
		Description:      ia.ExifInfo.Description,
		OriginalFileName: ia.OriginalFileName,
		ID:               string(ia.ID),
		CaptureDate:      ia.ExifInfo.DateTimeOriginal.Time,
		Trashed:          ia.IsTrashed,
		Archived:         ia.IsArchived,
		Favorite:         ia.IsFavorite,
		Rating:           ia.Rating,
		Latitude:         ia.ExifInfo.Latitude,
		Longitude:        ia.ExifInfo.Longitude,
		File:             fshelper.FSName(nil, ia.OriginalFileName),
		FileSize:         int(ia.ExifInfo.FileSizeInByte),
		Checksum:         ia.Checksum,
	}
	for _, album := range ia.Albums {
		a.Albums = append(a.Albums, assets.Album{
			Title:       album.AlbumName,
			Description: album.Description,
		})
	}
	for _, tag := range ia.Tags {
		a.Tags = append(a.Tags, tag.AsTag())
	}
	return a
}

// ExifInfo contains EXIF metadata for an asset.
type ExifInfo struct {
	Make             string         `json:"make"`
	Model            string         `json:"model"`
	ExifImageWidth   int            `json:"exifImageWidth"`
	ExifImageHeight  int            `json:"exifImageHeight"`
	FileSizeInByte   int64          `json:"fileSizeInByte"`
	Orientation      string         `json:"orientation"`
	DateTimeOriginal ImmichExifTime `json:"dateTimeOriginal,omitempty"`
	TimeZone         string         `json:"timeZone"`
	Latitude         float64        `json:"latitude,omitempty"`
	Longitude        float64        `json:"longitude,omitempty"`
	Description      string         `json:"description"`
	Rating           int            `json:"rating"`
}

// AssetResponse represents the server response after asset operations.
type AssetResponse struct {
	ID     AssetID `json:"id"`
	Status string  `json:"status"`
}

// Upload status constants.
const (
	UploadCreated   = "created"
	UploadReplaced  = "replaced"
	UploadDuplicate = "duplicate"
)

// AssetStatistics contains user asset statistics.
type AssetStatistics struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
	Total  int `json:"total"`
}

// UpdateAssetRequest represents fields that can be updated on an asset.
type UpdateAssetRequest struct {
	IsArchived       bool      `json:"isArchived,omitempty"`
	IsFavorite       bool      `json:"isFavorite,omitempty"`
	Latitude         float64   `json:"latitude,omitempty"`
	Longitude        float64   `json:"longitude,omitempty"`
	Description      string    `json:"description,omitempty"`
	Rating           int       `json:"rating,omitempty"`
	DateTimeOriginal time.Time `json:"dateTimeOriginal,omitempty"`
}
