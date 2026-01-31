package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	iimmich "github.com/sweepies/immich-go/internal/immich"
)

// countingServerClient counts API calls.
type countingServerClient struct {
	*mockServerClient
	uploadCount       int
	updateCount       int
	lastUploadedAsset *assets.Asset
}

func newCountingServerClient() *countingServerClient {
	return &countingServerClient{
		mockServerClient: newMockServerClient(),
	}
}

func (m *countingServerClient) AssetUpload(ctx context.Context, a *assets.Asset) (iimmich.AssetResponse, error) {
	m.uploadCount++
	m.lastUploadedAsset = a
	return m.mockServerClient.AssetUpload(ctx, a)
}

func (m *countingServerClient) UpdateAsset(ctx context.Context, id iimmich.AssetID, fields iimmich.UpdateAssetRequest) (*iimmich.Asset, error) {
	m.updateCount++
	return m.mockServerClient.UpdateAsset(ctx, id, fields)
}

func TestUploadMetadataPerformance(t *testing.T) {
	mock := newCountingServerClient()
	pctx := createTestContext(mock)

	// Create an asset with FromApplication metadata
	asset := createMockAsset("test.jpg", 1000, time.Now())
	asset.FromApplication = &assets.Metadata{
		Description: "Test Description",
		Rating:      5,
		Latitude:    12.34,
		Longitude:   56.78,
	}

	// Setup UploadStage
	stage := &UploadStage{
		Source:      newMockSource(), // Dummy source
		Concurrency: 1,
	}

	// Verify ShouldUpload behavior.
	advice, err := pctx.Index.ShouldUpload(asset, false)
	if err != nil {
		t.Fatalf("ShouldUpload failed: %v", err)
	}
	if advice.Advice != NotOnServer {
		t.Fatalf("Expected NotOnServer, got %v", advice.Advice)
	}

	// Create a group with the asset
	group := &assets.Group{
		Assets: []*assets.Asset{asset},
	}

	// Override the source in the stage to return this group
	stage.Source = newMockSource(group)

	err = stage.Run(context.Background(), pctx)
	if err != nil {
		t.Fatalf("Stage Run failed: %v", err)
	}

	// Verify counts
	if mock.uploadCount != 1 {
		t.Errorf("Expected 1 upload call, got %d", mock.uploadCount)
	}

	// Optimized: Expect 0 update calls
	if mock.updateCount != 0 {
		t.Errorf("Expected 0 update calls, got %d", mock.updateCount)
	}

	// Verify metadata was applied before upload
	if mock.lastUploadedAsset == nil {
		t.Fatal("lastUploadedAsset is nil")
	}
	if mock.lastUploadedAsset.Description != "Test Description" {
		t.Errorf("Expected description 'Test Description', got '%s'", mock.lastUploadedAsset.Description)
	}
	if mock.lastUploadedAsset.Rating != 5 {
		t.Errorf("Expected rating 5, got %d", mock.lastUploadedAsset.Rating)
	}
	// Floating point comparison might need tolerance, but we set it exactly.
	if mock.lastUploadedAsset.Latitude != 12.34 {
		t.Errorf("Expected latitude 12.34, got %f", mock.lastUploadedAsset.Latitude)
	}
}
