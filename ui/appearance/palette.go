package appearance

import (
	"image/color"

	"github.com/arfush/yo/catalog"
)

// Palette is the semantic color set of the application. Screens must use these
// tokens instead of raw colors.
type Palette struct {
	Canvas      color.NRGBA
	Surface     color.NRGBA
	SurfaceAlt  color.NRGBA
	Border      color.NRGBA
	Divider     color.NRGBA
	Text        color.NRGBA
	TextMuted   color.NRGBA
	OnAccent    color.NRGBA
	Accent      color.NRGBA
	AccentMuted color.NRGBA
	Danger      color.NRGBA
	Warning     color.NRGBA
	Success     color.NRGBA
	Info        color.NRGBA
	Focus       color.NRGBA
	Overlay     color.NRGBA
	Selection   color.NRGBA

	Mascot MascotPalette
}

// MascotPalette is the complete color set the future ui/kit/mascot needs.
type MascotPalette struct {
	Normal       MascotColors
	Alert        MascotColors
	Eye          color.NRGBA
	EyeHighlight color.NRGBA
	Glitch       MascotGlitch
}

// MascotColors is one mascot mood.
type MascotColors struct {
	Hair       color.NRGBA
	HairShadow color.NRGBA
	HairLight  color.NRGBA
	Face       color.NRGBA
	EarInner   color.NRGBA
	Paw        color.NRGBA
	Blush      color.NRGBA
	Dress      color.NRGBA
	DressLight color.NRGBA
}

// MascotGlitch holds the two glitch overlay colors.
type MascotGlitch struct {
	Cyan    color.NRGBA
	Magenta color.NRGBA
}

// accents is the part of a palette a site overrides on top of the neutral base.
type accents struct {
	accent      color.NRGBA
	accentMuted color.NRGBA
	focus       color.NRGBA
	selection   color.NRGBA
	mascot      MascotColors
}

// siteAccents maps every catalog site to the colors that carry its identity.
// The map is explicit so adding a site without a palette is a startup error.
var siteAccents = map[catalog.Site]accents{
	catalog.SiteYoanime: {
		accent:      color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff},
		accentMuted: color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff},
		focus:       color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff},
		selection:   color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0x66},
		mascot: MascotColors{
			Hair:       color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff},
			HairShadow: color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff},
			HairLight:  color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff},
			Face:       color.NRGBA{R: 0xe7, G: 0xe0, B: 0xf9, A: 0xff},
			EarInner:   color.NRGBA{R: 0xd2, G: 0xc2, B: 0xf8, A: 0xff},
			Paw:        color.NRGBA{R: 0xe1, G: 0xd7, B: 0xf9, A: 0xff},
			Blush:      color.NRGBA{R: 0xf0, G: 0xa8, B: 0xcf, A: 0xa0},
			Dress:      color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff},
			DressLight: color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff},
		},
	},
	catalog.SiteYomanga: {
		accent:      color.NRGBA{R: 0xf0, G: 0x8a, B: 0x2c, A: 0xff},
		accentMuted: color.NRGBA{R: 0xd4, G: 0x5f, B: 0x12, A: 0xff},
		focus:       color.NRGBA{R: 0xf0, G: 0x8a, B: 0x2c, A: 0xff},
		selection:   color.NRGBA{R: 0xd4, G: 0x5f, B: 0x12, A: 0x66},
		mascot: MascotColors{
			Hair:       color.NRGBA{R: 0xf0, G: 0x8a, B: 0x2c, A: 0xff},
			HairShadow: color.NRGBA{R: 0xd4, G: 0x5f, B: 0x12, A: 0xff},
			HairLight:  color.NRGBA{R: 0xf5, G: 0xa4, B: 0x5a, A: 0xff},
			Face:       color.NRGBA{R: 0xf3, G: 0xe5, B: 0xe1, A: 0xff},
			EarInner:   color.NRGBA{R: 0xf2, G: 0xd0, B: 0xb8, A: 0xff},
			Paw:        color.NRGBA{R: 0xf3, G: 0xdf, B: 0xd4, A: 0xff},
			Blush:      color.NRGBA{R: 0xf0, G: 0xa8, B: 0xcf, A: 0xa0},
			Dress:      color.NRGBA{R: 0xd4, G: 0x5f, B: 0x12, A: 0xff},
			DressLight: color.NRGBA{R: 0xf5, G: 0xa4, B: 0x5a, A: 0xff},
		},
	},
	catalog.SiteYoranobe: {
		accent:      color.NRGBA{R: 0x4c, G: 0x8d, B: 0xf6, A: 0xff},
		accentMuted: color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
		focus:       color.NRGBA{R: 0x4c, G: 0x8d, B: 0xf6, A: 0xff},
		selection:   color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0x66},
		mascot: MascotColors{
			Hair:       color.NRGBA{R: 0x4c, G: 0x8d, B: 0xf6, A: 0xff},
			HairShadow: color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
			HairLight:  color.NRGBA{R: 0x7b, G: 0xa6, B: 0xf9, A: 0xff},
			Face:       color.NRGBA{R: 0xdf, G: 0xe5, B: 0xf9, A: 0xff},
			EarInner:   color.NRGBA{R: 0xbe, G: 0xd1, B: 0xf8, A: 0xff},
			Paw:        color.NRGBA{R: 0xd5, G: 0xdf, B: 0xf9, A: 0xff},
			Blush:      color.NRGBA{R: 0xf0, G: 0xa8, B: 0xcf, A: 0xa0},
			Dress:      color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
			DressLight: color.NRGBA{R: 0x7b, G: 0xa6, B: 0xf9, A: 0xff},
		},
	},
}

