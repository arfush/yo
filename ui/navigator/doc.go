// Package navigator owns the screen stack and the route vocabulary. A screen
// requests a change through Handler; the navigator resolves the intent through a
// factory supplied by the composition root and installs the built screen into
// the shell.
//
// Must not import: core/*, root, ui/screen.
//
// Invariants: Navigate only queues and invalidates; Apply runs on the UI
// goroutine before the next paint. A screen is never nil, Back returns to the
// previous screen, and Close is idempotent.
package navigator
