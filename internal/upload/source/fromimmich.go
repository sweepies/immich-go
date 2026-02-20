package source

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/journal"
	"github.com/sweepies/immich-go/internal/filenames"
	"github.com/sweepies/immich-go/internal/gen"
	"github.com/sweepies/immich-go/internal/immichfs"
)

// FromImmichSource implements adapters.Source for reading from another Immich server.
type FromImmichSource struct {
	deps   adapters.SourceDependencies
	config *adapters.FromImmichConfig

	// Internal state - set during Browse
	client    *immich.ImmichClient
	user      immich.User
	ifs       *immichfs.ImmichFS
	ic        *filenames.InfoCollector
	albumIDs  []string
	tagIDs    []string
	peopleIDs []string
}

// Browse implements adapters.Source.
func (s *FromImmichSource) Browse(ctx context.Context) <-chan *assets.Group {
	gOut := make(chan *assets.Group)
	go func() {
		defer close(gOut)

		// Initialize the Immich client
		if err := s.initialize(ctx); err != nil {
			s.deps.Logger.Error("Failed to initialize from-immich source", "error", err)
			return
		}

		// Get and emit assets
		if err := s.getAssets(ctx, gOut); err != nil {
			s.deps.Logger.Error("Failed to get assets from Immich", "error", err)
			return
		}
	}()
	return gOut
}

// Close implements adapters.Source.
func (s *FromImmichSource) Close() error {
	return nil
}

// initialize sets up the Immich client and validates configuration.
func (s *FromImmichSource) initialize(ctx context.Context) error {
	var err error

	// Create Immich client
	s.client, err = immich.NewImmichClient(s.config.ServerURL, s.config.APIKey)
	if err != nil {
		return fmt.Errorf("failed to create Immich client: %w", err)
	}

	// Validate the connection and get user info
	user, err := s.client.ValidateConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate Immich connection: %w", err)
	}
	s.user = user

	// Create ImmichFS for file access
	s.ifs = immichfs.NewImmichFS(ctx, s.config.ServerURL, s.client)
	s.ic = filenames.NewInfoCollector(s.deps.TimeZone, s.client.SupportedMedia())

	// Validate filter options
	if err := s.validateFilters(ctx); err != nil {
		return err
	}

	return nil
}

// validateFilters validates and resolves filter options.
func (s *FromImmichSource) validateFilters(ctx context.Context) error {
	// Validate make/model/location filters
	if s.config.Make != "" {
		if err := s.checkSuggestion(ctx, immich.SearchSuggestionRequest{
			Type: immich.SearchSuggestionTypeCameraMake,
		}, s.config.Make); err != nil {
			return fmt.Errorf("invalid make: %w", err)
		}
	}
	if s.config.Model != "" {
		if err := s.checkSuggestion(ctx, immich.SearchSuggestionRequest{
			Type: immich.SearchSuggestionTypeCameraModel,
			Make: s.config.Make,
		}, s.config.Model); err != nil {
			return fmt.Errorf("invalid model: %w", err)
		}
	}
	if s.config.Country != "" {
		if err := s.checkSuggestion(ctx, immich.SearchSuggestionRequest{
			Type: immich.SearchSuggestionTypeCountry,
		}, s.config.Country); err != nil {
			return fmt.Errorf("invalid country: %w", err)
		}
	}
	if s.config.State != "" {
		if err := s.checkSuggestion(ctx, immich.SearchSuggestionRequest{
			Type:    immich.SearchSuggestionTypeState,
			Country: s.config.Country,
		}, s.config.State); err != nil {
			return fmt.Errorf("invalid state: %w", err)
		}
	}
	if s.config.City != "" {
		if err := s.checkSuggestion(ctx, immich.SearchSuggestionRequest{
			Type:    immich.SearchSuggestionTypeCity,
			Country: s.config.Country,
			State:   s.config.State,
		}, s.config.City); err != nil {
			return fmt.Errorf("invalid city: %w", err)
		}
	}

	// Resolve albums
	if err := s.resolveAlbums(ctx); err != nil {
		return err
	}

	// Resolve tags
	if err := s.resolveTags(ctx); err != nil {
		return err
	}

	// Resolve people
	if err := s.resolvePeople(ctx); err != nil {
		return err
	}

	return nil
}

