package source

import (
	"bytes"
	"context"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/exif"
	"github.com/sweepies/immich-go/internal/exif/sidecars/jsonsidecar"
	"github.com/sweepies/immich-go/internal/exif/sidecars/xmpsidecar"
	"github.com/sweepies/immich-go/internal/fileevent"
	"github.com/sweepies/immich-go/internal/filenames"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/filters"
	"github.com/sweepies/immich-go/internal/fshelper"
	"github.com/sweepies/immich-go/internal/gen"
	"github.com/sweepies/immich-go/internal/groups"
	"github.com/sweepies/immich-go/internal/groups/burst"
	"github.com/sweepies/immich-go/internal/groups/epsonfastfoto"
	"github.com/sweepies/immich-go/internal/groups/series"
	"github.com/sweepies/immich-go/internal/worker"
)

// FolderSource implements adapters.Source for folder-based imports.
// It supports folder, iCloud, and Picasa modes.
type FolderSource struct {
	deps   adapters.SourceDependencies
	config *adapters.FolderConfig
	fsyss  []fs.FS

	// Internal state
	infoCollector           *filenames.InfoCollector
	pool                    *worker.Pool
	wg                      sync.WaitGroup
	groupers                []groups.Grouper
	requiresDateInformation bool
	picasaAlbums            *gen.SyncMap[string, picasaAlbum]
	icloudMetas             *gen.SyncMap[string, icloudMeta]
	icloudMetaPass          bool
}

// picasaAlbum holds parsed Picasa album information.
type picasaAlbum struct {
	Name        string
	Description string
}

// icloudMeta holds parsed iCloud metadata.
type icloudMeta struct {
	originalCreationDate string
	albums               []assets.Album
}

// Browse implements adapters.Source.
func (s *FolderSource) Browse(ctx context.Context) <-chan *assets.Group {
	s.initialize()

	gOut := make(chan *assets.Group)
	go func() {
		defer close(gOut)

		// Two passes for iCloud takeouts
		if s.icloudMetaPass {
			for _, fsys := range s.fsyss {
				s.concurrentParseDir(ctx, fsys, ".", gOut)
			}
			s.wg.Wait()
			s.icloudMetaPass = false
		}

		for _, fsys := range s.fsyss {
			s.concurrentParseDir(ctx, fsys, ".", gOut)
		}
		s.wg.Wait()
		s.pool.Stop()
	}()
	return gOut
}

// Close implements adapters.Source.
func (s *FolderSource) Close() error {
	if s.pool != nil {
		s.pool.Stop()
	}
	return CloseFSs(s.fsyss)
}

// initialize sets up internal state for browsing.
func (s *FolderSource) initialize() {
	s.infoCollector = filenames.NewInfoCollector(s.deps.TimeZone, s.deps.SupportedMedia)
	s.pool = worker.NewPool(s.deps.ConcurrentTasks)

	s.requiresDateInformation = s.config.InclusionFlags.DateRange.IsSet() ||
		s.config.TakeDateFromFilename ||
		s.config.ManageBurst != filters.BurstNothing ||
		s.config.ManageHEICJPG != filters.HeicJpgNothing ||
		s.config.ManageRawJPG != filters.RawJPGNothing

	if s.config.PicasaAlbum {
		s.picasaAlbums = gen.NewSyncMap[string, picasaAlbum]()
	}
	if s.config.ICloudTakeout {
		s.icloudMetas = gen.NewSyncMap[string, icloudMeta]()
		s.icloudMetaPass = true
	}

	if s.config.InclusionFlags.DateRange.IsSet() {
		s.config.InclusionFlags.DateRange.SetTZ(s.deps.TimeZone)
	}

	// Set up groupers
	if s.config.ManageEpsonFastFoto {
		s.groupers = append(s.groupers, epsonfastfoto.Group{}.Group)
	}
	if s.config.ManageBurst != filters.BurstNothing {
		s.groupers = append(s.groupers, burst.Group)
	}
	s.groupers = append(s.groupers, series.Group)
}

func (s *FolderSource) concurrentParseDir(ctx context.Context, fsys fs.FS, dir string, gOut chan *assets.Group) {
	s.wg.Add(1)
	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		submitted := s.pool.TrySubmit(func() {
			defer s.wg.Done()
			defer cancel(nil)
			err := s.parseDir(ctx, fsys, dir, gOut)
			if err != nil {
				s.deps.Logger.Error(err.Error())
				cancel(err)
			}
		})
		if !submitted {
			cancel(nil)
			s.wg.Done()
		}
	}()
}

