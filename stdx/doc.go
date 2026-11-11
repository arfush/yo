// Package stdx extends the standard library with generic, domain-free helpers.
//
// Must not import: catalog, core, UI, root.
//
// Invariants: no I/O, no globals, no domain types; concurrency guarantees are
// documented per type.
package stdx
