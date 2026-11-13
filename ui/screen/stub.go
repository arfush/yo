package screen

import (
	"image"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit/mascot"
	"github.com/arfush/yo/ui/navigator"
)

// Stub geometry and timing, in logical pixels and seconds.
const (
	stubMascotSize = 253
	stubMoodPeriod = 5 * time.Second
)

// stubMood returns the mood of one five-second slot. The cycle is Normal,
// Alert, Loading, Panic.
func stubMood(slot int) mascot.Mood {
	switch slot % 4 {
	case 1:
		return mascot.Alert
	case 2:
		return mascot.Loading
	case 3:
		return mascot.Panic
	default:
		return mascot.Normal
	}
}

// Stub is the temporary startup screen: the mascot standing in the middle of
// the content area. It switches the mascot mood every five seconds. Not part of
// the product.
type Stub struct {
	navigator.ScreenBase
	since time.Time
}

// NewStub builds the placeholder screen.
func NewStub() *Stub {
	return &Stub{since: time.Now()}
}

// Paint draws the mascot centered in the content area.
func (s *Stub) Paint(p *qt.QPainter, content image.Rectangle, palette appearance.Palette) {
	if content.Empty() {
		return
	}
	now := time.Now()
	elapsed := now.Sub(s.since)
	periods := elapsed / stubMoodPeriod

	slot := min(stubMascotSize, min(content.Dx(), content.Dy()))
	rect := image.Rect(0, 0, slot, slot).Add(image.Pt(
		content.Min.X+(content.Dx()-slot)/2,
		content.Min.Y+(content.Dy()-slot)/2,
	))
	mascot.New(mascot.Options{
		Palette:   palette.Mascot,
		Mood:      stubMood(int(periods)),
		MoodSince: s.since.Add(periods * stubMoodPeriod),
		Hat:       mascot.HatNone,
	}).Paint(p, rect, now)
}
