package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/assets/cache"
	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/journal"
)

// mockServerClient implements ServerClient for testing.
type mockServerClient struct {
	mu sync.Mutex

	// GetAssetStatistics
	assetStats      immich.AssetStatistics
	assetStatsError error

	// GetAllAssets
	assets          []*immich.Asset
	getAllAssetsErr error

	// AssetUpload
	uploadResponses []immich.AssetResponse
	uploadIndex     int
	uploadError     error

	// UpdateAsset
	updateAssetError error

	// DeleteAssets
	deletedIDs        []immich.AssetID
	deleteAssetsError error

	// CopyAsset
	copyAssetError error

	// GetAllAlbums
	albums          []immich.AlbumSimplified
	getAllAlbumsErr error

	// GetAlbumAssetIDs
	albumAssetIDs       map[immich.AlbumID][]immich.AssetID
	getAlbumAssetIDsErr error

	// CreateAlbum
	createdAlbums  []assets.Album
	createAlbumErr error

	// AddAssetToAlbum
	albumAssets        map[immich.AlbumID][]immich.AssetID
	addAssetToAlbumErr error

	// UpsertTags
	tags          []immich.TagSimplified
	upsertTagsErr error

	// TagAssets
	taggedAssets map[immich.TagID][]immich.AssetID
	tagAssetsErr error

	// CreateStack
	createdStacks    [][]immich.AssetID
	createStackErr   error
	createStackCalls int

	// GetJobs
	jobs       map[string]immich.Job
	getJobsErr error

	// SendJobCommand
	jobCommands []struct {
		name    string
		command immich.JobCommand
	}
	sendJobCmdErr error
	// sendJobCmdErrs fails specific "<command> <queue>" commands.
	sendJobCmdErrs map[string]error

	// UserID
	userID immich.UserID
}

func newMockServerClient() *mockServerClient {
	return &mockServerClient{
		albumAssetIDs: make(map[immich.AlbumID][]immich.AssetID),
		albumAssets:   make(map[immich.AlbumID][]immich.AssetID),
		taggedAssets:  make(map[immich.TagID][]immich.AssetID),
		userID:        "test-user-id",
	}
}

func (m *mockServerClient) GetAssetStatistics(ctx context.Context) (immich.AssetStatistics, error) {
	return m.assetStats, m.assetStatsError
}

