package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
)

// userStatusMenu is a small popup anchored to a status pill. It is drawn by the
// shell overlay so it can extend over rows and the footer without being clipped
// by the content viewport.
type userStatusMenu struct {
	open    bool
	anchor  image.Rectangle
	current catalog.UserStatus
	id      int64

	// pillWidth and pillHeight come from the status block that opened the menu,
	// so history and title-page menus match their own pills.
	pillWidth  int
	pillHeight int

	clicks [5]widget.Clickable
	hovers [5]hoverAnim
	scrim  widget.Clickable
}

func (m *userStatusMenu) show(anchor image.Rectangle, current catalog.UserStatus, id int64, pillWidth, pillHeight int) {
	m.open = true
	m.anchor = anchor
	m.current = catalog.NormalizeUserStatus(current)
	m.id = id
	m.pillWidth = max(pillWidth, 1)
	m.pillHeight = max(pillHeight, 1)
}

func (m *userStatusMenu) close() {
	m.open = false
}

func (m *userStatusMenu) layout(gtx layout.Context, typo *typography, origin image.Point, onSelect func(catalog.UserStatus)) {
	if !m.open {
		return
	}
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		m.close()
		return
	}
	options := userStatusMenuOptionsFor(m.current)
	if len(options) == 0 {
		m.close()
		return
	}

	pad := gtx.Dp(userStatusMenuPad)
	gap := gtx.Dp(userStatusMenuGap)
	itemWidth := m.pillWidth
	itemHeight := m.pillHeight
	menuWidth := itemWidth + 2*pad
	menuHeight := len(options)*itemHeight + 2*pad
	if len(options) > 1 {
		menuHeight += (len(options) - 1) * gap
	}
	menu := placePopupMenu(
		m.anchor.Add(origin),
		menuWidth,
		menuHeight,
		gtx.Dp(userStatusMenuDrop),
		gtx.Dp(unit.Dp(8)),
		image.Rectangle{Max: size},
	)

	// The scrim goes first so the options registered after it stay on top and
	// receive their own clicks; anything else closes the menu.
	if layoutOverlayScrim(gtx, image.Rectangle{Max: size}, &m.scrim) {
		m.close()
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	drawPopupMenu(gtx, menu)

	for index, status := range options {
		rect := image.Rect(
			menu.Min.X+pad,
			menu.Min.Y+pad+index*(itemHeight+gap),
			menu.Min.X+pad+itemWidth,
			menu.Min.Y+pad+index*(itemHeight+gap)+itemHeight,
		)
		click := &m.clicks[index]
		hover := m.hovers[index].update(gtx, click.Hovered())
		clicked := click.Clicked(gtx)

		stack := op.Offset(rect.Min).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(rect.Size())
		click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
			drawUserStatusMenuItem(gtx, typo, image.Rectangle{Max: rect.Size()}, status, hover)
			return layout.Dimensions{Size: rect.Size()}
		})
		stack.Pop()
		if clicked {
			m.close()
			if onSelect != nil {
				onSelect(status)
			}
			gtx.Execute(op.InvalidateCmd{})
			return
		}
	}
}
