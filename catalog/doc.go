// Package catalog holds shared value types. It has no errors, I/O or UI.
//
// Must not import: core/*, UI (ui/*, gioui.org/*), root.
//
// Invariants: types are plain values. Site is the only library selector; every
// site returned by Sites must have a stable identifier and display name.
package catalog
