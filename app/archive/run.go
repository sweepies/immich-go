package archive

import (
	"errors"
	"os"

	"github.com/simulot/immich-go/adapters/folder"
	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/fshelper/osfs"
	"github.com/spf13/cobra"
)

func (ac *ArchiveCmd) Run(cmd *cobra.Command, source adapters.Source) error {
	// ready to run
	ctx := cmd.Context()
	log := ac.app.Log()
	log.Info("in ArchiveCmd.Run", "archivePath", ac.config.ArchivePath)

	// Initialize the FileProcessor using centralized factory
	ac.app.EnsureFileProcessor()

	p := ac.config.ArchivePath
	err := os.MkdirAll(p, 0o755)
	if err != nil {
		return err
	}

	destFS := osfs.DirFS(p)
	ac.dest, err = folder.NewLocalAssetWriter(destFS, ".")
	if err != nil {
		return err
	}
	// Close the underlying filesystem if it supports closing (defensive code for future FS types)
	if c, ok := destFS.(interface{ Close() error }); ok {
		defer func() {
			if closeErr := c.Close(); closeErr != nil {
				log.Error("failed to close filesystem", "error", closeErr)
			}
		}()
	}

	gChan := source.Browse(ctx)
	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case g, ok := <-gChan:
			if !ok {
				return nil
			}
			for _, a := range g.Assets {
				err := ac.dest.WriteAsset(ctx, a)
				if err == nil {
					err = a.Close()
				}
				if err != nil {
					ac.app.FileProcessor().RecordAssetError(ctx, a.File, int64(a.FileSize), fileevent.ErrorFileAccess, err)
					errCount++
					if errCount > 5 {
						err := errors.New("too many errors, aborting")
						log.Error(err.Error())
						return err
					}
				} else {
					// Asset successfully archived
					ac.app.FileProcessor().RecordAssetProcessed(ctx, a.File, int64(a.FileSize), fileevent.ProcessedFileArchived)
				}
			}
		}
	}
}
