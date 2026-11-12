// Package mascot draws the animated application mascot with Qt.
//
// Must not import: core/*, root, ui/* except ui/appearance and ui/kit.
//
// Invariants: Mascot is a value. Draw only paints; it never changes screen or
// domain state. The palette is required and comes from appearance. Animation
// is derived from the frame time the caller passes in.
package mascot
