package kit

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

const designSize = 42

// MascotOptions is the constructor input.
type MascotOptions struct {
	Palette   appearance.MascotPalette
	Mood      MascotMood
	MoodSince time.Time
	Hat       MascotHat
}

// Mascot is a drawable mascot value. Build it with NewMascot. The zero value is
// invalid and Paint panics on it.
type Mascot struct {
	palette appearance.MascotPalette
	mood    MascotMood
	since   time.Time
	hat     MascotHat
}

// NewMascot validates the options and builds a mascot. It panics on a zero
// palette, an unknown mood or an unknown hat: all are programmer errors.
func NewMascot(opts MascotOptions) Mascot {
	if opts.Palette == (appearance.MascotPalette{}) {
		panic("mascot: zero palette")
	}
	opts.Mood.validate()
	opts.Hat.validate()
	return Mascot{
		palette: opts.Palette,
		mood:    opts.Mood,
		since:   opts.MoodSince,
		hat:     opts.Hat,
	}
}

// Paint draws the mascot centered in rect at the given instant. An empty rect
// draws nothing.
func (m Mascot) Paint(p *qt6.QPainter, rect image.Rectangle, now time.Time) {
	if rect.Empty() {
		return
	}
	if m.palette == (appearance.MascotPalette{}) {
		panic("mascot: zero mascot")
	}
	m.mood.validate()
	m.hat.validate()

	s := scene{
		m:     m,
		g:     newGrid(rect),
		phase: seconds(now),
		style: m.mood.style(m.palette),
		pose:  m.mood.pose(m.elapsed(now)),
	}
	s.paint(p, 0)
	if m.mood == MascotPanic {
		glitchScene{
			bounds: rect,
			center: s.g.center,
			phase:  s.phase,
			px:     s.g.px,
			colors: m.palette.Glitch,
			paint:  s.paint,
		}.draw(p)
	}
}

// elapsed returns how long the current mood has been active. A zero since
// means "already settled", so the intro animation is skipped.
func (m Mascot) elapsed(now time.Time) time.Duration {
	if m.since.IsZero() {
		return alertTogetherPeriod
	}
	return now.Sub(m.since)
}

// seconds turns a frame time into animation seconds.
func seconds(now time.Time) float64 {
	return float64(now.UnixNano()) / float64(time.Second)
}

// MascotMood selects the mascot's color and motion preset.
type MascotMood uint8

// Moods in presentation order. The zero value is MascotNormal.
const (
	MascotNormal MascotMood = iota
	MascotAlert
	MascotLoading
	MascotPanic
)

// Motion presets per mood. Amplitudes are design units; periods are durations.
const (
	calmAmplitude    float32 = 0.8
	loadingAmplitude float32 = 2.0
	panicAmplitude   float32 = 1.1

	calmPeriod    = 3600 * time.Millisecond
	loadingPeriod = 1800 * time.Millisecond
	panicPeriod   = 360 * time.Millisecond
)

// moodStyle is the resolved look and motion for one mood.
type moodStyle struct {
	colors    appearance.MascotColors
	amplitude float32
	period    time.Duration
	loading   bool
}

// moodPose is the mood-specific pose at one instant. elapsed is how long the
// mood has been active, in seconds.
type moodPose struct {
	together   float32
	displeased bool
	panic      bool
}

// validate panics on an unknown mood.
func (m MascotMood) validate() {
	switch m {
	case MascotNormal, MascotAlert, MascotLoading, MascotPanic:
	default:
		panic("mascot: unknown mood")
	}
}

// style resolves the colors and motion for the mood. The normal, loading and
// panic moods share the normal mascot set; only the alert mood switches to the
// alert set.
func (m MascotMood) style(pal appearance.MascotPalette) moodStyle {
	switch m {
	case MascotNormal:
		return moodStyle{colors: pal.Normal, amplitude: calmAmplitude, period: calmPeriod}
	case MascotAlert:
		return moodStyle{colors: pal.Alert, amplitude: calmAmplitude, period: calmPeriod}
	case MascotLoading:
		return moodStyle{colors: pal.Normal, amplitude: loadingAmplitude, period: loadingPeriod, loading: true}
	case MascotPanic:
		return moodStyle{colors: pal.Normal, amplitude: panicAmplitude, period: panicPeriod}
	default:
		panic("mascot: unknown mood")
	}
}

