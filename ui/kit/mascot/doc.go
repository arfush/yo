// Package mascot draws the animated application mascot with Gio.
//
// Imports: std, gioui.org/f32, gioui.org/layout, gioui.org/op,
// gioui.org/op/clip, gioui.org/op/paint, ui/appearance.
// Must not import: core/*, root, ui/* except ui/appearance.
//
// Invariants: Mascot is a value. Draw only paints; it never changes screen or
// domain state. The palette is required and comes from appearance. Animation
// is derived from gtx.Now.
package mascot
