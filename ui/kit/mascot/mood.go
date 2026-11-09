package mascot

import (
	"time"

	"github.com/arfush/yo/ui/appearance"
)

// Mood selects the mascot's color and motion preset.
type Mood uint8

// Moods in presentation order. The zero value is Normal.
const (
	Normal Mood = iota
	Alert
	Loading
	Panic
)

// Mood motion presets. Amplitudes are design units; periods are durations.
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
		return moodStyle{colors: pal.Normal, amplitude: calmAmplitude, period: calmPeriod}
	case Alert:
		return moodStyle{colors: pal.Alert, amplitude: calmAmplitude, period: calmPeriod}
	case Loading:
		return moodStyle{colors: pal.Normal, amplitude: loadingAmplitude, period: loadingPeriod, loading: true}
	case Panic:
		return moodStyle{colors: pal.Normal, amplitude: panicAmplitude, period: panicPeriod}
	default:
		panic("mascot: unknown mood")
	}
}

// pose returns the pose at elapsed time.
func (m Mood) pose(elapsed time.Duration) moodPose {
	switch m {
	case Normal, Loading:
		return moodPose{}
	case Alert:
		return moodPose{
			together:   smoothStep(float32(elapsed.Seconds() / alertTogetherPeriod.Seconds())),
			displeased: true,
		}
	case Panic:
		return moodPose{panic: true}
	default:
		panic("mascot: unknown mood")
	}
}
