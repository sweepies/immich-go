package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/assets/cache"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/fshelper"
	"github.com/simulot/immich-go/internal/groups"
	iimmich "github.com/simulot/immich-go/internal/immich"
	"github.com/simulot/immich-go/internal/worker"
	"golang.org/x/sync/errgroup"
)

// DiscoveryStage fetches assets from the server and populates the index.
type DiscoveryStage struct {
	// ProgressUpdate is called with current/total progress during discovery.
	ProgressUpdate func(current, total int)
}

func (s *DiscoveryStage) Name() string { return "discovery" }

func (s *DiscoveryStage) Run(ctx context.Context, pctx *Context) error {
	statistics, err := pctx.Server.GetAssetStatistics(ctx)
	if err != nil {
		return fmt.Errorf("failed to get asset statistics: %w", err)
	}
	totalOnImmich := statistics.Total
	received := 0

	userID := pctx.Server.UserID()

	err = pctx.Server.GetAllAssets(ctx, func(a *iimmich.Asset) error {
		if s.ProgressUpdate != nil {
			defer func() {
				s.ProgressUpdate(received, totalOnImmich)
			}()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			received++
			if a.OwnerID != userID {
				pctx.Logger.Debug("Skipping asset with different owner",
					"assetOwnerID", a.OwnerID, "clientUserID", userID,
					"ID", a.ID, "FileName", a.OriginalFileName)
				return nil
			}
			if a.LibraryID != "" {
				pctx.Logger.Debug("Skipping asset with external library",
					"assetLibraryID", a.LibraryID, "ID", a.ID, "FileName", a.OriginalFileName)
				return nil
			}
			pctx.Index.AddImmichAsset(a)
			pctx.Logger.Debug("Indexed immich asset",
				"ID", a.ID, "FileName", a.OriginalFileName,
				"CaptureDate", a.ExifInfo.DateTimeOriginal, "CheckSum", a.Checksum)
			return nil
		}
	})
	if err != nil {
		return fmt.Errorf("failed to get assets from server: %w", err)
	}

	if s.ProgressUpdate != nil {
		s.ProgressUpdate(totalOnImmich, totalOnImmich)
	}
	pctx.Logger.Info(fmt.Sprintf("Assets on the server: %d", pctx.Index.Len()))
	return nil
}

// AlbumDiscoveryStage fetches albums from the server.
type AlbumDiscoveryStage struct {
	AlbumsCache *cache.CollectionCache[assets.Album]
	// AssetsReady is closed when the discovery stage completes (assets are ready).
	AssetsReady <-chan struct{}
}

func (s *AlbumDiscoveryStage) Name() string { return "album-discovery" }

func (s *AlbumDiscoveryStage) Run(ctx context.Context, pctx *Context) error {
	serverAlbums, err := pctx.Server.GetAllAlbums(ctx)
	if err != nil {
		return fmt.Errorf("can't get the album list from the server: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.AssetsReady:
		for _, a := range serverAlbums {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				r, err := pctx.Server.GetAlbumInfo(ctx, a.ID, false)
				if err != nil {
					pctx.Logger.Error("can't get the album info from the server", "album", a.AlbumName, "err", err)
					continue
				}
				ids := make([]string, 0, len(r.Assets))
				for _, aa := range r.Assets {
					ids = append(ids, string(aa.ID))
				}

				album := assets.NewAlbum(string(a.ID), a.AlbumName, a.Description)
				s.AlbumsCache.NewCollection(a.AlbumName, album, ids)
				pctx.Logger.Info("got album from the server", "album", a.AlbumName, "assets", len(r.Assets))

				// assign the album to the assets
				for _, id := range ids {
					asset := pctx.Index.GetByID(id)
					if asset == nil {
						pctx.Logger.Debug("processing the immich albums: asset not found in index", "id", id)
						continue
					}
					asset.Albums = append(asset.Albums, album)
				}
			}
		}
	}
	return nil
}

// UploadStage handles uploading assets to the server.
type UploadStage struct {
	Source       Source
	AlbumsCache  *cache.CollectionCache[assets.Album]
	TagsCache    *cache.CollectionCache[assets.Tag]
	Groupers     []groups.Grouper
	Filters      []filters.Filter
	Tags         []string
	SessionTag   string
	Overwrite    bool
	Concurrency  int
	OnError      func(err error) error // Called on each error, return non-nil to abort
	deleteList   []*assets.Asset       // Assets to delete after upload
	deleteListMu sync.Mutex
}

