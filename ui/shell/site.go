package shell

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
)

// Site switcher geometry, in device-independent dp.
const (
	siteMenuMinWidth   unit.Dp = 120
	siteMenuGap        unit.Dp = 6
	siteMenuMargin     unit.Dp = 4
	siteMenuPadding    unit.Dp = 4
	siteMenuItemHeight unit.Dp = 42
	siteMenuItemGap    unit.Dp = 2
	siteMenuItemRadius unit.Dp = 6
	siteMenuItemPad    unit.Dp = 7
	siteMenuItemGapX   unit.Dp = 10
)

// siteMenuBasePaletteAfter is the number of consecutive user dismissals of the
// site menu, without a switch, after which the chrome falls back to yobase.
const siteMenuBasePaletteAfter = 5

// siteMenu is the state of the anchored site switcher. The zero value is closed
// and valid.
type siteMenu struct {
	open   bool
	anchor image.Rectangle
}

// close hides the menu and clears the anchor.
func (m *siteMenu) close() {
	m.open = false
	m.anchor = image.Rectangle{}
}

// toggle opens the menu at anchor or closes it when it is already open.
func (m *siteMenu) toggle(anchor image.Rectangle) {
	if m.open {
		m.close()
		return
	}
	m.open = true
	m.anchor = anchor
}

// siteSwitchable reports whether the header can open the site switcher.
func (s *Shell) siteSwitchable() bool {
	return !s.frozen && len(catalog.Sites()) > 1
}

// layoutSiteTrigger registers the click target of the site block.
func (s *Shell) layoutSiteTrigger(gtx layout.Context, trigger image.Rectangle) bool {
	if trigger.Empty() {
		return false
	}
	clicked := s.siteClick.Clicked(gtx)
	stack := op.Offset(trigger.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := gtx
	local.Constraints = layout.Exact(trigger.Size())
	s.siteClick.Layout(local, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		semantic.Button.Add(gtx.Ops)
		return layout.Dimensions{Size: trigger.Size()}
	})
	return clicked
}

// switchSite applies a selection from the header switcher. The shell updates
// the holder because it owns the writable site source; the palette follows on
// the next frame. A selection always restores the site palette, even when the
// neutral fallback is showing for the same site.
func (s *Shell) switchSite(site catalog.Site) {
	if s.frozen {
		return
	}
	s.menu.close()
	s.siteDismissals = 0
	previous := s.site.Value()
	if previous == site {
		s.applySitePalette(site)
		return
	}
	s.site.Update(func(v *catalog.Site) { *v = site })
	s.logger.Info("site switched", "site", site.String(), "previous", previous.String())
	s.syncPalette()
}

// dismissSiteMenu closes an open menu after a user action and counts it toward
// the neutral fallback. Navigation and dialog changes close the menu directly:
// only a dismissal without a switch may reset the chrome to yobase.
func (s *Shell) dismissSiteMenu() {
	if !s.menu.open {
		return
	}
	s.menu.close()
	s.noteSiteMenuDismissed()
}

// noteSiteMenuDismissed counts a menu close that did not switch sites. After
// siteMenuBasePaletteAfter consecutive dismissals the chrome falls back to the
// neutral palette until the next selection.
func (s *Shell) noteSiteMenuDismissed() {
	s.siteDismissals++
	if s.siteDismissals == siteMenuBasePaletteAfter {
		s.applyPalette(appearance.BasePalette())
	}
}

