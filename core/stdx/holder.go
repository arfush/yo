package stdx

import "sync"

// Holder is the single writable owner of a value. It is safe for concurrent
// use; mu guards value.
type Holder[T any] struct {
	mu    sync.Mutex
	value T
}

// NewHolder returns a holder of v.
func NewHolder[T any](v T) *Holder[T] {
	return &Holder[T]{value: v}
}

// Value returns the current value. A nil holder is a programmer error and
// panics.
func (h *Holder[T]) Value() T {
	if h == nil {
		panic("stdx: nil holder")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.value
}

// Set replaces the current value. A nil holder is a programmer error and
// panics.
func (h *Holder[T]) Set(v T) {
	if h == nil {
		panic("stdx: nil holder")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.value = v
}

// Readonly returns a live read-only view. The view always reports the current
// value and has no setter. A nil holder is a programmer error and panics.
func (h *Holder[T]) Readonly() ReadonlyHolder[T] {
	if h == nil {
		panic("stdx: nil holder")
	}
	return ReadonlyHolder[T]{holder: h}
}

// ReadonlyHolder is a live read-only view of a Holder.
type ReadonlyHolder[T any] struct {
	holder *Holder[T]
}

// Value returns the current value of the underlying holder. A nil view is a
// programmer error and panics.
func (r ReadonlyHolder[T]) Value() T {
	if r.holder == nil {
		panic("stdx: nil readonly holder")
	}
	return r.holder.Value()
}
