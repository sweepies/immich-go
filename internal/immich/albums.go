package immich

import (
	"context"

	"github.com/simulot/immich-go/internal/assets"
)

// AlbumsService provides album-related server operations.
type AlbumsService interface {
	// GetAllAlbums returns all albums on the server.
	GetAllAlbums(ctx context.Context) ([]AlbumSimplified, error)

	// GetAlbumInfo returns detailed information about an album.
	GetAlbumInfo(ctx context.Context, id AlbumID, withoutAssets bool) (AlbumContent, error)

	// CreateAlbum creates a new album on the server.
	CreateAlbum(ctx context.Context, title, description string, assetIDs []AssetID) (assets.Album, error)

	// AddAssetToAlbum adds assets to an existing album.
	AddAssetToAlbum(ctx context.Context, albumID AlbumID, assetIDs []AssetID) ([]UpdateAlbumResult, error)

	// GetAssetAlbums returns all albums that contain the given asset.
	GetAssetAlbums(ctx context.Context, assetID AssetID) ([]AlbumSimplified, error)

	// DeleteAlbum deletes an album from the server.
	DeleteAlbum(ctx context.Context, id AlbumID) error
}

// AlbumSimplified represents a simplified album structure.
type AlbumSimplified struct {
	ID          AlbumID  `json:"id,omitempty"`
	AlbumName   string   `json:"albumName"`
	Description string   `json:"description,omitempty"`
	AssetIds    []string `json:"assetIds,omitempty"`
}

// AlbumContent represents an album with its contents.
type AlbumContent struct {
	ID          AlbumID  `json:"id,omitempty"`
	AlbumName   string   `json:"albumName"`
	Description string   `json:"description"`
	Shared      bool     `json:"shared"`
	Assets      []*Asset `json:"assets,omitempty"`
	AssetIDs    []string `json:"assetIds,omitempty"`
}

// UpdateAlbumResult represents the result of adding an asset to an album.
type UpdateAlbumResult struct {
	ID      AssetID `json:"id"`
	Success bool    `json:"success"`
	Error   string  `json:"error,omitempty"`
}

// AlbumsFromSimplified converts AlbumSimplified slice to assets.Album slice.
func AlbumsFromSimplified(albums []AlbumSimplified) []assets.Album {
	result := make([]assets.Album, 0, len(albums))
	for _, a := range albums {
		result = append(result, assets.Album{
			ID:          string(a.ID),
			Title:       a.AlbumName,
			Description: a.Description,
		})
	}
	return result
}
