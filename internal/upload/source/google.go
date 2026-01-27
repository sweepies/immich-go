package source

import (
	"bytes"
	"context"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/fshelper"
	"github.com/simulot/immich-go/internal/gen"
	"github.com/simulot/immich-go/internal/groups"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
)

var takeoutPartRegex = regexp.MustCompile(`-\d{3}$`)

// GoogleSource implements adapters.Source for Google Photos takeout imports.
type GoogleSource struct {
	deps   adapters.SourceDependencies
	config *adapters.GoogleConfig
	fsyss  []fs.FS

	// Internal state
	infoCollector *filenames.InfoCollector
	groupers      []groups.Grouper
	catalogs      map[string]googleDirCatalog
	albums        map[string]assets.Album
	fileTracker   *gen.SyncMap[googleFileKey, googleTrackingInfo]
	takeoutName   string
}

// googleFileKey tracks files by base name and size for duplicate detection.
type googleFileKey struct {
	baseName string
	size     int64
}

// googleTrackingInfo holds tracking information for files.
type googleTrackingInfo struct {
	paths    []string
	count    int
	metadata *assets.Metadata
	status   fileevent.Code
}

// googleDirCatalog captures all files in a given directory.
type googleDirCatalog struct {
	jsons          map[string]*assets.Metadata
	unMatchedFiles map[string]*googleAssetFile
	matchedFiles   map[string]*assets.Asset
}

// googleAssetFile keeps information collected during pass one.
type googleAssetFile struct {
	fsys   fs.FS
	base   string
	length int
	date   time.Time
	md     *assets.Metadata
}

// Browse implements adapters.Source.
func (s *GoogleSource) Browse(ctx context.Context) <-chan *assets.Group {
	s.initialize()

	ctx, cancel := context.WithCancelCause(ctx)
	gOut := make(chan *assets.Group)
	go func() {
		defer close(gOut)

		// Pass one: walk all file systems and build catalogs
		for _, w := range s.fsyss {
			err := s.passOneFsWalk(ctx, w)
			if err != nil {
				cancel(err)
				return
			}
		}

		// Solve the puzzle: match JSON metadata to files
		err := s.solvePuzzle(ctx)
		if err != nil {
			cancel(err)
			return
		}

		// Pass two: emit assets as groups
		err = s.passTwo(ctx, gOut)
		cancel(err)
	}()
	return gOut
}

// Close implements adapters.Source.
func (s *GoogleSource) Close() error {
	return CloseFSs(s.fsyss)
}

