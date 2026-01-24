package upload

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/jsonoutput"
	"golang.org/x/sync/errgroup"
)

func (uc *UpCmd) runNoUI(ctx context.Context, app *app.Application) error {
	ctx, cancel := context.WithCancelCause(ctx)
	lock := sync.RWMutex{}
	defer cancel(nil)

	stopProgress := make(chan any)
	var maxImmich, currImmich int

	immichUpdate := func(value, total int) {
		lock.Lock()
		currImmich, maxImmich = value, total
		lock.Unlock()
	}

	// Helper to calculate Immich read percentage
	getImmichPct := func() int {
		lock.Lock()
		defer lock.Unlock()
		if maxImmich > 0 {
			return 100 * currImmich / maxImmich
		}
		return 100
	}

	// Check if JSON output mode is enabled
	isJSONMode := app.Output == "json"
	isNonInteractive := app.NonInteractive

	logProgress := func() {
		counts := app.FileProcessor().Logger().GetCounts()
		immichPct := getImmichPct()
		app.Log().Info("progress",
			"immich_read_pct", immichPct,
			"assets_found", app.FileProcessor().Logger().TotalAssets(),
			"upload_errors", counts[fileevent.ErrorServerError],
			"uploaded", counts[fileevent.ProcessedUploadSuccess],
		)
	}

	// Function to output progress in JSON mode
	outputJSONProgress := func() {
		counts := app.FileProcessor().Logger().GetCounts()
		immichPct := getImmichPct()

		if err := jsonoutput.WriteProgress(
			immichPct,
			app.FileProcessor().Logger().TotalAssets(),
			counts[fileevent.ErrorServerError],
			counts[fileevent.ProcessedUploadSuccess],
		); err != nil {
			// Log error to stderr - if stdout is broken, at least notify via stderr
			app.Log().Error("failed to write JSON progress", "err", err)
		}
	}
	uiGrp := errgroup.Group{}

	uiGrp.Go(func() error {
		// Use different tick rates for different modes
		tickInterval := 500 * time.Millisecond
		if isNonInteractive {
			// In non-interactive mode, output less frequently (every 5 seconds)
			tickInterval = 5 * time.Second
		}
		ticker := time.NewTicker(tickInterval)
		defer func() {
			ticker.Stop()
			// Output final status
			if isJSONMode {
				outputJSONProgress()
			} else if isNonInteractive {
				logProgress()
			}
		}()
		for {
			select {
			case <-stopProgress:
				// Defer block will output final status
				return nil
			case <-ctx.Done():
				// Defer block will output final status
				return ctx.Err()
			case <-ticker.C:
				// Periodic progress updates
				if isJSONMode {
					outputJSONProgress()
				} else if isNonInteractive {
					logProgress()
				}
			}
		}
	})

	uiGrp.Go(func() error {
		processGrp := errgroup.Group{}
		var groupChan chan *assets.Group
		var err error

		processGrp.Go(func() error {
			// Get immich asset
			err := uc.getImmichAssets(ctx, immichUpdate)
			if err != nil {
				cancel(err)
			}
			return err
		})
		processGrp.Go(func() error {
			return uc.getImmichAlbums(ctx)
		})
		processGrp.Go(func() error {
			// Run Prepare
			groupChan = uc.adapter.Browse(ctx)
			return err
		})
		err = processGrp.Wait()
		if err != nil {
			err := context.Cause(ctx)
			if err != nil {
				cancel(err)
				return err
			}
		}
		err = uc.uploadLoop(ctx, groupChan)
		if err != nil {
			cancel(err)
		}

		counts := app.FileProcessor().Logger().GetCounts()
		messages := strings.Builder{}
		if counts[fileevent.ErrorUploadFailed]+counts[fileevent.ErrorServerError]+counts[fileevent.ErrorFileAccess]+counts[fileevent.ErrorIncomplete] > 0 {
			messages.WriteString("Some errors have occurred. Check stderr for details\n")
		}

		if messages.Len() > 0 {
			cancel(errors.New(messages.String()))
		}
		err = errors.Join(err, uc.finishing(ctx))
		close(stopProgress)
		return err
	})

	err := uiGrp.Wait()
	if err != nil {
		err = context.Cause(ctx)
	}
	return err
}
