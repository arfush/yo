// Package navigator owns the screen stack, the dialog stack and the route
// vocabulary. A screen requests a change through Handler; the navigator
// resolves the intent through a factory supplied by the composition root and
// installs the built screen or dialog into the shell.
//
// Imports: fmt, log/slog, ui/shell.
// Must not import: root, ui/screen, ui/dialog, core/*.
// Invariants: Navigate only queues and invalidates; Apply runs on the UI
// goroutine at frame start before layout. A screen is never nil, Back closes
// the top dialog before it pops a screen, and Close is idempotent.
package navigator
