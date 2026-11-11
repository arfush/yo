// Package shell draws the undecorated application window: the border, header,
// footer band and window controls. It owns where the active screen is drawn.
//
// Must not import: core/*, root, ui/navigator, ui/screen.
//
// Invariants: a screen, site holder and appearance holder are never nil; New
// and SetScreen panic on a nil screen. Prepare runs once per frame before Layout
// with the same context, syncs the palette from the site holder and records the
// screen. Layout draws the screen only inside the content rectangle and its
// footer only inside the footer band. Site changes go through Holder.Update.
// Only the UI goroutine calls the shell.
package shell
