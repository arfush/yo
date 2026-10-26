package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

// panelStyle describes the rounded surface used by settings groups, title
// panels, the log card and the shortcuts card. The zero value is transparent.
type panelStyle struct {
	fill   color.NRGBA
	border color.NRGBA
	radius unit.Dp
}

func (s panelStyle) layout(gtx layout.Context, rect image.Rectangle) image.Rectangle {
	if rect.Empty() {
		return image.Rectangle{}
	}
	radius := gtx.Dp(s.radius)
	if radius < 0 {
		radius = 0
	}
	fillRounded(gtx, s.border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	if inner.Empty() {
		return image.Rectangle{}
	}
	fillRounded(gtx, s.fill, inner, max(0, radius-gtx.Dp(unit.Dp(1))))
	return inner
}

// screenCardHeader is the accent-strip heading shared by the log and shortcuts
// cards. It returns the text column and the divider position so the caller can
// continue laying out its list.
type screenCardHeader struct {
	title         string
	subtitle      string
	titleSize     unit.Sp
	subtitleSize  unit.Sp
	titleColor    color.NRGBA
	subtitleColor color.NRGBA
}

func (h screenCardHeader) layout(gtx layout.Context, typo *typography, left, right, top int) (int, int) {
	accentWidth := gtx.Dp(unit.Dp(3))
	accentHeight := gtx.Dp(unit.Dp(22))
	accent := image.Rect(left, top, left+accentWidth, top+accentHeight)
	fillRounded(gtx, mainAccentColor1, accent, accentWidth/2)

	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := typo.measure(gtx, h.title, titleFace, h.titleSize)
	titleX := accent.Max.X + gtx.Dp(unit.Dp(10))
	titleY := top + (accentHeight-titleInk.Dy())/2
	typo.draw(gtx, h.title, titleFace, h.titleSize, h.titleColor, image.Pt(titleX, titleY))

	subtitleFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, subtitleInk := typo.measure(gtx, h.subtitle, subtitleFace, h.subtitleSize)
	subtitleY := accent.Max.Y + gtx.Dp(unit.Dp(5))
	drawTruncatedText(gtx, typo, h.subtitle, subtitleFace, h.subtitleSize, h.subtitleColor, image.Pt(titleX, subtitleY), right-titleX)
	dividerY := subtitleY + subtitleInk.Dy() + gtx.Dp(unit.Dp(14))
	fill(gtx, titlePanelBorder, image.Rect(left, dividerY, right, dividerY+max(1, gtx.Dp(unit.Dp(1)))))
	return titleX, dividerY
}

// emptyState centers a short hint in an otherwise empty list or panel.
func emptyState(gtx layout.Context, typo *typography, rect image.Rectangle, text string, size unit.Sp, col color.NRGBA) {
	if rect.Empty() || text == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	typo.drawCentered(gtx, text, face, size, col, centerOf(rect))
}

// panelForTitle returns the common title/log surface.
func panelForTitle() panelStyle {
	return panelStyle{fill: titlePanelFill, border: titlePanelBorder, radius: unit.Dp(14)}
}
