// Package resource provides embedded fonts, icons, images and translations.
//
// Must not import: core/*, root, ui/*.
//
// Invariants: New reads and parses every embedded asset once. A broken or
// missing asset is a build error, so it panics at startup or first use.
// Returned fonts, icons and images are shared and must not be modified.
// Assets live in font/, icon/ and lang/ under this package.
package resource