// layoutSiteMenu paints the switcher after the content, so the dropdown sits
// above every content rectangle.
func (s *Shell) layoutSiteMenu(gtx layout.Context, g geometry) {
	if !s.menu.open {
		return
	}
	if !s.siteSwitchable() {
		s.menu.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	sites := catalog.Sites()
	if len(sites) < 2 {
		s.menu.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	line := max(1, gtx.Dp(appearance.Hairline))
	pad := gtx.Dp(siteMenuPadding)
	itemPad := gtx.Dp(siteMenuItemPad)
	itemHeight := gtx.Dp(siteMenuItemHeight)
	itemGap := gtx.Dp(siteMenuItemGap)
	rows := len(sites) - 1
	height := 2*(line+pad) + rows*itemHeight + max(0, rows-1)*itemGap
	margin := gtx.Dp(siteMenuMargin)
	bounds := image.Rectangle{Max: g.size}

	anchor := siteMenuAnchor(s.menu.anchor, line+pad+itemPad,
		gtx.Dp(siteMenuMinWidth), margin, bounds)
	menu := placeSiteMenu(siteMenuBox{
		anchor: anchor,
		width:  anchor.Dx(),
		height: height,
		gap:    gtx.Dp(siteMenuGap),
		margin: margin,
		bounds: bounds,
	})
	if menu.Empty() {
		s.menu.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	if s.scrim.dismissed(gtx) {
		s.dismissSiteMenu()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	// The blocking scrim covers the whole window before the menu rows, so the
	// next press only dismisses the menu and never reaches the header or the
	// content below.
	s.scrim.layout(gtx, bounds)

	palette := s.palette()
	typo := s.typography()
	radius := gtx.Dp(appearance.RadiusS)
	kit.FillRounded(gtx, palette.Border, menu, radius)
	kit.FillRounded(gtx, palette.Surface, menu.Inset(line), max(0, radius-line))
	inner := menu.Inset(line + pad)
	if inner.Empty() {
		s.menu.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	current := s.site.Value()
	slot := 0
	for index, site := range sites {
		if site == current {
			continue
		}
		top := inner.Min.Y + slot*(itemHeight+itemGap)
		row := rect(inner.Min.X, top, inner.Max.X, top+itemHeight)
		if s.layoutSiteItem(gtx, siteMenuItem{bounds: row, index: index, site: site}, palette, typo) {
			s.switchSite(site)
			gtx.Execute(op.InvalidateCmd{})
		}
		slot++
	}
}

// siteMenuItem identifies one switcher row.
type siteMenuItem struct {
	bounds image.Rectangle
	index  int
	site   catalog.Site
}

// layoutSiteItem paints one switcher row and reports a click. The icon uses the
// target site accent, so the row previews the palette the site will adopt.
func (s *Shell) layoutSiteItem(
	gtx layout.Context,
	item siteMenuItem,
	palette appearance.Palette,
	typo appearance.Typography,
) bool {
	bounds := item.bounds
	if bounds.Empty() || item.index < 0 || item.index >= len(s.siteItems) {
		return false
	}
	click := &s.siteItems[item.index]
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()

	stack := op.Offset(bounds.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := gtx
	local.Constraints = layout.Exact(bounds.Size())
	click.Layout(local, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		semantic.Button.Add(gtx.Ops)
		size := bounds.Size()
		if hovered {
			kit.FillRounded(gtx, palette.SurfaceAlt,
				image.Rectangle{Max: size}, gtx.Dp(siteMenuItemRadius))
		}

		target := appearance.PaletteFor(item.site)
		iconSize := gtx.Dp(logoSize)
		pad := gtx.Dp(siteMenuItemPad)
		gap := gtx.Dp(siteMenuItemGapX)
		top := (size.Y - iconSize) / 2
		iconRect := rect(pad, top, pad+iconSize, top+iconSize)
		s.layoutSiteMark(gtx, iconRect, target)

		textGtx := gtx
		textGtx.Constraints = layout.Constraints{Max: image.Pt(
			max(1, size.X-2*pad-iconSize-gap),
			size.Y,
		)}
		macro := op.Record(gtx.Ops)
		dims := kit.Text(textGtx, typo, appearance.RoleLabel, item.site.String(), palette.Text)
		call := macro.Stop()
		at := image.Pt(pad+iconSize+gap, (size.Y-dims.Size.Y)/2)
		textStack := op.Offset(at).Push(gtx.Ops)
		call.Add(gtx.Ops)
		textStack.Pop()
		return layout.Dimensions{Size: size}
	})
	return clicked
}

// siteMenuAnchor widens the trigger leftwards so the menu ends just past the
// chevron while its first row content lines up with the header brand. It clamps
// the result into the window margin.
func siteMenuAnchor(trigger image.Rectangle, contentInset, minWidth, margin int, bounds image.Rectangle) image.Rectangle {
	right := min(trigger.Max.X, bounds.Max.X-margin)
	left := max(trigger.Min.X-contentInset, bounds.Min.X+margin)
	if right-left < minWidth {
		left = max(bounds.Min.X+margin, right-minWidth)
	}
	right = max(right, left)
	return image.Rect(left, trigger.Min.Y, right, trigger.Max.Y)
}

// siteMenuBox is the pixel geometry placeSiteMenu fits into bounds.
type siteMenuBox struct {
	anchor image.Rectangle
	width  int
	height int
	gap    int
	margin int
	bounds image.Rectangle
}

// placeSiteMenu aligns the menu with its anchor, prefers the space below it,
// then clamps the result into bounds. An impossible placement is empty.
func placeSiteMenu(box siteMenuBox) image.Rectangle {
	if box.width <= 0 || box.height <= 0 || box.bounds.Empty() {
		return image.Rectangle{}
	}
	margin := min(box.margin, max(0, min(box.bounds.Dx(), box.bounds.Dy())/2))

	x := box.anchor.Min.X
	if x+box.width > box.bounds.Max.X-margin {
		x = box.bounds.Max.X - margin - box.width
	}
	x = max(x, box.bounds.Min.X+margin)

	y := box.anchor.Max.Y + box.gap
	if y+box.height > box.bounds.Max.Y-margin {
		above := box.anchor.Min.Y - box.gap - box.height
		if above >= box.bounds.Min.Y+margin {
			y = above
		} else {
			y = box.bounds.Max.Y - margin - box.height
		}
	}
	y = max(y, box.bounds.Min.Y+margin)

	menu := image.Rect(x, y, x+box.width, y+box.height)
	if menu.Min.X < box.bounds.Min.X || menu.Min.Y < box.bounds.Min.Y ||
		menu.Max.X > box.bounds.Max.X || menu.Max.Y > box.bounds.Max.Y {
		return image.Rectangle{}
	}
	return menu
}

// siteScrim is the invisible full-window click target behind the open menu. It
// blocks the content below, so the next press only dismisses the menu.
type siteScrim struct {
	click widget.Clickable
}

// dismissed reports a queued click on the scrim. It must run before layout:
// Clickable.Layout drains the click events.
func (s *siteScrim) dismissed(gtx layout.Context) bool {
	return s.click.Clicked(gtx)
}

// layout installs the blocking click target over bounds. The menu rows are laid
// out after it, so they receive the pointer first.
func (s *siteScrim) layout(gtx layout.Context, bounds image.Rectangle) {
	if bounds.Empty() {
		return
	}
	stack := op.Offset(bounds.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := gtx
	local.Constraints = layout.Exact(bounds.Size())
	s.click.Layout(local, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: bounds.Size()}
	})
}