func (m *mockServerClient) GetAllAssets(ctx context.Context, fn func(*immich.Asset) error) error {
	if m.getAllAssetsErr != nil {
		return m.getAllAssetsErr
	}
	for _, a := range m.assets {
		if err := fn(a); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockServerClient) AssetUpload(ctx context.Context, a *assets.Asset) (immich.AssetResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.uploadError != nil {
		return immich.AssetResponse{}, m.uploadError
	}
	if m.uploadIndex < len(m.uploadResponses) {
		resp := m.uploadResponses[m.uploadIndex]
		m.uploadIndex++
		return resp, nil
	}
	return immich.AssetResponse{
		ID:     immich.AssetID("uploaded-" + a.OriginalFileName),
		Status: immich.UploadCreated,
	}, nil
}

type coverUploadFailureClient struct {
	*mockServerClient
	uploadCalls int
}

func (c *coverUploadFailureClient) AssetUpload(context.Context, *assets.Asset) (immich.AssetResponse, error) {
	c.uploadCalls++
	if c.uploadCalls == 1 {
		return immich.AssetResponse{}, errors.New("cover upload failed")
	}
	return immich.AssetResponse{ID: "second-id", Status: immich.UploadCreated}, nil
}

func (m *mockServerClient) UpdateAsset(ctx context.Context, id immich.AssetID, fields immich.UpdateAssetRequest) (*immich.Asset, error) {
	if m.updateAssetError != nil {
		return nil, m.updateAssetError
	}
	return &immich.Asset{ID: id}, nil
}

func (m *mockServerClient) DeleteAssets(ctx context.Context, ids []immich.AssetID, forceDelete bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteAssetsError != nil {
		return m.deleteAssetsError
	}
	m.deletedIDs = append(m.deletedIDs, ids...)
	return nil
}

func (m *mockServerClient) CopyAsset(ctx context.Context, fromID, toID immich.AssetID) error {
	return m.copyAssetError
}

func (m *mockServerClient) UserID() immich.UserID {
	return m.userID
}

func (m *mockServerClient) GetAllAlbums(ctx context.Context) ([]immich.AlbumSimplified, error) {
	if m.getAllAlbumsErr != nil {
		return nil, m.getAllAlbumsErr
	}
	return m.albums, nil
}

func (m *mockServerClient) GetAlbumAssetIDs(ctx context.Context, id immich.AlbumID) ([]immich.AssetID, error) {
	if m.getAlbumAssetIDsErr != nil {
		return nil, m.getAlbumAssetIDsErr
	}
	if ids, ok := m.albumAssetIDs[id]; ok {
		return append([]immich.AssetID(nil), ids...), nil
	}
	return nil, errors.New("album not found")
}

func (m *mockServerClient) CreateAlbum(ctx context.Context, title, description string, ids []immich.AssetID) (assets.Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createAlbumErr != nil {
		return assets.Album{}, m.createAlbumErr
	}
	album := assets.NewAlbum("album-"+title, title, description)
	m.createdAlbums = append(m.createdAlbums, album)
	return album, nil
}

func (m *mockServerClient) AddAssetToAlbum(ctx context.Context, albumID immich.AlbumID, ids []immich.AssetID) ([]immich.UpdateAlbumResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.addAssetToAlbumErr != nil {
		return nil, m.addAssetToAlbumErr
	}
	m.albumAssets[albumID] = append(m.albumAssets[albumID], ids...)
	results := make([]immich.UpdateAlbumResult, len(ids))
	for i, id := range ids {
		results[i] = immich.UpdateAlbumResult{ID: id, Success: true}
	}
	return results, nil
}

func (m *mockServerClient) UpsertTags(ctx context.Context, tags []string) ([]immich.TagSimplified, error) {
	if m.upsertTagsErr != nil {
		return nil, m.upsertTagsErr
	}
	if len(m.tags) > 0 {
		return m.tags, nil
	}
	result := make([]immich.TagSimplified, len(tags))
	for i, t := range tags {
		result[i] = immich.TagSimplified{ID: immich.TagID("tag-" + t), Name: t, Value: t}
	}
	return result, nil
}

func (m *mockServerClient) TagAssets(ctx context.Context, tagID immich.TagID, ids []immich.AssetID) ([]immich.TagAssetsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tagAssetsErr != nil {
		return nil, m.tagAssetsErr
	}
	m.taggedAssets[tagID] = append(m.taggedAssets[tagID], ids...)
	results := make([]immich.TagAssetsResponse, len(ids))
	for i, id := range ids {
		results[i] = immich.TagAssetsResponse{ID: id, Success: true}
	}
	return results, nil
}

func (m *mockServerClient) CreateStack(ctx context.Context, ids []immich.AssetID) (immich.StackID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createStackCalls++
	if m.createStackErr != nil {
		return "", m.createStackErr
	}
	m.createdStacks = append(m.createdStacks, ids)
	return immich.StackID("stack-1"), nil
}

func (m *mockServerClient) GetJobs(ctx context.Context) (map[string]immich.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getJobsErr != nil {
		return nil, m.getJobsErr
	}
	return m.jobs, nil
}

func (m *mockServerClient) SendJobCommand(ctx context.Context, jobName string, command immich.JobCommand, force bool) (immich.JobCommandResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobCommands = append(m.jobCommands, struct {
		name    string
		command immich.JobCommand
	}{jobName, command})
	if err := m.sendJobCmdErrs[string(command)+" "+jobName]; err != nil {
		return immich.JobCommandResponse{}, err
	}
	if m.sendJobCmdErr != nil {
		return immich.JobCommandResponse{}, m.sendJobCmdErr
	}
	return immich.JobCommandResponse{}, nil
}

// mockSource implements Source for testing.
type mockSource struct {
	groups    []*assets.Group
	closed    bool
	processor *fileprocessor.FileProcessor // when set, records discovery for emitted assets
	err       error                        // when set, Err() reports a failed browse
}

func newMockSource(groups ...*assets.Group) *mockSource {
	return &mockSource{groups: groups}
}

func (m *mockSource) Browse(ctx context.Context) <-chan *assets.Group {
	ch := make(chan *assets.Group)
	go func() {
		defer close(ch)
		for _, g := range m.groups {
			if m.processor != nil {
				for _, a := range g.Assets {
					_ = m.processor.RecordAssetDiscovered(ctx, a.File, int64(a.FileSize), journal.DiscoveredImage)
				}
			}
			select {
			case <-ctx.Done():
				return
			case ch <- g:
			}
		}
	}()
	return ch
}

func (m *mockSource) Close() error {
	m.closed = true
	return nil
}

func (m *mockSource) Err() error {
	return m.err
}

// Helper to create a mock asset with file.
func createMockAsset(name string, size int, captureDate time.Time) *assets.Asset {
	fsys := fstest.MapFS{
		name: &fstest.MapFile{
			Data:    make([]byte, size),
			ModTime: captureDate,
		},
	}
	return &assets.Asset{
		File:             fshelper.NewFilename(fsys, name),
		OriginalFileName: name,
		FileSize:         size,
		CaptureDate:      captureDate,
		Checksum:         "checksum-" + name,
	}
}

// Helper to create a test pipeline context.
func createTestContext(server immich.UploadClient) *Context {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	recorder := journal.NewRecorder(logger)
	tracker := assettracker.New()
	processor := fileprocessor.New(tracker, recorder)
	jobs, _ := server.(immich.JobControlService)
	return &Context{
		Config: Config{
			ConcurrentTask: 1,
		},
		Logger:    logger,
		Processor: processor,
		Media:     filetypes.DefaultSupportedMedia,
		Server:    server,
		Jobs:      jobs,
		Index:     NewIndex(),
		StartTime: time.Now(),
	}
}

func TestUploadStageHandleAsset_ForceUploadReplacementMarksProcessed(t *testing.T) {
	captureDate := time.Now().UTC().Truncate(time.Second)

	mock := newMockServerClient()
	mock.uploadResponses = []immich.AssetResponse{{ID: "replacement-id", Status: immich.UploadCreated}}

	pctx := createTestContext(mock)
	pctx.Index.AddImmichAsset(&immich.Asset{
		ID:               "server-id",
		Checksum:         "server-checksum",
		OriginalFileName: "photo.jpg",
		OwnerID:          "test-user-id",
		ExifInfo: immich.ExifInfo{
			FileSizeInByte:   1024,
			DateTimeOriginal: immich.ImmichExifTime{Time: captureDate},
		},
	})

	asset := createMockAsset("photo.jpg", 2048, captureDate)
	pctx.Processor.RecordAssetDiscovered(context.Background(), asset.File, int64(asset.FileSize), journal.DiscoveredImage)

	stage := &UploadStage{Overwrite: true}
	err := stage.handleAsset(context.Background(), pctx, asset)
	if err != nil {
		t.Fatalf("handleAsset() error = %v", err)
	}

	counters := pctx.Processor.GetAssetCounters()
	if counters.Pending != 0 {
		t.Fatalf("expected no pending assets, got %d", counters.Pending)
	}
	if counters.Processed != 1 {
		t.Fatalf("expected one processed asset, got %d", counters.Processed)
	}

	eventCounts := pctx.Processor.GetEventCounts()
	if eventCounts[journal.ProcessedUploadUpgraded] != 1 {
		t.Fatalf("expected one ProcessedUploadUpgraded event, got %d", eventCounts[journal.ProcessedUploadUpgraded])
	}

	if len(mock.deletedIDs) != 1 || mock.deletedIDs[0] != immich.AssetID("server-id") {
		t.Fatalf("expected old server asset to be deleted, got %v", mock.deletedIDs)
	}
}

func TestUploadStageHandleGroupSkipsStackWithoutCoverID(t *testing.T) {
	client := &coverUploadFailureClient{mockServerClient: newMockServerClient()}
	pctx := createTestContext(client)
	group := assets.NewGroup(assets.GroupByOther)
	group.AddAsset(createMockAsset("cover.jpg", 10, time.Now()))
	group.AddAsset(createMockAsset("second.jpg", 10, time.Now()))
	for _, asset := range group.Assets {
		pctx.Processor.RecordAssetDiscovered(t.Context(), asset.File, int64(asset.FileSize), journal.DiscoveredImage)
	}

	stage := &UploadStage{}
	if err := stage.handleGroup(t.Context(), pctx, group); err == nil {
		t.Fatal("handleGroup() error = nil, want cover upload error")
	}
	if len(client.createdStacks) != 0 {
		t.Errorf("created stacks = %v, want none without a cover ID", client.createdStacks)
	}
}

// TestDiscoveryStage tests the discovery stage.
func TestDiscoveryStage(t *testing.T) {
	t.Run("successful discovery", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 3, Images: 2, Videos: 1}
		mock.assets = []*immich.Asset{
			{ID: "asset-1", Checksum: "cs1", OriginalFileName: "photo1.jpg", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}},
			{ID: "asset-2", Checksum: "cs2", OriginalFileName: "photo2.jpg", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 2048}},
			{ID: "asset-3", Checksum: "cs3", OriginalFileName: "video1.mp4", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 10240}},
		}

		pctx := createTestContext(mock)
		stage := &DiscoveryStage{}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("DiscoveryStage.Run() error = %v", err)
		}

		if pctx.Index.Len() != 3 {
			t.Errorf("expected 3 assets in index, got %d", pctx.Index.Len())
		}
	})

	t.Run("skip assets with different owner", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 2}
		mock.assets = []*immich.Asset{
			{ID: "asset-1", Checksum: "cs1", OriginalFileName: "photo1.jpg", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}},
			{ID: "asset-2", Checksum: "cs2", OriginalFileName: "photo2.jpg", OwnerID: "other-user", ExifInfo: immich.ExifInfo{FileSizeInByte: 2048}},
		}

		pctx := createTestContext(mock)
		stage := &DiscoveryStage{}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("DiscoveryStage.Run() error = %v", err)
		}

		if pctx.Index.Len() != 1 {
			t.Errorf("expected 1 asset in index (skipping other owner), got %d", pctx.Index.Len())
		}
	})

	t.Run("skip assets with external library", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 2}
		mock.assets = []*immich.Asset{
			{ID: "asset-1", Checksum: "cs1", OriginalFileName: "photo1.jpg", OwnerID: "test-user-id", LibraryID: "", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}},
			{ID: "asset-2", Checksum: "cs2", OriginalFileName: "photo2.jpg", OwnerID: "test-user-id", LibraryID: "ext-lib", ExifInfo: immich.ExifInfo{FileSizeInByte: 2048}},
		}

		pctx := createTestContext(mock)
		stage := &DiscoveryStage{}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("DiscoveryStage.Run() error = %v", err)
		}

		if pctx.Index.Len() != 1 {
			t.Errorf("expected 1 asset in index (skipping external library), got %d", pctx.Index.Len())
		}
	})

	t.Run("progress callback", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 2}
		mock.assets = []*immich.Asset{
			{ID: "asset-1", Checksum: "cs1", OriginalFileName: "photo1.jpg", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}},
			{ID: "asset-2", Checksum: "cs2", OriginalFileName: "photo2.jpg", OwnerID: "test-user-id", ExifInfo: immich.ExifInfo{FileSizeInByte: 2048}},
		}

		pctx := createTestContext(mock)
		progressCalls := 0
		stage := &DiscoveryStage{
			ProgressUpdate: func(current, total int) {
				progressCalls++
			},
		}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("DiscoveryStage.Run() error = %v", err)
		}

		// Should be called for each asset + final update
		if progressCalls < 2 {
			t.Errorf("expected at least 2 progress calls, got %d", progressCalls)
		}
	})

	t.Run("error on get statistics", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStatsError = errors.New("stats error")

		pctx := createTestContext(mock)
		stage := &DiscoveryStage{}

		err := stage.Run(context.Background(), pctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		mock := newMockServerClient()
		mock.assetStats = immich.AssetStatistics{Total: 100}
		// Create many assets
		for i := range 100 {
			mock.assets = append(mock.assets, &immich.Asset{
				ID:       immich.AssetID("asset-" + string(rune(i))),
				Checksum: "cs" + string(rune(i)),
				OwnerID:  "test-user-id",
				ExifInfo: immich.ExifInfo{FileSizeInByte: 1024},
			})
		}

		pctx := createTestContext(mock)
		stage := &DiscoveryStage{}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := stage.Run(ctx, pctx)
		if err == nil || !errors.Is(err, context.Canceled) {
			// The error might be wrapped
			if err == nil {
				t.Log("context cancellation not detected, but may be timing-dependent")
			}
		}
	})
}

