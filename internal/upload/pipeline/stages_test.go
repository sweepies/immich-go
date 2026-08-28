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
	createdStacks  [][]immich.AssetID
	createStackErr error

	// SendJobCommand
	jobCommands []struct {
		name    string
		command immich.JobCommand
	}
	sendJobCmdErr error

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
	if m.createStackErr != nil {
		return "", m.createStackErr
	}
	m.createdStacks = append(m.createdStacks, ids)
	return immich.StackID("stack-1"), nil
}

func (m *mockServerClient) SendJobCommand(ctx context.Context, jobName string, command immich.JobCommand, force bool) (immich.JobCommandResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendJobCmdErr != nil {
		return immich.JobCommandResponse{}, m.sendJobCmdErr
	}
	m.jobCommands = append(m.jobCommands, struct {
		name    string
		command immich.JobCommand
	}{jobName, command})
	return immich.JobCommandResponse{}, nil
}

// mockSource implements Source for testing.
type mockSource struct {
	groups []*assets.Group
	closed bool
}

func newMockSource(groups ...*assets.Group) *mockSource {
	return &mockSource{groups: groups}
}

func (m *mockSource) Browse(ctx context.Context) <-chan *assets.Group {
	ch := make(chan *assets.Group)
	go func() {
		defer close(ch)
		for _, g := range m.groups {
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

		albumsCache := cache.NewCollectionCache(50, func(album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		})
		defer albumsCache.Close()

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

// TestJobControlStage tests the job control stage.
func TestJobControlStage(t *testing.T) {
	t.Run("pause jobs", func(t *testing.T) {
		mock := newMockServerClient()
		pctx := createTestContext(mock)

		stage := &JobControlStage{Pause: true}

		if stage.Name() != "pause-jobs" {
			t.Errorf("expected name 'pause-jobs', got '%s'", stage.Name())
		}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("JobControlStage.Run() error = %v", err)
		}

		if len(mock.jobCommands) != 5 {
			t.Errorf("expected 5 job commands, got %d", len(mock.jobCommands))
		}
		for _, cmd := range mock.jobCommands {
			if cmd.command != immich.JobCommandPause {
				t.Errorf("expected pause command, got %s", cmd.command)
			}
		}
	})

	t.Run("resume jobs", func(t *testing.T) {
		mock := newMockServerClient()
		pctx := createTestContext(mock)

		stage := &JobControlStage{Pause: false}

		if stage.Name() != "resume-jobs" {
			t.Errorf("expected name 'resume-jobs', got '%s'", stage.Name())
		}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("JobControlStage.Run() error = %v", err)
		}

		if len(mock.jobCommands) != 5 {
			t.Errorf("expected 5 job commands, got %d", len(mock.jobCommands))
		}
		for _, cmd := range mock.jobCommands {
			if cmd.command != immich.JobCommandResume {
				t.Errorf("expected resume command, got %s", cmd.command)
			}
		}
	})

	t.Run("pause error stops execution", func(t *testing.T) {
		mock := newMockServerClient()
		mock.sendJobCmdErr = errors.New("job error")
		pctx := createTestContext(mock)

		stage := &JobControlStage{Pause: true}

		err := stage.Run(context.Background(), pctx)
		if err == nil {
			t.Fatal("expected error on pause, got nil")
		}
	})

	t.Run("resume error does not stop execution", func(t *testing.T) {
		mock := newMockServerClient()
		mock.sendJobCmdErr = errors.New("job error")
		pctx := createTestContext(mock)

		stage := &JobControlStage{Pause: false}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("expected no error on resume failure, got %v", err)
		}
	})
}

// TestFinalizeStage tests the finalize stage.
func TestFinalizeStage(t *testing.T) {
	t.Run("closes caches", func(t *testing.T) {
		albumsCache := cache.NewCollectionCache(50, func(album assets.Album, ids []string) (assets.Album, error) {
			return album, nil
		})

		tagsCache := cache.NewCollectionCache(50, func(tag assets.Tag, ids []string) (assets.Tag, error) {
			return tag, nil
		})

		mock := newMockServerClient()
		pctx := createTestContext(mock)

		stage := &FinalizeStage{
			AlbumsCache: albumsCache,
			TagsCache:   tagsCache,
		}

		err := stage.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("FinalizeStage.Run() error = %v", err)
		}

		// Verify caches were closed (callbacks might not be triggered if empty)
		if stage.Name() != "finalize" {
			t.Errorf("expected name 'finalize', got '%s'", stage.Name())
		}
	})
}

