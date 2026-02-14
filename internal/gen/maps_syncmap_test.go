package gen

import (
	"sync"
	"testing"
)

func TestSyncMapUpdateStoresValueForMissingKey(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Update("counter", func(v int) int {
		return v + 1
	})

	got, ok := m.Load("counter")
	if !ok {
		t.Fatal("expected key counter to exist")
	}
	if got != 1 {
		t.Fatalf("expected counter=1, got %d", got)
	}
}

func TestSyncMapUpdateIsAtomicAcrossGoroutines(t *testing.T) {
	const (
		goroutines = 64
		increments = 250
	)

	m := NewSyncMap[string, int]()
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range increments {
				m.Update("counter", func(v int) int {
					return v + 1
				})
			}
		}()
	}
	wg.Wait()

	got, ok := m.Load("counter")
	if !ok {
		t.Fatal("expected key counter to exist")
	}
	want := goroutines * increments
	if got != want {
		t.Fatalf("expected counter=%d, got %d", want, got)
	}
}