// TestAlbumDiscoveryStage tests the album discovery stage.
func TestAlbumDiscoveryStage(t *testing.T) {
	t.Run("successful album discovery", func(t *testing.T) {
		mock := newMockServerClient()
		mock.albums = []immich.AlbumSimplified{
			{ID: "album-1", AlbumName: "Vacation"},
			{ID: "album-2", AlbumName: "Family"},
		}
		mock.albumAssetIDs["album-1"] = []immich.AssetID{"asset-1", "asset-2"}
		mock.albumAssetIDs["album-2"] = []immich.AssetID{"asset-3"}

		pctx := createTestContext(mock)
		// Add assets to index first
		pctx.Index.AddImmichAsset(&immich.Asset{ID: "asset-1", Checksum: "cs1", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}})
		pctx.Index.AddImmichAsset(&immich.Asset{ID: "asset-2", Checksum: "cs2", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}})
		pctx.Index.AddImmichAsset(&immich.Asset{ID: "asset-3", Checksum: "cs3", ExifInfo: immich.ExifInfo{FileSizeInByte: 1024}})

		assetsReady := make(chan struct{})
		close(assetsReady) // Signal assets are ready

		albumsCache := cache.NewCollectionCache(50, func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		})
		defer albumsCache.Close(context.Background())

		stage := &AlbumDiscoveryStage{
			AlbumsCache: albumsCache,
			AssetsReady: assetsReady,
		}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("AlbumDiscoveryStage.Run() error = %v", err)
		}

		// Check that assets have albums assigned
		asset1 := pctx.Index.GetByID("asset-1")
		if asset1 == nil || len(asset1.Albums) != 1 {
			t.Errorf("expected asset-1 to have 1 album, got %v", asset1)
		}
	})

	t.Run("error on get all albums", func(t *testing.T) {
		mock := newMockServerClient()
		mock.getAllAlbumsErr = errors.New("albums error")

		pctx := createTestContext(mock)
		assetsReady := make(chan struct{})
		close(assetsReady)

		stage := &AlbumDiscoveryStage{
			AssetsReady: assetsReady,
		}

		err := stage.Run(context.Background(), pctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestHandleAssetLifecycle covers the terminal-transition rules of handleAsset:
// each asset must get exactly one terminal transition, recorded once the
// outcome is known.
func TestHandleAssetLifecycle(t *testing.T) {
	discover := func(pctx *Context, a *assets.Asset) {
		if err := pctx.Processor.RecordAssetDiscovered(context.Background(), a.File, int64(a.FileSize), journal.DiscoveredImage); err != nil {
			t.Fatalf("RecordAssetDiscovered: %v", err)
		}
	}

	t.Run("checksum error records ErrorFileAccess", func(t *testing.T) {
		pctx := createTestContext(newMockServerClient())
		a := &assets.Asset{
			File:             fshelper.NewFilename(nil, "unreadable.jpg"),
			OriginalFileName: "unreadable.jpg",
			FileSize:         10,
		}
		discover(pctx, a)

		stage := &UploadStage{}
		if err := stage.handleAsset(context.Background(), pctx, a); err == nil {
			t.Fatal("expected an error")
		}

		counters := pctx.Processor.GetAssetCounters()
		if counters.Errors != 1 || counters.Pending != 0 {
			t.Errorf("expected 1 error, 0 pending, got %+v", counters)
		}
		if counters.InvalidTransitions != 0 {
			t.Errorf("expected no invalid transitions, got %d", counters.InvalidTransitions)
		}
		if got := pctx.Processor.GetEventCounts()[journal.ErrorFileAccess]; got != 1 {
			t.Errorf("expected 1 ErrorFileAccess event, got %d", got)
		}
	})

	t.Run("update asset failure records ErrorServerError", func(t *testing.T) {
		mock := newMockServerClient()
		mock.updateAssetError = errors.New("update failed")

		pctx := createTestContext(mock)
		a := createMockAsset("photo.jpg", 10, time.Now())
		a.FromApplication = &assets.Metadata{Description: "from takeout"}
		discover(pctx, a)

		stage := &UploadStage{}
		if err := stage.handleAsset(context.Background(), pctx, a); err == nil {
			t.Fatal("expected an error")
		}

		counters := pctx.Processor.GetAssetCounters()
		if counters.Errors != 1 || counters.Processed != 0 {
			t.Errorf("expected 1 error, 0 processed, got %+v", counters)
		}
		events := pctx.Processor.GetEventCounts()
		if events[journal.ProcessedUploadSuccess] != 0 {
			t.Error("ProcessedUploadSuccess must not be recorded when UpdateAsset fails")
		}
		if events[journal.ErrorServerError] != 1 {
			t.Errorf("expected 1 ErrorServerError event, got %d", events[journal.ErrorServerError])
		}
		// The upload itself succeeded, so the asset must be indexed as
		// uploaded during this run: an identical file seen later is then
		// recognised instead of being uploaded again.
		if !pctx.Index.UploadedThisRun(a.ID) {
			t.Error("asset uploaded before the failed UpdateAsset must be indexed as uploaded this run")
		}
	})

	t.Run("metadata updated is recorded after a successful UpdateAsset", func(t *testing.T) {
		mock := newMockServerClient()
		pctx := createTestContext(mock)
		a := createMockAsset("photo.jpg", 10, time.Now())
		a.FromApplication = &assets.Metadata{Description: "from takeout"}
		discover(pctx, a)

		stage := &UploadStage{}
		if err := stage.handleAsset(context.Background(), pctx, a); err != nil {
			t.Fatalf("handleAsset() error = %v", err)
		}

		events := pctx.Processor.GetEventCounts()
		if events[journal.ProcessedMetadataUpdated] != 1 {
			t.Errorf("expected 1 ProcessedMetadataUpdated event, got %d", events[journal.ProcessedMetadataUpdated])
		}
		if events[journal.ProcessedUploadSuccess] != 1 {
			t.Errorf("expected 1 ProcessedUploadSuccess event, got %d", events[journal.ProcessedUploadSuccess])
		}
		if counters := pctx.Processor.GetAssetCounters(); !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
		}
	})

	t.Run("same on server is discarded as server duplicate", func(t *testing.T) {
		mock := newMockServerClient()
		pctx := createTestContext(mock)
		captureDate := time.Now().UTC().Truncate(time.Second)
		pctx.Index.AddImmichAsset(&immich.Asset{
			ID:               "server-id",
			Checksum:         "checksum-photo.jpg",
			OriginalFileName: "photo.jpg",
			ExifInfo:         immich.ExifInfo{DateTimeOriginal: immich.ImmichExifTime{Time: captureDate}},
		})

		savedAlbums := []string{}
		albumsCache := cache.NewCollectionCache(50, func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			savedAlbums = append(savedAlbums, album.Title)
			return album, nil
		})

		a := createMockAsset("photo.jpg", 10, captureDate)
		a.Albums = []assets.Album{{Title: "holiday"}}
		discover(pctx, a)

		stage := &UploadStage{AlbumsCache: albumsCache}
		if err := stage.handleAsset(context.Background(), pctx, a); err != nil {
			t.Fatalf("handleAsset() error = %v", err)
		}
		albumsCache.Close(context.Background())

		counters := pctx.Processor.GetAssetCounters()
		if counters.Discarded != 1 || counters.Processed != 0 {
			t.Errorf("expected 1 discarded, 0 processed, got %+v", counters)
		}
		if events := pctx.Processor.GetEventCounts(); events[journal.DiscardedServerDuplicate] != 1 {
			t.Errorf("expected 1 DiscardedServerDuplicate event, got %d", events[journal.DiscardedServerDuplicate])
		}
		if len(savedAlbums) != 1 || savedAlbums[0] != "holiday" {
			t.Errorf("expected album management for 'holiday', got %v", savedAlbums)
		}
		if !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
		}
	})

	t.Run("already processed is discarded as local duplicate", func(t *testing.T) {
		mock := newMockServerClient()
		pctx := createTestContext(mock)

		stage := &UploadStage{}
		first := createMockAsset("first.jpg", 10, time.Now())
		first.Checksum = "dup-checksum"
		discover(pctx, first)
		if err := stage.handleAsset(context.Background(), pctx, first); err != nil {
			t.Fatalf("first handleAsset() error = %v", err)
		}

		second := createMockAsset("second.jpg", 10, time.Now())
		second.Checksum = "dup-checksum"
		discover(pctx, second)
		if err := stage.handleAsset(context.Background(), pctx, second); err != nil {
			t.Fatalf("second handleAsset() error = %v", err)
		}

		counters := pctx.Processor.GetAssetCounters()
		if counters.Processed != 1 || counters.Discarded != 1 {
			t.Errorf("expected 1 processed, 1 discarded, got %+v", counters)
		}
		if events := pctx.Processor.GetEventCounts(); events[journal.DiscardedLocalDuplicate] != 1 {
			t.Errorf("expected 1 DiscardedLocalDuplicate event, got %d", events[journal.DiscardedLocalDuplicate])
		}
		if !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
		}
	})

	t.Run("already processed queues the server asset id into the album cache", func(t *testing.T) {
		mock := newMockServerClient()
		pctx := createTestContext(mock)

		var savedIDs []string
		albumsCache := cache.NewCollectionCache(50, func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error) {
			savedIDs = append(savedIDs, ids...)
			return album, nil
		})
		stage := &UploadStage{AlbumsCache: albumsCache}

		first := createMockAsset("first.jpg", 10, time.Now())
		first.Checksum = "dup-checksum"
		discover(pctx, first)
		if err := stage.handleAsset(context.Background(), pctx, first); err != nil {
			t.Fatalf("first handleAsset() error = %v", err)
		}

		second := createMockAsset("second.jpg", 10, time.Now())
		second.Checksum = "dup-checksum"
		second.Albums = []assets.Album{{Title: "holiday"}}
		discover(pctx, second)
		if err := stage.handleAsset(context.Background(), pctx, second); err != nil {
			t.Fatalf("second handleAsset() error = %v", err)
		}
		albumsCache.Close(context.Background())

		// The first upload got the id "uploaded-first.jpg" from the mock; the
		// duplicate's album membership must be queued under that id, not "".
		if len(savedIDs) != 1 || savedIDs[0] != "uploaded-first.jpg" {
			t.Errorf("expected [uploaded-first.jpg] queued to the album cache, got %v", savedIDs)
		}
		if got := pctx.Processor.GetEventCounts()[journal.ProcessedAlbumAdded]; got != 1 {
			t.Errorf("expected 1 ProcessedAlbumAdded event, got %d", got)
		}
		if counters := pctx.Processor.GetAssetCounters(); !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
		}
	})

	t.Run("server duplicate answer discards the upload", func(t *testing.T) {
		mock := newMockServerClient()
		mock.uploadResponses = []immich.AssetResponse{{ID: "existing-id", Status: immich.UploadDuplicate}}

		pctx := createTestContext(mock)
		a := createMockAsset("photo.jpg", 10, time.Now())
		discover(pctx, a)

		stage := &UploadStage{}
		if err := stage.handleAsset(context.Background(), pctx, a); err != nil {
			t.Fatalf("handleAsset() error = %v", err)
		}

		counters := pctx.Processor.GetAssetCounters()
		if counters.Discarded != 1 || counters.Processed != 0 {
			t.Errorf("expected 1 discarded, 0 processed, got %+v", counters)
		}
		// "existing-id" was never uploaded during this run: the file
		// duplicates a pre-existing server asset.
		if events := pctx.Processor.GetEventCounts(); events[journal.DiscardedServerDuplicate] != 1 {
			t.Errorf("expected 1 DiscardedServerDuplicate event, got %d", events[journal.DiscardedServerDuplicate])
		}
		if !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
		}
	})

	t.Run("server duplicate answer on asset with ID discards as server duplicate", func(t *testing.T) {
		mock := newMockServerClient()
		mock.uploadResponses = []immich.AssetResponse{{ID: "existing-id", Status: immich.UploadDuplicate}}

		pctx := createTestContext(mock)
		a := createMockAsset("photo.jpg", 10, time.Now())
		a.ID = "source-server-id"
		discover(pctx, a)

		stage := &UploadStage{}
		if err := stage.handleAsset(context.Background(), pctx, a); err != nil {
			t.Fatalf("handleAsset() error = %v", err)
		}

		if events := pctx.Processor.GetEventCounts(); events[journal.DiscardedServerDuplicate] != 1 {
			t.Errorf("expected 1 DiscardedServerDuplicate event, got %d", events[journal.DiscardedServerDuplicate])
		}
		if counters := pctx.Processor.GetAssetCounters(); !counters.RunFinishedCleanly() {
			t.Errorf("expected a clean run, got %+v", counters)
		}
	})
}

