package immich

import "context"

// StacksService provides stack-related server operations.
type StacksService interface {
	// CreateStack creates a stack from multiple assets.
	// The first asset becomes the cover. Returns the stack ID.
	CreateStack(ctx context.Context, assetIDs []AssetID) (StackID, error)
}

// StackResponse represents the server response after creating a stack.
type StackResponse struct {
	ID             StackID `json:"id"`
	PrimaryAssetID AssetID `json:"primaryAssetId"`
}
