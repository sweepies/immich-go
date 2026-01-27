package immich

import (
	"context"

	"github.com/simulot/immich-go/internal/assets"
)

// TagsService provides tag-related server operations.
type TagsService interface {
	// GetAllTags returns all tags on the server.
	GetAllTags(ctx context.Context) ([]TagSimplified, error)

	// UpsertTags creates tags if they don't exist, returns all.
	UpsertTags(ctx context.Context, tags []string) ([]TagSimplified, error)

	// TagAssets applies a tag to assets.
	TagAssets(ctx context.Context, tagID TagID, assetIDs []AssetID) ([]TagAssetsResponse, error)

	// BulkTagAssets applies multiple tags to multiple assets.
	BulkTagAssets(ctx context.Context, tagIDs []TagID, assetIDs []AssetID) (BulkTagResult, error)
}

// TagSimplified represents a tag from the server.
type TagSimplified struct {
	ID    TagID  `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AsTag converts a TagSimplified to the internal assets.Tag type.
func (ts TagSimplified) AsTag() assets.Tag {
	return assets.Tag{
		ID:    string(ts.ID),
		Name:  ts.Name,
		Value: ts.Value,
	}
}

// TagAssetsResponse represents the result of tagging an asset.
type TagAssetsResponse struct {
	ID      AssetID `json:"id"`
	Success bool    `json:"success"`
	Error   string  `json:"error,omitempty"`
}

// BulkTagResult represents the result of bulk tagging.
type BulkTagResult struct {
	Count int `json:"count"`
}