func (s *FromImmichSource) checkSuggestion(ctx context.Context, q immich.SearchSuggestionRequest, suggestion string) error {
	suggestions, err := s.client.GetSearchSuggestions(ctx, q)
	if err != nil {
		return err
	}
	if slices.Contains(suggestions, suggestion) {
		return nil
	}
	return fmt.Errorf("'%s' not in suggestions, accepted values: %s", suggestion, formatQuotedStrings(suggestions))
}

func (s *FromImmichSource) resolveAlbums(ctx context.Context) error {
	if len(s.config.Albums) == 0 {
		return nil
	}
	albums, err := s.client.GetAllAlbums(ctx)
	if err != nil {
		return err
	}
	var unknownAlbums []string

	for _, fromAlbum := range s.config.Albums {
		found := false
		for _, a := range albums {
			if a.AlbumName == fromAlbum {
				s.albumIDs = gen.AddOnce(s.albumIDs, a.ID)
				found = true
			}
		}
		if !found {
			unknownAlbums = append(unknownAlbums, fromAlbum)
		}
	}

	if len(unknownAlbums) == 0 {
		return nil
	}

	var availables []string
	for _, a := range albums {
		availables = append(availables, a.AlbumName)
	}
	return fmt.Errorf("unknown album(s): %v, available: %v", formatQuotedStrings(unknownAlbums), formatQuotedStrings(availables))
}

func (s *FromImmichSource) resolveTags(ctx context.Context) error {
	if len(s.config.Tags) == 0 {
		return nil
	}
	tags, err := s.client.GetAllTags(ctx)
	if err != nil {
		return err
	}
	var unknownTags []string

	for _, fromTag := range s.config.Tags {
		found := false
		for _, t := range tags {
			if t.Value == fromTag {
				s.tagIDs = gen.AddOnce(s.tagIDs, t.ID)
				found = true
			}
		}
		if !found {
			unknownTags = append(unknownTags, fromTag)
		}
	}

	if len(unknownTags) == 0 {
		return nil
	}

	var availables []string
	for _, t := range tags {
		availables = append(availables, t.Value)
	}
	return fmt.Errorf("unknown tag(s): %v, available: %v", formatQuotedStrings(unknownTags), formatQuotedStrings(availables))
}

