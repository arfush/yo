package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
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

type shortcutsScreen struct {
	typo *typography
	info shortcutInfo

	back footerNamedButton

	onBack func()
}

func newShortcutsScreen(typo *typography, info shortcutInfo) *shortcutsScreen {
	return &shortcutsScreen{typo: typo, info: info, back: newBackFooterButton()}
}

func (w *shortcutsScreen) Close() {}

func (w *shortcutsScreen) layout(gtx layout.Context) {
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
	panelForTitle().layout(gtx, card)

	innerPad := gtx.Dp(unit.Dp(22))
	left := card.Min.X + innerPad
	right := card.Max.X - innerPad
	top := card.Min.Y + innerPad
	_, dividerY := screenCardHeader{
		title:         "Горячие клавиши",
		subtitle:      w.info.page,
		titleSize:     shortcutTitleSize,
		subtitleSize:  shortcutPageSize,
		titleColor:    mainTextColor,
		subtitleColor: mainMutedColor,
	}.layout(gtx, w.typo, left, right, top)

	rowsTop := dividerY + gtx.Dp(unit.Dp(8))
	rowsBottom := card.Max.Y - innerPad
	rowCount := len(w.info.items)
	if rowCount > 0 && rowsBottom > rowsTop {
		rowHeight := (rowsBottom - rowsTop) / rowCount
		if maxRow := gtx.Dp(unit.Dp(40)); rowHeight > maxRow {
			rowHeight = maxRow
		}
		keyColumn := 0
		for _, item := range w.info.items {
			keyColumn = max(keyColumn, shortcutKeyWidth(gtx, w.typo, item.keys))
		}
		keyGap := gtx.Dp(unit.Dp(18))
		keyHeight := gtx.Dp(shortcutKeyHeight)
		for index, item := range w.info.items {
			rowTop := rowsTop + index*rowHeight
			if rowTop+rowHeight > rowsBottom {
				break
			}
			shortcutRow{keys: item.keys, label: item.label}.layout(
				gtx, w.typo, left, keyColumn, keyGap, rowTop+rowHeight/2, keyHeight,
			)
		}
	}
}

// layoutFooter draws the summary's only action into the shared application
// footer. Keeping it outside the card matches the title and log screens.
func (w *shortcutsScreen) layoutFooter(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(titlePageButtonHeight)
	top := (size.Y - height) / 2
	button := image.Rect(padX, top, padX+gtx.Dp(unit.Dp(120)), top+height)
	if w.back.layout(gtx, w.typo, button) && w.onBack != nil {
		w.onBack()
	}
}
