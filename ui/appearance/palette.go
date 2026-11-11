package appearance

import (
	"image/color"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/resource"
)

// Palette is the semantic color set of the application. Screens must use these
// tokens instead of raw colors.
type Palette struct {
	// Surfaces.
	Canvas     color.NRGBA // window background
	Surface    color.NRGBA // header and footer
	SurfaceAlt color.NRGBA // controls and cells
	Panel      color.NRGBA // panels and dialogs
	PanelDim   color.NRGBA // panel at 78% opacity
	Tile       color.NRGBA // icon tile
	Scrim      color.NRGBA // 25% dim

	// Lines.
	Border       color.NRGBA // base border
	BorderStrong color.NRGBA // strong border
	TileBorder   color.NRGBA // icon tile border
	Divider      color.NRGBA // hairline; same as Panel

	// Text.
	Text      color.NRGBA // primary text
	TextMuted color.NRGBA // secondary text
	TextFaint color.NRGBA // tertiary text
	White     color.NRGBA // contrast white

	// Icons and controls.
	AppIcon          color.NRGBA // application mark
	ActionIcon       color.NRGBA // action icon
	DeleteIcon       color.NRGBA // delete icon
	DisabledIcon     color.NRGBA // disabled icon
	Button           color.NRGBA // icon-button background (all buttons)
	ButtonBorder     color.NRGBA // icon-button border
	ControlIcon      color.NRGBA // window control icon
	ControlIconHover color.NRGBA // hovered window control icon

	// Site accent.
	Accent      color.NRGBA // site accent
	AccentMuted color.NRGBA // same as Accent
	Ink         color.NRGBA // accent text and icons
	Badge       color.NRGBA // accent background
	Selection   color.NRGBA // selected row background
	Edge        color.NRGBA // accent border
	OnAccent    color.NRGBA // text on accent
	Focus       color.NRGBA // same as Accent

	ProgressBar color.NRGBA // neutral progress track

	// Statuses.
	Danger   color.NRGBA // dangerous action
	Warning  color.NRGBA // reserved; not in the current design
	Info     color.NRGBA // announcement
	InfoCard color.NRGBA // announcement card
	Online   color.NRGBA // online
	Success  color.NRGBA // completed

	Watching StatusColors   // watching
	Planned  StatusColors   // planned
	Favorite StatusColors   // favorite
	Dropped  StatusColors   // dropped
	Provider ProviderColors // provider tag

	Mascot MascotPalette
}

// StatusColors is one status token with text, background and border.
type StatusColors struct {
	Text       color.NRGBA
	Background color.NRGBA
	Border     color.NRGBA
}

// ProviderColors is the provider tag token.
type ProviderColors struct {
	Text       color.NRGBA
	Background color.NRGBA
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
	appIcon   resource.AppIconKey
	accent    color.NRGBA
	ink       color.NRGBA
	badge     color.NRGBA
	selection color.NRGBA
	edge      color.NRGBA
	onAccent  color.NRGBA
}

