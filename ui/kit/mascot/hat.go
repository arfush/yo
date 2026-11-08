package mascot

import (
	"math/rand/v2"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"

	"github.com/arfush/yo/ui/appearance"
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
// excluded. Keep it in sync with the Hat* constants.
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

// wardrobe bundles the geometry and colors every outfit draws with.
type wardrobe struct {
	g   grid
	pal appearance.MascotPalette
}

// draw paints the selected outfit above the face.
func (h Hat) draw(gtx layout.Context, w wardrobe) {
	switch h {
	case HatNone:
	case HatParty:
		drawPartyHat(gtx, w)
	case HatWitch:
		drawWitchHat(gtx, w)
	case HatCrown:
		drawCrownHat(gtx, w)
	case HatTop:
		drawTopHat(gtx, w)
	case HatBeanie:
		drawBeanieHat(gtx, w)
	case HatCap:
		drawCapHat(gtx, w)
	case HatSanta:
		drawSantaHat(gtx, w)
	case HatGraduation:
		drawGraduationHat(gtx, w)
	case HatUshanka:
		drawUshankaHat(gtx, w)
	case HatFlowers:
		drawFlowersHat(gtx, w)
	case HatCypher:
		drawCypherHat(gtx, w)
	default:
		panic("mascot: unknown hat \"" + string(h) + "\"")
	}
}

func drawPartyHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillPolygon(gtx, h.Pink, []f32.Point{
		g.pt(-5.8, -12.4), g.pt(5.8, -12.4), g.pt(0.6, -21.2),
	})
	fillRound(gtx, h.White, g.rect(-6.2, -13.4, 6.2, -12), g.px(0.6))
	fillCircle(gtx, h.White, g.point(0.8, -21.8), max(1, g.px(1.2)))
}

func drawWitchHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillEllipse(gtx, h.Dark, g.rect(-10.5, -14.2, 10.5, -11.3))
	fillPolygon(gtx, h.Dark, []f32.Point{
		g.pt(-5.4, -12.8), g.pt(4.8, -12.8), g.pt(3.2, -17.5),
		g.pt(1.8, -19.6), g.pt(-0.6, -21),
	})
	fillRound(gtx, h.Gold, g.rect(-5.5, -14.5, 5.5, -13.4), g.px(0.3))
	fillRound(gtx, h.Gold, g.rect(-1, -15.5, 1, -14.2), g.px(0.4))
}

func drawCrownHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillPolygon(gtx, h.Gold, []f32.Point{
		g.pt(-6.8, -12.6), g.pt(-4.8, -16.8), g.pt(-2.4, -13.6),
		g.pt(0, -18), g.pt(2.4, -13.6), g.pt(4.8, -16.8), g.pt(6.8, -12.6),
	})
	fillRound(gtx, h.Gold, g.rect(-6.8, -13.4, 6.8, -11.8), g.px(0.5))
	fillCircle(gtx, h.Pink, g.point(-4.8, -17.2), max(1, g.px(0.55)))
	fillCircle(gtx, h.Pink, g.point(0, -18.4), max(1, g.px(0.55)))
	fillCircle(gtx, h.Pink, g.point(4.8, -17.2), max(1, g.px(0.55)))
}

func drawTopHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillEllipse(gtx, h.Dark, g.rect(-9.4, -13.6, 9.4, -11.3))
	fillRound(gtx, h.Dark, g.rect(-5.8, -20.4, 5.8, -12), g.px(0.7))
	fillRound(gtx, h.Red, g.rect(-5.8, -14.6, 5.8, -13), g.px(0.2))
}

func drawBeanieHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillEllipse(gtx, h.Teal, g.rect(-7.8, -16.8, 7.8, -11))
	fillRound(gtx, h.White, g.rect(-8, -13.6, 8, -11.5), g.px(0.7))
	fillCircle(gtx, h.White, g.point(0, -17.6), max(1, g.px(1.5)))
}

func drawCapHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillRound(gtx, h.Sky, g.rect(4, -13.6, 12.4, -12.2), g.px(0.7))
	fillEllipse(gtx, h.Sky, g.rect(-7.6, -16.2, 7.6, -11.2))
	fillCircle(gtx, h.White, g.point(0, -15.9), max(1, g.px(0.55)))
}

func drawSantaHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillPolygon(gtx, h.Red, []f32.Point{
		g.pt(-6.8, -12.5), g.pt(6.8, -12.5), g.pt(1.6, -21.6),
	})
	fillRound(gtx, h.White, g.rect(-7.2, -13.6, 7.2, -11.8), g.px(0.8))
	fillCircle(gtx, h.White, g.point(1.8, -21.9), max(1, g.px(1.4)))
}

func drawGraduationHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillRound(gtx, h.Dark, g.rect(-5.4, -13.8, 5.4, -11.8), g.px(0.6))
	fillPolygon(gtx, h.Dark, []f32.Point{
		g.pt(-10, -13.2), g.pt(0, -17.2), g.pt(10, -13.2), g.pt(0, -11.2),
	})
	strokeLine(gtx, g.pt(0.4, -13.2), g.pt(7.4, -10.2), max(1, float32(g.px(0.35))), h.Gold)
	fillCircle(gtx, h.Gold, g.point(7.6, -10), max(1, g.px(0.6)))
}

func drawUshankaHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillRound(gtx, h.Brown, g.rect(-8.7, -13.8, -5.2, -8.2), g.px(1.4))
	fillRound(gtx, h.Brown, g.rect(5.2, -13.8, 8.7, -8.2), g.px(1.4))
	fillEllipse(gtx, h.Brown, g.rect(-7.6, -16.6, 7.6, -11))
	fillRound(gtx, h.Tan, g.rect(-7.8, -13.6, 7.8, -11.6), g.px(0.8))
}

func drawFlowersHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats
	fillRound(gtx, h.Green, g.rect(-8.2, -12.8, 8.2, -11.6), g.px(0.4))
	for index, x := range []float32{-6.2, -3.1, 0, 3.1, 6.2} {
		petal := h.Pink
		if index%2 == 1 {
			petal = h.White
		}
		fillCircle(gtx, petal, g.point(x, -12.2), max(1, g.px(1.25)))
		fillCircle(gtx, h.Gold, g.point(x, -12.2), max(1, g.px(0.45)))
	}
}

// drawCypherHat paints the white fedora with the copper eye emblem.
func drawCypherHat(gtx layout.Context, w wardrobe) {
	g, h := w.g, w.pal.Hats

	// Wide brim with a warm underside and a copper front edge.
	fillEllipse(gtx, h.Brown, g.rect(-15.2, -13.6, 15.2, -10))
	fillEllipse(gtx, h.Tan, g.rect(-15, -13.9, 15, -10.4))
	fillEllipse(gtx, h.Cream, g.rect(-14.4, -14.4, 14.4, -10.7))
	// Tall pinched crown.
	fillPolygon(gtx, h.Cream, []f32.Point{
		g.pt(-7.2, -12.6), g.pt(-5.6, -20.2), g.pt(-3, -21.4),
		g.pt(0, -20.6), g.pt(3, -21.4), g.pt(5.6, -20.2), g.pt(7.2, -12.6),
	})
	crease := max(1, float32(g.px(0.32)))
	strokeLine(gtx, g.pt(-3, -20.8), g.pt(0, -20), crease, h.Tan)
	strokeLine(gtx, g.pt(3, -20.8), g.pt(0, -20), crease, h.Tan)
	// Blue band and the copper eye emblem.
	fillRound(gtx, h.Dark, g.rect(-6.9, -14.7, 6.9, -12.8), g.px(0.35))
	fillRound(gtx, h.Sky, g.rect(-6.6, -14.5, 6.6, -14), g.px(0.2))
	fillPolygon(gtx, h.Gold, []f32.Point{
		g.pt(-3, -15), g.pt(3, -15), g.pt(0, -11.4),
	})
	fillPolygon(gtx, h.Dark, []f32.Point{
		g.pt(-2, -14.4), g.pt(2, -14.4), g.pt(0, -12.1),
	})
	fillCircle(gtx, h.Teal, g.point(0, -13.6), max(1, g.px(0.8)))
	fillCircle(gtx, w.pal.EyeHighlight, g.point(-0.25, -13.85), max(1, g.px(0.22)))
}
