package immich

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// StackResponse is the server response after creating a stack.
type StackResponse struct {
	ID             StackID `json:"id"`
	PrimaryAssetID AssetID `json:"primaryAssetId"`
}

// CreateStack creates a stack with the first asset as its cover.
func (ic *ImmichClient) CreateStack(ctx context.Context, ids []AssetID) (StackID, error) {
	n := 0
	for _, id := range ids {
		if id != "" {
			ids[n] = id
			n++
		}
	}
	ids = ids[:n]

	if len(ids) < 2 {
		return "", fmt.Errorf("stack must have at least 2 assets")
	}

	if ic.dryRun {
		return StackID(uuid.NewString()), nil
	}

	param := struct {
		AssetIDs []AssetID `json:"assetIds"`
	}{
		AssetIDs: ids,
	}

	var result StackResponse
	err := ic.newServerCall(ctx, "createStack").do(postRequest("/stacks", "application/json", setAcceptJSON(), setJSONBody(param)), responseJSON(&result))
	return result.ID, err
}
