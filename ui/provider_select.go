package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/party"
)

func (w *settingsScreen) layoutProviderTrigger(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	clicked := w.providerClick.Clicked(gtx)
	active := w.providerClick.Hovered() || w.providerOpen

	w.layoutClickableDraw(gtx, rect, &w.providerClick, func(gtx layout.Context) {
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(7))
		fill := tokenDomainFill
		if active {
			fill = tokenDomainFillHover
		}
		fillRounded(gtx, tokenDomainBorder, local, radius)
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		label := w.provider.Label()
		_, ink := w.typo.measure(gtx, label, face, settingsButtonSize)
		padX := gtx.Dp(unit.Dp(12))
		w.typo.draw(gtx, label, face, settingsButtonSize, mainTextColor, image.Pt(padX, (rect.Dy()-ink.Dy())/2))

		size := gtx.Dp(unit.Dp(9))
		center := image.Pt(rect.Dx()-padX-size/2, rect.Dy()/2)
		width := gtx.Dp(unit.Dp(1.6))
		if w.providerOpen {
			drawChevronUp(gtx, center, size, width, settingsNoteColor)
			return
		}
		drawChevronDown(gtx, center, size, width, settingsNoteColor)
	})
	return clicked
}

// layoutProviderMenu paints the open select. It is a plain overlay instead of
// an inline list so the panel row keeps its single-line height.
func (w *settingsScreen) layoutProviderMenu(gtx layout.Context, size image.Point, anchor image.Rectangle) {
	if anchor.Empty() {
		return
	}
	providers := party.Providers()
	pad := gtx.Dp(settingsProviderMenuPad)
	itemHeight := gtx.Dp(settingsProviderItemHeight)
	itemGap := gtx.Dp(settingsProviderItemGap)
	menu := placePopupList(
		anchor,
		providerMenuHeight(gtx, len(providers)),
		gtx.Dp(settingsProviderMenuGap),
		image.Rectangle{Max: size},
	)

	// The scrim goes first so the options registered after it stay on top and
	// receive their own clicks; anything else closes the menu.
	if layoutOverlayScrim(gtx, image.Rectangle{Max: size}, &w.providerScrim) {
		w.providerOpen = false
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	drawPopupMenu(gtx, menu)

	for index, provider := range providers {
		item := image.Rect(
			menu.Min.X+pad,
			menu.Min.Y+pad+index*(itemHeight+itemGap),
			menu.Max.X-pad,
			menu.Min.Y+pad+index*(itemHeight+itemGap)+itemHeight,
		)
		if w.layoutProviderItem(gtx, item, provider, &w.providerItems[index]) {
			w.setProvider(provider)
			w.saveRelays()
			w.providerOpen = false
			gtx.Execute(key.FocusCmd{})
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

// providerMenuHeight sums the options plus the gaps and the menu padding.
func providerMenuHeight(gtx layout.Context, count int) int {
	if count <= 0 {
		return 0
	}
	height := count*gtx.Dp(settingsProviderItemHeight) + 2*gtx.Dp(settingsProviderMenuPad)
	if count > 1 {
		height += (count - 1) * gtx.Dp(settingsProviderItemGap)
	}
	return height
}

func (w *settingsScreen) layoutProviderItem(gtx layout.Context, rect image.Rectangle, provider party.Provider, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	selected := provider == w.provider

	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(6))
		switch {
		case selected:
			fillRounded(gtx, settingsMenuSelected, local, radius)
		case hovered:
			fillRounded(gtx, tokenDomainFillHover, local, radius)
		}

		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		label := provider.Label()
		_, ink := w.typo.measure(gtx, label, face, settingsButtonSize)
		textColor := mainTextColor
		if selected {
			textColor = badgeTextColor
		}
		padX := gtx.Dp(unit.Dp(10))
		w.typo.draw(gtx, label, face, settingsButtonSize, textColor, image.Pt(padX, (rect.Dy()-ink.Dy())/2))
	})
	return clicked
}
