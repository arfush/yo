// Package platform owns OS integration that Gio does not expose. The first
// feature is the native window frame: Windows paints it through DWM, other
// platforms keep the default frame.
//
// Imports: std, gioui.org/app, gioui.org/io/event, golang.org/x/sys/windows.
// Must not import: ui/*, root.
// Invariants: helpers run on the UI goroutine and are no-ops when the OS
// feature is unavailable.
package platform
