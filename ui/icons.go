package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/arfush/yonime/ui/assets"
)

// drawAppIcon centers an IconVG asset in a square box. Keeping the lookup and
// centering here means every screen uses the same rasterization path and none
// of the controls paint their own glyph geometry.
func drawAppIcon(gtx layout.Context, name string, center image.Point, size int, col color.NRGBA) {
	if size <= 0 {
		return
	}
	icon := assets.Icon(name)
	if icon == nil {
		return
	}
	stack := op.Offset(image.Pt(center.X-size/2, center.Y-size/2)).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(image.Pt(size, size))
	icon.Layout(cgtx, col)
}

func drawLogIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	drawAppIcon(gtx, "log", center, g.px(17), col)
}

func drawUpdateIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	drawAppIcon(gtx, "update", center, g.px(22), col)
}

func drawSettingsIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	drawAppIcon(gtx, "settings", center, g.px(18), col)
}

func drawPeopleIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	drawAppIcon(gtx, "people", center, g.px(20), col)
}

func drawCopyIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	drawAppIcon(gtx, "copy", center, g.px(12), col)
}

func drawPlay(gtx layout.Context, center image.Point, size int, col color.NRGBA) {
	drawAppIcon(gtx, "play", center, max(1, size), col)
}

func drawTrash(gtx layout.Context, center image.Point, size int, col color.NRGBA) {
	drawAppIcon(gtx, "trash", center, max(1, size), col)
}

// chevronIcon keeps the original stroke-weight distinction between the thin
// footer arrow and the heavier arrows inside buttons. IconVG bakes the stroke
// into the path, so the matching asset is selected from the same width/size
// ratio the old painting code used.
func chevronIcon(name string, size, width int) string {
	if size <= 0 {
		return name
	}
	ratio := float64(width) / float64(size)
	switch {
	case ratio < 0.145:
		return name + "_thin"
	case ratio > 0.20:
		return name + "_heavy"
	case ratio > 0.168:
		return name + "_thick"
	default:
		return name
	}
}

func drawChevronLeft(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	drawAppIcon(gtx, chevronIcon("chevron_left", size, width), center, max(1, size), col)
}

func drawChevronRight(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	drawAppIcon(gtx, chevronIcon("chevron_right", size, width), center, max(1, size), col)
}

func drawChevronUp(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	drawAppIcon(gtx, chevronIcon("chevron_up", size, width), center, max(1, size), col)
}

func drawChevronDown(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	drawAppIcon(gtx, chevronIcon("chevron_down", size, width), center, max(1, size), col)
}

func drawUndoArrow(gtx layout.Context, center image.Point, size int, col color.NRGBA) {
	drawAppIcon(gtx, "undo", center, max(1, size), col)
}
