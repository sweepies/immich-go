package pipeline

import (
	"context"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	iimmich "github.com/sweepies/immich-go/internal/immich"
)

// ServerClient defines the interface for interacting with the Immich server.
// It is a narrow interface that includes only the methods needed by the upload pipeline.
// It uses typed IDs from internal/immich for type safety.
type ServerClient interface {
	AssetsClient
	AlbumsClient
	TagsClient
	StacksClient
	JobsClient
}

// AssetsClient provides asset-related server operations.
type AssetsClient interface {
	// GetAssetStatistics returns statistics about assets on the server.
	GetAssetStatistics(ctx context.Context) (iimmich.AssetStatistics, error)

	// GetAllAssets iterates over all assets on the server.
	GetAllAssets(ctx context.Context, fn func(*iimmich.Asset) error) error

	// AssetUpload uploads an asset to the server.
	AssetUpload(ctx context.Context, a *assets.Asset) (iimmich.AssetResponse, error)

	// UpdateAsset updates asset metadata on the server.
	UpdateAsset(ctx context.Context, id iimmich.AssetID, fields iimmich.UpdateAssetRequest) (*iimmich.Asset, error)

	// DeleteAssets deletes assets from the server.
	DeleteAssets(ctx context.Context, ids []iimmich.AssetID, forceDelete bool) error

	// CopyAsset copies metadata from one asset to another.
	CopyAsset(ctx context.Context, fromID, toID iimmich.AssetID) error

	// UserID returns the current user's ID.
	UserID() iimmich.UserID
}

// AlbumsClient provides album-related server operations.
type AlbumsClient interface {
	// GetAllAlbums returns all albums on the server.
	GetAllAlbums(ctx context.Context) ([]iimmich.AlbumSimplified, error)

	// GetAlbumInfo returns detailed information about an album.
	GetAlbumInfo(ctx context.Context, id iimmich.AlbumID, withoutAssets bool) (iimmich.AlbumContent, error)

	// CreateAlbum creates a new album on the server.
	CreateAlbum(ctx context.Context, title, description string, ids []iimmich.AssetID) (assets.Album, error)

	// AddAssetToAlbum adds assets to an existing album.
	AddAssetToAlbum(ctx context.Context, albumID iimmich.AlbumID, ids []iimmich.AssetID) ([]iimmich.UpdateAlbumResult, error)
}

// TagsClient provides tag-related server operations.
type TagsClient interface {
	// UpsertTags creates tags if they don't exist, returns all.
	UpsertTags(ctx context.Context, tags []string) ([]iimmich.TagSimplified, error)

	// TagAssets applies a tag to assets.
	TagAssets(ctx context.Context, tagID iimmich.TagID, ids []iimmich.AssetID) ([]iimmich.TagAssetsResponse, error)
}

// StacksClient provides stack-related server operations.
type StacksClient interface {
	// CreateStack creates a stack from multiple assets, returns the stack ID.
	CreateStack(ctx context.Context, ids []iimmich.AssetID) (iimmich.StackID, error)
}

// JobsClient provides server job control operations.
type JobsClient interface {
	// SendJobCommand sends a command to a server job.
	SendJobCommand(ctx context.Context, jobName string, command iimmich.JobCommand, force bool) (iimmich.JobCommandResponse, error)
}

// ServerClientAdapter adapts the existing immich client to the ServerClient interface.
// It converts between the old immich package types and the new internal/immich types.
type ServerClientAdapter struct {
	Immich      immich.ImmichInterface
	AdminImmich immich.ImmichInterface
	User        immich.User
}

// Ensure ServerClientAdapter implements ServerClient.
var _ ServerClient = (*ServerClientAdapter)(nil)

func (a *ServerClientAdapter) GetAssetStatistics(ctx context.Context) (iimmich.AssetStatistics, error) {
	stats, err := a.Immich.GetAssetStatistics(ctx)
	if err != nil {
		return iimmich.AssetStatistics{}, err
	}
	return iimmich.AssetStatistics{
		Images: stats.Images,
		Videos: stats.Videos,
		Total:  stats.Total,
	}, nil
}

func (a *ServerClientAdapter) GetAllAssets(ctx context.Context, fn func(*iimmich.Asset) error) error {
	return a.Immich.GetAllAssets(ctx, func(old *immich.Asset) error {
		return fn(convertAsset(old))
	})
}

func (a *ServerClientAdapter) AssetUpload(ctx context.Context, asset *assets.Asset) (iimmich.AssetResponse, error) {
	resp, err := a.Immich.AssetUpload(ctx, asset)
	if err != nil {
		return iimmich.AssetResponse{}, err
	}
	return iimmich.AssetResponse{
		ID:     iimmich.AssetID(resp.ID),
		Status: resp.Status,
	}, nil
}

