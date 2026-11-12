package shell

import (
	"image"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
)

// Site switcher geometry, in logical pixels.
const (
	siteMenuMinWidth   = 120
	siteMenuGap        = 6
	siteMenuMargin     = 4
	siteMenuPadding    = 4
	siteMenuItemHeight = 42
	siteMenuItemGap    = 2
	siteMenuItemRadius = 6
	siteMenuItemPad    = 7
	siteMenuItemGapX   = 10
)

// siteMenuBasePaletteAfter is the number of consecutive user dismissals of the
// site menu, without a switch, after which the chrome falls back to yobase.
const siteMenuBasePaletteAfter = 5

// menuLayout returns the dropdown box, its padded content and the rows of every
// site except the active one. The menu hangs below the trigger and flips above
// it when there is no room left.
func (s *Shell) menuLayout(l layout) (image.Rectangle, image.Rectangle, []image.Rectangle, []catalog.Site) {
	sites := catalog.Sites()
	if len(sites) < 2 || l.trigger.Empty() {
		return image.Rectangle{}, image.Rectangle{}, nil, nil
	}
	rows := len(sites) - 1
	line := max(1, int(appearance.Hairline))
	pad := siteMenuPadding
	height := 2*(line+pad) + rows*siteMenuItemHeight + max(0, rows-1)*siteMenuItemGap

	right := min(l.trigger.Max.X, l.size.X-siteMenuMargin)
	left := max(l.trigger.Min.X-(line+pad+siteMenuItemPad), siteMenuMargin)
	if right-left < siteMenuMinWidth {
		left = max(siteMenuMargin, right-siteMenuMinWidth)
	}
	right = max(right, left)
	y := l.trigger.Max.Y + siteMenuGap
	if y+height > l.size.Y-siteMenuMargin {
		above := l.trigger.Min.Y - siteMenuGap - height
		if above >= siteMenuMargin {
			y = above
		} else {
			y = l.size.Y - siteMenuMargin - height
		}
	}
	y = max(y, siteMenuMargin)
	menu := image.Rect(left, y, right, y+height)
	if menu.Empty() {
		return image.Rectangle{}, image.Rectangle{}, nil, nil
	}
	inner := menu.Inset(line + pad)
	rects := make([]image.Rectangle, 0, rows)
	visible := make([]catalog.Site, 0, rows)
	slot := 0
	current := s.site.Value()
	for _, site := range sites {
		if site == current {
			continue
		}
		top := inner.Min.Y + slot*(siteMenuItemHeight+siteMenuItemGap)
		rects = append(rects, image.Rect(inner.Min.X, top, inner.Max.X, top+siteMenuItemHeight))
		visible = append(visible, site)
		slot++
	}
	return menu, inner, rects, visible
}

// paintMenu draws the anchored site switcher above the content.
func (s *Shell) paintMenu(p *qt.QPainter, l layout) {
	menu, _, rows, sites := s.menuLayout(l)
	if menu.Empty() || len(rows) == 0 {
		return
	}
	palette := s.palette()
	app := s.appearance.Value()
	line := max(1, int(appearance.Hairline))
	radius := float64(appearance.RadiusS)
	kit.FillRounded(p, palette.Border, menu, radius)
	kit.FillRounded(p, palette.Panel, menu.Inset(line), max(0, radius-float64(line)))

	for i, site := range sites {
		row := rows[i]
		if s.menuRowHover == i {
			kit.FillRounded(p, palette.SurfaceAlt, row, float64(siteMenuItemRadius))
		}
		iconSize := logoSize
		top := row.Min.Y + (row.Dy()-iconSize)/2
		icon := image.Rect(
			row.Min.X+siteMenuItemPad, top,
			row.Min.X+siteMenuItemPad+iconSize, top+iconSize,
		)
		// The icon previews the palette the site will adopt.
		s.paintSiteMark(p, icon, appearance.PaletteFor(site))
		textX := row.Min.X + siteMenuItemPad + iconSize + siteMenuItemGapX
		drawTextLine(p, app, appearance.RoleLabel, site.String(), palette.Text, textX, row.Min.Y+row.Dy()/2)
	}
}
