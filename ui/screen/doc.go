// Package screen holds every application screen. Screens live directly in this
// package; there are no per-feature subpackages.
//
// Must not import: core/*, root, ui/shell.
//
// Invariants: Paint never blocks or starts goroutines. Screens place ui/kit
// widgets and never draw elements themselves. Navigation goes through the
// navigator handler passed at construction.
package screen
