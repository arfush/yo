package shell

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

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
	siteMenuRadius     unit.Dp = 8
	siteMenuItemHeight unit.Dp = 42
	siteMenuItemGap    unit.Dp = 2
	siteMenuItemRadius unit.Dp = 6
	siteMenuItemPad    unit.Dp = 7
	siteMenuItemGapX   unit.Dp = 10
)

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
	return s.dialog == nil && len(catalog.Sites()) > 1
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
// the next frame.
func (s *Shell) switchSite(site catalog.Site) {
	s.menu.close()
	if s.site.Value() == site {
		return
	}
	s.site.Update(func(v *catalog.Site) { *v = site })
	s.syncPalette()
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

	line := max(1, gtx.Dp(1))
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
	menu := placeSiteMenu(anchor, anchor.Dx(), height, gtx.Dp(siteMenuGap), margin, bounds)
	if menu.Empty() {
		s.menu.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	if s.scrim.dismissed(gtx, menu) {
		s.menu.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	s.scrim.layout(gtx, bounds)

	palette := s.palette()
	typo := s.typography()
	radius := gtx.Dp(siteMenuRadius)
	fillRounded(gtx, palette.Border, menu, radius)
	fillRounded(gtx, palette.Surface, menu.Inset(line), max(0, radius-line))
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
		item := rect(inner.Min.X, top, inner.Max.X, top+itemHeight)
		if s.layoutSiteItem(gtx, item, index, site, palette, typo) {
			s.switchSite(site)
			gtx.Execute(op.InvalidateCmd{})
		}
		slot++
	}
}

// layoutSiteItem paints one switcher row and reports a click. The icon uses the
// target site accent, so the row previews the palette the site will adopt.
func (s *Shell) layoutSiteItem(
	gtx layout.Context,
	bounds image.Rectangle,
	index int,
	site catalog.Site,
	palette appearance.Palette,
	typo appearance.Typography,
) bool {
	if bounds.Empty() || index < 0 || index >= len(s.siteItems) {
		return false
	}
	click := &s.siteItems[index]
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
			fillRounded(gtx, palette.SurfaceAlt,
				image.Rectangle{Max: size}, gtx.Dp(siteMenuItemRadius))
		}

		target := appearance.PaletteFor(site)
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
		dims := kit.Text(textGtx, typo, appearance.RoleLabel, site.String(), palette.Text)
		call := macro.Stop()
		at := image.Pt(pad+iconSize+gap, (size.Y-dims.Size.Y)/2)
		textStack := op.Offset(at).Push(gtx.Ops)
		call.Add(gtx.Ops)
		textStack.Pop()
		return layout.Dimensions{Size: size}
	})
	return clicked
}

// siteMenuAnchor widens the trigger leftwards so the menu ends at the chevron
// while its first row content lines up with the header brand. It never extends
// past the chevron or the window margin.
func siteMenuAnchor(trigger image.Rectangle, contentInset, minWidth, margin int, bounds image.Rectangle) image.Rectangle {
	right := min(trigger.Max.X, bounds.Max.X-margin)
	left := max(trigger.Min.X-contentInset, bounds.Min.X+margin)
	if right-left < minWidth {
		left = max(bounds.Min.X+margin, right-minWidth)
	}
	right = max(right, left)
	return image.Rect(left, trigger.Min.Y, right, trigger.Max.Y)
}

// placeSiteMenu aligns the menu with its anchor, prefers the space below it,
// then clamps the result into bounds. An impossible placement is empty.
func placeSiteMenu(anchor image.Rectangle, width, height, gap, margin int, bounds image.Rectangle) image.Rectangle {
	if width <= 0 || height <= 0 || bounds.Empty() {
		return image.Rectangle{}
	}
	margin = min(margin, max(0, min(bounds.Dx(), bounds.Dy())/2))

	x := anchor.Min.X
	if x+width > bounds.Max.X-margin {
		x = bounds.Max.X - margin - width
	}
	x = max(x, bounds.Min.X+margin)

	y := anchor.Max.Y + gap
	if y+height > bounds.Max.Y-margin {
		above := anchor.Min.Y - gap - height
		if above >= bounds.Min.Y+margin {
			y = above
		} else {
			y = bounds.Max.Y - margin - height
		}
	}
	y = max(y, bounds.Min.Y+margin)

	menu := image.Rect(x, y, x+width, y+height)
	if menu.Min.X < bounds.Min.X || menu.Min.Y < bounds.Min.Y ||
		menu.Max.X > bounds.Max.X || menu.Max.Y > bounds.Max.Y {
		return image.Rectangle{}
	}
	return menu
}

// siteScrim dismisses the open menu on a press outside it. It passes pointer
// events through, so the window controls and content keep working.
type siteScrim struct {
	tag bool
}

// dismissed consumes queued presses and reports whether one landed outside
// menu.
func (s *siteScrim) dismissed(gtx layout.Context, menu image.Rectangle) bool {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &s.tag, Kinds: pointer.Press})
		if !ok {
			return false
		}
		press, ok := ev.(pointer.Event)
		if !ok || press.Kind != pointer.Press {
			continue
		}
		if press.Source == pointer.Mouse && !press.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		if press.Position.Round().In(menu) {
			continue
		}
		return true
	}
}

// layout installs the pass-through press target over bounds.
func (s *siteScrim) layout(gtx layout.Context, bounds image.Rectangle) {
	if bounds.Empty() {
		return
	}
	stack := clip.Rect(bounds).Push(gtx.Ops)
	defer stack.Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	defer pass.Pop()
	event.Op(gtx.Ops, &s.tag)
}
