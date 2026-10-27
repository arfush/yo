package ui

import (
	"gioui.org/layout"

	"github.com/arfush/yoanime/ui/theme"
)

// These aliases preserve the short names used throughout the screen code while
// the actual palette and hover state live in ui/theme.
const (
	fontFamily        = theme.FontFamily
	hoverPollInterval = theme.HoverPollInterval
)

var (
	contentColor = theme.Content
	chromeColor  = theme.Chrome
	borderColor  = theme.Border
	dividerColor = theme.Divider

	iconColor = theme.Icon

	titleColor = theme.Title

	badgeFillColor   = theme.BadgeFill
	badgeBorderColor = theme.BadgeBorder
	badgeTextColor   = theme.BadgeText

	controlHoverColor = theme.ControlHover
	closeHoverColor   = theme.CloseHover

	footerControlFillColor      = theme.FooterControlFill
	footerControlBorderColor    = theme.FooterControlBorder
	footerControlHoverFill      = theme.FooterControlHoverFill
	footerControlHoverBorder    = theme.FooterControlHoverBorder
	footerControlIconColor      = theme.FooterControlIcon
	footerControlHoverIconColor = theme.FooterControlHoverIcon

	footerPartyActiveFill   = theme.FooterPartyActiveFill
	footerPartyActiveBorder = theme.FooterPartyActiveBorder
	footerPartyActiveIcon   = theme.FooterPartyActiveIcon

	footerLogAlertFill   = theme.FooterLogAlertFill
	footerLogAlertBorder = theme.FooterLogAlertBorder
	footerLogAlertIcon   = theme.FooterLogAlertIcon
)

// hoverAnim is the root-package name for the shared hover animation.
type hoverAnim struct {
	theme.Hover
}

func (a *hoverAnim) update(gtx layout.Context, hovered bool) float32 {
	return a.Hover.Update(gtx, hovered)
}