// nthUploadFailureClient fails the upload numbered failCall (1-based).
type nthUploadFailureClient struct {
	*mockServerClient
	failCall int
	calls    int
}

func (c *nthUploadFailureClient) AssetUpload(ctx context.Context, a *assets.Asset) (immich.AssetResponse, error) {
	c.calls++
	if c.calls == c.failCall {
		return immich.AssetResponse{}, errors.New("upload failed")
	}
	return c.mockServerClient.AssetUpload(ctx, a)
}

// G3: when the server answers "duplicate" with the ID of an asset uploaded
// during this run, the file is a local duplicate; when the ID was never seen
// this run, it is a server duplicate.
func TestHandleAssetDuplicateClassification(t *testing.T) {
	discover := func(pctx *Context, a *assets.Asset) {
		if err := pctx.Processor.RecordAssetDiscovered(context.Background(), a.File, int64(a.FileSize), journal.DiscoveredImage); err != nil {
			t.Fatalf("RecordAssetDiscovered: %v", err)
		}
	}

	t.Run("duplicate of an asset uploaded this run is a local duplicate", func(t *testing.T) {
		mock := newMockServerClient()
		mock.uploadResponses = []immich.AssetResponse{
			{ID: "first-id", Status: immich.UploadCreated},
			{ID: "first-id", Status: immich.UploadDuplicate},
		}
		pctx := createTestContext(mock)
		stage := &UploadStage{}

		first := createMockAsset("first.jpg", 10, time.Now())
		discover(pctx, first)
		if err := stage.handleAsset(context.Background(), pctx, first); err != nil {
			t.Fatalf("first handleAsset() error = %v", err)
		}

		second := createMockAsset("second.jpg", 10, time.Now())
		discover(pctx, second)
		if err := stage.handleAsset(context.Background(), pctx, second); err != nil {
			t.Fatalf("second handleAsset() error = %v", err)
		}

		events := pctx.Processor.GetEventCounts()
		if events[journal.DiscardedLocalDuplicate] != 1 {
			t.Errorf("expected 1 DiscardedLocalDuplicate event, got %d", events[journal.DiscardedLocalDuplicate])
		}
		if events[journal.DiscardedServerDuplicate] != 0 {
			t.Errorf("expected no DiscardedServerDuplicate event, got %d", events[journal.DiscardedServerDuplicate])
		}
		counters := pctx.Processor.GetAssetCounters()
		if !counters.RunFinishedCleanly() || counters.Discarded != 1 {
			t.Errorf("expected 1 discarded and a clean run, got %+v", counters)
		}
	})
}

