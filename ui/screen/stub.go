// Package screen holds every application screen. Screens live directly in this
// package; there are no per-feature subpackages.
//
// Imports: std, gioui.org/*, ui/appearance, ui/kit, ui/navigator.
// Must not import: root, ui/shell.
//
// Invariants: Layout never blocks or starts goroutines. Navigation goes through
// the navigator handler passed at construction.
package screen

import "gioui.org/layout"

// Stub is the empty fallback screen installed before the navigator has a route.
type Stub struct{}

// NewStub returns the fallback screen.
func NewStub() Stub { return Stub{} }

// Layout draws nothing; the shell owns the window background.
func (Stub) Layout(layout.Context) {}

// LayoutFooter draws nothing; the shell owns the footer band.
func (Stub) LayoutFooter(layout.Context) {}