// pose returns the pose at elapsed time.
func (m MascotMood) pose(elapsed time.Duration) moodPose {
	switch m {
	case MascotNormal, MascotLoading:
		return moodPose{}
	case MascotAlert:
		return moodPose{
			together:   smoothStep(float32(elapsed.Seconds() / alertTogetherPeriod.Seconds())),
			displeased: true,
		}
	case MascotPanic:
		return moodPose{panic: true}
	default:
		panic("mascot: unknown mood")
	}
}

// Motion periods and alert hand geometry. Hand coordinates are design units.
const (
	wavePeriod          = 2400 * time.Millisecond
	loadingRingPeriod   = 1400 * time.Millisecond
	panicTremblePeriod  = 90 * time.Millisecond
	panicTremble        = 0.6
	alertTogetherPeriod = time.Second

	alertHandRestX     = 11
	alertHandRestY     = 3.5
	alertHandTogetherX = 3.2
	alertHandTogetherY = 8.8
)

// scene is one resolved frame of the mascot.
type scene struct {
	m     Mascot
	g     grid
	phase float64
	style moodStyle
	pose  moodPose
}

// paint draws the character with an optional horizontal jitter in design units.
// The panic mood repaints clipped slices through it to build the glitch.
func (s scene) paint(p *qt6.QPainter, jitter float32) {
	c := s.style.colors
	bob := float32(math.Sin(s.phase*2*math.Pi/s.style.period.Seconds())) * s.style.amplitude

	p.Save()
	p.Translate2(float64(jitter*s.g.scale), float64(bob*s.g.scale))
	defer p.Restore()

	if s.style.loading {
		drawRing(p, s.g, s.phase, c.HairLight)
	}
	drawTail(p, s.g, c.HairShadow)
	drawBody(p, s.g, c)
	drawHead(p, s.g, c)
	drawFace(p, s.g, c, s.pose, s.m.palette)
	s.m.hat.draw(p, wardrobe{g: s.g, pal: s.m.palette, hats: hatColors()})
	left, right := s.hands()
	drawArms(p, s.g, c, left, right)
	drawPaw(p, s.g, left, c.Paw, c.HairShadow)
	drawPaw(p, s.g, right, c.Paw, c.HairShadow)
}

// hands returns the paw centers for this frame. The alert mood folds both hands
// onto the chest; the panic mood presses them to the cheeks with a tremble.
func (s scene) hands() (point, point) {
	if s.pose.displeased {
		return alertHands(s.g.pt, s.pose.together)
	}
	if s.pose.panic {
		tremble := float32(math.Sin(s.phase*2*math.Pi/panicTremblePeriod.Seconds())) * panicTremble
		return s.g.pt(-11+tremble, -1.6), s.g.pt(11+tremble, -1.6)
	}
	wave := float32(math.Sin(s.phase * 2 * math.Pi / wavePeriod.Seconds()))
	leftLift := smoothStep(wave*0.5+0.5) * 4.4
	rightLift := smoothStep(0.5-wave*0.5) * 4.4
	return s.g.pt(-11, 3.5-leftLift), s.g.pt(11, 3.5-rightLift)
}

// grid maps the 42x42 design space onto a centered square slot.
type grid struct {
	scale  float32
	center image.Point
}

func newGrid(rect image.Rectangle) grid {
	size := min(rect.Dx(), rect.Dy())
	return grid{
		scale:  float32(size) / designSize,
		center: image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2),
	}
}

func (g grid) px(v float32) int { return int(math.Round(float64(v * g.scale))) }

func (g grid) rect(x0, y0, x1, y1 float32) image.Rectangle {
	return image.Rect(
		g.center.X+g.px(x0), g.center.Y+g.px(y0),
		g.center.X+g.px(x1), g.center.Y+g.px(y1),
	)
}

func (g grid) pt(x, y float32) point {
	return pt(float32(g.center.X)+x*g.scale, float32(g.center.Y)+y*g.scale)
}

func (g grid) point(x, y float32) image.Point {
	return image.Pt(g.center.X+g.px(x), g.center.Y+g.px(y))
}

// drawRing paints the loading spinner: a faint full track plus a bright arc.
func drawRing(p *qt6.QPainter, g grid, phase float64, col color.NRGBA) {
	center := g.pt(0, 0)
	radius := 18.4 * g.scale
	width := max(float32(1.6), float32(g.px(1.8)))
	strokeCircle(p, center, radius, width, WithAlpha(col, 0x33))
	period := loadingRingPeriod.Seconds()
	turn := math.Mod(phase, period) / period
	strokeArc(p, center, radius, float32(turn*2*math.Pi)-math.Pi/2, float32(1.5*math.Pi), width, col)
}

