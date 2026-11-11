package cache

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

func mockSaveFn(ctx context.Context, coll string, ids []string) (string, error) {
	if len(ids) == 0 {
		return coll, errors.New("no new items")
	}
	return coll + "|" + strings.Join(ids, ","), nil
}

func TestNewCollection(t *testing.T) {
	ctx := context.Background()
	cc := NewCollectionCache[string](5, mockSaveFn)
	cc.NewCollection(ctx, "key1", "coll1", []string{"id1", "id2"})

	if c, ok := cc.collections.Load("key1"); !ok || c.collection != "coll1" {
		t.Error("expected collection coll1 for key1")
	} else {
		if c.items.Len() != 2 {
			t.Errorf("expected 2 items in collection, got %d", c.items.Len())
		}
		cc.Close(ctx)
	}
}

func TestAddItemsByRepeatedNewCollectionCalls(t *testing.T) {
	ctx := context.Background()
	wasCalled := false
	wrappedMockSaveFn := func(ctx context.Context, coll string, ids []string) (string, error) {
		wasCalled = true
		return mockSaveFn(ctx, coll, ids)
	}

	cc := NewCollectionCache[string](2, wrappedMockSaveFn)
	cc.NewCollection(ctx, "key1", "coll1", nil)

	// Adding items by calling NewCollection multiple times
	cc.AddIDToCollection(ctx, "key1", "coll1", "id1")
	cc.AddIDToCollection(ctx, "key1", "coll1", "id2")
	cc.AddIDToCollection(ctx, "key1", "coll1", "id3")

	cc.Close(ctx)
	if !wasCalled {
		t.Error("expected wrappedMockSaveFn to be called at least once")
	}
}

func TestMultipleCollectionsExceedingCacheSize(t *testing.T) {
	ctx := context.Background()
	wasCalledCount := make(map[string]int)

	wrappedMockSaveFn := func(ctx context.Context, coll string, ids []string) (string, error) {
		wasCalledCount[coll]++
		return coll, nil
	}

	cc := NewCollectionCache[string](2, wrappedMockSaveFn)

	// Initial collections
	cc.NewCollection(ctx, "key1", "coll1", []string{"id1", "id2"})
	cc.NewCollection(ctx, "key2", "coll2", []string{"id3", "id4"})

	// Exceed cache size by adding more IDs
	cc.AddIDToCollection(ctx, "key1", "coll1", "id5")
	if wasCalledCount["coll1"] != 0 {
		t.Errorf("expected saveFn called %d times for coll1, got %d", 0, wasCalledCount["coll1"])
	}
	cc.AddIDToCollection(ctx, "key1", "coll1", "id6")
	if wasCalledCount["coll1"] != 1 {
		t.Errorf("expected saveFn called %d times for coll1, got %d", 1, wasCalledCount["coll1"])
	}

	cc.AddIDToCollection(ctx, "key1", "coll1", "id7")
	if wasCalledCount["coll1"] != 1 {
		t.Errorf("expected saveFn called %d times for coll1, got %d", 1, wasCalledCount["coll1"])
	}

	cc.AddIDToCollection(ctx, "key2", "coll2", "id8")
	if wasCalledCount["coll2"] != 0 {
		t.Errorf("expected saveFn called %d times for coll2, got %d", 0, wasCalledCount["coll2"])
	}
	cc.Close(ctx)

	// Checking collections
	c1, ok := cc.collections.Load("key1")
	if !ok {
		t.Error("expected collection key1 to be present")
		return
	}
	if c1.collection != "coll1" {
		t.Errorf("expected collection coll1, got %s", c1.collection)
	}
	if c1.items.Len() != 5 {
		t.Errorf("expected 5 IDs in collection coll1, got %d", c1.items.Len())
	}

	c2, ok := cc.collections.Load("key2")
	if !ok {
		t.Error("expected collection key2 to be present")
		return
	}
	if c2.collection != "coll2" {
		t.Errorf("expected collection coll2, got %s", c2.collection)
	}
	if c2.items.Len() != 3 {
		t.Errorf("expected 3 IDs in collection coll2, got %d", c2.items.Len())
	}

	// Checking calls
	if wasCalledCount["coll1"] < 2 {
		t.Errorf("expected multiple saveFn calls for coll1, got %d", wasCalledCount["coll1"])
	}
	if wasCalledCount["coll2"] < 1 {
		t.Errorf("expected saveFn call for coll2, got %d", wasCalledCount["coll2"])
	}
}

func TestCollectionCacheConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	cc := NewCollectionCache[string](10, func(ctx context.Context, coll string, ids []string) (string, error) {
		return coll, nil
	})

	cc.NewCollection(ctx, "testKey", "testColl", nil)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("asset%d", n)
			cc.AddIDToCollection(ctx, "testKey", "testColl", id)
		}(i)
	}
	wg.Wait()

	c, ok := cc.collections.Load("testKey")
	if !ok {
		t.Fatal("expected testKey to exist")
	}
	if c.items.Len() != 50 {
		t.Errorf("expected 50 items, got %d", c.items.Len())
	}
	cc.Close(ctx)
}

