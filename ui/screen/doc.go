// Package screen holds every application screen. Screens live directly in this
// package; there are no per-feature subpackages.
//
// Must not import: core/*, root, ui/shell.
//
// Invariants: Paint never blocks or starts goroutines. Navigation goes through
// the navigator handler passed at construction.
package screen
