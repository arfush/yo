package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

var (
	catHairColor     = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	catHairShadow    = color.NRGBA{R: 0x6d, G: 0x3b, B: 0xd9, A: 0xff}
	catHairLight     = color.NRGBA{R: 0xb9, G: 0x9c, B: 0xff, A: 0xff}
	catFaceColor     = color.NRGBA{R: 0xf3, G: 0xec, B: 0xff, A: 0xff}
	catEarInnerColor = color.NRGBA{R: 0xe1, G: 0xbf, B: 0xff, A: 0xff}
	catPawColor      = color.NRGBA{R: 0xe7, G: 0xdd, B: 0xff, A: 0xff}
	catEyeColor      = color.NRGBA{R: 0x2a, G: 0x1e, B: 0x46, A: 0xff}
	catBlushColor    = color.NRGBA{R: 0xf0, G: 0xa8, B: 0xcf, A: 0xa0}
	catDressColor    = color.NRGBA{R: 0x7a, G: 0x3f, B: 0xe0, A: 0xff}
	catDressLight    = color.NRGBA{R: 0xb0, G: 0x8c, B: 0xff, A: 0xff}

	// The alert palette keeps the same shapes but shifts them to red for a load
	// error. Eyes stay dark so the face still reads at 42 dp.
	catAlertHair       = color.NRGBA{R: 0xe9, G: 0x3b, B: 0x50, A: 0xff}
	catAlertHairShadow = color.NRGBA{R: 0xa8, G: 0x22, B: 0x34, A: 0xff}
	catAlertHairLight  = color.NRGBA{R: 0xff, G: 0xa6, B: 0xb0, A: 0xff}
	catAlertFace       = color.NRGBA{R: 0xff, G: 0xe4, B: 0xe7, A: 0xff}
	catAlertEarInner   = color.NRGBA{R: 0xff, G: 0xc4, B: 0xcb, A: 0xff}
	catAlertPaw        = color.NRGBA{R: 0xff, G: 0xd6, B: 0xdb, A: 0xff}
	catAlertBlush      = color.NRGBA{R: 0xff, G: 0x8a, B: 0x9c, A: 0xa0}
	catAlertDress      = color.NRGBA{R: 0xc6, G: 0x2c, B: 0x3e, A: 0xff}
	catAlertDressLight = color.NRGBA{R: 0xff, G: 0x97, B: 0xa3, A: 0xff}
)

const (
	mascotWavePeriod = 2.4
	mascotBobPeriod  = 3.6
)

// mascotMood selects the palette and small motion of the URL-field mascot.
type mascotMood int

const (
	mascotNormal mascotMood = iota
	mascotAlert
	mascotLoading
)