func drawTail(p *qt6.QPainter, g grid, col color.NRGBA) {
	path := qt6.NewQPainterPath()
	from := g.pt(6, 12)
	path.MoveTo2(float64(from.X), float64(from.Y))
	control1, control2, end := g.pt(16.5, 13.5), g.pt(19, 5), g.pt(15, 0)
	path.CubicTo2(
		float64(control1.X), float64(control1.Y),
		float64(control2.X), float64(control2.Y),
		float64(end.X), float64(end.Y),
	)
	strokePath(p, path, float32(max(1, g.px(2.2))), col)
	path.Delete()
}

func drawArms(p *qt6.QPainter, g grid, c appearance.MascotColors, left, right point) {
	width := max(float32(1.5), float32(g.px(2.1)))
	strokeLine(p, g.pt(-6.5, 7.5), left, width, c.HairShadow)
	strokeLine(p, g.pt(6.5, 7.5), right, width, c.HairShadow)
}

func drawBody(p *qt6.QPainter, g grid, c appearance.MascotColors) {
	body := g.rect(-8.5, 4.5, 8.5, 16.5)
	fillGradient(p, body, g.px(5), c.Hair, c.Dress)
	FillRounded(p, c.DressLight, g.rect(-7.2, 12, 7.2, 12.9), float64(g.px(0.45)))
}

func drawHead(p *qt6.QPainter, g grid, c appearance.MascotColors) {
	fillEllipse(p, c.Hair, g.rect(-12.2, -13.5, 12.2, 6.5))
	fillPolygon(p, c.Hair, []point{
		g.pt(-11.5, -8), g.pt(-14, -18.5), g.pt(-4, -12.5), g.pt(-10.8, -5.6),
	})
	fillPolygon(p, c.Hair, []point{
		g.pt(11.5, -8), g.pt(14, -18.5), g.pt(4, -12.5), g.pt(10.8, -5.6),
	})
	fillPolygon(p, c.EarInner, []point{
		g.pt(-10.2, -9.5), g.pt(-12.4, -16), g.pt(-6.3, -12.2),
	})
	fillPolygon(p, c.EarInner, []point{
		g.pt(10.2, -9.5), g.pt(12.4, -16), g.pt(6.3, -12.2),
	})

	fillEllipse(p, c.Face, g.rect(-8.2, -8.7, 8.2, 5.2))
	FillRounded(p, c.Paw, g.rect(-3.6, 4.9, 3.6, 6.6), float64(g.px(0.9)))
	fillCircle(p, c.Hair, image.Pt(g.center.X, g.center.Y+g.px(5.8)), max(1, g.px(0.5)))

	fillEllipse(p, c.Hair, g.rect(-9.5, -7.5, -5.4, -0.5))
	fillEllipse(p, c.Hair, g.rect(5.4, -7.5, 9.5, -0.5))
	fillEllipse(p, c.Hair, g.rect(-7.6, -10.2, -0.4, -3.1))
	fillEllipse(p, c.Hair, g.rect(-0.6, -10.2, 7.6, -3.1))
	fillEllipse(p, c.Hair, g.rect(-4.6, -11, 0.8, -4.2))
	fillEllipse(p, WithAlpha(c.HairLight, 0x9a), g.rect(-7.2, -8.1, -3.6, -6.1))
}

func drawFace(p *qt6.QPainter, g grid, c appearance.MascotColors, pose moodPose, pal appearance.MascotPalette) {
	for _, side := range []float32{-1, 1} {
		x := side * 3.5
		if pose.panic {
			fillEllipse(p, c.HairLight, g.rect(x-1.5, -3.3, x+1.5, 2))
			fillEllipse(p, pal.Eye, g.rect(x-0.6, -1.8, x+0.6, 0.9))
		} else {
			eye := g.rect(x-1.15, -2.5, x+1.15, 1.2)
			fillEllipse(p, pal.Eye, eye)
			fillCircle(p, pal.EyeHighlight, image.Pt(
				eye.Min.X+g.px(0.8), eye.Min.Y+g.px(0.85),
			), max(1, g.px(0.48)))
		}
		fillEllipse(p, c.Blush, g.rect(side*6.2-1.2, 1.6, side*6.2+1.2, 2.8))
	}

	switch {
	case pose.panic:
		fillEllipse(p, pal.Eye, g.rect(-1.6, 2.3, 1.6, 4.7))
	case pose.displeased:
		brow := max(1, float32(g.px(0.65)))
		strokeLine(p, g.pt(-5.2, -3.1), g.pt(-2, -2.3), brow, pal.Eye)
		strokeLine(p, g.pt(5.2, -3.1), g.pt(2, -2.3), brow, pal.Eye)
		strokeLine(p, g.pt(-1.7, 4.1), g.pt(0, 3.1), brow, pal.Eye)
		strokeLine(p, g.pt(0, 3.1), g.pt(1.7, 4.1), brow, pal.Eye)
	default:
		fillEllipse(p, pal.Eye, g.rect(-1.1, 2.8, 1.1, 3.9))
		lip := max(1, float32(g.px(0.45)))
		strokeLine(p, g.pt(-1.2, 3.2), g.pt(-0.4, 4.3), lip, c.Face)
		strokeLine(p, g.pt(1.2, 3.2), g.pt(0.4, 4.3), lip, c.Face)
	}
}