// initialize sets up internal state for browsing.
func (s *GoogleSource) initialize() {
	s.infoCollector = filenames.NewInfoCollector(s.deps.TimeZone, s.deps.SupportedMedia)
	s.catalogs = make(map[string]googleDirCatalog)
	s.albums = make(map[string]assets.Album)
	s.fileTracker = gen.NewSyncMap[googleFileKey, googleTrackingInfo]()

	// Determine takeout name for tagging
	if s.config.TakeoutTag {
		for _, fsys := range s.fsyss {
			if named, ok := fsys.(fshelper.NameFS); ok {
				s.takeoutName = named.Name()
				break
			}
		}
		s.takeoutName = normalizeTakeoutName(s.takeoutName)
		if s.takeoutName == "" {
			s.config.TakeoutTag = false
		}
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

// passOneFsWalk scans all files in a file system to build the file catalog.
func (s *GoogleSource) passOneFsWalk(ctx context.Context, w fs.FS) error {
	return fs.WalkDir(w, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if d.IsDir() {
				return nil
			}

			dir, base := path.Split(name)
			dir = strings.TrimSuffix(dir, "/")
			ext := strings.ToLower(path.Ext(base))

			finfo, err := fs.Stat(w, name)
			if err != nil {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), 0, fileevent.ErrorFileAccess, "error", err.Error())
				return nil
			}

			// Exclude banned files
			if s.config.BannedFiles.Match(name) {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), 0, fileevent.DiscoveredBanned, "reason", "banned file")
				return nil
			}

			if s.deps.SupportedMedia.IsUseLess(name) {
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), 0, fileevent.DiscoveredUnknown, "reason", "useless file")
				return nil
			}

			if !s.config.InclusionFlags.IncludedExtensions.Include(ext) {
				s.deps.Processor.RecordAssetDiscardedImmediately(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscardedFiltered, "extension not included")
				return nil
			}
			if s.config.InclusionFlags.ExcludedExtensions.Exclude(ext) {
				s.deps.Processor.RecordAssetDiscardedImmediately(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscardedFiltered, "extension excluded")
				return nil
			}

			dirCatalog, ok := s.catalogs[dir]
			if !ok {
				dirCatalog.jsons = map[string]*assets.Metadata{}
				dirCatalog.unMatchedFiles = map[string]*googleAssetFile{}
				dirCatalog.matchedFiles = map[string]*assets.Asset{}
			}

			switch ext {
			case ".json":
				b, err := fs.ReadFile(w, name)
				if err != nil {
					return err
				}
				if bytes.Contains(b, []byte("immich-go version:")) {
					md, err := assets.UnMarshalMetadata(b)
					if err != nil {
						s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredUnsupported, "reason", "unknown JSON file")
						return nil
					}
					md.FileName = base
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredSidecar, "type", "immich-go metadata", "title", md.FileName)
					md.File = fshelper.FSName(w, name)
				} else {
					gmd, err := unmarshalGoogleJSON(b)
					if err == nil {
						switch {
						case gmd.isAsset():
							md := gmd.asMetadata(fshelper.FSName(w, name), s.config.PeopleTag)
							dirCatalog.jsons[base] = md
							s.deps.Logger.Debug("Asset JSON", "metadata", md)
							s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredSidecar, "type", "asset metadata", "title", md.FileName, "date", md.DateTaken)
						case gmd.isAlbum():
							s.deps.Logger.Debug("Album JSON", "metadata", gmd)
							if !s.config.KeepUntitled && gmd.Title == "" {
								s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredUnsupported, "reason", "discard untitled album")
								return nil
							}
							a := s.albums[dir]
							a.Title = gmd.Title
							if a.Title == "" {
								a.Title = filepath.Base(dir)
							}
							if e := gmd.Enrichments; e != nil {
								a.Description = e.Text
								a.Latitude = e.Latitude
								a.Longitude = e.Longitude
							}
							s.albums[dir] = a
							s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredSidecar, "type", "album metadata", "title", gmd.Title)
						default:
							s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredUnsupported, "reason", "unknown JSON file")
							return nil
						}
					} else {
						s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), int64(len(b)), fileevent.DiscoveredUnsupported, "reason", "unknown JSON file")
						return nil
					}
				}
			default:
				t := s.deps.SupportedMedia.TypeFromExt(ext)
				switch t {
				case filetypes.TypeUseless:
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscoveredUnknown, "reason", "useless file")
					return nil
				case filetypes.TypeUnknown:
					s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscoveredUnsupported, "reason", "unsupported file type")
					return nil
				case filetypes.TypeVideo:
					if strings.Contains(name, "Failed Videos") {
						s.deps.Processor.RecordAssetDiscardedImmediately(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscardedFiltered, "can't upload failed videos")
						return nil
					}
					s.deps.Processor.RecordAssetDiscovered(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscoveredVideo)
				case filetypes.TypeImage:
					s.deps.Processor.RecordAssetDiscovered(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscoveredImage)
				}

				key := googleFileKey{baseName: base, size: finfo.Size()}
				tracking, _ := s.fileTracker.Load(key)
				tracking.paths = append(tracking.paths, dir)
				tracking.count++
				s.fileTracker.Store(key, tracking)

				if _, ok := dirCatalog.unMatchedFiles[base]; ok {
					s.deps.Processor.RecordAssetDiscardedImmediately(ctx, fshelper.FSName(w, name), finfo.Size(), fileevent.DiscardedLocalDuplicate, "duplicated in the directory")
					return nil
				}

				dirCatalog.unMatchedFiles[base] = &googleAssetFile{
					fsys:   w,
					base:   base,
					length: int(finfo.Size()),
					date:   finfo.ModTime(),
				}
			}
			s.catalogs[dir] = dirCatalog
			return nil
		}
	})
}

