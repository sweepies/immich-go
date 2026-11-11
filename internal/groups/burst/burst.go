package burst

import (
	"context"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/filetypes"
)

const frameInterval = 500 * time.Millisecond

// Group groups photos taken within a period of less than 1 second with a digital camera.
// This addresses photos taken with a digital camera when there isn't any burst indication in the file namee
//
// Ex: IMG_0001.JPG, IMG_0002.JPG, etc. and the date taken is different by a fraction of second
// Ex: IMG_0001.JPG, IMG_0001.RAW, IMG_0002.JPG, IMG_0002.RAW, etc.
//
// Edited images, images identified as as burst already are not considered.
// The in channel receives assets sorted by date taken.
func Group(ctx context.Context, in <-chan *assets.Asset, out chan<- *assets.Asset, gOut chan<- *assets.Group) {
	var currentGroup []*assets.Asset
	var lastTaken time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case a, ok := <-in:
			if !ok {
				if len(currentGroup) > 0 {
					sendBurstGroup(ctx, out, gOut, currentGroup)
				}
				return
			}

			// exclude movies, edited or burst images
			// exclude images without a date taken
			ni := a.NameInfo
			eligible := ni.Type == filetypes.TypeImage &&
				!a.CaptureDate.IsZero() &&
				ni.Kind != assets.KindBurst &&
				ni.Kind != assets.KindEdited

			if !eligible {
				// An ineligible asset breaks the burst: flush the current
				// group, forward the asset as is and start over.
				if len(currentGroup) > 0 {
					sendBurstGroup(ctx, out, gOut, currentGroup)
					currentGroup = nil
				}
				select {
				case out <- a:
				case <-ctx.Done():
				}
				continue
			}

			// An eligible asset too far in time from the previous one starts
			// a new group.
			if len(currentGroup) > 0 && a.CaptureDate.Sub(lastTaken).Abs() > frameInterval {
				sendBurstGroup(ctx, out, gOut, currentGroup)
				currentGroup = nil
			}
			currentGroup = append(currentGroup, a)
			lastTaken = a.CaptureDate
		}
	}
}

func sendBurstGroup(ctx context.Context, out chan<- *assets.Asset, outg chan<- *assets.Group, as []*assets.Asset) {
	if len(as) == 0 {
		return
	}
	if len(as) < 2 {
		select {
		case out <- as[0]:
		case <-ctx.Done():
		}
		return
	}

	g := assets.NewGroup(assets.GroupByBurst, as...)
	g.CoverIndex = 0 // Assuming the first asset is the cover

	select {
	case <-ctx.Done():
		return
	case outg <- g:
	}
}
