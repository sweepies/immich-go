package immich

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/fshelper"
)

// immich Asset simplified
type Asset struct {
	Checksum       string     `json:"checksum"`
	ExifInfo       ExifInfo   `json:"exifInfo"`
	FileCreatedAt  ImmichTime `json:"fileCreatedAt"`
	FileModifiedAt ImmichTime `json:"fileModifiedAt"`
	// hasMetadata
	ID         AssetID `json:"id"`
	IsArchived bool    `json:"isArchived"`
	IsFavorite bool    `json:"isFavorite"`
	// isOffline
	IsTrashed        bool    `json:"isTrashed"`
	LibraryID        string  `json:"libraryId,omitempty"`
	LivePhotoVideoID AssetID `json:"livePhotoVideoId"`

	// The local date and time when the photo/video was taken,
	// derived from EXIF metadata. This represents the
	// photographer's local time regardless of timezone,
	// stored as a timezone-agnostic timestamp.
	// Used for timeline grouping by "local" days and months.
	LocalDateTime    ImmichTime `json:"localDateTime"`
	OriginalFileName string     `json:"originalFileName"`
	// originalMimeType
	OriginalPath string `json:"originalPath"`
	// owner
	OwnerID UserID `json:"ownerId"`
	// people
	Resized bool `json:"resized"`
	// stack
	Tags      []TagSimplified `json:"tags"`
	Thumbhash string          `json:"thumbhash"`
	Type      string          `json:"type"`
	// unassignedFaces
	UpdatedAt ImmichTime `json:"updatedAt"`
	// rating not listed on the API page?
	Rating int `json:"rating"`
	// StackParentID string            `json:"stackParentId"`
	Visibility string `json:"visibility"`

	Albums []AlbumSimplified `json:"-"` // Albums that asset belong to, not in the API
}

// NewAssetFromImmich creates an assets.Asset from an immich.Asset.
func (ia Asset) AsAsset() *assets.Asset {
	a := &assets.Asset{
		FileDate:         ia.FileModifiedAt.Time,
		Description:      ia.ExifInfo.Description,
		OriginalFileName: ia.OriginalFileName,
		ID:               ia.ID.String(),
		CaptureDate:      ia.ExifInfo.DateTimeOriginal.Time,
		Trashed:          ia.IsTrashed,
		Archived:         ia.IsArchived,
		Favorite:         ia.IsFavorite,
		Rating:           ia.Rating,
		Latitude:         ia.ExifInfo.Latitude,
		Longitude:        ia.ExifInfo.Longitude,
		File:             fshelper.NewFilename(nil, ia.OriginalFileName),
		FileSize:         int(ia.ExifInfo.FileSizeInByte),
		Checksum:         ia.Checksum,
	}
	for _, album := range ia.Albums {
		a.Albums = append(a.Albums, assets.Album{
			Title:       album.AlbumName,
			Description: album.Description,
		})
	}

	for _, tag := range ia.Tags {
		a.Tags = append(a.Tags, tag.AsTag())
	}
	return a
}

type ExifInfo struct {
	Make             string         `json:"make"`
	Model            string         `json:"model"`
	ExifImageWidth   int            `json:"exifImageWidth"`
	ExifImageHeight  int            `json:"exifImageHeight"`
	FileSizeInByte   int64          `json:"fileSizeInByte"`
	Orientation      string         `json:"orientation"`
	DateTimeOriginal ImmichExifTime `json:"dateTimeOriginal"`
	// 	ModifyDate       time.Time `json:"modifyDate"`
	TimeZone string `json:"timeZone"`
	// LensModel        string    `json:"lensModel"`
	// 	FNumber          float64   `json:"fNumber"`
	// 	FocalLength      float64   `json:"focalLength"`
	// 	Iso              int       `json:"iso"`
	// 	ExposureTime     string    `json:"exposureTime"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	// 	City             string    `json:"city"`
	// 	State            string    `json:"state"`
	// 	Country          string    `json:"country"`
	Description string `json:"description"`
	Rating      int    `json:"rating"`
}

type AssetResponse struct {
	ID     AssetID `json:"id"`
	Status string  `json:"status"`
}

const (
	UploadCreated   = "created"
	UploadDuplicate = "duplicate"
)

// formatDuration formats a duration as hours, minutes, seconds, and milliseconds.
func formatDuration(duration time.Duration) string {
	hours := duration / time.Hour
	duration -= hours * time.Hour

	minutes := duration / time.Minute
	duration -= minutes * time.Minute

	seconds := duration / time.Second
	duration -= seconds * time.Second

	milliseconds := duration / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d.%06d", hours, minutes, seconds, milliseconds)
}

func (ic *ImmichClient) AssetUpload(ctx context.Context, la *assets.Asset) (AssetResponse, error) {
	return ic.uploadAsset(ctx, la)
}

type GetAssetOptions struct {
	UserID        string
	IsFavorite    bool
	IsArchived    bool
	WithoutThumbs bool
	Skip          string
}

func (o *GetAssetOptions) Values() url.Values {
	if o == nil {
		return url.Values{}
	}
	v := url.Values{}
	v.Add("userId", o.UserID)
	v.Add("isFavorite", myBool(o.IsFavorite).String())
	v.Add("isArchived", myBool(o.IsArchived).String())
	v.Add("withoutThumbs", myBool(o.WithoutThumbs).String())
	v.Add("skip", o.Skip)
	return v
}

