package mascot

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/arfush/yo/ui/appearance"
)

const (
	designSize = 42

	// frameInterval is how far ahead the mascot asks for the next frame.
	frameInterval = time.Second / 60
)

// Options is the constructor input.
type Options struct {
	Palette   appearance.MascotPalette
	Mood      Mood
	MoodSince time.Time
	Hat       Hat
}

// Mascot is a drawable mascot value. Build it with New. The zero value is
// invalid and Draw panics on it.
type Mascot struct {
	palette appearance.MascotPalette
	mood    Mood
	since   time.Time
	hat     Hat
}

// New validates the options and builds a mascot. It panics on a zero palette,
// an unknown mood or an unknown hat: all are programmer errors.
func New(opts Options) Mascot {
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

// SetMood switches the mood and resets its start time. A zero since means the
// mood is already settled, so Alert skips its intro and draws the final pose.
func (m *Mascot) SetMood(mood Mood, since time.Time) {
	if m == nil {
		panic("mascot: nil mascot")
	}
	mood.validate()
	m.mood = mood
	m.since = since
}

// Draw paints the mascot centered in rect and requests the next frame. An empty
// rect draws nothing.
func (m Mascot) Draw(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	if m.palette == (appearance.MascotPalette{}) {
		panic("mascot: zero mascot")
	}
	m.mood.validate()
	m.hat.validate()
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(frameInterval)})

	s := scene{
		m:     m,
		g:     newGrid(rect),
		phase: seconds(gtx.Now),
		style: m.mood.style(m.palette),
		pose:  m.mood.pose(m.elapsed(gtx.Now)),
	}
	s.paint(gtx, 0)
	if m.mood == Panic {
		glitchScene{
			bounds: rect,
			center: s.g.center,
			phase:  s.phase,
			px:     s.g.px,
			colors: m.palette.Glitch,
			paint:  s.paint,
		}.draw(gtx)
	}
}

// elapsed returns how long the current mood has been active. A zero since
// means "already settled", so the intro animation is skipped.
func (m Mascot) elapsed(now time.Time) float64 {
	if m.since.IsZero() {
		return alertTogetherSeconds
	}
	return now.Sub(m.since).Seconds()
}

// seconds turns a frame time into animation seconds.
func seconds(now time.Time) float64 {
	return float64(now.UnixNano()) / float64(time.Second)
}
