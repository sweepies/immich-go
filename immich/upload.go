package immich

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sweepies/immich-go/internal/assets"
)

type callValues string

const (
	TimeFormat    string     = "2006-01-02T15:04:05.000Z"
	ctxCallValues callValues = "call-values"
)

func setContextValue(kv map[string]string) serverRequestOption {
	return func(sc *serverCall, req *http.Request) error {
		if sc.err != nil || kv == nil {
			return nil
		}
		sc.ctx = context.WithValue(sc.ctx, ctxCallValues, kv)
		return nil
	}
}

func (ic *ImmichClient) uploadAsset(ctx context.Context, la *assets.Asset) (AssetResponse, error) {
	if ic.dryRun {
		return AssetResponse{
			ID:     AssetID(uuid.NewString()),
			Status: UploadCreated,
		}, nil
	}
	if err := ic.ensureServerVersion(ctx); err != nil {
		return AssetResponse{}, fmt.Errorf("discover server version before upload: %w", err)
	}

	var ar AssetResponse
	ext := path.Ext(la.OriginalFileName)
	if strings.TrimSuffix(la.OriginalFileName, ext) == "" {
		la.OriginalFileName = "No Name" + ext // fix #88, #128
	}
	if strings.ToUpper(ext) == ".MP" {
		ext = ".MP4" // #405
		la.OriginalFileName = la.OriginalFileName + ".MP4"
	}

	mtype := ic.TypeFromExt(ext)
	switch mtype {
	case "video", "image":
	default:
		return ar, fmt.Errorf("type file not supported: %s", path.Ext(la.OriginalFileName))
	}

	f, err := la.OpenFile()
	if err != nil {
		return ar, err
	}
	defer f.Close()

	s, err := f.Stat()
	if err != nil {
		return ar, err
	}

	callValues := ic.prepareCallValues(la, s)
	body, pw := io.Pipe()
	m := multipart.NewWriter(pw)

	errChan := make(chan error, 1)
	go func() {
		defer func() {
			m.Close()
			pw.Close()
		}()

		var gErr error
		gErr = ic.writeMultipartFields(m, callValues)
		if gErr != nil {
			errChan <- gErr
			return
		}

		gErr = ic.writeFilePart(m, f, la.OriginalFileName)
		if gErr != nil {
			errChan <- gErr
			return
		}

		if la.FromSideCar != nil && strings.HasSuffix(strings.ToLower(la.FromSideCar.File.Name()), ".xmp") {
			gErr = ic.writeSideCarPart(m, la)
			if gErr != nil {
				errChan <- gErr
				return
			}
		}
		errChan <- nil
	}()

	errCall := ic.newServerCall(ctx, EndPointAssetUpload).
		do(postRequest("/assets", m.FormDataContentType(), setContextValue(callValues), setAcceptJSON(), setImmichChecksum(la), setBody(body)), responseJSON(&ar))
	gErr := <-errChan
	if ar.Status == "duplicate" && errors.Is(gErr, io.ErrClosedPipe) {
		gErr = nil // immich closes the connection when we upload the x-immich-checksum header and it finds a duplicate
	}
	err = errors.Join(err, errCall, gErr)
	return ar, err
}

func (ic *ImmichClient) prepareCallValues(la *assets.Asset, s fs.FileInfo) map[string]string {
	serverVersion := ic.ServerVersion()
	callValues := map[string]string{
		"fileModifiedAt": s.ModTime().UTC().Format(TimeFormat),
		"isFavorite":     myBool(la.Favorite).String(),
	}
	if serverVersion.Major() < 3 {
		callValues["deviceAssetId"] = fmt.Sprintf("%s-%d", path.Base(la.OriginalFileName), s.Size())
		callValues["deviceId"] = ic.DeviceUUID
	}
	if !la.CaptureDate.IsZero() {
		callValues["fileCreatedAt"] = la.CaptureDate.Format(TimeFormat)
	} else {
		callValues["fileCreatedAt"] = s.ModTime().UTC().Format(TimeFormat)
	}
	duration := time.Duration(0)
	if value, ok := formatUploadDuration(serverVersion, &duration); ok {
		callValues["duration"] = value
	}
	if la.Archived {
		callValues["visibility"] = "archive"
	} else {
		callValues["visibility"] = "timeline"
	}
	return callValues
}

func formatUploadDuration(version ServerVersion, duration *time.Duration) (string, bool) {
	if duration == nil {
		return "", false
	}
	if version.Major() >= 3 {
		return strconv.FormatInt(duration.Milliseconds(), 10), true
	}
	return formatDuration(*duration), true
}

func (ic *ImmichClient) writeMultipartFields(m *multipart.Writer, callValues map[string]string) error {
	for key, value := range callValues {
		err := m.WriteField(key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ic *ImmichClient) writeFilePart(m *multipart.Writer, f io.Reader, originalFileName string) error {
	w, err := m.CreateFormFile("assetData", originalFileName)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func (ic *ImmichClient) writeSideCarPart(m *multipart.Writer, la *assets.Asset) error {
	scName := path.Base(la.OriginalFileName) + ".xmp"

	w, err := m.CreateFormFile("sidecarData", scName)
	if err != nil {
		return err
	}
	scf, err := la.FromSideCar.File.Open()
	if err != nil {
		return err
	}
	defer scf.Close()
	_, err = io.Copy(w, scf)
	return err
}
