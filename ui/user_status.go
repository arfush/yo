package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yo/catalog"
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
	status catalog.UserStatus
	label  string
}

// The order matches the filter row on the main screen. "Все" has no library
// status and is drawn with the purple accent.
var userStatusFilterOptions = []userStatusFilterOption{
	{filter: filterAll, label: "Все"},
	{filter: filterWatching, status: catalog.UserStatusWatching, label: "Смотрю"},
	{filter: filterPlanned, status: catalog.UserStatusPlanned, label: "В планах"},
	{filter: filterFavorite, status: catalog.UserStatusFavorite, label: "Обожаю"},
	{filter: filterCompleted, status: catalog.UserStatusCompleted, label: "Досмотрел"},
	{filter: filterDropped, status: catalog.UserStatusDropped, label: "Брошено"},
}

// userStatusMenuOptions is the order shown when a status pill is clicked.
var userStatusMenuOptions = []catalog.UserStatus{
	catalog.UserStatusWatching,
	catalog.UserStatusPlanned,
	catalog.UserStatusFavorite,
	catalog.UserStatusCompleted,
	catalog.UserStatusDropped,
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

func userStatusLabel(status catalog.UserStatus) string {
	switch catalog.NormalizeUserStatus(status) {
	case catalog.UserStatusWatching:
		return "Смотрю"
	case catalog.UserStatusFavorite:
		return "Обожаю"
	case catalog.UserStatusCompleted:
		return "Досмотрел"
	case catalog.UserStatusDropped:
		return "Брошено"
	default:
		return "В планах"
	}
}

func userStatusColors(status catalog.UserStatus) (text, fill, border color.NRGBA) {
	switch catalog.NormalizeUserStatus(status) {
	case catalog.UserStatusWatching:
		return userStatusWatchingText, userStatusWatchingFill, userStatusWatchingEdge
	case catalog.UserStatusFavorite:
		return userStatusFavoriteText, userStatusFavoriteFill, userStatusFavoriteEdge
	case catalog.UserStatusCompleted:
		return userStatusCompletedText, userStatusCompletedFill, userStatusCompletedEdge
	case catalog.UserStatusDropped:
		return userStatusDroppedText, userStatusDroppedFill, userStatusDroppedEdge
	default:
		return userStatusPlannedText, userStatusPlannedFill, userStatusPlannedEdge
	}
}

func (f userStatusFilter) matches(status catalog.UserStatus) bool {
	if f == filterAll {
		return true
	}
	for _, option := range userStatusFilterOptions {
		if option.filter == f {
			return catalog.NormalizeUserStatus(status) == option.status
		}
	}
	return true
}

// userStatus returns the library status represented by a filter. The "Все"
// filter and unknown values have no status of their own.
func (f userStatusFilter) userStatus() (catalog.UserStatus, bool) {
	if f == filterAll {
		return 0, false
	}
	for _, option := range userStatusFilterOptions {
		if option.filter == f {
			return option.status, true
		}
	}
	return 0, false
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
func drawUserStatusPill(gtx layout.Context, typo *typography, rect image.Rectangle, status catalog.UserStatus, colored bool) {
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
func drawUserStatusMenuItem(gtx layout.Context, typo *typography, rect image.Rectangle, status catalog.UserStatus, hover float32) {
	if rect.Empty() {
		return
	}
	textColor, fillColor, borderColor := userStatusColors(status)
	textColor = mixColor(userStatusMutedText, textColor, hover*0.50)
	fillColor = mixColor(userStatusMutedFill, fillColor, hover*0.35)
	borderColor = mixColor(userStatusMutedEdge, borderColor, hover*0.50)
	drawPill(gtx, typo, rect, userStatusLabel(status), mainStatusSize, textColor, fillColor, borderColor)
}

const (
	userStatusMenuPad  = unit.Dp(4)
	userStatusMenuGap  = unit.Dp(2)
	userStatusMenuDrop = unit.Dp(7)
)

// userStatusMenuOptionsFor keeps the menu order but removes the status already
// shown in the pill that opened it.
func userStatusMenuOptionsFor(current catalog.UserStatus) []catalog.UserStatus {
	current = catalog.NormalizeUserStatus(current)
	options := make([]catalog.UserStatus, 0, len(userStatusMenuOptions)-1)
	for _, status := range userStatusMenuOptions {
		if status != current {
			options = append(options, status)
		}
	}
	return options
}