// solvePuzzle matches JSON metadata to files using various matching strategies.
func (s *GoogleSource) solvePuzzle(ctx context.Context) error {
	dirs := gen.MapKeysSorted(s.catalogs)
	for _, dir := range dirs {
		cat := s.catalogs[dir]
		jsons := gen.MapKeysSorted(cat.jsons)

		for _, matcher := range googleMatchers {
			for _, jsonName := range jsons {
				md := cat.jsons[jsonName]
				for f := range cat.unMatchedFiles {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						if matcher.fn(jsonName, f, s.deps.SupportedMedia) {
							i := cat.unMatchedFiles[f]
							i.md = md
							a := s.makeAsset(ctx, dir, i, md)
							cat.matchedFiles[f] = a
							s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(i.fsys, path.Join(dir, i.base)), 0, fileevent.ProcessedAssociatedMetadata, "json", jsonName, "matcher", matcher.name)
							delete(cat.unMatchedFiles, f)
						}
					}
				}
			}
		}

		s.catalogs[dir] = cat

		// Handle unmatched files
		if len(cat.unMatchedFiles) > 0 {
			files := gen.MapKeys(cat.unMatchedFiles)
			sort.Strings(files)
			for _, f := range files {
				i := cat.unMatchedFiles[f]
				s.deps.Processor.RecordNonAsset(ctx, fshelper.FSName(i.fsys, path.Join(dir, i.base)), 0, fileevent.ProcessedMissingMetadata)
				if s.config.KeepJSONLess {
					a := s.makeAsset(ctx, dir, i, nil)
					cat.matchedFiles[f] = a
					delete(cat.unMatchedFiles, f)
				}
			}
		}
	}
	return nil
}

