package gp

import (
	"errors"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/adapters/shared"
	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/assets"
	cliflags "github.com/simulot/immich-go/internal/cliFlags"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/fshelper"
	"github.com/simulot/immich-go/internal/gen"
	"github.com/simulot/immich-go/internal/groups"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
	"github.com/simulot/immich-go/internal/namematcher"
	"github.com/spf13/pflag"
)

// ImportFlags represents the command-line flags for the Google Photos takeout import command.
type TakeoutCmd struct {
	// CLI FLags
	CreateAlbums       bool
	ImportFromAlbum    string
	ImportIntoAlbum    string
	PartnerSharedAlbum string
	KeepTrashed        bool
	KeepPartner        bool
	KeepUntitled       bool
	KeepArchived       bool
	KeepJSONLess       bool
	InclusionFlags     cliflags.InclusionFlags
	BannedFiles        namematcher.List
	TakeoutTag         bool
	TakeoutName        string
	PeopleTag          bool
	shared.StackOptions

	// internal state
	app            *app.Application
	processor      *fileprocessor.FileProcessor
	supportedMedia filetypes.SupportedMedia
	infoCollector  *filenames.InfoCollector
	tz             *time.Location
	fsyss          []fs.FS
	catalogs       map[string]directoryCatalog                // file catalogs by directory in the set of the all takeout parts
	albums         map[string]assets.Album                    // track album names by folder
	fileTracker    *gen.SyncMap[fileKeyTracker, trackingInfo] // map[fileKeyTracker]trackingInfo // key is base name + file size,  value is list of file paths
	groupers       []groups.Grouper
	// filters        []filters.Filter
}

// RegisterFlagsFlat registers flags for the flattened CLI (without subcommands).
// This is used by the new upload/archive commands that use source mode flags.
func (toc *TakeoutCmd) RegisterFlagsFlat(flags *pflag.FlagSet, forUpload bool) {
	toc.BannedFiles, _ = namematcher.New(shared.DefaultBannedFiles...)
	toc.supportedMedia = filetypes.DefaultSupportedMedia

	// Use safe registration to avoid duplicate flag errors when multiple adapters register the same flags
	shared.SafeBoolVar(flags, &toc.CreateAlbums, "sync-albums", true, "Automatically create albums in Immich that match the albums in your Google Photos takeout")
	shared.SafeStringVar(flags, &toc.ImportFromAlbum, "from-album-name", "", "Only import photos from the specified Google Photos album")
	shared.SafeBoolVar(flags, &toc.KeepUntitled, "include-untitled-albums", false, "Include photos from albums without a title in the import process")
	shared.SafeBoolVarP(flags, &toc.KeepTrashed, "include-trashed", "t", false, "Import photos that are marked as trashed in Google Photos")
	shared.SafeBoolVarP(flags, &toc.KeepPartner, "include-partner", "p", true, "Import photos from your partner's Google Photos account")
	shared.SafeStringVar(flags, &toc.PartnerSharedAlbum, "partner-shared-album", "", "Add partner's photo to the specified album name")
	shared.SafeBoolVarP(flags, &toc.KeepArchived, "include-archived", "a", true, "Import archived Google Photos")
	shared.SafeBoolVarP(flags, &toc.KeepJSONLess, "include-unmatched", "u", false, "Import photos that do not have a matching JSON file in the takeout")
	shared.SafeVar(flags, &toc.BannedFiles, "ban-file", "Exclude a file based on a pattern (case-insensitive). Can be specified multiple times.")
	shared.SafeBoolVar(flags, &toc.TakeoutTag, "takeout-tag", true, "Tag uploaded photos with a tag \"{takeout}/takeout-YYYYMMDDTHHMMSSZ\"")
	shared.SafeBoolVar(flags, &toc.PeopleTag, "people-tag", true, "Tag uploaded photos with tags \"people/name\" found in the JSON file")

	if forUpload {
		toc.StackOptions.RegisterFlagsSafe(flags)
	}

	toc.InclusionFlags.RegisterFlagsSafe(flags, "")
}

// NewAdapter creates a Google Photos takeout adapter with the given configuration.
// This is the factory function for the flattened CLI approach.
func (toc *TakeoutCmd) NewAdapter(app *app.Application, args []string) (adapters.Reader, error) {
	var err error

	log := app.Log()
	toc.app = app
	toc.processor = app.FileProcessor()
	toc.tz = app.GetTZ()

	// Make an fs.FS per zip file or folder given on the CLI
	toc.fsyss, err = fshelper.ParsePath(args)
	if err != nil {
		return nil, err
	}
	if len(toc.fsyss) == 0 {
		log.Message("No file found matching the pattern: %s", strings.Join(args, ","))
		return nil, errors.New("no file found matching the pattern: " + strings.Join(args, ","))
	}

	if toc.TakeoutTag {
		for _, fsys := range toc.fsyss {
			if fsys, ok := fsys.(fshelper.NameFS); ok {
				toc.TakeoutName = fsys.Name()
				break
			}
		}

		if filepath.Ext(toc.TakeoutName) == ".zip" {
			base := filepath.Base(toc.TakeoutName)
			toc.TakeoutName = strings.TrimSuffix(base, filepath.Ext(base))
		}
		if toc.TakeoutName == "" {
			toc.TakeoutTag = false
		}
		toc.TakeoutName = _re3digits.ReplaceAllString(toc.TakeoutName, "")
	}
	if toc.ManageEpsonFastFoto {
		g := epsonfastfoto.Group{}
		toc.groupers = append(toc.groupers, g.Group)
	}
	if toc.ManageBurst != filters.BurstNothing {
		toc.groupers = append(toc.groupers, burst.Group)
	}
	toc.groupers = append(toc.groupers, series.Group)

	toc.supportedMedia = toc.app.GetSupportedMedia()
	toc.infoCollector = filenames.NewInfoCollector(toc.tz, toc.supportedMedia)

	return toc, nil
}

// Close closes any resources held by the adapter
func (toc *TakeoutCmd) Close() error {
	return fshelper.CloseFSs(toc.fsyss)
}

var _re3digits = regexp.MustCompile(`-\d{3}$`)
