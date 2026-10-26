package ui

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
)

const (
	mascotHatParty = iota
	mascotHatWitch
	mascotHatCrown
	mascotHatTop
	mascotHatBeanie
	mascotHatCap
	mascotHatSanta
	mascotHatGraduation
	mascotHatUshanka
	mascotHatFlowers
	mascotHatFedora

	// mascotHatCount is the number of outfits the waiting screen can pick from.
	mascotHatCount
	// mascotHatNone keeps a mascot bare outside the holiday windows.
	mascotHatNone = -1
)

var (
	hatPink  = color.NRGBA{R: 0xf2, G: 0x8f, B: 0xc2, A: 0xff}
	hatWhite = color.NRGBA{R: 0xf7, G: 0xf3, B: 0xff, A: 0xff}
	hatCream = color.NRGBA{R: 0xe9, G: 0xde, B: 0xc6, A: 0xff}
	hatDark  = color.NRGBA{R: 0x3b, G: 0x2a, B: 0x63, A: 0xff}
	hatGold  = color.NRGBA{R: 0xf5, G: 0xc2, B: 0x5c, A: 0xff}
	hatRed   = color.NRGBA{R: 0xd9, G: 0x4f, B: 0x6a, A: 0xff}
	hatTeal  = color.NRGBA{R: 0x5f, G: 0xc7, B: 0xc0, A: 0xff}
	hatSky   = color.NRGBA{R: 0x6f, G: 0x9c, B: 0xf5, A: 0xff}
	hatBrown = color.NRGBA{R: 0x7a, G: 0x5a, B: 0x4a, A: 0xff}
	hatTan   = color.NRGBA{R: 0xd8, G: 0xc3, B: 0xb0, A: 0xff}
	hatGreen = color.NRGBA{R: 0x6f, G: 0xb0, B: 0x79, A: 0xff}
)

// nextMascotHat returns the hat for a waiting screen. The pick is random so a
// room does not always start with the same look; the caller stores it, so the
// hat stays fixed for as long as the screen is open.
func nextMascotHat() int {
	return rand.IntN(mascotHatCount)
}

// seasonalMascotHat picks the outfit for the URL-field mascot. Each window is
// the holiday plus a week before and after it; outside these windows the
// mascot stays bare:
//
//	Dec 18 - Jan 8   Santa hat (Christmas and New Year)
//	Mar 1  - Mar 15  flower crown (8 March)
//	Aug 25 - Sep 8   graduation cap (1 September)
//	Oct 24 - Nov 7   witch hat (Halloween)
func seasonalMascotHat(now time.Time) int {
	month, day := now.Month(), now.Day()
	switch {
	case month == time.December && day >= 18, month == time.January && day <= 8:
		return mascotHatSanta
	case month == time.March && day <= 15:
		return mascotHatFlowers
	case month == time.August && day >= 25, month == time.September && day <= 8:
		return mascotHatGraduation
	case month == time.October && day >= 24, month == time.November && day <= 7:
		return mascotHatWitch
	default:
		return mascotHatNone
	}
}

