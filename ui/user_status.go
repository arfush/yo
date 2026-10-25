package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/library"
)

// userStatusFilter is the history filter. The zero value shows every title.
type userStatusFilter int

const (
	filterAll userStatusFilter = iota
	filterWatching
	filterPlanned
	filterFavorite
	filterCompleted
	filterDropped
)

type userStatusFilterOption struct {
	filter userStatusFilter
	status library.UserStatus
	label  string
}

// The order matches the filter row on the main screen. "Все" has no library
// status and is drawn with the purple accent.
var userStatusFilterOptions = []userStatusFilterOption{
	{filter: filterAll, label: "Все"},
	{filter: filterWatching, status: library.UserStatusWatching, label: "Смотрю"},
	{filter: filterPlanned, status: library.UserStatusPlanned, label: "В планах"},
	{filter: filterFavorite, status: library.UserStatusFavorite, label: "Обожаю"},
	{filter: filterCompleted, status: library.UserStatusCompleted, label: "Просмотрено"},
	{filter: filterDropped, status: library.UserStatusDropped, label: "Брошено"},
}

// userStatusMenuOptions is the order shown when a status pill is clicked.
var userStatusMenuOptions = []library.UserStatus{
	library.UserStatusWatching,
	library.UserStatusPlanned,
	library.UserStatusFavorite,
	library.UserStatusCompleted,
	library.UserStatusDropped,
}

var (
	userStatusWatchingText  = color.NRGBA{R: 0xf0, G: 0xb4, B: 0x6a, A: 0xff}
	userStatusWatchingFill  = color.NRGBA{R: 0x2c, G: 0x20, B: 0x14, A: 0xff}
	userStatusWatchingEdge  = color.NRGBA{R: 0x5a, G: 0x42, B: 0x25, A: 0xff}
	userStatusPlannedText   = color.NRGBA{R: 0x7f, G: 0xbf, B: 0xff, A: 0xff}
	userStatusPlannedFill   = color.NRGBA{R: 0x13, G: 0x24, B: 0x38, A: 0xff}
	userStatusPlannedEdge   = color.NRGBA{R: 0x29, G: 0x4b, B: 0x70, A: 0xff}
	userStatusFavoriteText  = color.NRGBA{R: 0xf2, G: 0xa0, B: 0xb0, A: 0xff}
	userStatusFavoriteFill  = color.NRGBA{R: 0x2d, G: 0x18, B: 0x20, A: 0xff}
	userStatusFavoriteEdge  = color.NRGBA{R: 0x5a, G: 0x2d, B: 0x3a, A: 0xff}
	userStatusCompletedText = color.NRGBA{R: 0x58, G: 0xdc, B: 0x9a, A: 0xff}
	userStatusCompletedFill = color.NRGBA{R: 0x10, G: 0x2c, B: 0x21, A: 0xff}
	userStatusCompletedEdge = color.NRGBA{R: 0x24, G: 0x55, B: 0x3e, A: 0xff}
	userStatusDroppedText   = color.NRGBA{R: 0xff, G: 0x64, B: 0x73, A: 0xff}
	userStatusDroppedFill   = color.NRGBA{R: 0x2e, G: 0x15, B: 0x19, A: 0xff}
	userStatusDroppedEdge   = color.NRGBA{R: 0x5c, G: 0x25, B: 0x2c, A: 0xff}

	userStatusAllText = color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff}
	userStatusAllFill = color.NRGBA{R: 0x1f, G: 0x18, B: 0x35, A: 0xff}
	userStatusAllEdge = color.NRGBA{R: 0x42, G: 0x32, B: 0x6e, A: 0xff}

	userStatusMutedText = color.NRGBA{R: 0x8f, G: 0x8a, B: 0x9e, A: 0xff}
	userStatusMutedFill = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	userStatusMutedEdge = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
)

func userStatusLabel(status library.UserStatus) string {
	switch library.NormalizeUserStatus(status) {
	case library.UserStatusWatching:
		return "Смотрю"
	case library.UserStatusFavorite:
		return "Обожаю"
	case library.UserStatusCompleted:
		return "Просмотрено"
	case library.UserStatusDropped:
		return "Брошено"
	default:
		return "В планах"
	}
}

