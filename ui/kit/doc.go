// Package kit provides reusable widgets and drawing helpers shared by screens
// and the shell.
//
// Must not import: core/*, root, ui/navigator, ui/screen, ui/shell.
//
// Invariants: helpers draw into the caller's layout.Context and return
// dimensions; they never mutate application state.
package kit
