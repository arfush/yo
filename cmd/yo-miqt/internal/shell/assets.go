package shell

import (
	"embed"
	"sync"

	qt "github.com/mappu/miqt/qt6"
)

//go:embed assets/font
var fontFS embed.FS

//go:embed assets/poster.png
var posterFS embed.FS

// Palette text sizes, in logical pixels, copied from appearance.Typography.
const (
	labelSizePx   = 13.2
	captionSizePx = 11
)

// fonts holds the Inter faces loaded into Qt. Qt selects the face by weight.
type fonts struct {
	family   string
	bold     *qt.QFont
	medium   *qt.QFont
	regular  *qt.QFont
	semibold *qt.QFont
	cache    map[fontKey]*qt.QFont
}

type fontKey struct {
	px     float64
	weight qt.QFont__Weight
}

var (
	fontsOnce sync.Once
	fontSet   *fonts

	posterOnce  sync.Once
	posterImage *qt.QImage
)

// loadFonts registers the four embedded Inter faces once and builds the faces
// the chrome uses. The label role is Inter Bold 13.2 and the caption role is
// Inter Medium 11.
func loadFonts() *fonts {
	fontsOnce.Do(func() {
		names := []string{
			"assets/font/Inter-Regular.ttf",
			"assets/font/Inter-Medium.ttf",
			"assets/font/Inter-SemiBold.ttf",
			"assets/font/Inter-Bold.ttf",
		}
		family := ""
		for _, name := range names {
			data, err := fontFS.ReadFile(name)
			if err != nil {
				panic("shell: read " + name + ": " + err.Error())
			}
			id := qt.QFontDatabase_AddApplicationFontFromData(data)
			if id < 0 {
				panic("shell: load font " + name)
			}
			if families := qt.QFontDatabase_ApplicationFontFamilies(id); len(families) > 0 {
				family = families[0]
			}
		}
		if family == "" {
			panic("shell: no Inter family")
		}
		fontSet = &fonts{
			family:   family,
			bold:     newFont(family, labelSizePx, qt.QFont__Bold),
			medium:   newFont(family, captionSizePx, qt.QFont__Medium),
			regular:  newFont(family, 12.5, qt.QFont__Normal),
			semibold: newFont(family, 15, qt.QFont__DemiBold),
			cache:    make(map[fontKey]*qt.QFont),
		}
	})
	return fontSet
}

// at returns one face at a custom size and weight.
func (f *fonts) at(px float64, weight qt.QFont__Weight) *qt.QFont {
	key := fontKey{px: px, weight: weight}
	if font, ok := f.cache[key]; ok {
		return font
	}
	font := newFont(f.family, px, weight)
	f.cache[key] = font
	return font
}

// newFont builds one Inter face in logical pixels. Qt point sizes are 1/72 inch
// while the Gio roles are in 96-dpi pixels, hence the 0.75 factor.
func newFont(family string, px float64, weight qt.QFont__Weight) *qt.QFont {
	f := qt.NewQFont()
	f.SetFamily(family)
	f.SetPointSizeF(px * 0.75)
	f.SetWeight(weight)
	f.SetHintingPreference(qt.QFont__PreferNoHinting)
	f.SetStyleStrategy(qt.QFont__PreferAntialias | qt.QFont__PreferQuality)
	return f
}

// loadPoster decodes the embedded poster sprite sheet once.
func loadPoster() *qt.QImage {
	posterOnce.Do(func() {
		data, err := posterFS.ReadFile("assets/poster.png")
		if err != nil {
			panic("shell: read poster: " + err.Error())
		}
		img := qt.NewQImage()
		if !img.LoadFromData2(&data[0], len(data)) {
			panic("shell: decode poster")
		}
		posterImage = img
	})
	return posterImage
}