func (a *ServerClientAdapter) UpdateAsset(ctx context.Context, id iimmich.AssetID, fields iimmich.UpdateAssetRequest) (*iimmich.Asset, error) {
	oldFields := immich.UpdAssetField{
		IsArchived:       fields.IsArchived,
		IsFavorite:       fields.IsFavorite,
		Latitude:         fields.Latitude,
		Longitude:        fields.Longitude,
		Description:      fields.Description,
		Rating:           fields.Rating,
		DateTimeOriginal: fields.DateTimeOriginal,
	}
	resp, err := a.Immich.UpdateAsset(ctx, string(id), oldFields)
	if err != nil {
		return nil, err
	}
	return convertAsset(resp), nil
}

func (a *ServerClientAdapter) DeleteAssets(ctx context.Context, ids []iimmich.AssetID, forceDelete bool) error {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}
	return a.Immich.DeleteAssets(ctx, strIDs, forceDelete)
}

func (a *ServerClientAdapter) CopyAsset(ctx context.Context, fromID, toID iimmich.AssetID) error {
	return a.Immich.CopyAsset(ctx, string(fromID), string(toID))
}

func (a *ServerClientAdapter) UserID() iimmich.UserID {
	return iimmich.UserID(a.User.ID)
}

func (a *ServerClientAdapter) GetAllAlbums(ctx context.Context) ([]iimmich.AlbumSimplified, error) {
	albums, err := a.Immich.GetAllAlbums(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]iimmich.AlbumSimplified, len(albums))
	for i, alb := range albums {
		result[i] = iimmich.AlbumSimplified{
			ID:          iimmich.AlbumID(alb.ID),
			AlbumName:   alb.AlbumName,
			Description: alb.Description,
			AssetIds:    alb.AssetIds,
		}
	}
	return result, nil
}

func (a *ServerClientAdapter) GetAlbumInfo(ctx context.Context, id iimmich.AlbumID, withoutAssets bool) (iimmich.AlbumContent, error) {
	content, err := a.Immich.GetAlbumInfo(ctx, string(id), withoutAssets)
	if err != nil {
		return iimmich.AlbumContent{}, err
	}
	assetList := make([]*iimmich.Asset, len(content.Assets))
	for i, asset := range content.Assets {
		assetList[i] = convertAsset(asset)
	}
	return iimmich.AlbumContent{
		ID:          iimmich.AlbumID(content.ID),
		AlbumName:   content.AlbumName,
		Description: content.Description,
		Shared:      content.Shared,
		Assets:      assetList,
		AssetIDs:    content.AssetIDs,
	}, nil
}

func (a *ServerClientAdapter) CreateAlbum(ctx context.Context, title, description string, ids []iimmich.AssetID) (assets.Album, error) {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}
	return a.Immich.CreateAlbum(ctx, title, description, strIDs)
}

func (a *ServerClientAdapter) AddAssetToAlbum(ctx context.Context, albumID iimmich.AlbumID, ids []iimmich.AssetID) ([]iimmich.UpdateAlbumResult, error) {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}
	results, err := a.Immich.AddAssetToAlbum(ctx, string(albumID), strIDs)
	if err != nil {
		return nil, err
	}
	newResults := make([]iimmich.UpdateAlbumResult, len(results))
	for i, r := range results {
		newResults[i] = iimmich.UpdateAlbumResult{
			ID:      iimmich.AssetID(r.ID),
			Success: r.Success,
			Error:   r.Error,
		}
	}
	return newResults, nil
}

func (a *ServerClientAdapter) UpsertTags(ctx context.Context, tags []string) ([]iimmich.TagSimplified, error) {
	oldTags, err := a.Immich.UpsertTags(ctx, tags)
	if err != nil {
		return nil, err
	}
	result := make([]iimmich.TagSimplified, len(oldTags))
	for i, t := range oldTags {
		result[i] = iimmich.TagSimplified{
			ID:    iimmich.TagID(t.ID),
			Name:  t.Name,
			Value: t.Value,
		}
	}
	return result, nil
}

func (a *ServerClientAdapter) TagAssets(ctx context.Context, tagID iimmich.TagID, ids []iimmich.AssetID) ([]iimmich.TagAssetsResponse, error) {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}
	results, err := a.Immich.TagAssets(ctx, string(tagID), strIDs)
	if err != nil {
		return nil, err
	}
	newResults := make([]iimmich.TagAssetsResponse, len(results))
	for i, r := range results {
		newResults[i] = iimmich.TagAssetsResponse{
			ID:      iimmich.AssetID(r.ID),
			Success: r.Success,
			Error:   r.Error,
		}
	}
	return newResults, nil
}