func (s *FolderSource) parseDir(ctx context.Context, fsys fs.FS, dir string, gOut chan *assets.Group) error {
	fsName := ""
	if named, ok := fsys.(interface{ Name() string }); ok {
		fsName = named.Name()
	}

	var as []*assets.Asset
	var entries []fs.DirEntry
	var err error

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		entries, err = fs.ReadDir(fsys, dir)
		if err != nil {
			return err
		}
	}

	dirFiles := make(map[string]string, len(entries))
	for _, entry := range entries {
		base := entry.Name()
		name := path.Join(dir, base)
		dirFiles[strings.ToLower(base)] = base
		ext := filepath.Ext(base)

		if entry.IsDir() {
			continue
		}

		// Skip banned files
		if s.config.BannedFiles.Match(name) {
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, entry.Name()), 0, fileevent.DiscoveredBanned, "reason", "banned file")
			continue
		}

		if s.deps.SupportedMedia.IsUseLess(name) {
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, entry.Name()), 0, fileevent.DiscoveredUnknown, "reason", "useless file")
			continue
		}

		mediaType := s.deps.SupportedMedia.TypeFromExt(ext)

		if mediaType == filetypes.TypeUnknown {
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredUnsupported, "reason", "unsupported file type")
			continue
		}

		switch mediaType {
		case filetypes.TypeUseless:
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredUnknown)
			continue
		case filetypes.TypeImage, filetypes.TypeVideo:
			// Will be recorded as discovered asset after assetFromFile creates it
		case filetypes.TypeSidecar:
			if s.config.IgnoreSideCarFiles {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredSidecar, "reason", "sidecar file ignored")
				continue
			}
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredSidecar)
			continue
		}

		if !s.config.InclusionFlags.IncludedExtensions.Include(ext) {
			if info, err := fs.Stat(fsys, name); err == nil {
				s.deps.Processor.RecordAssetDiscardedImmediately(ctx, fshelper.FSName(fsys, name), info.Size(), fileevent.DiscardedFiltered, "extension not included")
			}
			continue
		}

		if s.config.InclusionFlags.ExcludedExtensions.Exclude(ext) {
			if info, err := fs.Stat(fsys, name); err == nil {
				s.deps.Processor.RecordAssetDiscardedImmediately(ctx, fshelper.FSName(fsys, name), info.Size(), fileevent.DiscardedFiltered, "extension excluded")
			}
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			a, err := s.assetFromFile(ctx, fsys, name)
			if err != nil {
				s.deps.Processor.RecordAssetError(ctx, fshelper.FSName(fsys, name), 0, fileevent.ErrorFileAccess, err)
				return err
			}
			if a != nil {
				code := fileevent.DiscoveredImage
				if mediaType == filetypes.TypeVideo {
					code = fileevent.DiscoveredVideo
				}
				s.deps.Processor.RecordAssetDiscovered(ctx, a.File, int64(a.FileSize), code)
				as = append(as, a)
			}
		}
	}

	// Process subdirectories
	for _, entry := range entries {
		base := entry.Name()
		name := path.Join(dir, base)
		if entry.IsDir() {
			if s.config.BannedFiles.MatchDir(name) {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredBanned, "reason", "banned folder")
				continue
			}
			if s.config.Recursive && entry.Name() != "." {
				s.concurrentParseDir(ctx, fsys, name, gOut)
			}
		}
	}

	// Process assets through grouper pipeline
	in := make(chan *assets.Asset)
	go func() {
		defer close(in)

		sort.Slice(as, func(i, j int) bool {
			radicalI := as[i].Radical
			radicalJ := as[j].Radical
			if radicalI != radicalJ {
				return radicalI < radicalJ
			}
			return as[i].CaptureDate.Before(as[j].CaptureDate)
		})

		for _, a := range as {
			s.processAssetMetadata(ctx, fsys, a, fsName, dir, dirFiles)

			if !s.config.InclusionFlags.DateRange.InRange(a.CaptureDate) {
				a.Close()
				s.deps.Processor.RecordAssetDiscardedImmediately(ctx, a.File, int64(a.FileSize), fileevent.DiscardedFiltered, "asset outside date range")
				continue
			}

			s.assignAlbums(a, fsys, fsName, dir)

			select {
			case in <- a:
			case <-ctx.Done():
				return
			}
		}
	}()

	gs := groups.NewGrouperPipeline(ctx, s.groupers...).PipeGrouper(ctx, in)
	for g := range gs {
		select {
		case gOut <- g:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *FolderSource) assetFromFile(_ context.Context, fsys fs.FS, name string) (*assets.Asset, error) {
	a := &assets.Asset{
		File:             fshelper.FSName(fsys, name),
		OriginalFileName: filepath.Base(name),
	}
	i, err := fs.Stat(fsys, name)
	if err != nil {
		a.Close()
		return nil, err
	}
	a.FileSize = int(i.Size())
	a.FileDate = i.ModTime()

	n := path.Join(path.Dir(name), a.OriginalFileName)
	if named, ok := fsys.(interface{ Name() string }); ok {
		n = path.Join(named.Name(), n)
	}

	a.SetNameInfo(s.infoCollector.GetInfo(n))
	return a, nil
}

func (s *FolderSource) processAssetMetadata(ctx context.Context, fsys fs.FS, a *assets.Asset, fsName, dir string, dirFiles map[string]string) {
	// Check for JSON sidecar
	jsonName := findSidecar(dirFiles, a.OriginalFileName, ".json")
	if jsonName != "" {
		jsonPath := path.Join(dir, jsonName)
		buf, err := fs.ReadFile(fsys, jsonPath)
		if err != nil {
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, jsonPath), 0, fileevent.ErrorFileAccess, "error", err.Error())
		} else {
			// Update jsonName to include full path
			jsonName = jsonPath
			if bytes.Contains(buf, []byte("immich-go version")) {
				md := &assets.Metadata{}
				err = jsonsidecar.Read(bytes.NewReader(buf), md)
				if err != nil {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, jsonName), 0, fileevent.ErrorFileAccess, "error", err.Error())
				} else {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, jsonName), 0, fileevent.DiscoveredSidecar)
					md.File = fshelper.FSName(fsys, jsonName)
					a.FromApplication = a.UseMetadata(md)
					a.OriginalFileName = md.FileName
				}
			} else {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, jsonName), 0, fileevent.DiscoveredSidecar)
				s.deps.Logger.Warn("JSON file detected but not from immich-go", "file", fshelper.FSName(fsys, jsonName))
			}
		}
	}

	// Check for XMP sidecar
	xmpName := findSidecar(dirFiles, a.OriginalFileName, ".xmp")
	if xmpName != "" {
		xmpPath := path.Join(dir, xmpName)
		buf, err := fs.ReadFile(fsys, xmpPath)
		if err != nil {
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, xmpPath), 0, fileevent.ErrorFileAccess, "error", err.Error())
		} else {
			// Update xmpName to include full path
			xmpName = xmpPath
			md := &assets.Metadata{}
			err = xmpsidecar.ReadXMP(bytes.NewReader(buf), md)
			if err != nil {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, xmpName), 0, fileevent.ErrorFileAccess, "error", err.Error())
			} else {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, xmpName), 0, fileevent.DiscoveredSidecar)
				md.File = fshelper.FSName(fsys, xmpName)
				a.FromSideCar = a.UseMetadata(md)
			}
		}
	}

	// Read metadata from file if needed
	if s.requiresDateInformation && a.CaptureDate.IsZero() {
		f, err := a.OpenFile()
		if err == nil {
			md, err := exif.GetMetaData(f, a.Ext, s.deps.TimeZone)
			if err == nil {
				a.FromSourceFile = a.UseMetadata(md)
			}
			if (md == nil || md.DateTaken.IsZero()) && !a.Taken.IsZero() && s.config.TakeDateFromFilename {
				a.FromApplication = &assets.Metadata{
					DateTaken: a.Taken,
				}
				a.CaptureDate = a.FromApplication.DateTaken
			}
			f.Close()
		}
	}

	// Add folder as tags
	if s.config.FolderAsTags {
		t := fsName
		if dir != "." {
			t = path.Join(t, dir)
		}
		if t != "" {
			a.AddTag(t)
		}
	}
}