// TestFailedSaveKeepsPendingIDs checks that a saveFn failure does not drop the
// pending ids: they are retried once another maxCacheSize ids accumulate, not
// on every add, and delivered by a later flush.
func TestFailedSaveKeepsPendingIDs(t *testing.T) {
	ctx := context.Background()
	fail := true
	var calls [][]string
	saveFn := func(ctx context.Context, coll string, ids []string) (string, error) {
		calls = append(calls, slices.Clone(ids))
		if fail {
			return coll, errors.New("save failed")
		}
		return coll, nil
	}

	cc := NewCollectionCache[string](3, saveFn)

	cc.AddIDToCollection(ctx, "key1", "coll1", "id1")
	cc.AddIDToCollection(ctx, "key1", "coll1", "id2")
	cc.AddIDToCollection(ctx, "key1", "coll1", "id3")
	if len(calls) != 1 {
		t.Fatalf("expected 1 saveFn call after the first batch, got %d", len(calls))
	}

	// The failed ids must not be retried on every subsequent add.
	cc.AddIDToCollection(ctx, "key1", "coll1", "id4")
	cc.AddIDToCollection(ctx, "key1", "coll1", "id5")
	if len(calls) != 1 {
		t.Fatalf("expected no retry before maxCacheSize new ids accumulate, got %d calls", len(calls))
	}

	fail = false
	cc.AddIDToCollection(ctx, "key1", "coll1", "id6")
	if len(calls) != 2 {
		t.Fatalf("expected a retry once maxCacheSize new ids accumulated, got %d calls", len(calls))
	}

	// The retry must deliver every id the failed call dropped.
	want := []string{"id1", "id2", "id3", "id4", "id5", "id6"}
	got := slices.Clone(calls[1])
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("expected the retry to deliver %v, got %v", want, got)
	}

	cc.Close(ctx)
	if len(calls) != 3 {
		t.Errorf("expected a final flush on close, got %d calls", len(calls))
	}
}

// TestFailedSaveDeliveredAtClose checks that pending ids surviving failed
// saves are flushed one last time when the cache closes.
func TestFailedSaveDeliveredAtClose(t *testing.T) {
	ctx := context.Background()
	fail := true
	var calls [][]string
	saveFn := func(ctx context.Context, coll string, ids []string) (string, error) {
		calls = append(calls, slices.Clone(ids))
		if fail {
			return coll, errors.New("save failed")
		}
		return coll, nil
	}

	cc := NewCollectionCache[string](2, saveFn)
	cc.AddIDToCollection(ctx, "key1", "coll1", "id1")
	cc.AddIDToCollection(ctx, "key1", "coll1", "id2") // triggers a failing flush

	fail = false
	cc.Close(ctx)

	if len(calls) != 2 {
		t.Fatalf("expected the close flush to retry, got %d calls", len(calls))
	}
	got := slices.Clone(calls[1])
	slices.Sort(got)
	if !slices.Equal(got, []string{"id1", "id2"}) {
		t.Errorf("expected the close flush to deliver the failed ids, got %v", got)
	}
}

// TestCloseIsIdempotent checks a double Close neither panics nor flushes twice.
func TestCloseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	cc := NewCollectionCache[string](10, func(ctx context.Context, coll string, ids []string) (string, error) {
		callCount++
		return coll, nil
	})
	cc.AddIDToCollection(ctx, "key1", "coll1", "id1")

	cc.Close(ctx)
	cc.Close(ctx)

	if callCount != 1 {
		t.Errorf("expected a single flush across two Close calls, got %d", callCount)
	}
}

// TestAddAfterCloseReturnsFalse checks that adding to a closed cache reports
// false instead of panicking on the closed channel.
func TestAddAfterCloseReturnsFalse(t *testing.T) {
	ctx := context.Background()
	cc := NewCollectionCache[string](10, func(ctx context.Context, coll string, ids []string) (string, error) {
		return coll, nil
	})
	cc.Close(ctx)

	if cc.AddIDToCollection(ctx, "key1", "coll1", "id1") {
		t.Error("expected AddIDToCollection after Close to return false")
	}
	cc.NewCollection(ctx, "key2", "coll2", []string{"id1"})
}

// TestEmptyIDIsRefused checks that an empty asset id never enters a
// collection: AddIDToCollection refuses it and NewCollection drops it (F1).
func TestEmptyIDIsRefused(t *testing.T) {
	ctx := context.Background()
	var saved [][]string
	cc := NewCollectionCache[string](2, func(ctx context.Context, coll string, ids []string) (string, error) {
		saved = append(saved, slices.Clone(ids))
		return coll, nil
	})

	if cc.AddIDToCollection(ctx, "key1", "coll1", "") {
		t.Error("expected AddIDToCollection to refuse an empty id")
	}

	cc.NewCollection(ctx, "key2", "coll2", []string{"id1", ""})
	c, ok := cc.collections.Load("key2")
	if !ok {
		t.Fatal("expected key2 to exist")
	}
	if c.items.Len() != 1 {
		t.Errorf("expected the empty id to be dropped from the new collection, got %d items", c.items.Len())
	}

	cc.Close(ctx)
	for _, ids := range saved {
		if slices.Contains(ids, "") {
			t.Errorf("expected no empty id to reach saveFn, got %v", ids)
		}
	}
}

// TestCloseUsesGivenContext checks that the final flush runs on the context
// passed to Close, so the caller can supply a context that survives the
// cancelled run context.
func TestCloseUsesGivenContext(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())

	var flushCtx context.Context
	cc := NewCollectionCache[string](10, func(ctx context.Context, coll string, ids []string) (string, error) {
		flushCtx = ctx
		return coll, nil
	})
	cc.AddIDToCollection(runCtx, "key1", "coll1", "id1")
	cancel()

	flushGiven, stop := context.WithCancel(context.Background())
	defer stop()
	cc.Close(flushGiven)

	if flushCtx == nil {
		t.Fatal("expected the final flush to call saveFn")
	}
	if flushCtx != flushGiven {
		t.Error("expected the final flush to run on the context passed to Close")
	}
}
