package immich

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/filetypes"
)

// AssetID is a typed identifier for assets.
type AssetID string

func (id AssetID) String() string { return string(id) }

// AlbumID is a typed identifier for albums.
type AlbumID string

func (id AlbumID) String() string { return string(id) }

// TagID is a typed identifier for tags.
type TagID string

func (id TagID) String() string { return string(id) }

// StackID is a typed identifier for stacks.
type StackID string

func (id StackID) String() string { return string(id) }

// UserID is a typed identifier for users.
type UserID string

func (id UserID) String() string { return string(id) }

var (
	_ Client            = (*ImmichClient)(nil)
	_ UploadClient      = (*ImmichClient)(nil)
	_ JobControlService = (*ImmichClient)(nil)
)

// Client is the complete Immich API surface used by the application.
type Client interface {
	AssetsService
	ServerService
	AlbumsService
	TagsService
	StacksService
	JobsService
}

// UploadClient is the narrow user-client contract required by the upload pipeline.
// Administrative job control is supplied separately through JobsService.
type UploadClient interface {
	GetAssetStatistics(ctx context.Context) (AssetStatistics, error)
	GetAllAssets(ctx context.Context, fn func(*Asset) error) error
	AssetUpload(context.Context, *assets.Asset) (AssetResponse, error)
	UpdateAsset(ctx context.Context, id AssetID, fields UpdateAssetRequest) (*Asset, error)
	DeleteAssets(ctx context.Context, ids []AssetID, forceDelete bool) error
	CopyAsset(ctx context.Context, sourceID, targetID AssetID) error
	UserID() UserID

	GetAllAlbums(ctx context.Context) ([]AlbumSimplified, error)
	GetAlbumAssetIDs(ctx context.Context, albumID AlbumID) ([]AssetID, error)
	CreateAlbum(ctx context.Context, title, description string, ids []AssetID) (assets.Album, error)
	AddAssetToAlbum(ctx context.Context, albumID AlbumID, ids []AssetID) ([]UpdateAlbumResult, error)

	UpsertTags(ctx context.Context, tags []string) ([]TagSimplified, error)
	TagAssets(ctx context.Context, tagID TagID, ids []AssetID) ([]TagAssetsResponse, error)
	CreateStack(ctx context.Context, ids []AssetID) (StackID, error)
}

// AssetsService provides asset-related server operations.
type AssetsService interface {
	GetAssetInfo(ctx context.Context, id AssetID) (*Asset, error)
	DownloadAsset(ctx context.Context, id AssetID) (io.ReadCloser, error)
	UpdateAsset(ctx context.Context, id AssetID, fields UpdateAssetRequest) (*Asset, error)
	CopyAsset(ctx context.Context, sourceID, targetID AssetID) error
	GetAllAssets(ctx context.Context, fn func(*Asset) error) error
	UpdateAssets(
		ctx context.Context,
		ids []AssetID,
		isArchived bool,
		isFavorite bool,
		latitude float64,
		longitude float64,
		removeParent bool,
		stackParentID StackID,
	) error
	GetFilteredAssetsFn(ctx context.Context, so *searchOptions, filter func(*Asset) error) error
	GetAssetsByHash(ctx context.Context, hash string) ([]*Asset, error)
	GetAssetsByImageName(ctx context.Context, name string) ([]*Asset, error)
	AssetUpload(context.Context, *assets.Asset) (AssetResponse, error)
	DeleteAssets(ctx context.Context, ids []AssetID, forceDelete bool) error
}

// SuggestionService provides metadata-search suggestions.
type SuggestionService interface {
	GetSearchSuggestions(ctx context.Context, req SearchSuggestionRequest) (SearchSuggestions, error)
}

type RoundTripperDecorator func(rt http.RoundTripper) http.RoundTripper

// ServerService provides connection and server-level operations.
type ServerService interface {
	SetEndPoint(string)
	EnableAppTrace(rtd RoundTripperDecorator)
	SetDeviceUUID(string)
	PingServer(ctx context.Context) error
	ValidateConnection(ctx context.Context) (User, error)
	GetServerStatistics(ctx context.Context) (ServerStatistics, error)
	GetAssetStatistics(ctx context.Context) (AssetStatistics, error)
	SupportedMedia() filetypes.SupportedMedia
	GetAboutInfo(ctx context.Context) (AboutInfo, error)
	UserID() UserID
	ServerVersion() ServerVersion
}