// drawMascotHat paints one of the waiting-screen outfits. Every shape uses the
// same mascot coordinate system as drawCatgirl, so a hat bobs with the head
// instead of floating beside it.
func drawMascotHat(
	gtx layout.Context,
	hat int,
	r func(x0, y0, x1, y1 float32) image.Rectangle,
	pt func(x, y float32) f32.Point,
	px func(float32) int,
) {
	switch hat % mascotHatCount {
	case 0: // Party cone with a pompom.
		fillPolygon(gtx, []f32.Point{
			pt(-5.8, -12.4),
			pt(5.8, -12.4),
			pt(0.6, -21.2),
		}, hatPink)
		fillRounded(gtx, hatWhite, r(-6.2, -13.4, 6.2, -12.0), px(0.6))
		fillCircle(gtx, pointOf(pt(0.8, -21.8)), max(1, px(1.2)), hatWhite)
	case 1: // Witch hat with a gold band.
		fillEllipse(gtx, r(-10.5, -14.2, 10.5, -11.3), hatDark)
		fillPolygon(gtx, []f32.Point{
			pt(-5.4, -12.8),
			pt(4.8, -12.8),
			pt(3.2, -17.5),
			pt(1.8, -19.6),
			pt(-0.6, -21.0),
		}, hatDark)
		fillRounded(gtx, hatGold, r(-5.5, -14.5, 5.5, -13.4), px(0.3))
		fillRounded(gtx, hatGold, r(-1.0, -15.5, 1.0, -14.2), px(0.4))
	case 2: // Crown.
		fillPolygon(gtx, []f32.Point{
			pt(-6.8, -12.6),
			pt(-4.8, -16.8),
			pt(-2.4, -13.6),
			pt(0, -18.0),
			pt(2.4, -13.6),
			pt(4.8, -16.8),
			pt(6.8, -12.6),
		}, hatGold)
		fillRounded(gtx, hatGold, r(-6.8, -13.4, 6.8, -11.8), px(0.5))
		fillCircle(gtx, pointOf(pt(-4.8, -17.2)), max(1, px(0.55)), hatPink)
		fillCircle(gtx, pointOf(pt(0, -18.4)), max(1, px(0.55)), hatPink)
		fillCircle(gtx, pointOf(pt(4.8, -17.2)), max(1, px(0.55)), hatPink)
	case 3: // Top hat.
		fillEllipse(gtx, r(-9.4, -13.6, 9.4, -11.3), hatDark)
		fillRounded(gtx, hatDark, r(-5.8, -20.4, 5.8, -12.0), px(0.7))
		fillRounded(gtx, hatRed, r(-5.8, -14.6, 5.8, -13.0), px(0.2))
	case 4: // Beanie.
		fillEllipse(gtx, r(-7.8, -16.8, 7.8, -11.0), hatTeal)
		fillRounded(gtx, hatWhite, r(-8.0, -13.6, 8.0, -11.5), px(0.7))
		fillCircle(gtx, pointOf(pt(0, -17.6)), max(1, px(1.5)), hatWhite)
	case 5: // Baseball cap with the brim to the right.
		fillRounded(gtx, hatSky, r(4.0, -13.6, 12.4, -12.2), px(0.7))
		fillEllipse(gtx, r(-7.6, -16.2, 7.6, -11.2), hatSky)
		fillCircle(gtx, pointOf(pt(0, -15.9)), max(1, px(0.55)), hatWhite)
	case 6: // Santa hat.
		fillPolygon(gtx, []f32.Point{
			pt(-6.8, -12.5),
			pt(6.8, -12.5),
			pt(1.6, -21.6),
		}, hatRed)
		fillRounded(gtx, hatWhite, r(-7.2, -13.6, 7.2, -11.8), px(0.8))
		fillCircle(gtx, pointOf(pt(1.8, -21.9)), max(1, px(1.4)), hatWhite)
	case 7: // Graduation cap.
		fillRounded(gtx, hatDark, r(-5.4, -13.8, 5.4, -11.8), px(0.6))
		fillPolygon(gtx, []f32.Point{
			pt(-10.0, -13.2),
			pt(0, -17.2),
			pt(10.0, -13.2),
			pt(0, -11.2),
		}, hatDark)
		strokeLine(gtx, pt(0.4, -13.2), pt(7.4, -10.2), float32(max(1, px(0.35))), hatGold)
		fillCircle(gtx, pointOf(pt(7.6, -10.0)), max(1, px(0.6)), hatGold)
	case 8: // Ushanka with ear flaps.
		fillRounded(gtx, hatBrown, r(-8.7, -13.8, -5.2, -8.2), px(1.4))
		fillRounded(gtx, hatBrown, r(5.2, -13.8, 8.7, -8.2), px(1.4))
		fillEllipse(gtx, r(-7.6, -16.6, 7.6, -11.0), hatBrown)
		fillRounded(gtx, hatTan, r(-7.8, -13.6, 7.8, -11.6), px(0.8))
	case 9: // Flower crown.
		fillRounded(gtx, hatGreen, r(-8.2, -12.8, 8.2, -11.6), px(0.4))
		for index, x := range []float32{-6.2, -3.1, 0, 3.1, 6.2} {
			petal := hatPink
			if index%2 == 1 {
				petal = hatWhite
			}
			fillCircle(gtx, pointOf(pt(x, -12.2)), max(1, px(1.25)), petal)
			fillCircle(gtx, pointOf(pt(x, -12.2)), max(1, px(0.45)), hatGold)
		}
	case 10: // Cypher's cream fedora with a dark band and a teal lens.
		// The brim gets a warmer underside, like the shadowed edge of felt.
		fillEllipse(gtx, r(-12.4, -13.9, 12.4, -10.8), hatTan)
		fillEllipse(gtx, r(-12.0, -14.2, 12.0, -11.4), hatCream)
		// The crown tapers to a pinched fedora top.
		fillPolygon(gtx, []f32.Point{
			pt(-6.5, -12.8),
			pt(-5.1, -19.6),
			pt(-2.7, -20.8),
			pt(0, -20.0),
			pt(2.7, -20.8),
			pt(5.1, -19.6),
			pt(6.5, -12.8),
		}, hatCream)
		crease := float32(max(1, px(0.32)))
		strokeLine(gtx, pt(-2.8, -20.2), pt(0, -19.4), crease, hatTan)
		strokeLine(gtx, pt(2.8, -20.2), pt(0, -19.4), crease, hatTan)
		fillRounded(gtx, hatDark, r(-6.3, -14.5, 6.3, -12.9), px(0.35))
		// A small teal camera lens in a gold setting is the Cypher nod.
		fillCircle(gtx, pointOf(pt(0, -13.7)), max(1, px(0.9)), hatGold)
		fillCircle(gtx, pointOf(pt(0, -13.7)), max(1, px(0.55)), hatTeal)
		fillCircle(gtx, pointOf(pt(0, -13.7)), max(1, px(0.18)), hatDark)
	}
}

// pointOf converts the fractional mascot-space point into the integer grid
// used by fillCircle, which needs a stable center across frames.
func pointOf(p f32.Point) image.Point {
	return image.Pt(int(math.Round(float64(p.X))), int(math.Round(float64(p.Y))))
}