// drawCatgirl paints the cat-girl mascot in the URL field. Her paws alternate
// in a small "nya" gesture while the character gently bobs. Alert switches her
// palette to red for the load-error feedback; loading adds a few orbiting
// sparkles and a slightly livelier bob without changing the palette. Hat is one
// of the mascotHatCount outfits, or mascotHatNone for a bare head.
func drawCatgirl(gtx layout.Context, rect image.Rectangle, now time.Time, hat int, mood mascotMood) {
	if rect.Empty() {
		return
	}
	hair, hairShadow, hairLight := catHairColor, catHairShadow, catHairLight
	face, earInner, paw := catFaceColor, catEarInnerColor, catPawColor
	blush, dress, dressLight := catBlushColor, catDressColor, catDressLight
	if mood == mascotAlert {
		hair, hairShadow, hairLight = catAlertHair, catAlertHairShadow, catAlertHairLight
		face, earInner, paw = catAlertFace, catAlertEarInner, catAlertPaw
		blush, dress, dressLight = catAlertBlush, catAlertDress, catAlertDressLight
	}

	scale := float32(rect.Dx()) / 42
	phase := float64(now.UnixNano()) / 1e9
	bobAmplitude, bobPeriod := float32(0.8), mascotBobPeriod
	if mood == mascotLoading {
		bobAmplitude, bobPeriod = 1.2, 2.8
	}
	bob := float32(math.Sin(phase*2*math.Pi/bobPeriod)) * bobAmplitude
	wave := float32(math.Sin(phase * 2 * math.Pi / mascotWavePeriod))
	leftLift := smoothStep(wave*0.5+0.5) * 4.4
	rightLift := smoothStep(0.5-wave*0.5) * 4.4

	cx := rect.Min.X + rect.Dx()/2
	cy := rect.Min.Y + rect.Dy()/2

	// The bob is smaller than one device pixel per frame. Rounding it into the
	// shape coordinates made the character snap between whole pixels;
	// translating the whole drawing by a fractional affine keeps the motion
	// continuous.
	motion := op.Affine(f32.Affine2D{}.Offset(f32.Pt(0, bob*scale))).Push(gtx.Ops)
	defer motion.Pop()

	px := func(v float32) int {
		return int(math.Round(float64(v * scale)))
	}
	r := func(x0, y0, x1, y1 float32) image.Rectangle {
		return image.Rect(cx+px(x0), cy+px(y0), cx+px(x1), cy+px(y1))
	}
	pt := func(x, y float32) f32.Point {
		return f32.Pt(float32(cx)+x*scale, float32(cy)+y*scale)
	}

	// Tail, tucked behind the dress and clearly visible on the right.
	var tail clip.Path
	tail.Begin(gtx.Ops)
	tail.MoveTo(pt(6, 12))
	tail.CubeTo(
		pt(16.5, 13.5),
		pt(19, 5),
		pt(15, 0),
	)
	tailStack := clip.Stroke{Path: tail.End(), Width: float32(math.Max(1, float64(px(2.2))))}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, hairShadow)
	tailStack.Pop()

	// Arms sit behind the body.
	leftHand := pt(-11, 3.5-leftLift)
	rightHand := pt(11, 3.5-rightLift)
	armWidth := float32(math.Max(1.5, float64(px(2.1))))
	strokeLine(gtx, pt(-6.5, 7.5), leftHand, armWidth, hairShadow)
	strokeLine(gtx, pt(6.5, 7.5), rightHand, armWidth, hairShadow)

	// Body and simple dress.
	body := r(-8.5, 4.5, 8.5, 16.5)
	fillRoundedGradient(gtx, body, body, px(5), hair, dress)
	fillRounded(gtx, dressLight, r(-7.2, 12.0, 7.2, 12.9), px(0.45))

	// Hair silhouette and cat ears. The last point of each outer ear is its
	// root: it sits inside the head ellipse, so the ear base never leaves a
	// wedge of background between the ear and the head.
	fillEllipse(gtx, r(-12.2, -13.5, 12.2, 6.5), hair)
	fillPolygon(gtx, []f32.Point{
		pt(-11.5, -8),
		pt(-14, -18.5),
		pt(-4, -12.5),
		pt(-10.8, -5.6),
	}, hair)
	fillPolygon(gtx, []f32.Point{
		pt(11.5, -8),
		pt(14, -18.5),
		pt(4, -12.5),
		pt(10.8, -5.6),
	}, hair)
	fillPolygon(gtx, []f32.Point{
		pt(-10.2, -9.5),
		pt(-12.4, -16),
		pt(-6.3, -12.2),
	}, earInner)
	fillPolygon(gtx, []f32.Point{
		pt(10.2, -9.5),
		pt(12.4, -16),
		pt(6.3, -12.2),
	}, earInner)

	// Face and a small collar.
	fillEllipse(gtx, r(-8.2, -8.7, 8.2, 5.2), face)
	fillRounded(gtx, paw, r(-3.6, 4.9, 3.6, 6.6), px(0.9))
	fillCircle(gtx, image.Pt(cx, cy+px(5.8)), max(1, px(0.5)), hair)

	// Side locks frame the face without adding tails behind the body.
	fillEllipse(gtx, r(-9.5, -7.5, -5.4, -0.5), hair)
	fillEllipse(gtx, r(5.4, -7.5, 9.5, -0.5), hair)
	// Fringe, with a small highlight on the left. The tops reach above the
	// hair crown: at the junction of the two side locks the face ellipse used
	// to peek through as a stray light dot.
	fillEllipse(gtx, r(-7.6, -10.2, -0.4, -3.1), hair)
	fillEllipse(gtx, r(-0.6, -10.2, 7.6, -3.1), hair)
	fillEllipse(gtx, r(-4.6, -11.0, 0.8, -4.2), hair)
	fillEllipse(gtx, r(-7.2, -8.1, -3.6, -6.1), withAlpha(hairLight, 0x9a))

	// Eyes, blush and a tiny cat mouth.
	for _, side := range []float32{-1, 1} {
		eyeX := side * 3.5
		eye := r(eyeX-1.15, -2.5, eyeX+1.15, 1.2)
		fillEllipse(gtx, eye, catEyeColor)
		fillCircle(gtx, image.Pt(eye.Min.X+px(0.8), eye.Min.Y+px(0.85)), max(1, px(0.48)), color.NRGBA{R: 255, G: 255, B: 255, A: 0xff})
		blushRect := r(side*6.2-1.2, 1.6, side*6.2+1.2, 2.8)
		fillEllipse(gtx, blushRect, blush)
	}
	fillEllipse(gtx, r(-1.1, 2.8, 1.1, 3.9), catEyeColor)
	strokeLine(gtx,
		pt(-1.2, 3.2),
		pt(-0.4, 4.3),
		float32(math.Max(1, float64(px(0.45)))),
		face,
	)
	strokeLine(gtx,
		pt(1.2, 3.2),
		pt(0.4, 4.3),
		float32(math.Max(1, float64(px(0.45)))),
		face,
	)

	// Paws sit in front so the gesture stays readable at 42 dp.
	if mood == mascotLoading {
		// Three tiny sparkles orbit the right ear like a quiet loading pulse.
		// They stay outside the face so the expression remains readable at 42 dp.
		for index := 0; index < 3; index++ {
			angle := phase*2*math.Pi/1.7 + float64(index)*2*math.Pi/3
			alpha := uint8(0x55 + int(0x6a*(0.5+0.5*math.Sin(phase*4+float64(index)))))
			fillCircle(
				gtx,
				image.Pt(cx+px(float32(10.5+3.3*math.Cos(angle))), cy+px(float32(-15.5+2.2*math.Sin(angle)))),
				max(1, px(0.65)),
				withAlpha(hairLight, alpha),
			)
		}
	}
	if hat != mascotHatNone {
		drawMascotHat(gtx, hat, r, pt, px)
	}
	drawPaw(gtx, leftHand, scale, paw, hair)
	drawPaw(gtx, rightHand, scale, paw, hair)
}

