// Package theme holds the visual tokens shared by the UI layers.
//
// Keeping the palette in one package makes it obvious which colors are part of
// the application chrome and lets generic widgets accept the same values
// without importing screen code.
package theme

import "image/color"

const FontFamily = "Inter"

var (
	Content = color.NRGBA{R: 0x0d, G: 0x0e, B: 0x14, A: 0xff}
	Chrome  = color.NRGBA{R: 0x11, G: 0x12, B: 0x19, A: 0xff}
	Border  = color.NRGBA{R: 0x20, G: 0x21, B: 0x27, A: 0xff}
	Divider = color.NRGBA{R: 0x1b, G: 0x1c, B: 0x23, A: 0xff}

	Icon = color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff}

	Title = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}

	BadgeFill   = color.NRGBA{R: 0x29, G: 0x22, B: 0x3e, A: 0xff}
	BadgeBorder = color.NRGBA{R: 0x39, G: 0x30, B: 0x4f, A: 0xff}
	BadgeText   = color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff}

	ControlHover = color.NRGBA{R: 0x9a, G: 0x9a, B: 0x9f, A: 0xff}
	CloseHover   = color.NRGBA{R: 0xff, G: 0x64, B: 0x73, A: 0xff}

	FooterControlFill        = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	FooterControlBorder      = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
	FooterControlHoverFill   = color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}
	FooterControlHoverBorder = color.NRGBA{R: 0x4a, G: 0x3c, B: 0x69, A: 0xff}
	FooterControlIcon        = color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff}
	FooterControlHoverIcon   = color.NRGBA{R: 0xd7, G: 0xc9, B: 0xff, A: 0xff}

	// A room that is still open keeps the watch-party control dim green, so the
	// main screen shows that a session exists.
	FooterPartyActiveFill   = color.NRGBA{R: 0x16, G: 0x27, B: 0x1e, A: 0xff}
	FooterPartyActiveBorder = color.NRGBA{R: 0x2c, G: 0x5a, B: 0x40, A: 0xff}
	FooterPartyActiveIcon   = color.NRGBA{R: 0x6c, G: 0xc8, B: 0x95, A: 0xff}

	// A fresh error tints the log button dull red for five seconds.
	FooterLogAlertFill   = color.NRGBA{R: 0x2a, G: 0x17, B: 0x1b, A: 0xff}
	FooterLogAlertBorder = color.NRGBA{R: 0x5a, G: 0x2a, B: 0x31, A: 0xff}
	FooterLogAlertIcon   = color.NRGBA{R: 0xd7, G: 0x7a, B: 0x80, A: 0xff}
)
