// Package screen holds every application screen. Screens live directly in this
// package; there are no per-feature subpackages.
//
// Imports: std, gioui.org/*, core/stdx, ui/appearance, ui/kit, ui/navigator.
// Must not import: root, ui/shell.
//
// Invariants: Layout never blocks or starts goroutines. Navigation goes through
// the navigator handler passed at construction.
package screen