func (a *ServerClientAdapter) CreateStack(ctx context.Context, ids []iimmich.AssetID) (iimmich.StackID, error) {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}
	id, err := a.Immich.CreateStack(ctx, strIDs)
	return iimmich.StackID(id), err
}

func (a *ServerClientAdapter) SendJobCommand(ctx context.Context, jobName string, command iimmich.JobCommand, force bool) (iimmich.JobCommandResponse, error) {
	resp, err := a.AdminImmich.SendJobCommand(ctx, jobName, immich.JobCommand(command), force)
	if err != nil {
		return iimmich.JobCommandResponse{}, err
	}
	return iimmich.JobCommandResponse{
		JobCounts: struct {
			Active    int `json:"active"`
			Completed int `json:"completed"`
			Delayed   int `json:"delayed"`
			Failed    int `json:"failed"`
			Paused    int `json:"paused"`
			Waiting   int `json:"waiting"`
		}{
			Active:    resp.JobCounts.Active,
			Completed: resp.JobCounts.Completed,
			Delayed:   resp.JobCounts.Delayed,
			Failed:    resp.JobCounts.Failed,
			Paused:    resp.JobCounts.Paused,
			Waiting:   resp.JobCounts.Waiting,
		},
		QueueStatus: struct {
			IsActive bool `json:"isActive"`
			IsPause  bool `json:"isPause"`
		}{
			IsActive: resp.QueueStatus.IsActive,
			IsPause:  resp.QueueStatus.IsPause,
		},
	}, nil
}

// convertAsset converts an old immich.Asset to the new iimmich.Asset type.
func convertAsset(old *immich.Asset) *iimmich.Asset {
	if old == nil {
		return nil
	}
	tags := make([]iimmich.TagSimplified, len(old.Tags))
	for i, t := range old.Tags {
		tags[i] = iimmich.TagSimplified{
			ID:    iimmich.TagID(t.ID),
			Name:  t.Name,
			Value: t.Value,
		}
	}
	albums := make([]iimmich.AlbumSimplified, len(old.Albums))
	for i, a := range old.Albums {
		albums[i] = iimmich.AlbumSimplified{
			ID:          iimmich.AlbumID(a.ID),
			AlbumName:   a.AlbumName,
			Description: a.Description,
			AssetIds:    a.AssetIds,
		}
	}
	return &iimmich.Asset{
		ID:               iimmich.AssetID(old.ID),
		Checksum:         old.Checksum,
		DeviceAssetID:    old.DeviceAssetID,
		DeviceID:         old.DeviceID,
		Duration:         old.Duration,
		ExifInfo:         convertExifInfo(old.ExifInfo),
		FileCreatedAt:    iimmich.ImmichTime{Time: old.FileCreatedAt.Time},
		FileModifiedAt:   iimmich.ImmichTime{Time: old.FileModifiedAt.Time},
		IsArchived:       old.IsArchived,
		IsFavorite:       old.IsFavorite,
		IsTrashed:        old.IsTrashed,
		LibraryID:        old.LibraryID,
		LivePhotoVideoID: iimmich.AssetID(old.LivePhotoVideoID),
		LocalDateTime:    iimmich.ImmichTime{Time: old.LocalDateTime.Time},
		OriginalFileName: old.OriginalFileName,
		OriginalPath:     old.OriginalPath,
		OwnerID:          iimmich.UserID(old.OwnerID),
		Resized:          old.Resized,
		Tags:             tags,
		Thumbhash:        old.Thumbhash,
		Type:             old.Type,
		UpdatedAt:        iimmich.ImmichTime{Time: old.UpdatedAt.Time},
		Rating:           old.Rating,
		Visibility:       old.Visibility,
		Albums:           albums,
	}
}

// convertExifInfo converts old immich.ExifInfo to the new iimmich.ExifInfo type.
func convertExifInfo(old immich.ExifInfo) iimmich.ExifInfo {
	return iimmich.ExifInfo{
		Make:             old.Make,
		Model:            old.Model,
		ExifImageWidth:   old.ExifImageWidth,
		ExifImageHeight:  old.ExifImageHeight,
		FileSizeInByte:   old.FileSizeInByte,
		Orientation:      old.Orientation,
		DateTimeOriginal: iimmich.ImmichExifTime{Time: old.DateTimeOriginal.Time},
		TimeZone:         old.TimeZone,
		Latitude:         old.Latitude,
		Longitude:        old.Longitude,
		Description:      old.Description,
		Rating:           old.Rating,
	}
}
