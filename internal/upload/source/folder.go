package source

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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

const icloudMetadataExt = ".csv"

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
	icloudDiscoveredAssets  *gen.SyncMap[string, *icloudDirDiscovery]
}

// icloudDirDiscovery holds information collected during the first pass of iCloud takeout.
type icloudDirDiscovery struct {
	fsys     fs.FS
	fsName   string
	dir      string
	dirFiles map[string]string
	assets   []*assets.Asset
}

// picasaAlbum holds parsed Picasa album information.
type picasaAlbum struct {
	Name        string
	Description string
}

// icloudMeta holds parsed iCloud metadata.
type icloudMeta struct {
	originalCreationDate time.Time
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
			s.processICloudDiscoveredAssets(ctx, gOut)
		} else {
			for _, fsys := range s.fsyss {
				s.concurrentParseDir(ctx, fsys, ".", gOut)
			}
		}
		s.wg.Wait()
		s.pool.Stop()
	}()
	return gOut
}

func (s *FolderSource) processICloudDiscoveredAssets(ctx context.Context, gOut chan *assets.Group) {
	s.icloudDiscoveredAssets.Range(func(_ string, d *icloudDirDiscovery) bool {
		s.wg.Add(1)
		go func() {
			submitted := s.pool.Submit(func() {
				defer s.wg.Done()
				err := s.processAssets(ctx, d.fsys, d.fsName, d.dir, d.dirFiles, d.assets, gOut)
				if err != nil {
					s.deps.Logger.Error(err.Error())
				}
			})
			if !submitted {
				s.wg.Done()
			}
		}()
		return true
	})
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
		s.icloudDiscoveredAssets = gen.NewSyncMap[string, *icloudDirDiscovery]()
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
	go func() {
		submitted := s.pool.TrySubmit(func() {
			defer s.wg.Done()
			err := s.parseDir(ctx, fsys, dir, gOut)
			if err != nil {
				s.deps.Logger.Error(err.Error())
			}
		})
		if !submitted {
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

		if s.icloudMetaPass && ext == icloudMetadataExt {
			if strings.HasSuffix(strings.ToLower(dir), "albums") {
				album, err := useICloudAlbum(s.icloudMetas, fsys, name)
				if err != nil {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.ErrorFileAccess, "error", err.Error())
				} else {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredMetadata, "album", album)
				}
				continue
			}
			if s.config.ICloudMemoriesAsAlbums && strings.HasSuffix(strings.ToLower(dir), "memories") {
				album, err := useICloudMemory(s.icloudMetas, fsys, name)
				if err != nil {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.ErrorFileAccess, "error", err.Error())
				} else {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredMetadata, "album", album)
				}
				continue
			}
			if strings.HasPrefix(strings.ToLower(base), "photo details") {
				err := useICloudPhotoDetails(s.icloudMetas, fsys, name)
				if err != nil {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.ErrorFileAccess, "error", err.Error())
				} else {
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredMetadata)
				}
				continue
			}
		}

		if s.config.ICloudTakeout && !s.icloudMetaPass && ext == icloudMetadataExt {
			continue
		}

		// Skip banned files
		if s.config.BannedFiles.Match(name) {
			s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, entry.Name()), 0, fileevent.DiscoveredBanned, "reason", "banned file")
			continue
		}

		if s.config.PicasaAlbum && isPicasaIni(base) {
			album, err := readPicasaIni(fsys, name)
			if err != nil {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.ErrorFileAccess, "error", err.Error())
			} else {
				s.picasaAlbums.Store(dir, album)
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(fsys, name), 0, fileevent.DiscoveredMetadata, "album", album.Name)
			}
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

	if s.icloudMetaPass {
		if len(as) > 0 {
			s.icloudDiscoveredAssets.Store(fmt.Sprintf("%p:%s", fsys, dir), &icloudDirDiscovery{
				fsys:     fsys,
				fsName:   fsName,
				dir:      dir,
				dirFiles: dirFiles,
				assets:   as,
			})
		}
		return nil
	}

	return s.processAssets(ctx, fsys, fsName, dir, dirFiles, as, gOut)
}

func (s *FolderSource) processAssets(ctx context.Context, fsys fs.FS, fsName, dir string, dirFiles map[string]string, as []*assets.Asset, gOut chan *assets.Group) error {
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
		if s.config.ICloudTakeout {
			if meta, ok := s.icloudMetas.Load(a.OriginalFileName); ok && !meta.originalCreationDate.IsZero() {
				a.FromApplication = &assets.Metadata{DateTaken: meta.originalCreationDate}
				a.CaptureDate = a.FromApplication.DateTaken
			}
		}

		if a.CaptureDate.IsZero() {
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

	if s.config.ICloudTakeout {
		if meta, ok := s.icloudMetas.Load(a.OriginalFileName); ok && len(meta.albums) > 0 {
			a.Albums = meta.albums
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

func isPicasaIni(name string) bool {
	return strings.EqualFold(name, ".picasa.ini") || strings.EqualFold(name, "picasa.ini")
}

func readPicasaIni(fsys fs.FS, filename string) (picasaAlbum, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return picasaAlbum{}, err
	}
	defer file.Close()

	album, err := parsePicasaIni(file)
	if err != nil {
		return picasaAlbum{}, fmt.Errorf("error parsing picasa ini file: %w", err)
	}
	return album, nil
}

func parsePicasaIni(r io.Reader) (picasaAlbum, error) {
	scanner := bufio.NewScanner(r)
	var currentSection string
	var album picasaAlbum

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}

		if currentSection != "Picasa" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return picasaAlbum{}, errors.New("invalid line: " + line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			album.Name = value
		case "description":
			album.Description = value
		}
	}

	if err := scanner.Err(); err != nil {
		return picasaAlbum{}, err
	}

	return album, nil
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
