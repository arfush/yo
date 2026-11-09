// Package root is the composition root of yo.
//
// Imported only by cmd/yo; core and UI must not import root.
//
// Invariants: creates concrete types and screens, owns lifecycle, and passes
// screen factories to ui/navigator. It contains no business logic or
// navigation decisions.
package root