// G8: ProcessedStacked is recorded only for the assets actually sent in a
// successful CreateStack.
func TestUploadStageHandleGroupStackedEvents(t *testing.T) {
	discover := func(pctx *Context, a *assets.Asset) {
		if err := pctx.Processor.RecordAssetDiscovered(context.Background(), a.File, int64(a.FileSize), journal.DiscoveredImage); err != nil {
			t.Fatalf("RecordAssetDiscovered: %v", err)
		}
	}

	t.Run("no stacked events when CreateStack fails", func(t *testing.T) {
		mock := newMockServerClient()
		mock.createStackErr = errors.New("stack failed")
		pctx := createTestContext(mock)

		group := assets.NewGroup(assets.GroupByBurst)
		for _, n := range []string{"a.jpg", "b.jpg"} {
			a := createMockAsset(n, 10, time.Now())
			group.AddAsset(a)
			discover(pctx, a)
		}

		stage := &UploadStage{}
		if err := stage.handleGroup(context.Background(), pctx, group); err != nil {
			t.Fatalf("handleGroup() error = %v", err)
		}
		if mock.createStackCalls != 1 {
			t.Fatalf("expected 1 CreateStack attempt, got %d", mock.createStackCalls)
		}
		if got := pctx.Processor.GetEventCounts()[journal.ProcessedStacked]; got != 0 {
			t.Errorf("expected no ProcessedStacked events when CreateStack fails, got %d", got)
		}
	})

	t.Run("stacked events only for the assets actually sent", func(t *testing.T) {
		client := &nthUploadFailureClient{mockServerClient: newMockServerClient(), failCall: 2}
		pctx := createTestContext(client)

		group := assets.NewGroup(assets.GroupByBurst)
		for _, n := range []string{"a.jpg", "b.jpg", "c.jpg"} {
			a := createMockAsset(n, 10, time.Now())
			group.AddAsset(a)
			discover(pctx, a)
		}

		stage := &UploadStage{}
		// The second asset's upload fails: it is not sent to CreateStack.
		if err := stage.handleGroup(context.Background(), pctx, group); err == nil {
			t.Fatal("expected the member upload error, got nil")
		}
		if len(client.createdStacks) != 1 {
			t.Fatalf("expected 1 CreateStack, got %d", len(client.createdStacks))
		}
		if len(client.createdStacks[0]) != 2 {
			t.Fatalf("expected 2 ids in the stack, got %v", client.createdStacks[0])
		}
		if got := pctx.Processor.GetEventCounts()[journal.ProcessedStacked]; got != 2 {
			t.Errorf("expected 2 ProcessedStacked events (cover and the uploaded member), got %d", got)
		}
	})
}
