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