func (s *FromImmichSource) resolvePeople(ctx context.Context) error {
	if len(s.config.People) == 0 {
		return nil
	}

	peopleMap, err := s.client.GetPeopleByNames(ctx, s.config.People)
	if err != nil {
		return fmt.Errorf("failed to resolve people names: %w", err)
	}

	var unknownPeople []string
	s.peopleIDs = nil

	for _, fromPerson := range s.config.People {
		if person, found := peopleMap[fromPerson]; found {
			s.peopleIDs = gen.AddOnce(s.peopleIDs, person.ID)
		} else {
			unknownPeople = append(unknownPeople, fromPerson)
		}
	}

	if len(unknownPeople) > 0 {
		var availablePeople []string
		err := s.client.GetAllPeopleIterator(ctx, func(person *immich.PersonResponseDto) error {
			if person.Name != "" {
				availablePeople = append(availablePeople, person.Name)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("unknown people: %v (failed to get available: %w)", formatQuotedStrings(unknownPeople), err)
		}
		return fmt.Errorf("unknown people: %v, available: %v", formatQuotedStrings(unknownPeople), formatQuotedStrings(availablePeople))
	}

	return nil
}

// getAssets fetches assets from the Immich server and emits them as groups.
func (s *FromImmichSource) getAssets(ctx context.Context, gOut chan *assets.Group) error {
	minRating := min(max(0, s.config.MinimalRating), 5)

	so := immich.SearchOptions()

	if !s.config.OnlyArchived && !s.config.OnlyTrashed && !s.config.OnlyFavorite {
		so.All()
	} else {
		if s.config.OnlyArchived {
			so.WithOnlyArchived()
		}
		if s.config.OnlyTrashed {
			so.WithOnlyTrashed()
		}
		if s.config.OnlyFavorite {
			so.WithOnlyFavorite()
		}
	}

	if s.config.Make != "" {
		so.WithOnlyMake(s.config.Make)
	}
	if s.config.Model != "" {
		so.WithOnlyMake(s.config.Model)
	}
	if s.config.Country != "" {
		so.WithOnlyCountry(s.config.Country)
	}
	if s.config.State != "" {
		so.WithOnlyState(s.config.State)
	}
	if s.config.City != "" {
		so.WithOnlyCity(s.config.City)
	}

	if s.config.OnlyNoAlbum {
		so.WithNotInAlbum()
	} else if len(s.albumIDs) > 0 {
		so.WithAlbums(s.albumIDs...)
	}

	if s.config.InclusionFlags.DateRange.IsSet() {
		so.WithDateRange(s.config.InclusionFlags.DateRange)
	}

	if minRating > 1 {
		so.WithMinimalRate(minRating)
	}

	if len(s.tagIDs) > 0 {
		so.WithTags(s.tagIDs...)
	}

	if len(s.peopleIDs) > 0 {
		so.WithPeople(s.peopleIDs...)
	}

	return s.client.GetFilteredAssetsFn(ctx, so, func(a *immich.Asset) error {
		// Filter on owner (partner assets)
		if !s.config.IncludePartners && a.OwnerID != s.user.ID {
			return nil
		}

		// Fetch full asset details
		a, err := s.client.GetAssetInfo(ctx, a.ID)
		if err != nil {
			s.deps.Logger.Error("Failed to get asset info", "id", a.ID, "error", err)
			return nil
		}

		asset := a.AsAsset()
		asset.FromApplication = &assets.Metadata{
			FileName:    a.OriginalFileName,
			Latitude:    a.ExifInfo.Latitude,
			Longitude:   a.ExifInfo.Longitude,
			Description: a.ExifInfo.Description,
			DateTaken:   a.ExifInfo.DateTimeOriginal.Time,
			Trashed:     a.IsTrashed,
			Archived:    a.IsArchived,
			Favorited:   a.IsFavorite,
			Rating:      byte(a.ExifInfo.Rating),
			Tags:        asset.Tags,
		}
		asset.UseMetadata(asset.FromApplication)
		asset.File = journal.NewFilename(s.ifs, a.ID)

		// Record asset discovery
		code := journal.DiscoveredImage
		if a.Type == "VIDEO" {
			code = journal.DiscoveredVideo
		}
		s.deps.Processor.RecordAssetDiscovered(ctx, asset.File, int64(asset.FileSize), code)

		// Get album information
		simplifiedAlbums, err := s.client.GetAssetAlbums(ctx, a.ID)
		if err != nil {
			s.deps.Logger.Warn("Failed to get asset albums", "id", a.ID, "error", err)
		} else {
			albums := immich.AlbumsFromAlbumSimplified(simplifiedAlbums)
			// Clear album IDs (they exist on source server, not destination)
			for i := range albums {
				albums[i].ID = ""
			}
			asset.Albums = albums
		}

		// Clear tag IDs for destination server
		for t := range asset.Tags {
			asset.Tags[t].ID = ""
		}

		g := assets.NewGroup(assets.GroupByNone, asset)
		select {
		case gOut <- g:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

func formatQuotedStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("'%s'", s)
	}
	return strings.Join(quoted, ", ")
}
