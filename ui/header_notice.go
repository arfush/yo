package ui

import (
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

// headerNoticeTone selects the palette of the single header sentence. The
// shell owns the priority: undo, then error, then the informational rate-limit
// countdown.
type headerNoticeTone int

const (
	headerNoticeError headerNoticeTone = iota
	headerNoticeUndo
	headerNoticeRate
)

type headerNotice struct {
	text string
	tone headerNoticeTone
}

func (n headerNotice) style() (color.NRGBA, unit.Sp) {
	switch n.tone {
	case headerNoticeUndo:
		return undoCountdownColor, mainClearSize
	case headerNoticeRate:
		return rateLimitTextColor, rateLimitTextSize
	default:
		return mainDangerColor, mainClearSize
	}
}

func (n headerNotice) empty() bool {
	return n.text == ""
}

// layoutHeaderNotice paints one centered line between the version badge and
// the window controls. Truncation happens before measuring so a long phrase
// cannot overlap the native controls.
func layoutHeaderNotice(gtx layout.Context, typo *typography, g geometry, badgeRight int, notice headerNotice) {
	if notice.empty() {
		return
	}
	left := badgeRight + g.px(14)
	right := g.controls.minimize.Min.X - g.px(14)
	if right <= left {
		return
	}
	col, size := notice.style()
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	label := truncateText(gtx, typo, notice.text, face, size, right-left)
	if label == "" {
		return
	}
	glyphs, ink := typo.measure(gtx, label, face, size)
	if len(glyphs) == 0 || ink.Empty() {
		return
	}
	x := left + (right-left-ink.Dx())/2
	if notice.tone == headerNoticeError {
		// The error keeps the established 2%-of-window nudge to the left.
		x -= int(math.Round(float64(g.size.X) * 0.02))
	}
	if x < left {
		x = left
	}
	if x+ink.Dx() > right {
		x = right - ink.Dx()
	}
	capHeight := int(glyphs[0].Y) - ink.Min.Y
	if capHeight <= 0 || capHeight > ink.Dy() {
		capHeight = ink.Dy()
	}
	typo.paintCentered(
		gtx,
		label,
		face,
		size,
		col,
		f32.Pt(float32(x)+float32(ink.Dx())/2, float32(g.headerCenterY)),
		capHeight,
	)
}
