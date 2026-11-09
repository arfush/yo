// Package kit provides reusable widgets and drawing helpers shared by screens
// and the shell.
//
// Imports: std, gioui.org/layout, gioui.org/op, gioui.org/op/paint,
// gioui.org/widget, ui/appearance.
// Must not import: core/*, root, ui/navigator, ui/screen, ui/shell.
//
// Invariants: helpers draw into the caller's layout.Context and return
// dimensions; they never mutate application state.
package kit

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/arfush/yo/ui/appearance"
)

// Text draws one line of value with the typography role and color. Long text
// is truncated with an ellipsis instead of wrapping.
func Text(gtx layout.Context, typo appearance.Typography, role appearance.TextRole, value string, col color.NRGBA) layout.Dimensions {
	material := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	call := material.Stop()
	label := widget.Label{MaxLines: 1, Truncator: "…"}
	return label.Layout(gtx, typo.Shaper(), typo.Font(role), typo.TextSize(role), value, call)
}
