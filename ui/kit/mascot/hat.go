package mascot

import (
	"image/color"
	"math/rand/v2"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
)

// Hat selects the mascot's outfit. The zero value is HatNone.
type Hat string

// Hat* are the available outfits. HatNone draws a bare head.
const (
	HatNone       Hat = ""
	HatParty      Hat = "party"
	HatWitch      Hat = "witch"
	HatCrown      Hat = "crown"
	HatTop        Hat = "top"
	HatBeanie     Hat = "beanie"
	HatCap        Hat = "cap"
	HatSanta      Hat = "santa"
	HatGraduation Hat = "graduation"
	HatUshanka    Hat = "ushanka"
	HatFlowers    Hat = "flowers"
	HatCypher     Hat = "cypher"
)

// allHats is the catalog RandomHat chooses from. HatNone is intentionally
// excluded. It is a package-level mutable slice on purpose: the catalog is tiny
// and static, so this is a deliberate exception to the no-package-vars rule.
// Keep it in sync with the Hat* constants.
var allHats = []Hat{
	HatParty,
	HatWitch,
	HatCrown,
	HatTop,
	HatBeanie,
	HatCap,
	HatSanta,
	HatGraduation,
	HatUshanka,
	HatFlowers,
	HatCypher,
}

// Hats returns every outfit in presentation order. The caller owns the slice.
func Hats() []Hat {
	out := make([]Hat, len(allHats))
	copy(out, allHats)
	return out
}

// RandomHat returns a random outfit. It never returns HatNone.
func RandomHat() Hat {
	return allHats[rand.IntN(len(allHats))]
}

// SeasonalHat picks the holiday outfit for a date. Each window covers the
// holiday plus about a week on both sides:
//
//	Dec 18 - Jan 8   Santa hat (Christmas and New Year)
//	Mar 1  - Mar 15  flower crown (8 March)
//	Aug 25 - Sep 8   graduation cap (1 September)
//	Oct 24 - Nov 7   witch hat (Halloween)
func SeasonalHat(now time.Time) Hat {
	month, day := now.Month(), now.Day()
	switch {
	case month == time.December && day >= 18, month == time.January && day <= 8:
		return HatSanta
	case month == time.March && day <= 15:
		return HatFlowers
	case month == time.August && day >= 25, month == time.September && day <= 8:
		return HatGraduation
	case month == time.October && day >= 24, month == time.November && day <= 7:
		return HatWitch
	default:
		return HatNone
	}
}

// validate panics on an unknown hat.
func (h Hat) validate() {
	switch h {
	case HatNone, HatParty, HatWitch, HatCrown, HatTop, HatBeanie,
		HatCap, HatSanta, HatGraduation, HatUshanka, HatFlowers, HatCypher:
	default:
		panic("mascot: unknown hat \"" + string(h) + "\"")
	}
}

// hatPalette is the fixed outfit color set. Hats do not follow the theme, so
// these colors belong to the mascot instead of ui/appearance.
type hatPalette struct {
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

// hatColors returns the fixed outfit colors. It is a function because Go has no
// const structs with image/color values.
func hatColors() hatPalette {
	return hatPalette{
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
	}
}

// wardrobe bundles the geometry and colors every outfit draws with.
type wardrobe struct {
	g    grid
	pal  appearance.MascotPalette
	hats hatPalette
}

// draw paints the selected outfit above the face.
func (h Hat) draw(p *qt.QPainter, w wardrobe) {
	switch h {
	case HatNone:
	case HatParty:
		drawPartyHat(p, w)
	case HatWitch:
		drawWitchHat(p, w)
	case HatCrown:
		drawCrownHat(p, w)
	case HatTop:
		drawTopHat(p, w)
	case HatBeanie:
		drawBeanieHat(p, w)
	case HatCap:
		drawCapHat(p, w)
	case HatSanta:
		drawSantaHat(p, w)
	case HatGraduation:
		drawGraduationHat(p, w)
	case HatUshanka:
		drawUshankaHat(p, w)
	case HatFlowers:
		drawFlowersHat(p, w)
	case HatCypher:
		drawCypherHat(p, w)
	default:
		panic("mascot: unknown hat \"" + string(h) + "\"")
	}
}

func drawPartyHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	fillPolygon(p, h.Pink, []point{
		g.pt(-5.8, -12.4), g.pt(5.8, -12.4), g.pt(0.6, -21.2),
	})
	kit.FillRounded(p, h.White, g.rect(-6.2, -13.4, 6.2, -12), float64(g.px(0.6)))
	fillCircle(p, h.White, g.point(0.8, -21.8), max(1, g.px(1.2)))
}

func drawWitchHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	fillEllipse(p, h.Dark, g.rect(-10.5, -14.2, 10.5, -11.3))
	fillPolygon(p, h.Dark, []point{
		g.pt(-5.4, -12.8), g.pt(4.8, -12.8), g.pt(3.2, -17.5),
		g.pt(1.8, -19.6), g.pt(-0.6, -21),
	})
	kit.FillRounded(p, h.Gold, g.rect(-5.5, -14.5, 5.5, -13.4), float64(g.px(0.3)))
	kit.FillRounded(p, h.Gold, g.rect(-1, -15.5, 1, -14.2), float64(g.px(0.4)))
}

func drawCrownHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	fillPolygon(p, h.Gold, []point{
		g.pt(-6.8, -12.6), g.pt(-4.8, -16.8), g.pt(-2.4, -13.6),
		g.pt(0, -18), g.pt(2.4, -13.6), g.pt(4.8, -16.8), g.pt(6.8, -12.6),
	})
	kit.FillRounded(p, h.Gold, g.rect(-6.8, -13.4, 6.8, -11.8), float64(g.px(0.5)))
	fillCircle(p, h.Pink, g.point(-4.8, -17.2), max(1, g.px(0.55)))
	fillCircle(p, h.Pink, g.point(0, -18.4), max(1, g.px(0.55)))
	fillCircle(p, h.Pink, g.point(4.8, -17.2), max(1, g.px(0.55)))
}

func drawTopHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	fillEllipse(p, h.Dark, g.rect(-9.4, -13.6, 9.4, -11.3))
	kit.FillRounded(p, h.Dark, g.rect(-5.8, -20.4, 5.8, -12), float64(g.px(0.7)))
	kit.FillRounded(p, h.Red, g.rect(-5.8, -14.6, 5.8, -13), float64(g.px(0.2)))
}

func drawBeanieHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	fillEllipse(p, h.Teal, g.rect(-7.8, -16.8, 7.8, -11))
	kit.FillRounded(p, h.White, g.rect(-8, -13.6, 8, -11.5), float64(g.px(0.7)))
	fillCircle(p, h.White, g.point(0, -17.6), max(1, g.px(1.5)))
}

func drawCapHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	kit.FillRounded(p, h.Sky, g.rect(4, -13.6, 12.4, -12.2), float64(g.px(0.7)))
	fillEllipse(p, h.Sky, g.rect(-7.6, -16.2, 7.6, -11.2))
	fillCircle(p, h.White, g.point(0, -15.9), max(1, g.px(0.55)))
}

func drawSantaHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	fillPolygon(p, h.Red, []point{
		g.pt(-6.8, -12.5), g.pt(6.8, -12.5), g.pt(1.6, -21.6),
	})
	kit.FillRounded(p, h.White, g.rect(-7.2, -13.6, 7.2, -11.8), float64(g.px(0.8)))
	fillCircle(p, h.White, g.point(1.8, -21.9), max(1, g.px(1.4)))
}

func drawGraduationHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	kit.FillRounded(p, h.Dark, g.rect(-5.4, -13.8, 5.4, -11.8), float64(g.px(0.6)))
	fillPolygon(p, h.Dark, []point{
		g.pt(-10, -13.2), g.pt(0, -17.2), g.pt(10, -13.2), g.pt(0, -11.2),
	})
	strokeLine(p, g.pt(0.4, -13.2), g.pt(7.4, -10.2), max(1, float32(g.px(0.35))), h.Gold)
	fillCircle(p, h.Gold, g.point(7.6, -10), max(1, g.px(0.6)))
}

func drawUshankaHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	kit.FillRounded(p, h.Brown, g.rect(-8.7, -13.8, -5.2, -8.2), float64(g.px(1.4)))
	kit.FillRounded(p, h.Brown, g.rect(5.2, -13.8, 8.7, -8.2), float64(g.px(1.4)))
	fillEllipse(p, h.Brown, g.rect(-7.6, -16.6, 7.6, -11))
	kit.FillRounded(p, h.Tan, g.rect(-7.8, -13.6, 7.8, -11.6), float64(g.px(0.8)))
}

func drawFlowersHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats
	kit.FillRounded(p, h.Green, g.rect(-8.2, -12.8, 8.2, -11.6), float64(g.px(0.4)))
	for index, x := range []float32{-6.2, -3.1, 0, 3.1, 6.2} {
		petal := h.Pink
		if index%2 == 1 {
			petal = h.White
		}
		fillCircle(p, petal, g.point(x, -12.2), max(1, g.px(1.25)))
		fillCircle(p, h.Gold, g.point(x, -12.2), max(1, g.px(0.45)))
	}
}

// drawCypherHat paints the white fedora with the copper eye emblem.
func drawCypherHat(p *qt.QPainter, w wardrobe) {
	g, h := w.g, w.hats

	// Wide brim with a warm underside and a copper front edge.
	fillEllipse(p, h.Brown, g.rect(-15.2, -13.6, 15.2, -10))
	fillEllipse(p, h.Tan, g.rect(-15, -13.9, 15, -10.4))
	fillEllipse(p, h.Cream, g.rect(-14.4, -14.4, 14.4, -10.7))
	// Tall pinched crown.
	fillPolygon(p, h.Cream, []point{
		g.pt(-7.2, -12.6), g.pt(-5.6, -20.2), g.pt(-3, -21.4),
		g.pt(0, -20.6), g.pt(3, -21.4), g.pt(5.6, -20.2), g.pt(7.2, -12.6),
	})
	crease := max(1, float32(g.px(0.32)))
	strokeLine(p, g.pt(-3, -20.8), g.pt(0, -20), crease, h.Tan)
	strokeLine(p, g.pt(3, -20.8), g.pt(0, -20), crease, h.Tan)
	// Blue band and the copper eye emblem.
	kit.FillRounded(p, h.Dark, g.rect(-6.9, -14.7, 6.9, -12.8), float64(g.px(0.35)))
	kit.FillRounded(p, h.Sky, g.rect(-6.6, -14.5, 6.6, -14), float64(g.px(0.2)))
	fillPolygon(p, h.Gold, []point{
		g.pt(-3, -15), g.pt(3, -15), g.pt(0, -11.4),
	})
	fillPolygon(p, h.Dark, []point{
		g.pt(-2, -14.4), g.pt(2, -14.4), g.pt(0, -12.1),
	})
	fillCircle(p, h.Teal, g.point(0, -13.6), max(1, g.px(0.8)))
	fillCircle(p, w.pal.EyeHighlight, g.point(-0.25, -13.85), max(1, g.px(0.22)))
}
