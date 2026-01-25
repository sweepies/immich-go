// Package source provides factory functions for creating asset sources (adapters).
// It centralizes adapter setup logic that was previously in app/upload/upload.go,
// allowing adapters to be created with explicit dependencies rather than through
// the service-locator pattern.
package source

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/fshelper"
)

// Factory creates sources (adapters) from configuration.
// It encapsulates the dependencies needed by adapters.
type Factory struct {
	log             *slog.Logger
	processor       *fileprocessor.FileProcessor
	supportedMedia  filetypes.SupportedMedia
	tz              *time.Location
	concurrentTasks int
}

// NewFactory creates a new source factory with the given dependencies.
func NewFactory(
	log *slog.Logger,
	processor *fileprocessor.FileProcessor,
	supportedMedia filetypes.SupportedMedia,
	tz *time.Location,
	concurrentTasks int,
) *Factory {
	return &Factory{
		log:             log,
		processor:       processor,
		supportedMedia:  supportedMedia,
		tz:              tz,
		concurrentTasks: concurrentTasks,
	}
}

// Dependencies returns the SourceDependencies for adapters.
func (f *Factory) Dependencies() adapters.SourceDependencies {
	return adapters.SourceDependencies{
		Logger:          f.log,
		Processor:       f.processor,
		SupportedMedia:  f.supportedMedia,
		TimeZone:        f.tz,
		ConcurrentTasks: f.concurrentTasks,
	}
}

// ParsePaths parses path arguments into file systems.
// This is a helper that can be used by adapters.
func ParsePaths(paths []string) ([]fs.FS, error) {
	if len(paths) == 0 {
		return nil, errors.New("no paths provided")
	}
	fsyss, err := fshelper.ParsePath(paths)
	if err != nil {
		return nil, err
	}
	if len(fsyss) == 0 {
		return nil, errors.New("no files found matching the pattern: " + strings.Join(paths, ","))
	}
	return fsyss, nil
}

// CloseFSs closes a slice of file systems.
func CloseFSs(fsyss []fs.FS) error {
	return fshelper.CloseFSs(fsyss)
}

// CreateFromConfig creates a source based on the mode and configuration.
// This is the main entry point for creating sources.
func (f *Factory) CreateFromConfig(ctx context.Context, mode adapters.SourceMode, cfg any) (adapters.Source, error) {
	switch mode {
	case adapters.SourceModeFolder:
		folderCfg, ok := cfg.(*adapters.FolderConfig)
		if !ok {
			return nil, errors.New("invalid configuration for folder source")
		}
		return f.CreateFolderSource(ctx, folderCfg)

	case adapters.SourceModeICloud:
		folderCfg, ok := cfg.(*adapters.FolderConfig)
		if !ok {
			return nil, errors.New("invalid configuration for iCloud source")
		}
		folderCfg.ICloudTakeout = true
		return f.CreateFolderSource(ctx, folderCfg)

	case adapters.SourceModePicasa:
		folderCfg, ok := cfg.(*adapters.FolderConfig)
		if !ok {
			return nil, errors.New("invalid configuration for Picasa source")
		}
		folderCfg.PicasaAlbum = true
		return f.CreateFolderSource(ctx, folderCfg)

	case adapters.SourceModeGoogle:
		googleCfg, ok := cfg.(*adapters.GoogleConfig)
		if !ok {
			return nil, errors.New("invalid configuration for Google source")
		}
		return f.CreateGoogleSource(ctx, googleCfg)

	case adapters.SourceModeFromImmich:
		immichCfg, ok := cfg.(*adapters.FromImmichConfig)
		if !ok {
			return nil, errors.New("invalid configuration for from-immich source")
		}
		return f.CreateFromImmichSource(ctx, immichCfg)

	default:
		return nil, errors.New("unknown source mode: " + string(mode))
	}
}

// CreateFolderSource creates a folder-based source.
// This handles folder, iCloud, and Picasa modes.
func (f *Factory) CreateFolderSource(_ context.Context, cfg *adapters.FolderConfig) (adapters.Source, error) {
	fsyss, err := ParsePaths(cfg.Paths)
	if err != nil {
		return nil, err
	}

	return &FolderSource{
		deps:   f.Dependencies(),
		config: cfg,
		fsyss:  fsyss,
	}, nil
}

// CreateGoogleSource creates a Google Photos takeout source.
func (f *Factory) CreateGoogleSource(_ context.Context, cfg *adapters.GoogleConfig) (adapters.Source, error) {
	fsyss, err := ParsePaths(cfg.Paths)
	if err != nil {
		return nil, err
	}

	return &GoogleSource{
		deps:   f.Dependencies(),
		config: cfg,
		fsyss:  fsyss,
	}, nil
}

// CreateFromImmichSource creates a source that reads from another Immich server.
func (f *Factory) CreateFromImmichSource(_ context.Context, cfg *adapters.FromImmichConfig) (adapters.Source, error) {
	return &FromImmichSource{
		deps:   f.Dependencies(),
		config: cfg,
	}, nil
}