func userStatusColors(status library.UserStatus) (text, fill, border color.NRGBA) {
	switch library.NormalizeUserStatus(status) {
	case library.UserStatusWatching:
		return userStatusWatchingText, userStatusWatchingFill, userStatusWatchingEdge
	case library.UserStatusFavorite:
		return userStatusFavoriteText, userStatusFavoriteFill, userStatusFavoriteEdge
	case library.UserStatusCompleted:
		return userStatusCompletedText, userStatusCompletedFill, userStatusCompletedEdge
	case library.UserStatusDropped:
		return userStatusDroppedText, userStatusDroppedFill, userStatusDroppedEdge
	default:
		return userStatusPlannedText, userStatusPlannedFill, userStatusPlannedEdge
	}
}

func (f userStatusFilter) matches(status library.UserStatus) bool {
	if f == filterAll {
		return true
	}
	for _, option := range userStatusFilterOptions {
		if option.filter == f {
			return library.NormalizeUserStatus(status) == option.status
		}
	}
	return true
}

func userStatusPillWidth(gtx layout.Context, typo *typography) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	width := 0
	for _, option := range userStatusFilterOptions {
		if option.filter == filterAll {
			continue
		}
		if _, ink := typo.measure(gtx, option.label, face, mainStatusSize); ink.Dx() > width {
			width = ink.Dx()
		}
	}
	return 2*gtx.Dp(mainStatusPaddingX) + width
}

func filterPillWidth(gtx layout.Context, typo *typography, label string) int {
	_, ink := typo.measure(gtx, label, font.Font{Typeface: fontFamily, Weight: font.Medium}, mainFilterSize)
	return 2*gtx.Dp(mainFilterPaddingX) + ink.Dx()
}

// drawUserStatusPill paints the same shape as the release-status pills. When
// colored is false the pill is the neutral gray used by an inactive filter.
func drawUserStatusPill(gtx layout.Context, typo *typography, rect image.Rectangle, status library.UserStatus, colored bool) {
	if rect.Empty() {
		return
	}
	textColor, fillColor, borderColor := userStatusColors(status)
	if !colored {
		textColor, fillColor, borderColor = userStatusMutedText, userStatusMutedFill, userStatusMutedEdge
	}
	drawPill(gtx, typo, rect, userStatusLabel(status), mainStatusSize, textColor, fillColor, borderColor)
}

func drawFilterPill(gtx layout.Context, typo *typography, rect image.Rectangle, option userStatusFilterOption, selected bool, hover float32) {
	if rect.Empty() {
		return
	}
	textColor := userStatusMutedText
	fillColor := userStatusMutedFill
	borderColor := userStatusMutedEdge
	if selected {
		if option.filter == filterAll {
			textColor, fillColor, borderColor = userStatusAllText, userStatusAllFill, userStatusAllEdge
		} else {
			textColor, fillColor, borderColor = userStatusColors(option.status)
		}
	}
	if hover > 0 {
		textColor = mixColor(textColor, color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}, hover*0.35)
		borderColor = mixColor(borderColor, color.NRGBA{R: 0x6b, G: 0x5a, B: 0x91, A: 0xff}, hover*0.55)
	}
	drawPill(gtx, typo, rect, option.label, mainFilterSize, textColor, fillColor, borderColor)
}

