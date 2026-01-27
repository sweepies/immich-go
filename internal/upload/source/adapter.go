package source

import (
	"context"
	"io"

	legacyadapters "github.com/sweepies/immich-go/adapters"
	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assets"
)

// LegacyReaderAdapter wraps an old-style adapters.Reader to implement the new adapters.Source interface.
// This enables gradual migration from the old adapter pattern to the new one.
type LegacyReaderAdapter struct {
	reader legacyadapters.Reader
	closer io.Closer
}

// NewLegacyReaderAdapter creates an adapter that wraps an old-style Reader.
func NewLegacyReaderAdapter(reader legacyadapters.Reader, closer io.Closer) *LegacyReaderAdapter {
	return &LegacyReaderAdapter{
		reader: reader,
		closer: closer,
	}
}

// Browse implements adapters.Source.
func (a *LegacyReaderAdapter) Browse(ctx context.Context) <-chan *assets.Group {
	// The legacy Reader returns a bidirectional channel, we return it as receive-only
	return a.reader.Browse(ctx)
}

// Close implements adapters.Source.
func (a *LegacyReaderAdapter) Close() error {
	if a.closer != nil {
		return a.closer.Close()
	}
	return nil
}

// Ensure LegacyReaderAdapter implements adapters.Source
var _ adapters.Source = (*LegacyReaderAdapter)(nil)
