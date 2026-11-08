package appearance

import "image/color"

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
	Hats         MascotHats
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

// MascotHats holds the hat colors used by the mascot.
type MascotHats struct {
	Pink  color.NRGBA
	White color.NRGBA
	Cream color.NRGBA
	Dark  color.NRGBA
	Gold  color.NRGBA
	Red   color.NRGBA
	Teal  color.NRGBA
	Sky   color.NRGBA
	Brown color.NRGBA
	Tan   color.NRGBA
	Green color.NRGBA
}

// MascotGlitch holds the two glitch overlay colors.
type MascotGlitch struct {
	Cyan    color.NRGBA
	Magenta color.NRGBA
}

// newPalette builds the neutral palette from the reference sources. Site
// palettes and the catalog do not exist yet, so this is the base look.
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
			Hats: MascotHats{
				Pink:  color.NRGBA{R: 0xf2, G: 0x8f, B: 0xc2, A: 0xff},
				White: color.NRGBA{R: 0xf7, G: 0xf3, B: 0xff, A: 0xff},
				Cream: color.NRGBA{R: 0xe9, G: 0xde, B: 0xc6, A: 0xff},
				Dark:  color.NRGBA{R: 0x3b, G: 0x2a, B: 0x63, A: 0xff},
				Gold:  color.NRGBA{R: 0xf5, G: 0xc2, B: 0x5c, A: 0xff},
				Red:   color.NRGBA{R: 0xd9, G: 0x4f, B: 0x6a, A: 0xff},
				Teal:  color.NRGBA{R: 0x5f, G: 0xc7, B: 0xc0, A: 0xff},
				Sky:   color.NRGBA{R: 0x6f, G: 0x9c, B: 0xf5, A: 0xff},
				Brown: color.NRGBA{R: 0x7a, G: 0x5a, B: 0x4a, A: 0xff},
				Tan:   color.NRGBA{R: 0xd8, G: 0xc3, B: 0xb0, A: 0xff},
				Green: color.NRGBA{R: 0x6f, G: 0xb0, B: 0x79, A: 0xff},
			},
			Glitch: MascotGlitch{
				Cyan:    color.NRGBA{R: 0x7d, G: 0xf5, B: 0xff, A: 0x55},
				Magenta: color.NRGBA{R: 0xff, G: 0x6f, B: 0xd0, A: 0x55},
			},
		},
	}
}