// PaletteFor returns the palette of site. A zero or unknown site is a
// programmer error and panics instead of silently keeping the neutral base.
func PaletteFor(site catalog.Site) Palette {
	a, ok := siteAccents[site]
	if !ok {
		panic("appearance: unknown site \"" + string(site) + "\"")
	}
	p := newPalette()
	p.Accent = a.accent
	p.AccentMuted = a.accentMuted
	p.Focus = a.focus
	p.Selection = a.selection
	p.Mascot.Normal = a.mascot
	return p
}

// newPalette builds the neutral palette that every site palette extends.
func newPalette() Palette {
	return Palette{
		Canvas:      color.NRGBA{R: 0x0d, G: 0x0e, B: 0x14, A: 0xff},
		Surface:     color.NRGBA{R: 0x11, G: 0x12, B: 0x19, A: 0xff},
		SurfaceAlt:  color.NRGBA{R: 0x20, G: 0x1f, B: 0x28, A: 0xff},
		Border:      color.NRGBA{R: 0x20, G: 0x21, B: 0x27, A: 0xff},
		Divider:     color.NRGBA{R: 0x1b, G: 0x1c, B: 0x23, A: 0xff},
		Text:        color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff},
		TextMuted:   color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff},
		OnAccent:    color.NRGBA{R: 0x20, G: 0x1b, B: 0x2c, A: 0xff},
		Accent:      color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff},
		AccentMuted: color.NRGBA{R: 0x6b, G: 0x65, B: 0x7a, A: 0xff},
		Danger:      color.NRGBA{R: 0xf2, G: 0x7d, B: 0x8b, A: 0xff},
		Warning:     color.NRGBA{R: 0xf2, G: 0xc9, B: 0x4c, A: 0xff},
		Success:     color.NRGBA{R: 0x65, G: 0xd6, B: 0xa3, A: 0xff},
		Info:        color.NRGBA{R: 0xc9, G: 0xc4, B: 0xd6, A: 0xff},
		Focus:       color.NRGBA{R: 0xc6, G: 0xbd, B: 0xd9, A: 0xff},
		Overlay:     color.NRGBA{R: 0x0d, G: 0x0e, B: 0x14, A: 0xb8},
		Selection:   color.NRGBA{R: 0x6b, G: 0x65, B: 0x7a, A: 0x66},

		Mascot: MascotPalette{
			Normal: MascotColors{
				Hair:       color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff},
				HairShadow: color.NRGBA{R: 0x6b, G: 0x65, B: 0x7a, A: 0xff},
				HairLight:  color.NRGBA{R: 0xc9, G: 0xc4, B: 0xd6, A: 0xff},
				Face:       color.NRGBA{R: 0xeb, G: 0xe8, B: 0xf2, A: 0xff},
				EarInner:   color.NRGBA{R: 0xdc, G: 0xd8, B: 0xe5, A: 0xff},
				Paw:        color.NRGBA{R: 0xe6, G: 0xe3, B: 0xee, A: 0xff},
				Blush:      color.NRGBA{R: 0xf0, G: 0xa8, B: 0xcf, A: 0xa0},
				Dress:      color.NRGBA{R: 0x6b, G: 0x65, B: 0x7a, A: 0xff},
				DressLight: color.NRGBA{R: 0xc9, G: 0xc4, B: 0xd6, A: 0xff},
			},
			Alert: MascotColors{
				Hair:       color.NRGBA{R: 0xe9, G: 0x3b, B: 0x50, A: 0xff},
				HairShadow: color.NRGBA{R: 0xa8, G: 0x22, B: 0x34, A: 0xff},
				HairLight:  color.NRGBA{R: 0xff, G: 0xa6, B: 0xb0, A: 0xff},
				Face:       color.NRGBA{R: 0xff, G: 0xe4, B: 0xe7, A: 0xff},
				EarInner:   color.NRGBA{R: 0xff, G: 0xc4, B: 0xcb, A: 0xff},
				Paw:        color.NRGBA{R: 0xff, G: 0xd6, B: 0xdb, A: 0xff},
				Blush:      color.NRGBA{R: 0xff, G: 0x8a, B: 0x9c, A: 0xa0},
				Dress:      color.NRGBA{R: 0xc6, G: 0x2c, B: 0x3e, A: 0xff},
				DressLight: color.NRGBA{R: 0xff, G: 0x97, B: 0xa3, A: 0xff},
			},
			Eye:          color.NRGBA{R: 0x2a, G: 0x1e, B: 0x46, A: 0xff},
			EyeHighlight: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			Glitch: MascotGlitch{
				Cyan:    color.NRGBA{R: 0x7d, G: 0xf5, B: 0xff, A: 0x55},
				Magenta: color.NRGBA{R: 0xff, G: 0x6f, B: 0xd0, A: 0x55},
			},
		},
	}
}
