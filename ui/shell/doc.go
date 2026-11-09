// Package shell draws the undecorated application window: the border, header,
// footer band and window controls. It owns where the active screen and dialog
// are drawn.
//
// Imports: std, gioui.org/app, gioui.org/gpu/headless, gioui.org/io/semantic,
// gioui.org/layout, gioui.org/op, gioui.org/op/clip, gioui.org/op/paint,
// gioui.org/widget, ui/appearance, ui/kit, ui/resource.
// Must not import: core/*, root, ui/navigator, ui/screen, ui/dialog.
//
// Invariants: a screen is never nil; New and SetScreen panic on nil. Prepare
// runs once per frame before Layout with the same context and records the
// screen; the offscreen backdrop render happens there, never in Layout. Layout
// draws the screen only inside the content rectangle and its footer only inside
// the footer band. A dialog, when set, replaces the body with a blurred copy of
// the screen, may replace the footer, and is drawn over the body; nil clears it.
// Only the UI goroutine calls the shell.
package shell
