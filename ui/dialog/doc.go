// Package dialog holds every application dialog. Dialogs live directly in this
// package; there are no per-feature subpackages.
//
// Imports: std, gioui.org/*, ui/appearance, ui/kit, ui/navigator.
// Must not import: root, ui/shell.
//
// Invariants: Layout never blocks or starts goroutines. A dialog closes itself
// through the navigator handler; the shell only draws it. Dialogs are part of
// navigation: the navigator owns the dialog stack.
package dialog