func (s *UploadStage) Name() string { return "upload" }

func (s *UploadStage) Run(ctx context.Context, pctx *Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	groupChan := s.Source.Browse(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		workers := worker.NewPool(s.Concurrency)
		defer workers.Stop()

		for {
			select {
			case <-ctx.Done():
				cancel(ctx.Err())
				return
			case g, ok := <-groupChan:
				if !ok {
					return
				}
				workers.Submit(func() {
					err := s.handleGroup(ctx, pctx, g)
					if err != nil {
						if s.OnError != nil {
							err = s.OnError(err)
						}
						if err != nil {
							cancel(err)
						}
					}
				})
			}
		}
	}()

	wg.Wait()
	err := context.Cause(ctx)

	// Cleanup: delete server assets if needed
	if len(s.deleteList) > 0 {
		ids := make([]iimmich.AssetID, 0, len(s.deleteList))
		for _, da := range s.deleteList {
			ids = append(ids, iimmich.AssetID(da.ID))
		}
		if delErr := pctx.Server.DeleteAssets(ctx, ids, false); delErr != nil {
			return fmt.Errorf("can't delete server's assets: %w", delErr)
		}
	}

	return err
}

func (s *UploadStage) handleGroup(ctx context.Context, pctx *Context, g *assets.Group) error {
	var errGroup error

	g = filters.ApplyFilters(g, s.Filters...)

	// discard rejected assets
	for _, a := range g.Removed {
		a.Asset.Close()
		pctx.Processor.RecordAssetDiscarded(ctx, a.Asset.File, int64(a.Asset.FileSize), fileevent.DiscardedNotSelected, a.Reason)
	}

	// Upload assets from the group
	for _, a := range g.Assets {
		err := s.handleAsset(ctx, pctx, a)
		errGroup = errors.Join(errGroup, err)
	}

	// Manage groups - stack assets after filtering and upload
	if len(g.Assets) > 1 && g.Grouping != assets.GroupByNone {
		ids := []iimmich.AssetID{iimmich.AssetID(g.Assets[g.CoverIndex].ID)}
		for i, a := range g.Assets {
			pctx.Processor.RecordNonAsset(ctx, g.Assets[i].File, 0, fileevent.ProcessedStacked)
			if i != g.CoverIndex && a.ID != "" {
				ids = append(ids, iimmich.AssetID(a.ID))
			}
		}
		if len(ids) > 1 {
			_, err := pctx.Server.CreateStack(ctx, ids)
			if err != nil {
				pctx.Logger.Error("Can't create stack", "error", err)
			}
		}
	}

	return errGroup
}

func (s *UploadStage) handleAsset(ctx context.Context, pctx *Context, a *assets.Asset) error {
	defer func() {
		a.Close()
	}()

	advice, err := pctx.Index.ShouldUpload(a, s.Overwrite)
	if err != nil {
		return err
	}

	switch advice.Advice {
	case NotOnServer:
		serverStatus, err := s.uploadAsset(ctx, pctx, a)
		if err != nil {
			return err
		}
		s.processUploadedAsset(ctx, pctx, a, serverStatus)
		return nil

	case SmallerOnServer:
		a.Albums = append(a.Albums, advice.ServerAsset.Albums...)
		serverStatus, err := s.replaceAsset(ctx, pctx, a, advice.ServerAsset)
		if err != nil {
			return err
		}
		s.processUploadedAsset(ctx, pctx, a, serverStatus)
		pctx.Processor.RecordAssetProcessed(ctx, a.File, int64(a.FileSize), fileevent.ProcessedUploadUpgraded)
		return nil

	case AlreadyProcessed:
		pctx.Processor.RecordNonAsset(ctx, a.File, int64(a.FileSize), fileevent.DiscardedLocalDuplicate)
		pctx.Processor.RecordAssetProcessed(ctx, a.File, int64(a.FileSize), fileevent.ProcessedMetadataUpdated)
		s.manageAssetAlbums(ctx, pctx, a.File, a.ID, a.Albums)
		return nil

	case SameOnServer:
		a.ID = advice.ServerAsset.ID
		a.Albums = append(a.Albums, advice.ServerAsset.Albums...)
		pctx.Processor.RecordNonAsset(ctx, a.File, int64(a.FileSize), fileevent.DiscardedServerDuplicate)
		pctx.Processor.RecordAssetProcessed(ctx, a.File, int64(a.FileSize), fileevent.ProcessedMetadataUpdated)
		s.manageAssetAlbums(ctx, pctx, a.File, a.ID, a.Albums)

	case BetterOnServer:
		a.ID = advice.ServerAsset.ID
		pctx.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.ProcessedMetadataUpdated, advice.Message)
		s.manageAssetAlbums(ctx, pctx, a.File, a.ID, a.Albums)

	case ForceUpload:
		var serverStatus string
		var err error

		if advice.ServerAsset != nil {
			a.Albums = append(a.Albums, advice.ServerAsset.Albums...)
			serverStatus, err = s.replaceAsset(ctx, pctx, a, advice.ServerAsset)
		} else {
			serverStatus, err = s.uploadAsset(ctx, pctx, a)
		}
		if err != nil {
			return err
		}
		s.processUploadedAsset(ctx, pctx, a, serverStatus)
		return nil
	}

	return nil
}

