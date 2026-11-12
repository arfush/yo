// Package platform owns OS integration that the UI toolkit does not expose:
// concrete per-user paths such as the journal location.
//
// Must not import: catalog, other core packages, UI, root.
//
// Invariants: exported helpers name concrete resources (journal), not generic
// OS locations. Path helpers are safe from any goroutine.
package platform
