// Package platform owns OS integration that Gio does not expose: concrete
// per-user paths and the native window frame. Windows paints the frame through
// DWM; other platforms keep the default frame.
//
// Must not import: catalog, other core packages, UI, root.
//
// Invariants: exported helpers name concrete resources (journal, window), not
// generic OS locations. Path helpers are safe from any goroutine. A Window is
// driven from the UI goroutine and owns the background repaint goroutine;
// Close stops it, and it also stops when the window is replaced or gone.
package platform
