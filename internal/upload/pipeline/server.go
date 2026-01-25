package pipeline

import (
	"context"

	"github.com/simulot/immich-go/immich"
	"github.com/simulot/immich-go/internal/assets"
)

// ServerClient defines the interface for interacting with the Immich server.
// It is a narrow interface that includes only the methods needed by the upload pipeline.
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
	GetAssetStatistics(ctx context.Context) (immich.UserStatistics, error)

	// GetAllAssets iterates over all assets on the server.
	GetAllAssets(ctx context.Context, fn func(*immich.Asset) error) error

	// AssetUpload uploads an asset to the server.
	AssetUpload(ctx context.Context, a *assets.Asset) (immich.AssetResponse, error)

	// UpdateAsset updates asset metadata on the server.
	UpdateAsset(ctx context.Context, id string, fields immich.UpdAssetField) (*immich.Asset, error)

	// DeleteAssets deletes assets from the server.
	DeleteAssets(ctx context.Context, ids []string, forceDelete bool) error

	// CopyAsset copies metadata from one asset to another.
	CopyAsset(ctx context.Context, fromID, toID string) error

	// UserID returns the current user's ID.
	UserID() string
}

// AlbumsClient provides album-related server operations.
type AlbumsClient interface {
	// GetAllAlbums returns all albums on the server.
	GetAllAlbums(ctx context.Context) ([]immich.AlbumSimplified, error)

	// GetAlbumInfo returns detailed information about an album.
	GetAlbumInfo(ctx context.Context, id string, withoutAssets bool) (immich.AlbumContent, error)

	// CreateAlbum creates a new album on the server.
	CreateAlbum(ctx context.Context, title, description string, ids []string) (assets.Album, error)

	// AddAssetToAlbum adds assets to an existing album.
	AddAssetToAlbum(ctx context.Context, albumID string, ids []string) ([]immich.UpdateAlbumResult, error)
}

// TagsClient provides tag-related server operations.
type TagsClient interface {
	// UpsertTags creates tags if they don't exist, returns all.
	UpsertTags(ctx context.Context, tags []string) ([]immich.TagSimplified, error)

	// TagAssets applies a tag to assets.
	TagAssets(ctx context.Context, tagID string, ids []string) ([]immich.TagAssetsResponse, error)
}

// StacksClient provides stack-related server operations.
type StacksClient interface {
	// CreateStack creates a stack from multiple assets, returns the stack ID.
	CreateStack(ctx context.Context, ids []string) (string, error)
}

// JobsClient provides server job control operations.
type JobsClient interface {
	// SendJobCommand sends a command to a server job.
	SendJobCommand(ctx context.Context, jobName string, command immich.JobCommand, force bool) (immich.SendJobCommandResponse, error)
}

// ServerClientAdapter adapts the existing immich client to the ServerClient interface.
type ServerClientAdapter struct {
	Immich      immich.ImmichInterface
	AdminImmich immich.ImmichInterface
	User        immich.User
}

// Ensure ServerClientAdapter implements ServerClient.
var _ ServerClient = (*ServerClientAdapter)(nil)

func (a *ServerClientAdapter) GetAssetStatistics(ctx context.Context) (immich.UserStatistics, error) {
	return a.Immich.GetAssetStatistics(ctx)
}

func (a *ServerClientAdapter) GetAllAssets(ctx context.Context, fn func(*immich.Asset) error) error {
	return a.Immich.GetAllAssets(ctx, fn)
}

func (a *ServerClientAdapter) AssetUpload(ctx context.Context, asset *assets.Asset) (immich.AssetResponse, error) {
	return a.Immich.AssetUpload(ctx, asset)
}

func (a *ServerClientAdapter) UpdateAsset(ctx context.Context, id string, fields immich.UpdAssetField) (*immich.Asset, error) {
	return a.Immich.UpdateAsset(ctx, id, fields)
}

func (a *ServerClientAdapter) DeleteAssets(ctx context.Context, ids []string, forceDelete bool) error {
	return a.Immich.DeleteAssets(ctx, ids, forceDelete)
}

func (a *ServerClientAdapter) CopyAsset(ctx context.Context, fromID, toID string) error {
	return a.Immich.CopyAsset(ctx, fromID, toID)
}

func (a *ServerClientAdapter) UserID() string {
	return a.User.ID
}

func (a *ServerClientAdapter) GetAllAlbums(ctx context.Context) ([]immich.AlbumSimplified, error) {
	return a.Immich.GetAllAlbums(ctx)
}

func (a *ServerClientAdapter) GetAlbumInfo(ctx context.Context, id string, withoutAssets bool) (immich.AlbumContent, error) {
	return a.Immich.GetAlbumInfo(ctx, id, withoutAssets)
}

func (a *ServerClientAdapter) CreateAlbum(ctx context.Context, title, description string, ids []string) (assets.Album, error) {
	return a.Immich.CreateAlbum(ctx, title, description, ids)
}

func (a *ServerClientAdapter) AddAssetToAlbum(ctx context.Context, albumID string, ids []string) ([]immich.UpdateAlbumResult, error) {
	return a.Immich.AddAssetToAlbum(ctx, albumID, ids)
}

func (a *ServerClientAdapter) UpsertTags(ctx context.Context, tags []string) ([]immich.TagSimplified, error) {
	return a.Immich.UpsertTags(ctx, tags)
}

func (a *ServerClientAdapter) TagAssets(ctx context.Context, tagID string, ids []string) ([]immich.TagAssetsResponse, error) {
	return a.Immich.TagAssets(ctx, tagID, ids)
}

func (a *ServerClientAdapter) CreateStack(ctx context.Context, ids []string) (string, error) {
	return a.Immich.CreateStack(ctx, ids)
}

func (a *ServerClientAdapter) SendJobCommand(ctx context.Context, jobName string, command immich.JobCommand, force bool) (immich.SendJobCommandResponse, error) {
	return a.AdminImmich.SendJobCommand(ctx, jobName, command, force)
}