// passTwo emits matched assets as groups.
func (s *GoogleSource) passTwo(ctx context.Context, gOut chan *assets.Group) error {
	dirs := gen.MapKeys(s.catalogs)
	sort.Strings(dirs)

	for _, dir := range dirs {
		if len(s.catalogs[dir].matchedFiles) > 0 {
			err := s.handleDir(ctx, dir, gOut)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// handleDir processes a directory of matched files.
func (s *GoogleSource) handleDir(ctx context.Context, dir string, gOut chan *assets.Group) error {
	catalog := s.catalogs[dir]
	dirEntries := make([]*assets.Asset, 0, len(catalog.matchedFiles))

	// Filter files
	for name := range catalog.matchedFiles {
		a := catalog.matchedFiles[name]
		key := googleFileKey{baseName: name, size: int64(a.FileSize)}
		track, _ := s.fileTracker.Load(key)

		if track.status == fileevent.ProcessedUploadSuccess {
			a.Close()
			s.deps.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedLocalDuplicate, "local duplicate")
			continue
		}

		if code := s.filterOnMetadata(ctx, a); code != fileevent.Code(0) {
			continue
		}
		dirEntries = append(dirEntries, a)
	}

	// Process assets through grouper pipeline
	in := make(chan *assets.Asset)
	go func() {
		defer close(in)

		sort.Slice(dirEntries, func(i, j int) bool {
			radicalI := dirEntries[i].Radical
			radicalJ := dirEntries[j].Radical
			if radicalI != radicalJ {
				return radicalI < radicalJ
			}
			return dirEntries[i].CaptureDate.Before(dirEntries[j].CaptureDate)
		})

		for _, a := range dirEntries {
			s.assignAlbums(a, dir)

			if s.config.TakeoutTag {
				a.AddTag(s.takeoutName)
			}

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
			for _, a := range g.Assets {
				key := googleFileKey{baseName: path.Base(a.File.Name()), size: int64(a.FileSize)}
				track, _ := s.fileTracker.Load(key)
				track.status = fileevent.ProcessedUploadSuccess
				s.fileTracker.Store(key, track)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func normalizeTakeoutName(name string) string {
	if filepath.Ext(name) == ".zip" {
		name = strings.TrimSuffix(name, ".zip")
	}
	return takeoutPartRegex.ReplaceAllString(name, "")
}

// makeAsset creates an asset from a file and optional metadata.
func (s *GoogleSource) makeAsset(_ context.Context, dir string, f *googleAssetFile, md *assets.Metadata) *assets.Asset {
	file := path.Join(dir, f.base)
	a := &assets.Asset{
		File:             fshelper.FSName(f.fsys, file),
		FileSize:         f.length,
		OriginalFileName: f.base,
		FileDate:         f.date,
	}

	if md != nil && md.FileName != "" {
		title := md.FileName
		titleExt := path.Ext(title)
		fileExt := path.Ext(file)

		if titleExt != fileExt {
			title = strings.TrimSuffix(title, titleExt)
			titleExt = path.Ext(title)
			if titleExt != fileExt {
				title = strings.TrimSuffix(title, titleExt) + fileExt
			}
		}
		a.FromApplication = a.UseMetadata(md)
		a.OriginalFileName = title
	}
	a.SetNameInfo(s.infoCollector.GetInfo(a.OriginalFileName))
	return a
}

// filterOnMetadata filters assets based on their metadata.
func (s *GoogleSource) filterOnMetadata(ctx context.Context, a *assets.Asset) fileevent.Code {
	if !s.config.KeepArchived && a.Visibility == assets.VisibilityArchive {
		s.deps.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedFiltered, "discarding archived file")
		a.Close()
		return fileevent.DiscardedFiltered
	}
	if !s.config.KeepPartner && a.FromPartner {
		s.deps.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedFiltered, "discarding partner file")
		a.Close()
		return fileevent.DiscardedFiltered
	}
	if !s.config.KeepTrashed && a.Trashed {
		s.deps.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedFiltered, "discarding trashed file")
		a.Close()
		return fileevent.DiscardedFiltered
	}
	if s.config.InclusionFlags.DateRange.IsSet() && !s.config.InclusionFlags.DateRange.InRange(a.CaptureDate) {
		s.deps.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedFiltered, "discarding files out of date range")
		a.Close()
		return fileevent.DiscardedFiltered
	}
	if s.config.ImportFromAlbum != "" {
		keep := false
		dir := path.Dir(a.File.Name())
		if dir == "." {
			dir = ""
		}
		if album, ok := s.albums[dir]; ok {
			keep = keep || album.Title == s.config.ImportFromAlbum
		}
		if !keep {
			s.deps.Processor.RecordAssetDiscarded(ctx, a.File, int64(a.FileSize), fileevent.DiscardedFiltered, "discarding files not in the specified album")
			a.Close()
			return fileevent.DiscardedFiltered
		}
	}
	return fileevent.Code(0)
}

// assignAlbums assigns album information to an asset.
func (s *GoogleSource) assignAlbums(a *assets.Asset, dir string) {
	if !s.config.CreateAlbums {
		return
	}

	if s.config.ImportIntoAlbum != "" {
		a.Albums = []assets.Album{{Title: s.config.ImportIntoAlbum}}
	} else {
		key := googleFileKey{baseName: filepath.Base(a.File.Name()), size: int64(a.FileSize)}
		track, _ := s.fileTracker.Load(key)
		for _, p := range track.paths {
			if album, ok := s.albums[p]; ok {
				title := album.Title
				if title == "" {
					if !s.config.KeepUntitled {
						continue
					}
					title = filepath.Base(p)
				}
				a.Albums = append(a.Albums, assets.Album{
					Title:       title,
					Description: album.Description,
					Latitude:    album.Latitude,
					Longitude:   album.Longitude,
				})
			}
		}
	}

	if s.config.PartnerSharedAlbum != "" && a.FromPartner {
		a.Albums = append(a.Albums, assets.Album{Title: s.config.PartnerSharedAlbum})
	}
	if a.FromApplication != nil {
		a.FromApplication.Albums = a.Albums
	}

	// Use album GPS if asset has none
	if a.Latitude == 0 && a.Longitude == 0 {
		for _, album := range a.Albums {
			if album.Latitude != 0 || album.Longitude != 0 {
				a.Latitude = album.Latitude
				a.Longitude = album.Longitude
				break
			}
		}
	}
}

// ============================================================================
// Google Photos JSON metadata types and parsing
// ============================================================================

// googleMetaData represents the JSON metadata from Google Photos takeout.
type googleMetaData struct {
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	PhotoTakenTime     *googleTimeObject  `json:"photoTakenTime"`
	GeoDataExif        *googleGeoData     `json:"geoDataExif"`
	GeoData            *googleGeoData     `json:"geoData"`
	Trashed            bool               `json:"trashed,omitempty"`
	Archived           bool               `json:"archived,omitempty"`
	Favorited          bool               `json:"favorited,omitempty"`
	Enrichments        *googleEnrichments `json:"enrichments,omitempty"`
	People             []googlePerson     `json:"people,omitempty"`
	GooglePhotosOrigin googlePhotosOrigin `json:"googlePhotosOrigin"`
}

type googlePhotosOrigin struct {
	FromPartnerSharing bool `json:"fromPartnerSharing,omitempty"`
}

type googlePerson struct {
	Name string `json:"name"`
}

type googleTimeObject struct {
	Timestamp string `json:"timestamp"`
}

func (gt *googleTimeObject) Time() time.Time {
	ts, _ := strconv.ParseInt(gt.Timestamp, 10, 64)
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).In(time.Local)
}

type googleGeoData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type googleEnrichments struct {
	Text      string
	Latitude  float64
	Longitude float64
}

func (gmd *googleMetaData) isAlbum() bool {
	if gmd == nil || gmd.isAsset() {
		return false
	}
	return gmd.Title != ""
}

func (gmd *googleMetaData) isAsset() bool {
	if gmd == nil || gmd.PhotoTakenTime == nil {
		return false
	}
	return gmd.PhotoTakenTime.Timestamp != ""
}

func (gmd *googleMetaData) isPartner() bool {
	if gmd == nil {
		return false
	}
	return gmd.GooglePhotosOrigin.FromPartnerSharing
}

func (gmd *googleMetaData) asMetadata(name fshelper.FSAndName, tagPeople bool) *assets.Metadata {
	md := &assets.Metadata{
		File:        name,
		FileName:    sanitizeTitle(gmd.Title),
		Description: gmd.Description,
		Trashed:     gmd.Trashed,
		Archived:    gmd.Archived,
		Favorited:   gmd.Favorited,
		FromPartner: gmd.isPartner(),
	}
	if gmd.GeoDataExif != nil {
		md.Latitude, md.Longitude = gmd.GeoDataExif.Latitude, gmd.GeoDataExif.Longitude
		if md.Latitude == 0 && md.Longitude == 0 && gmd.GeoData != nil {
			md.Latitude, md.Longitude = gmd.GeoData.Latitude, gmd.GeoData.Longitude
		}
	} else if gmd.GeoData != nil {
		md.Latitude, md.Longitude = gmd.GeoData.Latitude, gmd.GeoData.Longitude
	}
	if gmd.PhotoTakenTime != nil && gmd.PhotoTakenTime.Timestamp != "" && gmd.PhotoTakenTime.Timestamp != "0" {
		md.DateTaken = gmd.PhotoTakenTime.Time()
	}
	if tagPeople {
		for _, p := range gmd.People {
			md.AddTag("People/" + p.Name)
		}
	}
	return md
}

func unmarshalGoogleJSON(data []byte) (*googleMetaData, error) {
	return fshelper.UnmarshalJSON[googleMetaData](data)
}

func sanitizeTitle(title string) string {
	return regexp.MustCompile(`[\r\n\\/:*?"<>|]`).ReplaceAllString(title, "_")
}

// ============================================================================
// Google Photos JSON/file matchers
// ============================================================================

type googleMatcherFn func(jsonName string, fileName string, sm filetypes.SupportedMedia) bool

var googleMatchers = []struct {
	name string
	fn   googleMatcherFn
}{
	{name: "matchFastTrack", fn: matchFastTrack},
	{name: "matchNormal", fn: matchNormal},
	{name: "matchForgottenDuplicates", fn: matchForgottenDuplicates},
	{name: "matchEditedName", fn: matchEditedName},
}

func matchFastTrack(jsonName string, fileName string, _ filetypes.SupportedMedia) bool {
	jsonName = strings.TrimSuffix(jsonName, path.Ext(jsonName))
	return jsonName == fileName
}

func matchNormal(jsonName string, fileName string, _ filetypes.SupportedMedia) bool {
	fileName, fileIndex := getFileIndex(fileName)
	jsonName, jsonIndex := getFileIndex(jsonName)

	if fileIndex != jsonIndex {
		return false
	}

	// supplemental-metadata check
	p2 := strings.LastIndex(jsonName, ".")
	if p2 > 1 {
		p1 := strings.LastIndex(jsonName[:p2], ".")
		if p1 > 1 {
			if strings.HasPrefix("supplemental-metadata", jsonName[p1+1:p2]) {
				jsonName = jsonName[:p1] + jsonName[p2:]
			}
		}
	}

	jsonName = strings.TrimSuffix(jsonName, path.Ext(jsonName))
	if jsonName == fileName {
		return true
	}

	if len(fileName) > 46 {
		if utf8.RuneCountInString(fileName) > 46 {
			fileName = string([]rune(fileName)[:46])
			if fileName == jsonName {
				return true
			}
		} else {
			fileName = strings.TrimSuffix(fileName, path.Ext(fileName))
			_, size := utf8.DecodeLastRuneInString(fileName)
			fileName = fileName[:len(fileName)-size]
			if fileName == jsonName {
				return true
			}
		}
	}
	return false
}

func matchEditedName(jsonName string, fileName string, sm filetypes.SupportedMedia) bool {
	if _, index := getFileIndex(fileName); index != "" {
		return false
	}
	base := strings.TrimSuffix(jsonName, path.Ext(jsonName))
	p1 := strings.LastIndex(base, ".")
	if p1 > 1 {
		if strings.HasPrefix("supplemental-metadata", base[p1+1:]) {
			base = jsonName[:p1]
		}
	}

	ext := path.Ext(base)
	if ext != "" && sm.IsMedia(ext) {
		base = strings.TrimSuffix(base, ext)
		fileName = strings.TrimSuffix(fileName, path.Ext(fileName))
	}
	return strings.HasPrefix(fileName, base)
}

func matchForgottenDuplicates(jsonName string, fileName string, _ filetypes.SupportedMedia) bool {
	jsonName = strings.TrimSuffix(jsonName, path.Ext(jsonName))
	fileName = strings.TrimSuffix(fileName, path.Ext(fileName))
	if strings.HasPrefix(fileName, jsonName) {
		a, b := utf8.RuneCountInString(jsonName), utf8.RuneCountInString(fileName)
		if b-a < 10 {
			return true
		}
	}
	return false
}

func getFileIndex(name string) (string, string) {
	p1File := strings.LastIndex(name, "(")
	if p1File >= 0 {
		p2File := strings.LastIndex(name, ")")
		if p2File >= 0 && p2File > p1File {
			fileIndex := name[p1File+1 : p2File]
			if _, err := strconv.Atoi(fileIndex); err == nil {
				return name[:p1File] + name[p2File+1:], fileIndex
			}
		}
	}
	return name, ""
}
