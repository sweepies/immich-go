package immich

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// CreateStack create a stack with the given assets, the 1st asset is the cover, return the stack ID
func (ic *ImmichClient) CreateStack(ctx context.Context, ids []string) (string, error) {
	// remove the empty ids
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
		return uuid.NewString(), nil
	}

	param := struct {
		AssetIds []string `json:"assetIds"`
	}{
		AssetIds: ids,
	}

	var result struct {
		ID             string `json:"id"`
		PrimaryAssetID string `json:"primaryAssetId"`
	}

	err := ic.newServerCall(ctx, "createStack").do(postRequest("/stacks", "application/json", setAcceptJSON(), setJSONBody(param)), responseJSON(&result))
	return result.ID, err
}