// siteAccents maps every catalog site to the colors that carry its identity.
// The map is explicit so adding a site without a palette is a startup error.
var siteAccents = map[catalog.Site]accents{
	catalog.SiteYoanime: {
		appIcon:   resource.AppIconYoanime,
		accent:    color.NRGBA{R: 0x78, G: 0x50, B: 0xd5, A: 0xff},
		ink:       color.NRGBA{R: 0x95, G: 0x6a, B: 0xf5, A: 0xff},
		badge:     color.NRGBA{R: 0x2d, G: 0x24, B: 0x41, A: 0xff},
		selection: color.NRGBA{R: 0x1d, G: 0x1b, B: 0x29, A: 0xff},
		edge:      color.NRGBA{R: 0x42, G: 0x34, B: 0x54, A: 0xff},
		onAccent:  color.NRGBA{R: 0xee, G: 0xed, B: 0xf4, A: 0xff},
	},
	catalog.SiteYomanga: {
		appIcon:   resource.AppIconYomanga,
		accent:    color.NRGBA{R: 0xe8, G: 0xa2, B: 0x66, A: 0xff},
		ink:       color.NRGBA{R: 0xe8, G: 0xa2, B: 0x66, A: 0xff},
		badge:     color.NRGBA{R: 0x30, G: 0x22, B: 0x18, A: 0xff},
		selection: color.NRGBA{R: 0x30, G: 0x22, B: 0x18, A: 0xff},
		edge:      color.NRGBA{R: 0x66, G: 0x43, B: 0x29, A: 0xff},
		onAccent:  color.NRGBA{R: 0x11, G: 0x12, B: 0x16, A: 0xff},
	},
	catalog.SiteYoranobe: {
		appIcon:   resource.AppIconYoranobe,
		accent:    color.NRGBA{R: 0x8a, G: 0xb4, B: 0xf8, A: 0xff},
		ink:       color.NRGBA{R: 0x8a, G: 0xb4, B: 0xf8, A: 0xff},
		badge:     color.NRGBA{R: 0x1b, G: 0x27, B: 0x3d, A: 0xff},
		selection: color.NRGBA{R: 0x1b, G: 0x27, B: 0x3d, A: 0xff},
		edge:      color.NRGBA{R: 0x3a, G: 0x52, B: 0x7a, A: 0xff},
		onAccent:  color.NRGBA{R: 0x11, G: 0x12, B: 0x16, A: 0xff},
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
	p.AccentMuted = a.accent
	p.Ink = a.ink
	p.Badge = a.badge
	p.Selection = a.selection
	p.Edge = a.edge
	p.OnAccent = a.onAccent
	p.Focus = a.accent
	p.ControlIconHover = mix(a.accent, p.Text, 0.55)
	p.Mascot.Normal = mascotColors(p.Accent, p.Ink, p.Text, p.Canvas)
	return p
}

// AppIconFor returns the application icon that identifies site. A zero or
// unknown site is a programmer error and panics instead of silently keeping the
// neutral icon.
func AppIconFor(site catalog.Site) resource.AppIconKey {
	a, ok := siteAccents[site]
	if !ok {
		panic("appearance: unknown site \"" + string(site) + "\"")
	}
	return a.appIcon
}

// BasePalette returns the neutral yobase palette. The shell falls back to it
// when the site switcher is dismissed without a selection; PaletteFor extends
// the same colors with a site accent.
func BasePalette() Palette { return newPalette() }

// newPalette builds the neutral palette that every site palette extends.
func newPalette() Palette {
	p := Palette{
		Canvas:     color.NRGBA{R: 0x11, G: 0x12, B: 0x16, A: 0xff},
		Surface:    color.NRGBA{R: 0x17, G: 0x18, B: 0x1e, A: 0xff},
		SurfaceAlt: color.NRGBA{R: 0x22, G: 0x23, B: 0x2c, A: 0xff},
		Panel:      color.NRGBA{R: 0x1b, G: 0x1c, B: 0x23, A: 0xff},
		PanelDim:   color.NRGBA{R: 0x1b, G: 0x1c, B: 0x23, A: 0xc7},
		Tile:       color.NRGBA{R: 0x14, G: 0x13, B: 0x19, A: 0xff},
		Scrim:      color.NRGBA{A: 0x40},

		Border:       color.NRGBA{R: 0x30, G: 0x31, B: 0x3c, A: 0xff},
		BorderStrong: color.NRGBA{R: 0x40, G: 0x3d, B: 0x4d, A: 0xff},
		TileBorder:   color.NRGBA{R: 0x37, G: 0x34, B: 0x3f, A: 0xff},
		Divider:      color.NRGBA{R: 0x1b, G: 0x1c, B: 0x23, A: 0xff},

		Text:      color.NRGBA{R: 0xee, G: 0xed, B: 0xf4, A: 0xff},
		TextMuted: color.NRGBA{R: 0x9d, G: 0x9b, B: 0xaa, A: 0xff},
		TextFaint: color.NRGBA{R: 0x73, G: 0x71, B: 0x7f, A: 0xff},
		White:     color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},

		AppIcon:      color.NRGBA{R: 0xb5, G: 0xae, B: 0xc4, A: 0xff},
		ActionIcon:   color.NRGBA{R: 0xab, G: 0xae, B: 0xb9, A: 0xff},
		DeleteIcon:   color.NRGBA{R: 0xc3, G: 0xc5, B: 0xce, A: 0xff},
		DisabledIcon: color.NRGBA{R: 0x68, G: 0x6a, B: 0x75, A: 0xff},
		Button:       color.NRGBA{R: 0x26, G: 0x27, B: 0x2d, A: 0xff},
		ButtonBorder: color.NRGBA{R: 0x39, G: 0x3a, B: 0x42, A: 0xff},
		ControlIcon:  color.NRGBA{R: 0x9d, G: 0x9b, B: 0xaa, A: 0xff},

		Accent:      color.NRGBA{R: 0x9d, G: 0x9b, B: 0xaa, A: 0xff},
		AccentMuted: color.NRGBA{R: 0x9d, G: 0x9b, B: 0xaa, A: 0xff},
		Ink:         color.NRGBA{R: 0xee, G: 0xed, B: 0xf4, A: 0xff},
		Badge:       color.NRGBA{R: 0x22, G: 0x23, B: 0x2c, A: 0xff},
		Selection:   color.NRGBA{R: 0x22, G: 0x23, B: 0x2c, A: 0xff},
		Edge:        color.NRGBA{R: 0x40, G: 0x3d, B: 0x4d, A: 0xff},
		OnAccent:    color.NRGBA{R: 0x11, G: 0x12, B: 0x16, A: 0xff},
		Focus:       color.NRGBA{R: 0x9d, G: 0x9b, B: 0xaa, A: 0xff},

		ProgressBar: color.NRGBA{R: 0x34, G: 0x30, B: 0x3f, A: 0xff},

		Danger:   color.NRGBA{R: 0xdf, G: 0x92, B: 0x9b, A: 0xff},
		Warning:  color.NRGBA{R: 0xf2, G: 0xc9, B: 0x4c, A: 0xff},
		Info:     color.NRGBA{R: 0x80, G: 0xc4, B: 0xd3, A: 0xff},
		InfoCard: color.NRGBA{R: 0x5c, G: 0x96, B: 0xf5, A: 0xff},
		Online:   color.NRGBA{R: 0x78, G: 0xc7, B: 0xa2, A: 0xff},
		Success:  color.NRGBA{R: 0x78, G: 0xc8, B: 0xa0, A: 0xff},

		Watching: StatusColors{
			Text:       color.NRGBA{R: 0xf2, G: 0xbe, B: 0x87, A: 0xff},
			Background: color.NRGBA{R: 0x34, G: 0x29, B: 0x1f, A: 0xff},
			Border:     color.NRGBA{R: 0x66, G: 0x50, B: 0x3b, A: 0xff},
		},
		Planned: StatusColors{
			Text:       color.NRGBA{R: 0x8a, G: 0xbc, B: 0xf0, A: 0xff},
			Background: color.NRGBA{R: 0x1d, G: 0x2a, B: 0x3c, A: 0xff},
			Border:     color.NRGBA{R: 0x30, G: 0x46, B: 0x5e, A: 0xff},
		},
		Favorite: StatusColors{
			Text:       color.NRGBA{R: 0xf1, G: 0xa2, B: 0xad, A: 0xff},
			Background: color.NRGBA{R: 0x36, G: 0x23, B: 0x29, A: 0xff},
			Border:     color.NRGBA{R: 0x69, G: 0x41, B: 0x4b, A: 0xff},
		},
		Dropped: StatusColors{
			Text:       color.NRGBA{R: 0xd8, G: 0x8a, B: 0xa8, A: 0xff},
			Background: color.NRGBA{R: 0x34, G: 0x1b, B: 0x29, A: 0xff},
			Border:     color.NRGBA{R: 0x71, G: 0x34, B: 0x4e, A: 0xff},
		},
		Provider: ProviderColors{
			Text:       color.NRGBA{R: 0xd4, G: 0xb6, B: 0x7d, A: 0xff},
			Background: color.NRGBA{R: 0x34, G: 0x2c, B: 0x1c, A: 0xff},
		},

		Mascot: MascotPalette{
			Eye:          color.NRGBA{R: 0x11, G: 0x12, B: 0x16, A: 0xff},
			EyeHighlight: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			Glitch: MascotGlitch{
				Cyan:    color.NRGBA{R: 0x7d, G: 0xf5, B: 0xff, A: 0x55},
				Magenta: color.NRGBA{R: 0xff, G: 0x6f, B: 0xd0, A: 0x55},
			},
		},
	}
	p.ControlIconHover = mix(p.Accent, p.Text, 0.55)
	p.Mascot.Normal = mascotColors(p.Accent, p.Ink, p.Text, p.Canvas)
	p.Mascot.Alert = mascotColors(
		p.Danger,
		mix(p.Danger, p.White, 0.5),
		mix(p.White, p.Danger, 0.12),
		p.Canvas,
	)
	return p
}

// mascotColors derives one mascot mood from semantic palette colors.
func mascotColors(accent, ink, face, canvas color.NRGBA) MascotColors {
	return MascotColors{
		Hair:       accent,
		HairShadow: mix(accent, canvas, 0.55),
		HairLight:  ink,
		Face:       face,
		EarInner:   mix(accent, face, 0.65),
		Paw:        mix(accent, face, 0.8),
		Blush:      color.NRGBA{R: 0xf0, G: 0xa8, B: 0xcf, A: 0xa0},
		Dress:      accent,
		DressLight: ink,
	}
}

// mix linearly interpolates between a and b without leaving the uint8 range.
func mix(a, b color.NRGBA, t float32) color.NRGBA {
	lerp := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return color.NRGBA{
		R: lerp(a.R, b.R),
		G: lerp(a.G, b.G),
		B: lerp(a.B, b.B),
		A: lerp(a.A, b.A),
	}
}
