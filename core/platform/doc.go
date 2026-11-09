// Package platform owns OS integration that Gio does not expose: concrete
// per-user paths and the native window frame. Windows paints the frame through
// DWM; other platforms keep the default frame.
//
// Must not import: catalog, other core packages, UI, root.
//
// Invariants: exported helpers name concrete resources (journal, window), not
// generic OS locations. Path helpers are safe from any goroutine. Window
// helpers run on the UI goroutine; the held-button repaint loop runs on a
// background goroutine that stops when the window is replaced or gone.
package platform
