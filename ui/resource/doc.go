// Package resource provides embedded fonts, icons, images and translations.
//
// Must not import: core/*, root, ui/*. Qt is allowed: vector icons are
// returned ready for QPainter.
//
// Invariants: New reads and parses every embedded asset once, including SVG
// icon paths. A broken or missing asset is a build error, so it panics at
// startup. Returned fonts, images and icon paths are shared: callers must not
// modify or delete them, and icon paths may be used only on the UI goroutine.
// Assets live in font/, icon/ and lang/ under this package.
package resource
