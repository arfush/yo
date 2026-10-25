package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
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
	{filter: filterCompleted, status: library.UserStatusCompleted, label: "Досмотрел"},
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
		return "Досмотрел"
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

// userStatusPillWidth is shared with the release-status pills, so the two tags
// in a history row or on the title page always have exactly the same size.
func userStatusPillWidth(gtx layout.Context, typo *typography) int {
	return statusPillWidth(gtx, typo)
}

// filterPillWidth lets the history header shrink the pills when the window is
// narrower than the designed size. The label and padding must use the same
// metrics as drawFilterPill, otherwise the hit area would drift from the
// painted pill.
func filterPillWidth(gtx layout.Context, typo *typography, label string, size unit.Sp, padding unit.Dp) int {
	_, ink := typo.measure(gtx, label, font.Font{Typeface: fontFamily, Weight: font.Medium}, size)
	return 2*gtx.Dp(padding) + ink.Dx()
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

func drawFilterPill(gtx layout.Context, typo *typography, rect image.Rectangle, option userStatusFilterOption, selected bool, hover float32, size unit.Sp) {
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
	drawPill(gtx, typo, rect, option.label, size, textColor, fillColor, borderColor)
}

// drawUserStatusMenuItem keeps the menu quiet until the pointer reaches an
// option: the neutral gray is tinted with the status color, but never as bright
// as the already selected pill above the menu.
func drawUserStatusMenuItem(gtx layout.Context, typo *typography, rect image.Rectangle, status library.UserStatus, hover float32) {
	if rect.Empty() {
		return
	}
	textColor, fillColor, borderColor := userStatusColors(status)
	textColor = mixColor(userStatusMutedText, textColor, hover*0.50)
	fillColor = mixColor(userStatusMutedFill, fillColor, hover*0.35)
	borderColor = mixColor(userStatusMutedEdge, borderColor, hover*0.50)
	drawPill(gtx, typo, rect, userStatusLabel(status), mainStatusSize, textColor, fillColor, borderColor)
}

func drawPill(gtx layout.Context, typo *typography, rect image.Rectangle, label string, size unit.Sp, textColor, fillColor, borderColor color.NRGBA) {
	radius := gtx.Dp(unit.Dp(7))
	fillRounded(gtx, borderColor, rect, radius)
	fillRounded(gtx, fillColor, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	// Every pill is centered on the same cap-height band measured from one
	// reference glyph. Measuring each label's own ink makes labels with a
	// descender, an accent or a round overshoot sit half a pixel lower or
	// higher than their neighbours even though they share the baseline.
	capHeight := 0
	if _, refInk := typo.measure(gtx, "Н", face, size); !refInk.Empty() {
		capHeight = refInk.Dy()
	}
	typo.paintCenteredOnBaseline(
		gtx,
		label,
		face,
		size,
		textColor,
		f32.Pt(float32(rect.Min.X)+float32(rect.Dx())/2, float32(rect.Min.Y)+float32(rect.Dy())/2),
		capHeight,
	)
}

const (
	userStatusMenuPad  = unit.Dp(4)
	userStatusMenuGap  = unit.Dp(2)
	userStatusMenuDrop = unit.Dp(7)
)

// userStatusMenuOptionsFor keeps the menu order but removes the status already
// shown in the pill that opened it.
func userStatusMenuOptionsFor(current library.UserStatus) []library.UserStatus {
	current = library.NormalizeUserStatus(current)
	options := make([]library.UserStatus, 0, len(userStatusMenuOptions)-1)
	for _, status := range userStatusMenuOptions {
		if status != current {
			options = append(options, status)
		}
	}
	return options
}

// userStatusMenu is a small popup anchored to a status pill. It is drawn by the
// shell overlay so it can extend over rows and the footer without being clipped
// by the content viewport.
type userStatusMenu struct {
	open    bool
	anchor  image.Rectangle
	current library.UserStatus
	id      int64

	// pillWidth and pillHeight come from the status block that opened the menu,
	// so history and title-page menus match their own pills.
	pillWidth  int
	pillHeight int

	clicks [5]widget.Clickable
	hovers [5]hoverAnim
	scrim  widget.Clickable
}

func (m *userStatusMenu) show(anchor image.Rectangle, current library.UserStatus, id int64, pillWidth, pillHeight int) {
	m.open = true
	m.anchor = anchor
	m.current = library.NormalizeUserStatus(current)
	m.id = id
	m.pillWidth = max(pillWidth, 1)
	m.pillHeight = max(pillHeight, 1)
}

func (m *userStatusMenu) close() {
	m.open = false
}

func (m *userStatusMenu) layout(gtx layout.Context, typo *typography, origin image.Point, onSelect func(library.UserStatus)) {
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
	menu := placeUserStatusMenu(
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

	radius := gtx.Dp(unit.Dp(8))
	fillRounded(gtx, titlePanelBorder, menu, radius)
	fillRounded(gtx, settingsMenuFill, menu.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

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

// placeUserStatusMenu prefers to open below the pill and flips above it when
// the footer would clip the panel. Horizontally the panel is centered on the
// pill rather than aligned to its left edge.
func placeUserStatusMenu(anchor image.Rectangle, width, height, drop, margin int, bounds image.Rectangle) image.Rectangle {
	x := anchor.Min.X + (anchor.Dx()-width)/2
	y := anchor.Max.Y + drop
	if x+width > bounds.Max.X-margin {
		x = bounds.Max.X - margin - width
	}
	if x < bounds.Min.X+margin {
		x = bounds.Min.X + margin
	}
	if y+height > bounds.Max.Y-margin {
		y = anchor.Min.Y - drop - height
	}
	if y < bounds.Min.Y+margin {
		y = bounds.Min.Y + margin
	}
	return image.Rect(x, y, x+width, y+height)
}

// layoutOverlayScrim registers an invisible full-window click target. It is
// laid out before the menu options, which therefore sit above it.
func layoutOverlayScrim(gtx layout.Context, rect image.Rectangle, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	// Clicked must run before Layout: Layout drains and discards the click
	// events, so checking afterwards can never report a click.
	clicked := click.Clicked(gtx)
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}
