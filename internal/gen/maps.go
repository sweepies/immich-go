package gen

import (
	"cmp"
	"maps"
	"slices"
	"sync"
)

func MapKeys[K comparable, T any](m map[K]T) []K {
	return slices.AppendSeq(make([]K, 0, len(m)), maps.Keys(m))
}

func MapKeysSorted[K cmp.Ordered, T any](m map[K]T) []K {
	r := slices.AppendSeq(make([]K, 0, len(m)), maps.Keys(m))
	slices.Sort(r)
	return r
}

func MapFilterKeys[K comparable, T any](m map[K]T, f func(i T) bool) []K {
	r := make([]K, 0, len(m))
	for k, v := range maps.All(m) {
		if f(v) {
			r = append(r, k)
		}
	}
	return r
}

type SyncMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{m: make(map[K]V)}
}

func (m *SyncMap[K, V]) Load(k K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[k]
	return v, ok
}

func (m *SyncMap[K, V]) Store(k K, v V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[k] = v
}

func (m *SyncMap[K, V]) Delete(k K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, k)
}

func (m *SyncMap[K, V]) Update(k K, fn func(V) V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[k] = fn(m.m[k])
}

func (m *SyncMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.AppendSeq(make([]K, 0, len(m.m)), maps.Keys(m.m))
}
