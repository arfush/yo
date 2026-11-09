// Package kit provides reusable widgets and drawing helpers shared by screens
// and the shell.
//
// Imports: std, gioui.org/layout, gioui.org/op, gioui.org/op/paint,
// gioui.org/widget, ui/appearance.
// Must not import: core/*, root, ui/navigator, ui/screen, ui/shell.
//
// Invariants: helpers draw into the caller's layout.Context and return
// dimensions; they never mutate application state.
package kit
