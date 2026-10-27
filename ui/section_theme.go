package ui

import "image/color"

// sectionPalette is the small set of colors that defines a library section.
// Everything else in the accent UI is derived from these values, so adding a
// section cannot leave a control in the previous section's color.
type sectionPalette struct {
	accent1 color.NRGBA
	accent2 color.NRGBA
	badge   color.NRGBA
	edge    color.NRGBA
	ink     color.NRGBA
}

// palette returns the accent colors for a section. The anime palette is the
// original violet theme; manga and ranobe reuse the same contrast relationships
// with warm orange and cool blue accents.
func (s librarySection) palette() sectionPalette {
	switch s {
	case sectionManga:
		return sectionPalette{
			accent1: color.NRGBA{R: 0xf0, G: 0x8a, B: 0x2c, A: 0xff},
			accent2: color.NRGBA{R: 0xd4, G: 0x5f, B: 0x12, A: 0xff},
			badge:   color.NRGBA{R: 0x38, G: 0x21, B: 0x17, A: 0xff},
			edge:    color.NRGBA{R: 0x5c, G: 0x36, B: 0x1f, A: 0xff},
			ink:     color.NRGBA{R: 0xf5, G: 0xa4, B: 0x5a, A: 0xff},
		}
	case sectionRanobe:
		return sectionPalette{
			accent1: color.NRGBA{R: 0x4c, G: 0x8d, B: 0xf6, A: 0xff},
			accent2: color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
			badge:   color.NRGBA{R: 0x17, G: 0x22, B: 0x3b, A: 0xff},
			edge:    color.NRGBA{R: 0x1e, G: 0x32, B: 0x63, A: 0xff},
			ink:     color.NRGBA{R: 0x7b, G: 0xa6, B: 0xf9, A: 0xff},
		}
	default:
		return sectionPalette{
			accent1: color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff},
			accent2: color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff},
			badge:   color.NRGBA{R: 0x29, G: 0x22, B: 0x3e, A: 0xff},
			edge:    color.NRGBA{R: 0x39, G: 0x30, B: 0x4f, A: 0xff},
			ink:     color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff},
		}
	}
}

// applySectionTheme recolors every accent-driven control for the active
// section. The UI is immediate-mode and single-threaded, so the palette is
// stored in the same package variables the screens already read every frame.
func applySectionTheme(section librarySection) {
	p := section.palette()
	highlight := mixColor(p.accent1, titleColor, 0.55)

	mainAccentColor1 = p.accent1
	mainAccentColor2 = p.accent2
	mainSelectionColor = withAlpha(p.accent1, 0x66)
	mainInputFocusBorder = mixColor(chromeColor, p.accent2, 0.72)
	mainDisabledColor1 = mixColor(contentColor, p.accent2, 0.32)
	mainDisabledColor2 = mixColor(contentColor, p.accent2, 0.20)
	mainRowSelectedColor = mixColor(contentColor, p.accent1, 0.10)
	mainDragDotActive = p.ink
	mainStatusLiveText = p.ink
	mainStatusLiveFill = p.badge
	mainStatusLiveBorder = p.edge

	settingsToggleOnColor = p.accent1
	settingsMenuSelected = mixColor(contentColor, p.accent2, 0.26)
	readerChapterActive = mixColor(contentColor, p.accent2, 0.26)

	titleVoiceSelectedFill = mixColor(contentColor, p.accent2, 0.18)
	titleVoiceSelectedBorder = p.accent1
	titleVoiceHoverBorder = mixColor(contentColor, p.accent1, 0.26)
	titleButtonHover = mixColor(contentColor, p.accent1, 0.12)
	titleButtonHoverBorder = mixColor(contentColor, p.accent2, 0.38)

	logAccent = mixColor(p.accent1, titleColor, 0.35)
	undoActionColor = mixColor(p.accent1, titleColor, 0.28)
	undoActionHoverColor = mixColor(p.accent1, titleColor, 0.60)

	userStatusAllText = p.ink
	userStatusAllFill = p.badge
	userStatusAllEdge = p.edge

	badgeFillColor = p.badge
	badgeBorderColor = p.edge
	badgeTextColor = p.ink

	footerControlHoverFill = mixColor(chromeColor, p.accent2, 0.14)
	footerControlHoverBorder = mixColor(chromeColor, p.accent2, 0.45)
	footerControlHoverIconColor = highlight
	windowControlHoverIconColor = highlight
}
