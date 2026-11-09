// Package log owns the application journal: structured slog output to a
// rotating file, console streams and a bounded in-memory ring.
//
// Must not import: catalog, other core packages, UI, root.
//
// Invariants: one Journal per process, created by root and closed on exit.
// Journal is safe for concurrent use. Each module gets its own logger from
// Journal.Logger and stores it in a field; there are no package loggers.
// Secrets must not be logged; secret-like attribute keys are redacted.
package log
