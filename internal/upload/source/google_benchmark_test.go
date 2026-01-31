package source

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sweepies/immich-go/internal/adapters"
	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/assettracker"
	"github.com/sweepies/immich-go/internal/fileevent"
	"github.com/sweepies/immich-go/internal/fileprocessor"
	"github.com/sweepies/immich-go/internal/filenames"
	"github.com/sweepies/immich-go/internal/filetypes"
	"github.com/sweepies/immich-go/internal/gen"
)

func BenchmarkSolvePuzzle(b *testing.B) {
	// Setup a large number of files
	const numFiles = 5000

	ctx := context.Background()

	// Prepare data
	jsons := make(map[string]*assets.Metadata)
	unMatchedFiles := make(map[string]*googleAssetFile)

	// Add FastTrack matches (50%)
	for i := 0; i < numFiles/2; i++ {
		base := fmt.Sprintf("IMG_%04d.jpg", i)
		jsonName := base + ".json"

		jsons[jsonName] = &assets.Metadata{FileName: base}
		unMatchedFiles[base] = &googleAssetFile{
			base:   base,
			length: 1000,
			date:   time.Now(),
		}
	}

	// Add Normal matches with index (25%)
	for i := numFiles/2; i < numFiles*3/4; i++ {
		base := fmt.Sprintf("IMG_%04d(1).jpg", i)
		jsonName := fmt.Sprintf("IMG_%04d.jpg(1).json", i)

		jsons[jsonName] = &assets.Metadata{FileName: base}
		unMatchedFiles[base] = &googleAssetFile{
			base:   base,
			length: 1000,
			date:   time.Now(),
		}
	}

	// Add Truncated matches (10%)
	longBase := strings.Repeat("A", 60) + ".jpg"
	for i := numFiles*3/4; i < numFiles*0.85; i++ {
		// Unique-ify
		base := fmt.Sprintf("%d%s", i, longBase)
		// Truncate to 46 chars
		if len(base) > 46 {
			// json name is truncated version
			trunc := string([]rune(base)[:46])
			jsonName := trunc + ".json"

			jsons[jsonName] = &assets.Metadata{FileName: base}
			unMatchedFiles[base] = &googleAssetFile{
				base:   base,
				length: 1000,
				date:   time.Now(),
			}
		}
	}

	// Add some unmatched noise
	for i := 0; i < 100; i++ {
		base := fmt.Sprintf("NOISE_%04d.jpg", i)
		unMatchedFiles[base] = &googleAssetFile{
			base:   base,
			length: 1000,
			date:   time.Now(),
		}
		jsonName := fmt.Sprintf("NOISE_JSON_%04d.json", i)
		jsons[jsonName] = &assets.Metadata{FileName: base}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recorder := fileevent.NewRecorder(logger)
	tracker := assettracker.New()
	processor := fileprocessor.New(tracker, recorder)
	supportedMedia := filetypes.DefaultSupportedMedia

	// Initialize Source
	s := &GoogleSource{
		deps: adapters.SourceDependencies{
			SupportedMedia: supportedMedia,
			Logger:         logger,
			Processor:      processor,
			TimeZone:       time.UTC,
		},
		config: &adapters.GoogleConfig{},
		catalogs: map[string]googleDirCatalog{
			"testdir": {
				jsons:          make(map[string]*assets.Metadata),
				unMatchedFiles: make(map[string]*googleAssetFile),
				matchedFiles:   make(map[string]*assets.Asset),
			},
		},
		fileTracker: gen.NewSyncMap[googleFileKey, googleTrackingInfo](),
		infoCollector: filenames.NewInfoCollector(time.UTC, supportedMedia),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Reset state
		cat := s.catalogs["testdir"]
		// Deep copy maps to reset
		cat.jsons = make(map[string]*assets.Metadata)
		for k, v := range jsons {
			cat.jsons[k] = v
		}
		cat.unMatchedFiles = make(map[string]*googleAssetFile)
		for k, v := range unMatchedFiles {
			// Copy struct
			copyF := *v
			cat.unMatchedFiles[k] = &copyF
		}
		cat.matchedFiles = make(map[string]*assets.Asset)
		s.catalogs["testdir"] = cat
		b.StartTimer()

		if err := s.solvePuzzle(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
