package cache

import (
	"context"
	"sync"

	"github.com/sweepies/immich-go/internal/gen/syncmap"
	"github.com/sweepies/immich-go/internal/gen/syncset"
)

type saveFn[T any] func(ctx context.Context, coll T, ids []string) (T, error)

type CollectionCache[T comparable] struct {
	maxCacheSize      int
	collections       *syncmap.SyncMap[string, *Collection[T]] // collection of Collections
	chanNewCollection chan func()
	workerDone        chan struct{}
	saveFn            saveFn[T]
	mu                sync.Mutex
	closed            bool
}

// NewCollectionCache creates a new collection cache with the given max cache size.
func NewCollectionCache[T comparable](maxCacheSize int, saveFn saveFn[T]) *CollectionCache[T] {
	cc := &CollectionCache[T]{
		maxCacheSize:      maxCacheSize,
		collections:       syncmap.New[string, *Collection[T]](),
		chanNewCollection: make(chan func(), 100),
		workerDone:        make(chan struct{}),
		saveFn:            saveFn,
	}
	go func() {
		defer close(cc.workerDone)
		for f := range cc.chanNewCollection {
			f()
		}
	}()

	return cc
}

// NewCollection manage the cache for the given collection and the given list of initial ids.
// If the collection already exists, the ids are added to the existing collection.
// It does nothing once the cache is closed.
func (cc *CollectionCache[T]) NewCollection(ctx context.Context, key string, coll T, ids []string) {
	cc.mu.Lock()
	if cc.closed {
		cc.mu.Unlock()
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	cc.chanNewCollection <- func() {
		defer wg.Done()
		c, ok := cc.collections.Load(key)
		if !ok {
			c := newCollection(coll, cc.maxCacheSize, cc.saveFn, ids)
			cc.collections.Store(key, c)
		} else {
			for _, id := range ids {
				c.addID(ctx, id)
			}
		}
	}
	cc.mu.Unlock()
	wg.Wait()
}

// AddIDToCollection adds id to the collection stored under key, creating it
// from collObj when needed. It returns true when the id was newly added, and
// false when the id was already present or the cache is closed.
func (cc *CollectionCache[T]) AddIDToCollection(ctx context.Context, key string, collObj T, id string) bool {
	cc.mu.Lock()
	if cc.closed {
		cc.mu.Unlock()
		return false
	}
	added := false
	wg := sync.WaitGroup{}
	wg.Add(1)
	cc.chanNewCollection <- func() {
		defer wg.Done()
		c, ok := cc.collections.Load(key)
		if !ok {
			c = newCollection(collObj, cc.maxCacheSize, cc.saveFn, nil)
			cc.collections.Store(key, c)
		}
		added = c.addID(ctx, id)
	}
	cc.mu.Unlock()
	wg.Wait()
	return added
}

// Close flushes every collection's pending ids through saveFn and shuts the
// cache down. The flush runs inside the cache's worker goroutine so it is
// serialised with queued adds; it uses ctx, which the caller should leave
// uncancelled. Adds after Close are dropped. Close is idempotent: a second
// call waits for the first flush to finish and returns.
func (cc *CollectionCache[T]) Close(ctx context.Context) {
	cc.mu.Lock()
	if !cc.closed {
		cc.closed = true
		cc.chanNewCollection <- func() {
			cc.collections.Range(func(key string, c *Collection[T]) bool {
				c.flush(ctx)
				return true
			})
		}
		close(cc.chanNewCollection)
	}
	cc.mu.Unlock()
	<-cc.workerDone
}

// Collection accumulates asset ids for one collection (album or tag). It is
// only ever touched on the cache's worker goroutine.
type Collection[T comparable] struct {
	collection    T
	items         *syncset.Set[string]
	newItems      *syncset.Set[string]
	sinceLastSave int
	saveFn        saveFn[T]
	maxCacheSize  int
}

func newCollection[T comparable](collection T, maxCacheSize int, saveFn saveFn[T], initialIDs []string) *Collection[T] {
	ids := make([]string, 0, len(initialIDs))
	for _, id := range initialIDs {
		if id != "" {
			ids = append(ids, id)
		}
	}
	c := &Collection[T]{
		collection:   collection,
		items:        syncset.New(ids...),
		newItems:     syncset.New[string](),
		saveFn:       saveFn,
		maxCacheSize: maxCacheSize,
	}

	return c
}

func (c *Collection[T]) addID(ctx context.Context, id string) bool {
	if id == "" {
		return false
	}
	if !c.items.Add(id) {
		return false
	}
	c.newItems.Add(id)
	c.sinceLastSave++
	if c.sinceLastSave >= c.maxCacheSize {
		c.flush(ctx)
	}
	return true
}

// flush attempts to save the pending ids. On success the collection is updated
// and the pending set is cleared. On failure the pending ids are kept: the next
// attempt happens once another maxCacheSize ids have accumulated, and once more
// when the cache is closed.
func (c *Collection[T]) flush(ctx context.Context) {
	c.sinceLastSave = 0
	collection, err := c.saveFn(ctx, c.collection, c.newItems.Items())
	if err == nil {
		c.collection = collection
		c.newItems = syncset.New[string]()
	}
}
