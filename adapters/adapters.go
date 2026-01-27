package adapters

import (
	"context"
	"io"

	"github.com/sweepies/immich-go/internal/assets"
)

// Reader is the legacy interface for asset sources.
// Deprecated: Use internal/adapters.Source instead for new code.
type Reader interface {
	Browse(cxt context.Context) chan *assets.Group
}

// Source is the new interface for asset sources.
// It extends Reader with proper resource cleanup.
type Source interface {
	// Browse returns a channel of asset groups from the source.
	Browse(ctx context.Context) <-chan *assets.Group

	// Close releases any resources held by the source.
	io.Closer
}

type AssetWriter interface {
	WriteAsset(context.Context, *assets.Asset) error
	// WriteGroup(ctx context.Context, group *assets.Group) error
}
