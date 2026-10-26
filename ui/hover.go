package ui

import (
	"image/color"

	"gioui.org/layout"
)

func drawControlHover(gtx layout.Context, g geometry, progress float32, base color.NRGBA) {
	if progress < 0.002 {
		return
	}
	base.A = uint8(float32(base.A) * progress)
	strokeRoundedRect(gtx, g.hoverRect(), float32(g.hoverRadius), float32(g.leastPx(1)), base)
}
