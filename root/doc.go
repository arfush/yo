// Package root is the composition root of yo.
//
// Imports: catalog, core/*, ui/*.
// Imported by: cmd/yo. Must not be imported by core/* or ui/*.
//
// Invariants: creates concrete types and screens, owns lifecycle, and passes
// screen factories to ui/navigator. It contains no business logic or
// navigation decisions.
package root
