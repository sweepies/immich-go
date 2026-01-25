package source

import (
	"context"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/assets"
)

// FromImmichSource implements adapters.Source for reading from another Immich server.
type FromImmichSource struct {
	deps   adapters.SourceDependencies
	config *adapters.FromImmichConfig
}

// Browse implements adapters.Source.
// For now, this provides a stub implementation.
// TODO: Migrate full implementation from adapters/fromimmich/command.go
func (s *FromImmichSource) Browse(ctx context.Context) <-chan *assets.Group {
	gOut := make(chan *assets.Group)
	go func() {
		defer close(gOut)
		// The from-immich adapter requires an active Immich client connection.
		// For Phase 4, we provide the interface; full migration happens in Phase 5/6.
		s.deps.Logger.Warn("FromImmichSource.Browse: using stub implementation - use legacy adapter for full functionality")
	}()
	return gOut
}

// Close implements adapters.Source.
func (s *FromImmichSource) Close() error {
	return nil
}