func drawPill(gtx layout.Context, typo *typography, rect image.Rectangle, label string, size unit.Sp, textColor, fillColor, borderColor color.NRGBA) {
	radius := gtx.Dp(unit.Dp(7))
	fillRounded(gtx, borderColor, rect, radius)
	fillRounded(gtx, fillColor, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := typo.measure(gtx, label, face, size)
	textY := rect.Min.Y + (rect.Dy()-ink.Dy())/2
	if label == "Завершён" || label == "Просмотрено" {
		textY += gtx.Dp(unit.Dp(1))
	}
	typo.draw(gtx, label, face, size, textColor, image.Pt(rect.Min.X+(rect.Dx()-ink.Dx())/2, textY))
}

// userStatusMenu is a small popup anchored to a status pill. It is drawn by the
// shell overlay so it can extend over rows and the footer without being clipped
// by the content viewport.
type userStatusMenu struct {
	open    bool
	anchor  image.Rectangle
	current library.UserStatus
	id      int64

	clicks  [5]widget.Clickable
	hovers  [5]hoverAnim
	dismiss widget.Clickable
}

func (m *userStatusMenu) show(anchor image.Rectangle, current library.UserStatus, id int64) {
	m.open = true
	m.anchor = anchor
	m.current = library.NormalizeUserStatus(current)
	m.id = id
}

func (m *userStatusMenu) close() {
	m.open = false
}

func (m *userStatusMenu) layout(gtx layout.Context, typo *typography, origin image.Point, onSelect func(library.UserStatus)) {
	if !m.open {
		return
	}

	// A click anywhere outside the menu dismisses it. The menu options are
	// drawn afterwards, so they sit above this full-window handler.
	gtx.Constraints = layout.Exact(gtx.Constraints.Max)
	_, dismissed := m.dismiss.Update(gtx)
	if dismissed {
		m.close()
		return
	}

	cardWidth := gtx.Dp(unit.Dp(176))
	itemHeight := gtx.Dp(unit.Dp(31))
	padding := gtx.Dp(unit.Dp(6))
	cardHeight := len(userStatusMenuOptions)*itemHeight + 2*padding
	margin := gtx.Dp(unit.Dp(8))
	x := m.anchor.Min.X
	y := m.anchor.Max.Y + gtx.Dp(unit.Dp(7))
	// The overlay is laid out in window coordinates but anchored in content
	// coordinates, so the free space to the right and below is reduced by the
	// content origin.
	maxX := gtx.Constraints.Max.X - origin.X - cardWidth - margin
	maxY := gtx.Constraints.Max.Y - origin.Y - cardHeight - margin
	if x > maxX {
		x = maxX
	}
	if x < margin {
		x = margin
	}
	if y > maxY {
		y = m.anchor.Min.Y - cardHeight - gtx.Dp(unit.Dp(7))
	}
	if y < margin {
		y = margin
	}
	card := image.Rect(x, y, x+cardWidth, y+cardHeight)

	// The anchor and the clamp above are content-local; move the card into
	// window coordinates before painting it.
	stack := op.Offset(origin).Push(gtx.Ops)
	defer stack.Pop()

	shadow := card.Add(image.Pt(0, gtx.Dp(unit.Dp(6))))
	fillRounded(gtx, color.NRGBA{A: 0x88}, shadow, gtx.Dp(unit.Dp(12)))
	fillRounded(gtx, userStatusMutedEdge, card, gtx.Dp(unit.Dp(12)))
	fillRounded(gtx, color.NRGBA{R: 0x17, G: 0x15, B: 0x1f, A: 0xff}, card.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(11)))

	for index, status := range userStatusMenuOptions {
		rect := image.Rect(
			card.Min.X+padding,
			card.Min.Y+padding+index*itemHeight,
			card.Max.X-padding,
			card.Min.Y+padding+(index+1)*itemHeight,
		)
		click := &m.clicks[index]
		hover := m.hovers[index].update(gtx, click.Hovered())
		if hover > 0 {
			fillRounded(gtx, mixColor(color.NRGBA{R: 0x17, G: 0x15, B: 0x1f, A: 0xff}, color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}, hover), rect, gtx.Dp(unit.Dp(8)))
		}
		pill := rect.Inset(gtx.Dp(unit.Dp(1)))
		drawUserStatusPill(gtx, typo, pill, status, true)
		if library.NormalizeUserStatus(m.current) == status {
			strokeRoundedRect(gtx, pill, float32(gtx.Dp(unit.Dp(7))), float32(gtx.Dp(unit.Dp(1))), color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0x66})
		}

		stack := op.Offset(rect.Min).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(rect.Size())
		_, clicked := click.Update(cgtx)
		stack.Pop()
		if clicked {
			m.close()
			if onSelect != nil {
				onSelect(status)
			}
			return
		}
	}
}