// AlbumsService provides album operations.
type AlbumsService interface {
	GetAllAlbums(ctx context.Context) ([]AlbumSimplified, error)
	GetAlbumInfo(ctx context.Context, id AlbumID, withoutAssets bool) (AlbumContent, error)
	GetAlbumAssetIDs(ctx context.Context, albumID AlbumID) ([]AssetID, error)
	CreateAlbum(ctx context.Context, title, description string, ids []AssetID) (assets.Album, error)
	AddAssetToAlbum(ctx context.Context, albumID AlbumID, ids []AssetID) ([]UpdateAlbumResult, error)
	GetAssetAlbums(ctx context.Context, assetID AssetID) ([]AlbumSimplified, error)
	DeleteAlbum(ctx context.Context, id AlbumID) error
}

// TagsService provides tag operations.
type TagsService interface {
	GetAllTags(ctx context.Context) ([]TagSimplified, error)
	UpsertTags(ctx context.Context, tags []string) ([]TagSimplified, error)
	TagAssets(ctx context.Context, tagID TagID, assetIDs []AssetID) ([]TagAssetsResponse, error)
	BulkTagAssets(ctx context.Context, tagIDs []TagID, assetIDs []AssetID) (BulkTagResult, error)
}

// StacksService provides stack operations.
type StacksService interface {
	CreateStack(ctx context.Context, ids []AssetID) (StackID, error)
}

// JobControlService is the narrow administrative contract used by uploads.
type JobControlService interface {
	SendJobCommand(ctx context.Context, jobID string, command JobCommand, force bool) (JobCommandResponse, error)
}

// JobsService provides complete administrative background-job control.
type JobsService interface {
	JobControlService
	GetJobs(ctx context.Context) (map[string]Job, error)
	CreateJob(ctx context.Context, name JobName) error
}

// PeopleService provides person lookup operations.
type PeopleService interface {
	GetAllPeople(ctx context.Context, opts ...GetAllPeopleOptions) (*PeopleResponseDto, error)
	GetAllPeopleIterator(ctx context.Context, fn func(*PersonResponseDto) error, opts ...GetAllPeopleOptions) error
	GetPersonByName(ctx context.Context, name string, opts ...GetAllPeopleOptions) (*PersonResponseDto, error)
	GetPeopleByNames(ctx context.Context, names []string, opts ...GetAllPeopleOptions) (map[string]*PersonResponseDto, error)
}

type myBool bool

func (b myBool) String() string {
	if b {
		return "true"
	}
	return "false"
}

type ImmichTime struct {
	time.Time
}

// ImmichTime.UnmarshalJSON read time from the JSON string.
// The json provides a time UTC, but the server and the images dates are given in local time.
// The get the correct time into the struct, we capture the UTC time and return it in the local zone.
//
// workaround for: error at connection to immich server: cannot parse "+174510-04-28T00:49:44.000Z" as "2006" #28
// capture the error

func (t *ImmichTime) UnmarshalJSON(b []byte) error {
	var ts time.Time
	if len(b) < 3 {
		t.Time = time.Time{}
		return nil
	}
	b = b[1 : len(b)-1]
	ts, err := time.ParseInLocation("2006-01-02T15:04:05.000Z", string(b), time.UTC)
	if err != nil {
		t.Time = time.Time{}
		return nil
	}
	t.Time = ts.In(time.Local)
	return nil
}

func (t ImmichTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return json.Marshal("")
	}

	return json.Marshal(t.Format("\"" + time.RFC3339 + "\""))
}

type ImmichExifTime struct {
	time.Time
}

// ImmichTime.UnmarshalJSON read time from the JSON string.
// The json provides a time UTC, but the server and the images dates are given in local time.
// The get the correct time into the struct, we capture the UTC time and return it in the local zone.
//
// workaround for: error at connection to immich server: cannot parse "+174510-04-28T00:49:44.000Z" as "2006" #28
// capture the error

func (t *ImmichExifTime) UnmarshalJSON(b []byte) error {
	var ts time.Time
	if len(b) < 3 {
		t.Time = time.Time{}
		return nil
	}
	b = b[1 : len(b)-1]
	var err error
	var pattern string
	str := string(b)

	switch len(b) {
	case 29:
		pattern = "2006-01-02T15:04:05.000+00:00"
	case 28:
		pattern = "2006-01-02T15:04:05.00+00:00"
	case 27:
		pattern = "2006-01-02T15:04:05.0+00:00"
	case 25:
		pattern = "2006-01-02T15:04:05+00:00"
	}

	if pattern != "" {
		ts, err = time.ParseInLocation(pattern, str, time.UTC)
		if err != nil {
			t.Time = time.Time{}
			return nil
		}
	}

	t.Time = ts.In(time.Local)
	return nil
}

func (t ImmichExifTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return json.Marshal("")
	}

	return json.Marshal(t.Format("\"" + time.RFC3339 + "\""))
}
