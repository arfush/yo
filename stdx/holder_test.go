package stdx_test

import (
	"sync"
	"testing"

	"github.com/arfush/yo/stdx"
)

func TestHolderValueAndSet(t *testing.T) {
	t.Parallel()

	h := stdx.NewHolder(42)
	if got := h.Value(); got != 42 {
		t.Fatalf("Value = %d; want 42", got)
	}

	h.Set(7)
	if got := h.Value(); got != 7 {
		t.Fatalf("Value after Set = %d; want 7", got)
	}
}

func TestHolderUpdate(t *testing.T) {
	t.Parallel()

	h := stdx.NewHolder(40)
	h.Update(func(v *int) { *v += 2 })
	if got := h.Value(); got != 42 {
		t.Fatalf("Value after Update = %d; want 42", got)
	}
}

func TestReadonlyIsLive(t *testing.T) {
	t.Parallel()

	h := stdx.NewHolder("first")
	view := h.Readonly()
	if got := view.Value(); got != "first" {
		t.Fatalf("view = %q; want first", got)
	}

	h.Set("second")
	if got := h.Value(); got != "second" {
		t.Fatalf("holder = %q; want second", got)
	}
	if got := view.Value(); got != "second" {
		t.Fatalf("view = %q; want current second", got)
	}
}

func TestNilHolderPanics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func()
	}{
		{"holder value", func() {
			var h *stdx.Holder[int]
			h.Value()
		}},
		{"holder set", func() {
			var h *stdx.Holder[int]
			h.Set(1)
		}},
		{"holder update", func() {
			var h *stdx.Holder[int]
			h.Update(func(*int) {})
		}},
		{"nil update", func() {
			h := stdx.NewHolder(1)
			h.Update(nil)
		}},
		{"holder readonly", func() {
			var h *stdx.Holder[int]
			h.Readonly()
		}},
		{"readonly value", func() {
			var view stdx.ReadonlyHolder[int]
			view.Value()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertPanics(t, tt.call)
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	const writers = 8
	const writes = 100

	h := stdx.NewHolder(0)
	view := h.Readonly()

	var wg sync.WaitGroup
	for n := 1; n <= writers; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				h.Set(n)
				h.Update(func(v *int) { *v = n })
				_ = h.Value()
				_ = view.Value()
			}
		}(n)
	}
	wg.Wait()

	got := h.Value()
	if got < 1 || got > writers {
		t.Fatalf("Value = %d; want a written value in 1..%d", got, writers)
	}
	if view.Value() != got {
		t.Fatalf("view = %d; want current %d", view.Value(), got)
	}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("call did not panic")
		}
	}()
	fn()
}
