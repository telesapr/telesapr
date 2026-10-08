package core

import (
	"maps"
	"slices"
)

// IDList is a handle-keyed container that preserves insertion order and
// supports efficient lookup by handle. It is implemented as a slice
// paired with a handle->index map so it can be extended during
// iteration (the canonical SolveSpace pattern of "the slice may
// realloc, but the handle stays stable").
type IDList[H comparable, T any] struct {
	Items []T
	Index map[H]int
}

// NewIDList returns an empty IDList with optional capacity hints.
func NewIDList[H comparable, T any](n int) IDList[H, T] {
	return IDList[H, T]{
		Items: make([]T, 0, n),
		Index: make(map[H]int, n),
	}
}

// Len reports the number of items in the list.
func (l *IDList[H, T]) Len() int { return len(l.Items) }

// At returns the item at index i without bounds checking.
func (l *IDList[H, T]) At(i int) T { return l.Items[i] }

// Set overwrites the item at index i.
func (l *IDList[H, T]) Set(i int, v T) { l.Items[i] = v }

// Add appends an item to the list, with the caller-supplied handle.
func (l *IDList[H, T]) Add(h H, v T) int {
	idx := len(l.Items)
	l.Items = append(l.Items, v)
	if l.Index == nil {
		l.Index = make(map[H]int)
	}
	l.Index[h] = idx
	return idx
}

// Clear removes all items.
func (l *IDList[H, T]) Clear() {
	l.Items = l.Items[:0]
	if l.Index != nil {
		for k := range l.Index {
			delete(l.Index, k)
		}
	}
}

// Reserve ensures the underlying slice has capacity for at least n more items.
func (l *IDList[H, T]) Reserve(n int) {
	if cap(l.Items)-len(l.Items) < n {
		grown := make([]T, len(l.Items), len(l.Items)+n)
		copy(grown, l.Items)
		l.Items = grown
	}
}

// ReserveMore ensures capacity for at least n total items.
func (l *IDList[H, T]) ReserveMore(n int) {
	if cap(l.Items) < n {
		grown := make([]T, len(l.Items), n)
		copy(grown, l.Items)
		l.Items = grown
	}
}

// MoveSelfInto transfers ownership of all items from src into dst,
// clearing src. Used to copy param/entity lists during regeneration.
func (l *IDList[H, T]) MoveSelfInto(dst *IDList[H, T]) {
	dst.Items = append(dst.Items, l.Items...)
	if dst.Index == nil {
		dst.Index = make(map[H]int, len(l.Index))
	}
	maps.Copy(dst.Index, l.Index)
	l.Clear()
}

// FindByID returns the item for handle h and true, or zero/false if absent.
func (l *IDList[H, T]) FindByID(h H) (T, bool) {
	if l.Index == nil {
		var zero T
		return zero, false
	}
	i, ok := l.Index[h]
	if !ok {
		var zero T
		return zero, false
	}
	return l.Items[i], true
}

// FindByIDPtr returns a pointer to the item for handle h, or nil if absent.
func (l *IDList[H, T]) FindByIDPtr(h H) *T {
	if l.Index == nil {
		return nil
	}
	i, ok := l.Index[h]
	if !ok {
		return nil
	}
	return &l.Items[i]
}

// SortedHandles returns all handles in insertion order.
func (l *IDList[H, T]) SortedHandles(less func(a, b H) bool) []H {
	out := make([]H, 0, len(l.Index))
	for h := range l.Index {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b H) int {
		less := func(a, b H) bool { return less(a, b) }
		switch {
		case less(a, b):
			return -1
		case less(b, a):
			return 1
		}
		return 0
	})
	return out
}

// IDs returns all handles that have been added. Iteration order is
// map order and therefore not deterministic; callers that need a
// stable order should sort or use SortedHandles. If callers want
// insertion order they should walk Items directly and key their own
// reverse map.
func (l *IDList[H, T]) IDs() []H {
	out := make([]H, 0, len(l.Index))
	for h := range l.Index {
		out = append(out, h)
	}
	return out
}
