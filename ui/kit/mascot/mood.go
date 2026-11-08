package mascot

import "github.com/arfush/yo/ui/appearance"

// Mood selects the mascot's color and motion preset.
type Mood uint8

// Moods in presentation order. The zero value is Normal.
const (
	Normal Mood = iota
	Alert
	Loading
	Panic
)

// moodStyle is the resolved look and motion for one mood.
type moodStyle struct {
	colors    appearance.MascotColors
	amplitude float32
	period    float64
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
func (m Mood) validate() {
	switch m {
	case Normal, Alert, Loading, Panic:
	default:
		panic("mascot: unknown mood")
	}
}

// style resolves the colors and motion for the mood. Normal, Loading and Panic
// share the normal mascot set; only Alert switches to the alert set.
func (m Mood) style(pal appearance.MascotPalette) moodStyle {
	switch m {
	case Normal:
		return moodStyle{colors: pal.Normal, amplitude: 0.8, period: 3.6}
	case Alert:
		return moodStyle{colors: pal.Alert, amplitude: 0.8, period: 3.6}
	case Loading:
		return moodStyle{colors: pal.Normal, amplitude: 2.0, period: 1.8, loading: true}
	case Panic:
		return moodStyle{colors: pal.Normal, amplitude: 1.1, period: 0.36}
	default:
		panic("mascot: unknown mood")
	}
}

// pose returns the pose at elapsed seconds.
func (m Mood) pose(elapsed float64) moodPose {
	switch m {
	case Normal, Loading:
		return moodPose{}
	case Alert:
		return moodPose{
			together:   smoothStep(float32(elapsed / alertTogetherSeconds)),
			displeased: true,
		}
	case Panic:
		return moodPose{panic: true}
	default:
		panic("mascot: unknown mood")
	}
}