func (s *FolderSource) assignAlbums(a *assets.Asset, fsys fs.FS, fsName, dir string) {
	if s.config.ImportIntoAlbum != "" {
		a.Albums = []assets.Album{{Title: s.config.ImportIntoAlbum}}
		return
	}

	if s.config.PicasaAlbum {
		if album, ok := s.picasaAlbums.Load(dir); ok {
			a.Albums = []assets.Album{{Title: album.Name, Description: album.Description}}
			return
		}
	}

	if s.config.UsePathAsAlbumName != adapters.FolderModeNone && s.config.UsePathAsAlbumName != "" {
		var albumName string
		switch s.config.UsePathAsAlbumName {
		case adapters.FolderModeFolder:
			if dir == "." {
				albumName = fsName
			} else {
				albumName = filepath.Base(dir)
			}
		case adapters.FolderModePath:
			parts := []string{}
			if fsName != "" {
				parts = append(parts, fsName)
			}
			if dir != "." {
				parts = append(parts, strings.Split(dir, "/")...)
			}
			albumName = strings.Join(parts, s.config.AlbumNamePathSeparator)
		}
		if albumName != "" {
			a.Albums = []assets.Album{{Title: albumName}}
		}
	}
}

func findSidecar(files map[string]string, baseName string, ext string) string {
	// Check baseName + ext
	name := baseName + ext
	if realName, ok := files[strings.ToLower(name)]; ok {
		return realName
	}

	fileExt := path.Ext(baseName)
	if !filetypes.DefaultSupportedMedia.IsMedia(fileExt) {
		return ""
	}
	nameNoExt := strings.TrimSuffix(baseName, fileExt)
	name = nameNoExt + ext
	if realName, ok := files[strings.ToLower(name)]; ok {
		return realName
	}
	return ""
}