func smoothStep(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func drawPaw(gtx layout.Context, center f32.Point, scale float32, paw, toes color.NRGBA) {
	radius := float32(math.Max(2, float64(3.0*scale)))
	// Paws move with the wave, so their centers land between pixels. Draw the
	// circles on the integer grid and slide them by the remainder.
	base := image.Pt(int(math.Floor(float64(center.X))), int(math.Floor(float64(center.Y))))
	frac := f32.Pt(center.X-float32(base.X), center.Y-float32(base.Y))
	stack := op.Affine(f32.Affine2D{}.Offset(frac)).Push(gtx.Ops)
	defer stack.Pop()
	fillCircle(gtx, base, int(math.Round(float64(radius))), paw)
	pad := int(math.Max(1, math.Round(float64(0.7*scale))))
	fillCircle(gtx, image.Pt(base.X-int(math.Round(float64(1.1*scale))), base.Y+pad), pad, withAlpha(toes, 0xb0))
	fillCircle(gtx, image.Pt(base.X+int(math.Round(float64(1.1*scale))), base.Y+pad), pad, withAlpha(toes, 0xb0))
}

func fillEllipse(gtx layout.Context, rect image.Rectangle, col color.NRGBA) {
	if rect.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.Ellipse(rect).Op(gtx.Ops))
}

func fillPolygon(gtx layout.Context, points []f32.Point, col color.NRGBA) {
	if len(points) < 3 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(points[0])
	for _, point := range points[1:] {
		path.LineTo(point)
	}
	path.Close()
	stack := clip.Outline{Path: path.End()}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}
