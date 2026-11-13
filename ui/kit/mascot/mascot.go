package mascot

import (
	"image"
	"time"

	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

const designSize = 42

// Options is the constructor input.
type Options struct {
	Palette   appearance.MascotPalette
	Mood      Mood
	MoodSince time.Time
	Hat       Hat
}

// Mascot is a drawable mascot value. Build it with New. The zero value is
// invalid and Paint panics on it.
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
	if m.mood == Panic {
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
