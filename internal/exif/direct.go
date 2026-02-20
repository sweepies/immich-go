package exif

/*
	Read metadata from a file not using exiftool.

	TODO: Use sync.Pool for buffers
*/
import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/sweepies/immich-go/internal/assets"
)

// MetadataFromDirectRead read the file using GO package
func MetadataFromDirectRead(f io.Reader, name string, localTZ *time.Location) (*assets.Metadata, error) {
	var md *assets.Metadata
	var err error
	ext := strings.ToLower(path.Ext(name))

	switch ext {
	case ".heic", ".heif":
		md, err = readHEIFMetadata(f, localTZ)
	case ".jpg", ".jpeg", ".dng", ".cr2", ".arw", ".raf", ".nef":
		md, err = readExifMetadata(f, localTZ)
	case ".mp4", ".mov":
		md, err = readMP4Metadata(f)
	case ".cr3":
		md, err = readCR3Metadata(f, localTZ)
	default:
		return nil, fmt.Errorf("can't read metadata for this format '%s'", ext)
	}
	if err != nil {
		return nil, fmt.Errorf("can't read metadata: %w", err)
	}

	return md, nil
}

// readExifMetadata locate the Exif part and return the date of capture
func readExifMetadata(r io.Reader, localTZ *time.Location) (*assets.Metadata, error) {
	// try to read the Exif data directly
	readBuffer := bufferPool.Get().(*bytes.Buffer)
	readBuffer.Reset()
	defer bufferPool.Put(readBuffer)

	r2 := io.TeeReader(r, readBuffer)
	x, err := exif.Decode(r2)
	if err == nil || !exif.IsCriticalError(err) {
		return getExifMetadata(x, localTZ)
	}

	b := slicePool.Get().([]byte)
	defer slicePool.Put(b)

	// search for the Exif header
	r, err = searchPattern(io.MultiReader(readBuffer, r), []byte("Exif\x00\x00"), b)
	if err == nil {
		x, err = exif.Decode(r)
		if err == nil || !exif.IsCriticalError(err) {
			return getExifMetadata(x, localTZ)
		}
	}
	return nil, err
}

const searchBufferSize = 32 * 1024

var (
	slicePool = sync.Pool{
		New: func() any {
			return make([]byte, searchBufferSize)
		},
	}

	bufferPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, searchBufferSize))
		},
	}
)

// readHEIFMetadata locate the Exif part and return the date of capture
func readHEIFMetadata(r io.Reader, localTZ *time.Location) (*assets.Metadata, error) {
	b := slicePool.Get().([]byte)
	defer slicePool.Put(b)

	r, err := searchPattern(r, []byte{0x45, 0x78, 0x69, 0x66, 0, 0, 0x4d, 0x4d}, b)
	if err != nil {
		return nil, err
	}

	filler := make([]byte, 6)
	_, err = r.Read(filler)
	if err != nil {
		return nil, err
	}
	x, err := exif.Decode(r)
	if err == nil || !exif.IsCriticalError(err) {
		return getExifMetadata(x, localTZ)
	}
	return nil, err
}

// readMP4Metadata locate the mvhd atom and decode the date of capture
func readMP4Metadata(r io.Reader) (*assets.Metadata, error) {
	b := slicePool.Get().([]byte)
	defer slicePool.Put(b)

	r, err := searchPattern(r, []byte{'m', 'v', 'h', 'd'}, b)
	if err != nil {
		return nil, err
	}
	atom, err := decodeMvhdAtom(r)
	if err != nil {
		return nil, err
	}
	t := atom.CreationTime
	if t.Year() < 2000 {
		t = atom.ModificationTime
	}
	if t.Year() < 2000 {
		t = time.Time{}
	}
	return &assets.Metadata{DateTaken: t}, nil
}

// readCR3Metadata locate the CMT1 atom and decode the date of capture
func readCR3Metadata(r io.Reader, localTZ *time.Location) (*assets.Metadata, error) {
	b := slicePool.Get().([]byte)
	defer slicePool.Put(b)

	r, err := searchPattern(r, []byte("CMT1"), b)
	if err != nil {
		return nil, err
	}

	filler := make([]byte, 4)
	_, err = r.Read(filler)
	if err != nil {
		return nil, err
	}
	x, err := exif.Decode(r)
	if err == nil || !exif.IsCriticalError(err) {
		return getExifMetadata(x, localTZ)
	}
	return nil, err
}

// type exifDumper struct{}

// func (exifDumper) Walk(name exif.FieldName, tag *tiff.Tag) error {
// 	fmt.Printf("%s: %s\n", name, tag)
// 	return nil
// }

// getExifMetadata extract the date and location from the Exif data

func getExifMetadata(x *exif.Exif, local *time.Location) (*assets.Metadata, error) {
	var err error

	// _ = x.Walk(exifDumper{})

	md := &assets.Metadata{}
	// md.DateTaken, err = readGPSTimeStamp(x, local)
	// if err != nil || md.DateTaken.IsZero() {
	// GPS Time Stamp is not reliable

	md.DateTaken, err = readDateTime(x, exif.DateTimeOriginal, exif.SubSecTimeOriginal, local)
	if err != nil {
		md.DateTaken, err = readDateTime(x, exif.DateTime, exif.SubSecTime, local)
	}
	if err == nil {
		lat, lon, err := x.LatLong()
		if err == nil {
			md.Latitude = lat
			md.Longitude = lon
		}
	}
	return md, err
}

// readDateTime with subsecond when possible
func readDateTime(x *exif.Exif, dateTag exif.FieldName, subSecTag exif.FieldName, local *time.Location) (time.Time, error) {
	date, err := getTagSting(x, dateTag)
	if err != nil {
		return time.Time{}, err
	}
	subSec, err := getTagSting(x, subSecTag)
	if err == nil {
		subSec += "000"
		date = date + "." + subSec[:3]
	}
	return parseExifTime(date, local)
}

func parseExifTime(date string, local *time.Location) (time.Time, error) {
	date = strings.TrimSpace(date)
	var year, month, day, hour, minutes, sec, milli int
	// Normalize date separators only in the date portion (YYYY-MM-DD or YYYY/MM/DD → YYYY:MM:DD).
	// Avoid replacing '-' in the time/offset portion (e.g. timezone offset "-05:00").
	if len(date) >= 10 {
		prefix := strings.Map(func(r rune) rune {
			if r == '-' || r == '/' {
				return ':'
			}
			return r
		}, date[:10])
		date = prefix + date[10:]
	}
	_, err := fmt.Sscanf(date, "%d:%d:%d %d:%d:%d.%d", &year, &month, &day, &hour, &minutes, &sec, &milli)
	if (err != nil && err.Error() != "unexpected EOF") || year < 1900 || month == 0 || day == 0 {
		return time.Time{}, fmt.Errorf("invalid date format")
	}
	d := time.Date(year, time.Month(month), day, hour, minutes, sec, milli*int(time.Millisecond), local)
	return d, nil
}

func getTagSting(x *exif.Exif, tagName exif.FieldName) (string, error) {
	t, err := x.Get(tagName)
	if err != nil {
		return "", err
	}
	s := strings.TrimRight(strings.TrimLeft(t.String(), `"`), `"`)
	return s, nil
}
