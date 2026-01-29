package pipeline

import (
	"fmt"
	"math"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sweepies/immich-go/internal/assets"
	"github.com/sweepies/immich-go/internal/gen/syncmap"
	"github.com/sweepies/immich-go/internal/gen/syncset"
	iimmich "github.com/sweepies/immich-go/internal/immich"
)

// AdviceCode represents the decision about whether to upload an asset.
type AdviceCode int

func (a AdviceCode) String() string {
	switch a {
	case IDontKnow:
		return "IDontKnow"
	case SmallerOnServer:
		return "SmallerOnServer"
	case BetterOnServer:
		return "BetterOnServer"
	case SameOnServer:
		return "SameOnServer"
	case NotOnServer:
		return "NotOnServer"
	case AlreadyProcessed:
		return "AlreadyProcessed"
	case ForceUpload:
		return "ForceUpload"
	}
	return fmt.Sprintf("advice(%d)", a)
}

const (
	IDontKnow AdviceCode = iota
	SmallerOnServer
	BetterOnServer
	SameOnServer
	NotOnServer
	AlreadyProcessed
	ForceUpload
)

// Advice represents the decision about an asset upload.
type Advice struct {
	Advice      AdviceCode
	Message     string
	ServerAsset *assets.Asset
	LocalAsset  *assets.Asset
}

// Index tracks assets both on the server and locally processed.
// It provides de-duplication and upload decision logic.
type Index struct {
	lock sync.Mutex

	// map of assetID to asset, local and server ones
	immichAssets *syncmap.SyncMap[string, *assets.Asset]

	// set of Uploaded Checksums
	uploadsChecksum *syncset.Set[string]

	// map of base name to assetID
	byName *syncmap.SyncMap[string, []string]

	// map of SHA1 to assetID
	byChecksum *syncmap.SyncMap[string, *assets.Asset]

	assetNumber int64
}

// NewIndex creates a new asset index.
func NewIndex() *Index {
	return &Index{
		immichAssets:    syncmap.New[string, *assets.Asset](),
		byChecksum:      syncmap.New[string, *assets.Asset](),
		byName:          syncmap.New[string, []string](),
		uploadsChecksum: syncset.New[string](),
	}
}

// AddImmichAsset adds an asset from the server to the index.
// Returns the asset and true if added, or existing asset and false if already present.
func (idx *Index) AddImmichAsset(ia *iimmich.Asset) (*assets.Asset, bool) {
	idx.lock.Lock()
	defer idx.lock.Unlock()

	if ia.ID == "" {
		panic("asset ID is empty")
	}

	if existing, ok := idx.immichAssets.Load(string(ia.ID)); ok {
		return existing, false
	}
	a := ia.AsAsset()
	return idx.add(a, false), true
}

// AddLocalAsset adds a locally processed asset to the index.
// Returns the asset and true if added, or existing asset and false if already present.
func (idx *Index) AddLocalAsset(ia *assets.Asset) (*assets.Asset, bool) {
	idx.lock.Lock()
	defer idx.lock.Unlock()

	if existing, ok := idx.immichAssets.Load(ia.ID); ok {
		return existing, false
	}
	if existing, ok := idx.byChecksum.Load(ia.Checksum); ok {
		return existing, false
	}
	return idx.add(ia, true), true
}

// GetByID returns an asset by its ID, or nil if not found.
func (idx *Index) GetByID(id string) *assets.Asset {
	a, _ := idx.immichAssets.Load(id)
	return a
}

// Len returns the number of assets in the index.
func (idx *Index) Len() int {
	return int(atomic.LoadInt64(&idx.assetNumber))
}

func (idx *Index) add(a *assets.Asset, local bool) *assets.Asset {
	if a.ID == "" {
		panic("asset ID is empty")
	}
	if a.Checksum == "" {
		panic("asset checksum is empty")
	}

	if _, ok := idx.byChecksum.Load(a.Checksum); ok {
		panic("asset checksum already exists")
	}

	if idx.uploadsChecksum.Contains(a.Checksum) {
		panic("asset checksum already exists in uploads")
	}

	atomic.AddInt64(&idx.assetNumber, 1)
	idx.immichAssets.Store(a.ID, a)
	idx.byChecksum.Store(a.Checksum, a)
	filename := a.OriginalFileName

	if local {
		idx.uploadsChecksum.Add(a.Checksum)
	}

	l, _ := idx.byName.Load(filename)
	l = append(l, a.ID)
	idx.byName.Store(filename, l)
	return a
}

// ReplaceAsset replaces an old asset with a new one in the index.
func (idx *Index) ReplaceAsset(newA *assets.Asset, oldA *assets.Asset) *assets.Asset {
	if newA.ID == "" {
		panic("asset ID is empty")
	}
	if newA.Checksum == "" {
		panic("asset checksum is empty")
	}
	idx.lock.Lock()
	defer idx.lock.Unlock()
	oldA.Trashed = true
	idx.immichAssets.Store(newA.ID, newA)
	idx.byChecksum.Store(newA.Checksum, newA)
	idx.uploadsChecksum.Add(newA.Checksum)

	filename := newA.OriginalFileName
	l, _ := idx.byName.Load(filename)
	l = append(l, newA.ID)
	idx.byName.Store(filename, l)
	return newA
}

