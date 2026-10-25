package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

const (
	shortcutTitleSize = unit.Sp(17)
	shortcutPageSize  = unit.Sp(11.5)
	shortcutKeySize   = unit.Sp(10.5)
	shortcutTextSize  = unit.Sp(12)
	shortcutKeyHeight = unit.Dp(23)
)

// shortcutItem is one row of the summary. Several alternatives or a chord are
// written in one key cap ("← / →", "Ctrl + K"), which keeps the description
// column aligned and is easier to scan than a row of tiny caps.
type shortcutItem struct {
	keys  string
	label string
}

type shortcutInfo struct {
	page  string
	items []shortcutItem
}

// shortcutSource lets a screen describe only the keys that work on it. The
// application falls back to a small generic list for screens without their own
// shortcuts.
type shortcutSource interface {
	shortcutInfo() shortcutInfo
}

type shortcutsWindow struct {
	typo *typography
	info shortcutInfo

	backClick widget.Clickable
	backHover hoverAnim

	onBack func()
}

func newShortcutsWindow(typo *typography, info shortcutInfo) *shortcutsWindow {
	return &shortcutsWindow{typo: typo, info: info}
}

func (w *shortcutsWindow) Close() {}

func (w *shortcutsWindow) layout(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}

	pad := gtx.Dp(unit.Dp(22))
	cardWidth := min(size.X-2*pad, gtx.Dp(unit.Dp(720)))
	// Cap the height so the summary stays a compact card on a maximized
	// window instead of stretching its eight rows over the whole screen.
	cardHeight := min(size.Y-2*pad, gtx.Dp(unit.Dp(480)))
	if cardWidth <= 0 || cardHeight <= 0 {
		return
	}
	card := image.Rect(
		(size.X-cardWidth)/2,
		(size.Y-cardHeight)/2,
		(size.X+cardWidth)/2,
		(size.Y+cardHeight)/2,
	)
	radius := gtx.Dp(unit.Dp(14))
	fillRounded(gtx, titlePanelBorder, card, radius)
	fillRounded(gtx, titlePanelFill, card.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	innerPad := gtx.Dp(unit.Dp(22))
	left := card.Min.X + innerPad
	right := card.Max.X - innerPad
	top := card.Min.Y + innerPad

	accentWidth := gtx.Dp(unit.Dp(3))
	accentHeight := gtx.Dp(unit.Dp(22))
	accent := image.Rect(left, top, left+accentWidth, top+accentHeight)
	fillRounded(gtx, mainAccentColor1, accent, accentWidth/2)

	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := w.typo.measure(gtx, "Горячие клавиши", titleFace, shortcutTitleSize)
	titleX := accent.Max.X + gtx.Dp(unit.Dp(10))
	titleY := top + (accentHeight-titleInk.Dy())/2
	w.typo.draw(gtx, "Горячие клавиши", titleFace, shortcutTitleSize, mainTextColor, image.Pt(titleX, titleY))

	pageFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, pageInk := w.typo.measure(gtx, w.info.page, pageFace, shortcutPageSize)
	pageY := accent.Max.Y + gtx.Dp(unit.Dp(5))
	w.typo.draw(gtx, w.info.page, pageFace, shortcutPageSize, mainMutedColor, image.Pt(titleX, pageY))

	dividerY := pageY + pageInk.Dy() + gtx.Dp(unit.Dp(14))
	fill(gtx, titlePanelBorder, image.Rect(left, dividerY, right, dividerY+max(1, gtx.Dp(unit.Dp(1)))))

	buttonHeight := gtx.Dp(unit.Dp(34))
	buttonWidth := gtx.Dp(unit.Dp(120))
	footerY := card.Max.Y - innerPad - buttonHeight
	button := image.Rect(left, footerY, left+buttonWidth, footerY+buttonHeight)

	rowsTop := dividerY + gtx.Dp(unit.Dp(8))
	rowsBottom := button.Min.Y - gtx.Dp(unit.Dp(10))
	rowCount := len(w.info.items)
	if rowCount > 0 && rowsBottom > rowsTop {
		rowHeight := (rowsBottom - rowsTop) / rowCount
		if maxRow := gtx.Dp(unit.Dp(40)); rowHeight > maxRow {
			rowHeight = maxRow
		}
		keyColumn := 0
		for _, item := range w.info.items {
			keyColumn = max(keyColumn, w.keyWidth(gtx, item.keys))
		}
		keyGap := gtx.Dp(unit.Dp(18))
		keyHeight := gtx.Dp(shortcutKeyHeight)
		textFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
		for index, item := range w.info.items {
			rowTop := rowsTop + index*rowHeight
			if rowTop+rowHeight > rowsBottom {
				break
			}
			centerY := rowTop + rowHeight/2
			keyRect := image.Rect(left, centerY-keyHeight/2, left+keyColumn, centerY+keyHeight/2)
			drawPill(
				gtx,
				w.typo,
				keyRect,
				item.keys,
				shortcutKeySize,
				mainTextColor,
				mainControlFill,
				mainControlBorder,
			)
			_, labelInk := w.typo.measure(gtx, item.label, textFace, shortcutTextSize)
			labelY := centerY - labelInk.Dy()/2
			w.typo.draw(gtx, item.label, textFace, shortcutTextSize, mainTextColor, image.Pt(left+keyColumn+keyGap, labelY))
		}
	}

	if w.layoutBack(gtx, button) && w.onBack != nil {
		w.onBack()
	}
}

func (w *shortcutsWindow) keyWidth(gtx layout.Context, keys string) int {
	if keys == "" {
		return 0
	}
	_, ink := w.typo.measure(gtx, keys, font.Font{Typeface: fontFamily, Weight: font.Medium}, shortcutKeySize)
	return max(ink.Dx()+2*gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(26)))
}

func (w *shortcutsWindow) layoutBack(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	_, clicked := w.backClick.Update(gtx)
	progress := w.backHover.update(gtx, w.backClick.Hovered())

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	w.backClick.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(8))
		fill := mixColor(titleButtonFill, titleButtonHover, progress)
		border := mixColor(titleButtonBorder, titleButtonHoverBorder, progress)
		fillRounded(gtx, border, local, radius)
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
		strokeRoundedRect(
			gtx,
			local.Inset(gtx.Dp(unit.Dp(1))),
			float32(radius),
			float32(gtx.Dp(unit.Dp(1))),
			border,
		)

		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		_, ink := w.typo.measure(gtx, "Вернуться", face, titlePageButtonSize)
		_, capInk := w.typo.measure(gtx, "Н", face, titlePageButtonSize)
		capHeight := capInk.Dy()
		iconSize := gtx.Dp(unit.Dp(11))
		gap := gtx.Dp(unit.Dp(8))
		groupWidth := iconSize + gap + ink.Dx()
		start := (local.Dx() - groupWidth) / 2
		centerY := float32(local.Dy()) / 2
		drawChevronLeft(
			gtx,
			image.Pt(start+iconSize/2, int(centerY)),
			iconSize,
			gtx.Dp(unit.Dp(1.5)),
			titleButtonText,
		)
		w.typo.paintCentered(
			gtx,
			"Вернуться",
			face,
			titlePageButtonSize,
			titleButtonText,
			f32.Pt(float32(start+iconSize+gap)+float32(ink.Dx())/2, centerY),
			capHeight,
		)
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}