func drawPaw(p *qt6.QPainter, g grid, center point, paw, toes color.NRGBA) {
	radius := max(2, int(math.Round(float64(3*g.scale))))
	base := image.Pt(int(math.Floor(float64(center.X))), int(math.Floor(float64(center.Y))))
	fracX := center.X - float32(base.X)
	fracY := center.Y - float32(base.Y)

	p.Save()
	p.Translate2(float64(fracX), float64(fracY))
	fillCircle(p, paw, base, radius)
	pad := max(1, int(math.Round(float64(0.7*g.scale))))
	toe := WithAlpha(toes, 0xb0)
	spread := int(math.Round(float64(1.1 * g.scale)))
	fillCircle(p, toe, image.Pt(base.X-spread, base.Y+pad), pad)
	fillCircle(p, toe, image.Pt(base.X+spread, base.Y+pad), pad)
	p.Restore()
}

// alertHands returns the mirrored paw centers for an alert pose. together is
// the eased progress from 0 (hands apart) to 1 (hands on the chest).
func alertHands(at func(x, y float32) point, together float32) (left, right point) {
	x := lerp(alertHandRestX, alertHandTogetherX, together)
	y := lerp(alertHandRestY, alertHandTogetherY, together)
	return at(-x, y), at(x, y)
}

// Panic glitch timing. The character stays intact for most of the cycle, then a
// short burst tears a few horizontal slices sideways.
const (
	glitchCycle = 1350 * time.Millisecond
	glitchBurst = 300 * time.Millisecond
	glitchStep  = time.Second / 18
	glitchBands = 3
	glitchShift = 3.0
)

// glitchScene repaints shifted slices of an already painted panic frame.
type glitchScene struct {
	bounds image.Rectangle
	center image.Point
	phase  float64
	px     func(float32) int
	colors appearance.MascotGlitch
	paint  func(*qt6.QPainter, float32)
}

func (g glitchScene) draw(p *qt6.QPainter) {
	if math.Mod(g.phase, glitchCycle.Seconds()) > glitchBurst.Seconds() {
		return
	}
	step := uint64(g.phase / glitchStep.Seconds())
	for index := uint64(0); index < glitchBands; index++ {
		noise := glitchNoise(step*glitchBands + index)
		top := -16 + float32(noise&0xff)/255*30
		height := 1.2 + float32((noise>>8)&3)*0.9
		band := image.Rect(
			g.bounds.Min.X,
			g.center.Y+g.px(top),
			g.bounds.Max.X,
			g.center.Y+g.px(top+height),
		).Intersect(g.bounds)
		if band.Empty() {
			continue
		}

		p.Save()
		p.SetClipRect2(band.Min.X, band.Min.Y, band.Dx(), band.Dy())
		g.paint(p, glitchShiftAmount(noise))
		p.Restore()

		tint := g.colors.Cyan
		if index%2 == 1 {
			tint = g.colors.Magenta
		}
		line := image.Rect(
			g.center.X+g.px(-14),
			band.Min.Y,
			g.center.X+g.px(14),
			band.Min.Y+max(1, g.px(0.35)),
		)
		FillRect(p, tint, line)
	}
}

// glitchShiftAmount turns hash bits into a signed sideways shift in design
// units, biased away from zero so each slice visibly moves.
func glitchShiftAmount(noise uint64) float32 {
	sign := float32(1)
	if noise&0x10 == 0 {
		sign = -1
	}
	return sign * (1.2 + float32((noise>>5)&0xff)/255*(glitchShift-1.2))
}

// glitchNoise hashes a frame step so a still frame and its redraw agree.
func glitchNoise(seed uint64) uint64 {
	seed ^= seed >> 33
	seed *= 0xff51afd7ed558ccd
	seed ^= seed >> 33
	seed *= 0xc4ceb9fe1a85ec53
	seed ^= seed >> 33
	return seed
}