func (s *UploadStage) uploadAsset(ctx context.Context, pctx *Context, a *assets.Asset) (string, error) {
	defer pctx.Logger.Debug("upload asset", "file", a)

	if s.SessionTag != "" {
		a.AddTag(s.SessionTag)
	}
	for _, tag := range s.Tags {
		a.AddTag(tag)
	}

	ar, err := pctx.Server.AssetUpload(ctx, a)
	if err != nil {
		pctx.Processor.RecordAssetError(ctx, a.File, int64(a.FileSize), fileevent.ErrorServerError, err)
		return "", err
	}
	if ar.Status == iimmich.UploadDuplicate {
		originalName := "unknown"
		original := pctx.Index.GetByID(string(ar.ID))
		if original != nil {
			originalName = original.OriginalFileName
		}
		if a.ID == "" {
			pctx.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedLocalDuplicate,
				fmt.Sprintf("already present in input as %s", originalName))
		} else {
			pctx.Processor.RecordAssetProcessed(ctx, a.File, int64(a.FileSize), fileevent.DiscardedServerDuplicate)
		}
	} else {
		pctx.Processor.RecordAssetProcessed(ctx, a.File, int64(a.FileSize), fileevent.ProcessedUploadSuccess)
	}
	a.ID = string(ar.ID)

	if a.FromApplication != nil && ar.Status != iimmich.UploadDuplicate {
		a.UseMetadata(a.FromApplication)
		_, err := pctx.Server.UpdateAsset(ctx, ar.ID, iimmich.UpdateAssetRequest{
			Description:      a.Description,
			Latitude:         a.Latitude,
			Longitude:        a.Longitude,
			Rating:           a.Rating,
			DateTimeOriginal: a.CaptureDate,
		})
		if err != nil {
			pctx.Processor.RecordAssetError(ctx, a.File, int64(a.FileSize), fileevent.ErrorServerError, err)
			return "", err
		}
		pctx.Processor.Logger().Record(ctx, fileevent.ProcessedMetadataUpdated, a.File)
	}
	pctx.Index.AddLocalAsset(a)
	return ar.Status, nil
}

func (s *UploadStage) replaceAsset(ctx context.Context, pctx *Context, newAsset, oldAsset *assets.Asset) (string, error) {
	ar, err := pctx.Server.AssetUpload(ctx, newAsset)
	if err != nil {
		pctx.Processor.RecordAssetError(ctx, newAsset.File, int64(newAsset.FileSize), fileevent.ErrorServerError, err)
		return "", err
	}
	newAsset.ID = string(ar.ID)
	if ar.Status == iimmich.UploadDuplicate {
		pctx.Processor.RecordAssetProcessed(ctx, newAsset.File, int64(newAsset.FileSize), fileevent.DiscardedServerDuplicate)
		return iimmich.UploadDuplicate, nil
	}

	err = pctx.Server.CopyAsset(ctx, iimmich.AssetID(oldAsset.ID), ar.ID)
	if err != nil {
		pctx.Processor.RecordAssetError(ctx, newAsset.File, int64(newAsset.FileSize), fileevent.ErrorServerError, err)
		return "", err
	}

	err = pctx.Server.DeleteAssets(ctx, []iimmich.AssetID{iimmich.AssetID(oldAsset.ID)}, true)
	if err != nil {
		pctx.Processor.RecordAssetError(ctx, newAsset.File, int64(newAsset.FileSize), fileevent.ErrorServerError, err)
		return "", err
	}
	pctx.Index.ReplaceAsset(newAsset, oldAsset)
	return "", nil
}

