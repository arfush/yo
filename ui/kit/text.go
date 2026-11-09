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
	return text(gtx, typo, role, value, col, widget.Label{MaxLines: 1, Truncator: "…"})
}

// TextBlock draws value with the typography role and color, wrapping long text
// into as many lines as it needs.
func TextBlock(gtx layout.Context, typo appearance.Typography, role appearance.TextRole, value string, col color.NRGBA) layout.Dimensions {
	return text(gtx, typo, role, value, col, widget.Label{})
}

func text(gtx layout.Context, typo appearance.Typography, role appearance.TextRole, value string, col color.NRGBA, label widget.Label) layout.Dimensions {
	material := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	call := material.Stop()
	return label.Layout(gtx, typo.Shaper(), typo.Font(role), typo.TextSize(role), value, call)
}
