package source

import (
	"context"
	"io/fs"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/filters"
	"github.com/simulot/immich-go/internal/groups"
	"github.com/simulot/immich-go/internal/groups/burst"
	"github.com/simulot/immich-go/internal/groups/epsonfastfoto"
	"github.com/simulot/immich-go/internal/groups/series"
)

// GoogleSource implements adapters.Source for Google Photos takeout imports.
type GoogleSource struct {
	deps   adapters.SourceDependencies
	config *adapters.GoogleConfig
	fsyss  []fs.FS

	// Internal state
	infoCollector *filenames.InfoCollector
	groupers      []groups.Grouper
}

// Browse implements adapters.Source.
// For now, this delegates to the existing googlephotos package implementation.
// TODO: Migrate full implementation from adapters/googlePhotos/googlephotos.go
func (s *GoogleSource) Browse(ctx context.Context) <-chan *assets.Group {
	s.initialize()

	gOut := make(chan *assets.Group)
	go func() {
		defer close(gOut)
		// The Google Photos adapter has complex multi-pass logic.
		// For Phase 4, we provide the interface; full migration happens in Phase 5/6.
		s.deps.Logger.Warn("GoogleSource.Browse: using stub implementation - use legacy adapter for full functionality")
	}()
	return gOut
}

// Close implements adapters.Source.
func (s *GoogleSource) Close() error {
	return CloseFSs(s.fsyss)
}

// initialize sets up internal state for browsing.
func (s *GoogleSource) initialize() {
	s.infoCollector = filenames.NewInfoCollector(s.deps.TimeZone, s.deps.SupportedMedia)

	// Set up groupers
	if s.config.ManageEpsonFastFoto {
		s.groupers = append(s.groupers, epsonfastfoto.Group{}.Group)
	}
	if s.config.ManageBurst != filters.BurstNothing {
		s.groupers = append(s.groupers, burst.Group)
	}
	s.groupers = append(s.groupers, series.Group)
}
