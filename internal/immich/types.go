package immich

import (
	"encoding/json"
	"time"
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

// ImmichTime handles Immich server time parsing with timezone normalization.
type ImmichTime struct {
	time.Time
}

func (t *ImmichTime) UnmarshalJSON(b []byte) error {
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

// ImmichExifTime handles EXIF timestamp parsing with various formats.
type ImmichExifTime struct {
	time.Time
}

func (t *ImmichExifTime) UnmarshalJSON(b []byte) error {
	if len(b) < 3 {
		t.Time = time.Time{}
		return nil
	}
	b = b[1 : len(b)-1]
	var ts time.Time
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