// IsAlreadyProcessed checks if an asset with the given checksum was already processed.
func (idx *Index) IsAlreadyProcessed(checksum string) bool {
	return idx.uploadsChecksum.Contains(checksum)
}

// ShouldUpload determines whether an asset should be uploaded.
// It considers checksums, filenames, dates, and sizes to make the decision.
func (idx *Index) ShouldUpload(la *assets.Asset, overwrite bool) (*Advice, error) {
	// Optimization: Check metadata first to avoid expensive checksumming
	filename := path.Base(la.File.Name())

	// check all files with the same name
	ids, ok := idx.byName.Load(filename)

	if ok && len(ids) > 0 {
		dateTaken := la.CaptureDate
		if dateTaken.IsZero() {
			dateTaken = la.FileDate
		}
		size := int64(la.FileSize)

		for _, id := range ids {
			sa, ok := idx.immichAssets.Load(id)
			if !ok {
				continue
			}

			compareDate := compareDate(dateTaken, sa.CaptureDate)
			compareSize := size - int64(sa.FileSize)

			switch {
			case compareDate == 0 && overwrite:
				return idx.adviceForceUpload(sa), nil
			case compareDate == 0 && compareSize == 0:
				return idx.adviceSameOnServer(sa), nil
			case compareDate == 0 && compareSize > 0:
				return idx.adviceSmallerOnServer(sa), nil
			case compareDate == 0 && compareSize < 0:
				return idx.adviceBetterOnServer(sa), nil
			}
		}
	}

	checksum, err := la.GetChecksum()
	if err != nil {
		return nil, err
	}

	if sa, ok := idx.byChecksum.Load(checksum); ok {
		if idx.IsAlreadyProcessed(checksum) {
			return idx.adviceAlreadyProcessed(sa), nil
		}
		return idx.adviceSameOnServer(sa), nil
	}

	return idx.adviceNotOnServer(), nil
}

func (idx *Index) adviceSameOnServer(sa *assets.Asset) *Advice {
	return &Advice{
		Advice:      SameOnServer,
		Message:     fmt.Sprintf("An asset with the same name:%q, date:%q and size:%s exists on the server. No need to upload.", sa.OriginalFileName, sa.CaptureDate.Format(time.DateTime), formatBytes(int64(sa.FileSize))),
		ServerAsset: sa,
	}
}

func (idx *Index) adviceSmallerOnServer(sa *assets.Asset) *Advice {
	return &Advice{
		Advice:      SmallerOnServer,
		Message:     fmt.Sprintf("An asset with the same name:%q and date:%q but with smaller size:%s exists on the server. Replace it.", sa.OriginalFileName, sa.CaptureDate.Format(time.DateTime), formatBytes(int64(sa.FileSize))),
		ServerAsset: sa,
	}
}

func (idx *Index) adviceBetterOnServer(sa *assets.Asset) *Advice {
	return &Advice{
		Advice:      BetterOnServer,
		Message:     fmt.Sprintf("An asset with the same name:%q and date:%q but with bigger size:%s exists on the server. No need to upload.", sa.OriginalFileName, sa.CaptureDate.Format(time.DateTime), formatBytes(int64(sa.FileSize))),
		ServerAsset: sa,
	}
}

func (idx *Index) adviceAlreadyProcessed(sa *assets.Asset) *Advice {
	return &Advice{
		Advice:      AlreadyProcessed,
		Message:     fmt.Sprintf("An asset with the same checksum:%q has been already processed. No need to upload.", sa.Checksum),
		ServerAsset: sa,
	}
}

func (idx *Index) adviceNotOnServer() *Advice {
	return &Advice{
		Advice:  NotOnServer,
		Message: "This a new asset, upload it.",
	}
}

func (idx *Index) adviceForceUpload(sa *assets.Asset) *Advice {
	return &Advice{
		Advice:      ForceUpload,
		Message:     "This asset is marked for force upload.",
		ServerAsset: sa,
	}
}

func compareDate(d1 time.Time, d2 time.Time) int {
	diff := d1.Sub(d2)

	switch {
	case diff < -5*time.Second:
		return -1
	case diff >= 5*time.Second:
		return +1
	}
	return 0
}

func formatBytes(s int64) string {
	suffixes := []string{"B", "KB", "MB", "GB"}
	bytes := float64(s)
	base := 1024.0
	if bytes < base {
		return fmt.Sprintf("%.0f %s", bytes, suffixes[0])
	}
	exp := int64(0)
	for bytes >= base && exp < int64(len(suffixes)-1) {
		bytes /= base
		exp++
	}
	roundedSize := math.Round(bytes*10) / 10
	return fmt.Sprintf("%.1f %s", roundedSize, suffixes[exp])
}
