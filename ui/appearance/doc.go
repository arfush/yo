// Package appearance owns how the UI looks and reads: palette, typography,
// language and formatting.
//
// Imports: std, catalog, gioui.org/font, gioui.org/text, gioui.org/unit,
// golang.org/x/text/language, ui/resource.
// Must not import: core/*, root, ui/* except ui/resource.
//
// Invariants: Appearance is a value stored in stdx.Holder. The composition
// root and the shell mutate it through Holder.Update; consumers read it through
// a ReadonlyHolder. PaletteFor maps every catalog site to a palette; Appearance
// never stores the active site. The text shaper is shared and must stay on the
// UI goroutine. New panics on nil resources, an empty or unknown language, or a
// missing embedded font.
package appearance
