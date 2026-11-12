// Package kit provides reusable widgets and drawing helpers shared by screens
// and the shell. Screens draw their elements only through these widgets.
//
// Must not import: core/*, root, ui/navigator, ui/screen, ui/shell.
//
// Invariants: helpers draw into the caller's QPainter and never mutate
// application state. Widgets draw at the painter origin and expose their size,
// so callers own placement and hit-testing.
package kit
