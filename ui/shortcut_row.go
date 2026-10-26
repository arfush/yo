package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

// shortcutRow is one key/label line in the shortcuts card. The key column is
// measured once by the caller so every row starts its label on the same edge.
type shortcutRow struct {
	keys  string
	label string
}

func shortcutKeyWidth(gtx layout.Context, typo *typography, keys string) int {
	if keys == "" {
		return 0
	}
	_, ink := typo.measure(gtx, keys, font.Font{Typeface: fontFamily, Weight: font.Medium}, shortcutKeySize)
	return max(ink.Dx()+2*gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(26)))
}

func (r shortcutRow) layout(gtx layout.Context, typo *typography, left, keyColumn, keyGap, centerY, keyHeight int) {
	keyRect := image.Rect(left, centerY-keyHeight/2, left+keyColumn, centerY+keyHeight/2)
	drawPill(
		gtx,
		typo,
		keyRect,
		r.keys,
		shortcutKeySize,
		mainTextColor,
		mainControlFill,
		mainControlBorder,
	)
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, labelInk := typo.measure(gtx, r.label, face, shortcutTextSize)
	labelY := centerY - labelInk.Dy()/2
	typo.draw(gtx, r.label, face, shortcutTextSize, mainTextColor, image.Pt(left+keyColumn+keyGap, labelY))
}
