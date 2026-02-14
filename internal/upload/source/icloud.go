package source

import (
	"encoding/csv"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/gen"
)

const iCloudOriginalCreationDateLayout = "Monday January 2,2006 15:04 PM GMT"

func useICloudMemory(m *gen.SyncMap[string, icloudMeta], fsys fs.FS, filename string) (string, error) {
	albumName := "Memory " + strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	return albumName, useICloudAlbumCSV(m, fsys, filename, albumName)
}

func useICloudAlbum(m *gen.SyncMap[string, icloudMeta], fsys fs.FS, filename string) (string, error) {
	albumName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	return albumName, useICloudAlbumCSV(m, fsys, filename, albumName)
}

func useICloudAlbumCSV(m *gen.SyncMap[string, icloudMeta], fsys fs.FS, filename, albumName string) error {
	file, err := fsys.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return errors.Join(err, errors.New("failed to read all csv records"))
	}
	if len(records) == 0 {
		return nil
	}

	for _, record := range records[1:] {
		if len(record) != 1 {
			return errors.New("invalid record")
		}
		fileName := record[0]
		m.Update(fileName, func(meta icloudMeta) icloudMeta {
			meta.albums = append(meta.albums, assets.Album{Title: albumName})
			return meta
		})
	}

	return nil
}

func useICloudPhotoDetails(m *gen.SyncMap[string, icloudMeta], fsys fs.FS, filename string) error {
	file, err := fsys.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return errors.Join(err, errors.New("failed to read all csv records"))
	}
	if len(records) == 0 {
		return nil
	}

	for _, record := range records[1:] {
		if len(record) != 8 {
			return errors.New("invalid record")
		}

		fileName := record[0]
		originalCreationDate := record[5]
		t, err := time.Parse(iCloudOriginalCreationDateLayout, originalCreationDate)
		if err != nil {
			return errors.Join(err, errors.New("invalid original creation date"))
		}

		m.Update(fileName, func(meta icloudMeta) icloudMeta {
			meta.originalCreationDate = t
			return meta
		})
	}

	return nil
}