func (s *UploadStage) manageAssetAlbums(ctx context.Context, pctx *Context, f fshelper.FSAndName, ID string, albums []assets.Album) {
	if len(albums) == 0 {
		return
	}

	for _, album := range albums {
		al := assets.NewAlbum("", album.Title, album.Description)
		if s.AlbumsCache.AddIDToCollection(al.Title, album, ID) {
			pctx.Processor.Logger().Record(ctx, fileevent.ProcessedAlbumAdded, f, "album", al.Title)
		}
	}
}

func (s *UploadStage) manageAssetTags(ctx context.Context, pctx *Context, a *assets.Asset) {
	if len(a.Tags) == 0 {
		return
	}

	for _, t := range a.Tags {
		if s.TagsCache.AddIDToCollection(t.Name, t, a.ID) {
			pctx.Processor.Logger().Record(ctx, fileevent.ProcessedTagged, a.File, "tag", t.Value)
		}
	}
}

func (s *UploadStage) processUploadedAsset(ctx context.Context, pctx *Context, a *assets.Asset, serverStatus string) {
	if serverStatus != iimmich.UploadDuplicate {
		s.manageAssetAlbums(ctx, pctx, a.File, a.ID, a.Albums)
		s.manageAssetTags(ctx, pctx, a)
	}
}

// FinalizeStage handles cleanup operations after upload.
type FinalizeStage struct {
	AlbumsCache  *cache.CollectionCache[assets.Album]
	TagsCache    *cache.CollectionCache[assets.Tag]
	SaveAlbum    func(ctx context.Context, album assets.Album, ids []string) (assets.Album, error)
	SaveTag      func(ctx context.Context, tag assets.Tag, ids []string) (assets.Tag, error)
	ResumeJobs   bool
	jobsPaused   bool
	JobsPausedMu sync.Mutex
}

func (s *FinalizeStage) Name() string { return "finalize" }

func (s *FinalizeStage) SetJobsPaused(paused bool) {
	s.JobsPausedMu.Lock()
	defer s.JobsPausedMu.Unlock()
	s.jobsPaused = paused
}

func (s *FinalizeStage) Run(ctx context.Context, pctx *Context) error {
	// Close caches - this triggers the save callbacks
	if s.AlbumsCache != nil {
		s.AlbumsCache.Close()
	}
	if s.TagsCache != nil {
		s.TagsCache.Close()
	}
	return nil
}

// JobControlStage handles pausing/resuming Immich background jobs.
type JobControlStage struct {
	Pause bool
}

func (s *JobControlStage) Name() string {
	if s.Pause {
		return "pause-jobs"
	}
	return "resume-jobs"
}

func (s *JobControlStage) Run(ctx context.Context, pctx *Context) error {
	jobs := []string{"thumbnailGeneration", "metadataExtraction", "videoConversion", "faceDetection", "smartSearch"}

	command := iimmich.JobCommandResume
	if s.Pause {
		command = iimmich.JobCommandPause
	}

	// For resume, use a fresh context in case the original was cancelled
	runCtx := ctx
	if !s.Pause {
		runCtx = context.Background()
	}

	for _, name := range jobs {
		_, err := pctx.Server.SendJobCommand(runCtx, name, command, true)
		if err != nil {
			pctx.Logger.Error("Immich Job command sent", string(command), name, "err", err.Error())
			if s.Pause {
				return err
			}
			// Don't fail on resume errors
			continue
		}
		pctx.Logger.Info("Immich Job command sent", string(command), name)
	}
	return nil
}

// ParallelStage runs multiple stages in parallel.
type ParallelStage struct {
	Stages []Stage
}

func (s *ParallelStage) Name() string { return "parallel" }

func (s *ParallelStage) Run(ctx context.Context, pctx *Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, stage := range s.Stages {
		stage := stage
		g.Go(func() error {
			return stage.Run(ctx, pctx)
		})
	}

	return g.Wait()
}
