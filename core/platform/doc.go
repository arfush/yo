// Package platform owns OS integration that Gio does not expose: concrete
// per-user paths and the native window frame. Windows paints the frame through
// DWM; other platforms keep the default frame.
//
// Imports: std, gioui.org/app, gioui.org/io/event, golang.org/x/sys/windows.
// Must not import: ui/*, root.
// Invariants: exported helpers name concrete resources (journal, window), not
// generic OS locations. Window helpers run on the UI goroutine and are no-ops
// when the OS feature is unavailable; path helpers are safe from any goroutine.
package platform