func (ic *ImmichClient) DeleteAssets(ctx context.Context, ids []AssetID, forceDelete bool) error {
	if ic.dryRun {
		return nil
	}
	req := struct {
		Force bool      `json:"force"`
		IDs   []AssetID `json:"ids"`
	}{
		IDs:   ids,
		Force: forceDelete,
	}

	return ic.newServerCall(ctx, "DeleteAsset").do(deleteRequest("/assets", setJSONBody(&req)))
}

// getAssetInfo
// https://api.immich.app/endpoints/assets/getAssetInfo
func (ic *ImmichClient) GetAssetInfo(ctx context.Context, id AssetID) (*Asset, error) {
	r := Asset{}
	err := ic.newServerCall(ctx, "GetAssetInfo").do(getRequest("/assets/"+id.String(), setAcceptJSON()), responseJSON(&r))
	return &r, err
}

func (ic *ImmichClient) UpdateAssets(ctx context.Context, ids []AssetID,
	isArchived bool, isFavorite bool,
	latitude float64, longitude float64,
	removeParent bool, stackParentID StackID,
) error {
	if ic.dryRun {
		return nil
	}
	type updateAssetsRequest struct {
		IDs           []AssetID `json:"ids"`
		IsArchived    bool      `json:"isArchived"`
		IsFavorite    bool      `json:"isFavorite"`
		Latitude      float64   `json:"latitude"`
		Longitude     float64   `json:"longitude"`
		RemoveParent  bool      `json:"removeParent"`
		StackParentID StackID   `json:"stackParentId,omitempty"`
	}

	param := updateAssetsRequest{
		IDs:           ids,
		IsArchived:    isArchived,
		IsFavorite:    isFavorite,
		Latitude:      latitude,
		Longitude:     longitude,
		RemoveParent:  removeParent,
		StackParentID: stackParentID,
	}
	return ic.newServerCall(ctx, "updateAssets").do(putRequest("/assets", setJSONBody(param)))
}

// UpdateAssetRequest contains fields that can be updated on an asset.
type UpdateAssetRequest struct {
	IsArchived       bool      `json:"isArchived,omitempty"`
	IsFavorite       bool      `json:"isFavorite,omitempty"`
	Latitude         float64   `json:"latitude,omitempty"`
	Longitude        float64   `json:"longitude,omitempty"`
	Description      string    `json:"description,omitempty"`
	Rating           int       `json:"rating,omitempty"`
	DateTimeOriginal time.Time `json:"dateTimeOriginal,omitempty"`
}

// MarshalJSON includes both coordinates when either coordinate is non-zero.
func (u UpdateAssetRequest) MarshalJSON() ([]byte, error) {
	// withGPS is a struct that always includes Latitude and Longitude in the JSON output.
	type withGPS struct {
		IsArchived       bool      `json:"isArchived,omitempty"`
		IsFavorite       bool      `json:"isFavorite,omitempty"`
		Latitude         float64   `json:"latitude"`
		Longitude        float64   `json:"longitude"`
		Description      string    `json:"description,omitempty"`
		Rating           int       `json:"rating,omitempty"`
		DateTimeOriginal time.Time `json:"dateTimeOriginal,omitempty"`
	}

	// alias omits zero-valued coordinates.
	type alias UpdateAssetRequest

	// Check if Latitude or Longitude is non-zero, and use withGPS if true.
	if u.Latitude != 0 || u.Longitude != 0 {
		return json.Marshal(withGPS(u))
	}

	// Otherwise, use alias to omit Latitude and Longitude.
	return json.Marshal(alias(u))
}

func (ic *ImmichClient) UpdateAsset(ctx context.Context, id AssetID, param UpdateAssetRequest) (*Asset, error) {
	if ic.dryRun {
		return nil, nil
	}
	r := Asset{}
	err := ic.newServerCall(ctx, "updateAsset").do(putRequest("/assets/"+id.String(), setJSONBody(param)), responseJSON(&r))
	return &r, err
}

func (ic *ImmichClient) DownloadAsset(ctx context.Context, id AssetID) (io.ReadCloser, error) {
	var rc io.ReadCloser

	err := ic.newServerCall(ctx, "DownloadAsset").do(getRequest(fmt.Sprintf("/assets/%s/original", id), setOctetStream()), responseOctetStream(&rc))
	return rc, err
}

// CopyAsset copies metadata from sourceID to targetID.
func (ic *ImmichClient) CopyAsset(ctx context.Context, sourceID, targetID AssetID) error {
	if ic.dryRun {
		return nil
	}

	type assetCopyRequest struct {
		SourceID    AssetID `json:"sourceId"`
		TargetID    AssetID `json:"targetId"`
		Albums      bool    `json:"albums"`
		Favorite    bool    `json:"favorite"`
		SharedLinks bool    `json:"sharedLinks"`
		Sidecar     bool    `json:"sidecar"`
		Stack       bool    `json:"stack"`
	}

	req := assetCopyRequest{
		SourceID:    sourceID,
		TargetID:    targetID,
		Albums:      true,
		Favorite:    true,
		SharedLinks: true,
		Sidecar:     true,
		Stack:       true,
	}

	return ic.newServerCall(ctx, EndPointCopyAsset).do(
		putRequest("/assets/copy", setJSONBody(&req)),
	)
}