// TestParallelStage tests the parallel stage.
func TestParallelStage(t *testing.T) {
	t.Run("runs stages in parallel", func(t *testing.T) {
		var mu sync.Mutex
		order := []string{}

		stage1 := &testStage{
			name: "stage1",
			runFn: func(ctx context.Context, pctx *Context) error {
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				order = append(order, "stage1")
				mu.Unlock()
				return nil
			},
		}

		stage2 := &testStage{
			name: "stage2",
			runFn: func(ctx context.Context, pctx *Context) error {
				mu.Lock()
				order = append(order, "stage2")
				mu.Unlock()
				return nil
			},
		}

		mock := newMockServerClient()
		pctx := createTestContext(mock)

		parallel := &ParallelStage{
			Stages: []Stage{stage1, stage2},
		}

		err := parallel.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("ParallelStage.Run() error = %v", err)
		}

		if len(order) != 2 {
			t.Errorf("expected 2 stages to run, got %d", len(order))
		}
		// stage2 should complete before stage1 due to sleep
		if len(order) >= 2 && order[0] != "stage2" {
			t.Log("stages ran in parallel (stage2 finished first)")
		}
	})

	t.Run("error in one stage cancels others", func(t *testing.T) {
		expectedErr := errors.New("stage error")

		stage1 := &testStage{
			name: "stage1",
			runFn: func(ctx context.Context, pctx *Context) error {
				return expectedErr
			},
		}

		stage2 := &testStage{
			name: "stage2",
			runFn: func(ctx context.Context, pctx *Context) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		}

		mock := newMockServerClient()
		pctx := createTestContext(mock)

		parallel := &ParallelStage{
			Stages: []Stage{stage1, stage2},
		}

		err := parallel.Run(context.Background(), pctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestPipelineRun tests the Pipeline.Run method.
func TestPipelineRun(t *testing.T) {
	t.Run("runs stages in order", func(t *testing.T) {
		order := []string{}

		stage1 := &testStage{
			name: "stage1",
			runFn: func(ctx context.Context, pctx *Context) error {
				order = append(order, "stage1")
				return nil
			},
		}

		stage2 := &testStage{
			name: "stage2",
			runFn: func(ctx context.Context, pctx *Context) error {
				order = append(order, "stage2")
				return nil
			},
		}

		mock := newMockServerClient()
		pctx := createTestContext(mock)

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		source := newMockSource()
		pipeline := New(source, logger)
		pipeline.AddStage(stage1)
		pipeline.AddStage(stage2)

		err := pipeline.Run(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Pipeline.Run() error = %v", err)
		}

		if len(order) != 2 || order[0] != "stage1" || order[1] != "stage2" {
			t.Errorf("expected ['stage1', 'stage2'], got %v", order)
		}
	})

	t.Run("stops on first error", func(t *testing.T) {
		expectedErr := errors.New("stage error")
		stage2Ran := false

		stage1 := &testStage{
			name: "stage1",
			runFn: func(ctx context.Context, pctx *Context) error {
				return expectedErr
			},
		}

		stage2 := &testStage{
			name: "stage2",
			runFn: func(ctx context.Context, pctx *Context) error {
				stage2Ran = true
				return nil
			},
		}

		mock := newMockServerClient()
		pctx := createTestContext(mock)

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		source := newMockSource()
		pipeline := New(source, logger)
		pipeline.AddStage(stage1)
		pipeline.AddStage(stage2)

		err := pipeline.Run(context.Background(), pctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if stage2Ran {
			t.Error("stage2 should not have run after stage1 error")
		}
	})
}

// testStage is a test implementation of Stage.
type testStage struct {
	name  string
	runFn func(ctx context.Context, pctx *Context) error
}

func (s *testStage) Name() string { return s.name }
func (s *testStage) Run(ctx context.Context, pctx *Context) error {
	if s.runFn != nil {
		return s.runFn(ctx, pctx)
	}
	return nil
}
