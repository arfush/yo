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

	Title = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}

	BadgeFill   = color.NRGBA{R: 0x29, G: 0x22, B: 0x3e, A: 0xff}
	BadgeBorder = color.NRGBA{R: 0x39, G: 0x30, B: 0x4f, A: 0xff}
	BadgeText   = color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff}

	ControlHover = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}

	WindowControlIcon      = color.NRGBA{R: 0x78, G: 0x73, B: 0x8d, A: 0xff}
	WindowControlHoverIcon = color.NRGBA{R: 0xba, G: 0x9a, B: 0xff, A: 0xff}

	FooterControlFill        = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	FooterControlBorder      = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
	FooterControlHoverFill   = color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}
	FooterControlHoverBorder = color.NRGBA{R: 0x4a, G: 0x3c, B: 0x69, A: 0xff}
	FooterControlIcon        = color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff}
	FooterControlHoverIcon   = color.NRGBA{R: 0xd7, G: 0xc9, B: 0xff, A: 0xff}

	// An open watch-party room and a fresh error only recolour the icon: the
	// round button surface stays identical to every other footer control.
	FooterPartyActiveIcon = color.NRGBA{R: 0x6c, G: 0xc8, B: 0x95, A: 0xff}
	FooterLogAlertIcon    = color.NRGBA{R: 0xd7, G: 0x7a, B: 0x80, A: 0xff}
)
