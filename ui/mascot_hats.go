package ui

import (
	"image"
	"image/color"
	"math"
	"sync/atomic"

	"gioui.org/f32"
	"gioui.org/layout"
)

const (
	// mascotHatCount is the number of outfits the waiting screen can pick from.
	mascotHatCount = 10
	// mascotHatNone keeps the main-window mascot bare.
	mascotHatNone = -1
)

var (
	hatPink  = color.NRGBA{R: 0xf2, G: 0x8f, B: 0xc2, A: 0xff}
	hatWhite = color.NRGBA{R: 0xf7, G: 0xf3, B: 0xff, A: 0xff}
	hatDark  = color.NRGBA{R: 0x3b, G: 0x2a, B: 0x63, A: 0xff}
	hatGold  = color.NRGBA{R: 0xf5, G: 0xc2, B: 0x5c, A: 0xff}
	hatRed   = color.NRGBA{R: 0xd9, G: 0x4f, B: 0x6a, A: 0xff}
	hatTeal  = color.NRGBA{R: 0x5f, G: 0xc7, B: 0xc0, A: 0xff}
	hatSky   = color.NRGBA{R: 0x6f, G: 0x9c, B: 0xf5, A: 0xff}
	hatBrown = color.NRGBA{R: 0x7a, G: 0x5a, B: 0x4a, A: 0xff}
	hatTan   = color.NRGBA{R: 0xd8, G: 0xc3, B: 0xb0, A: 0xff}
	hatGreen = color.NRGBA{R: 0x6f, G: 0xb0, B: 0x79, A: 0xff}

	// mascotHatCursor makes every reopened waiting screen pick the next hat.
	// The app shows one waiting screen at a time, so a package counter is
	// enough and guarantees that two openings never show the same hat.
	mascotHatCursor atomic.Uint32
)

// nextMascotHat returns the hat for a waiting screen. Cycling instead of
// hashing the room key is deliberate: reopening the same room must still look
// new, but the hat must stay fixed for as long as the screen is open.
func nextMascotHat() int {
	return int((mascotHatCursor.Add(1) - 1) % uint32(mascotHatCount))
}

// drawMascotHat paints one of the waiting-screen outfits. Every shape uses the
// same mascot coordinate system as drawCatgirl, so a hat bobs and leans with
// the head instead of floating beside it.
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
	}
}

// pointOf converts the fractional mascot-space point into the integer grid
// used by fillCircle, which needs a stable center across frames.
func pointOf(p f32.Point) image.Point {
	return image.Pt(int(math.Round(float64(p.X))), int(math.Round(float64(p.Y))))
}
