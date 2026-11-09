package mascot

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/arfush/yo/ui/appearance"
)

func testPalette() appearance.MascotPalette {
	normal := color.NRGBA{R: 0x20, G: 0x30, B: 0x40, A: 0xff}
	alert := color.NRGBA{R: 0x50, G: 0x10, B: 0x20, A: 0xff}
	colors := func(ink color.NRGBA) appearance.MascotColors {
		return appearance.MascotColors{
			Hair: ink, HairShadow: ink, HairLight: ink, Face: ink,
			EarInner: ink, Paw: ink, Blush: ink, Dress: ink, DressLight: ink,
		}
	}
	return appearance.MascotPalette{
		Normal:       colors(normal),
		Alert:        colors(alert),
		Eye:          normal,
		EyeHighlight: normal,
		Glitch: appearance.MascotGlitch{
			Cyan:    color.NRGBA{R: 0x10, G: 0x50, B: 0x60, A: 0x55},
			Magenta: color.NRGBA{R: 0x60, G: 0x10, B: 0x50, A: 0x55},
		},
	}
}

func testContext(now time.Time) layout.Context {
	return layout.Context{Ops: new(op.Ops), Now: now}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("call did not panic")
		}
	}()
	fn()
}

func TestNewRejectsZeroPalette(t *testing.T) {
	assertPanics(t, func() { New(Options{}) })
}

func TestNewRejectsUnknownMood(t *testing.T) {
	assertPanics(t, func() { New(Options{Palette: testPalette(), Mood: Mood(99)}) })
}

func TestZeroMascotDrawPanics(t *testing.T) {
	assertPanics(t, func() {
		(Mascot{}).Draw(testContext(time.Unix(1, 0)), image.Rect(0, 0, 42, 42))
	})
}

func TestZeroMascotEmptyRectIsIgnored(t *testing.T) {
	(Mascot{}).Draw(testContext(time.Unix(1, 0)), image.Rectangle{})
}

func TestSetMoodRejectsUnknownMood(t *testing.T) {
	m := New(Options{Palette: testPalette()})
	assertPanics(t, func() { m.SetMood(Mood(200), time.Time{}) })
}

func TestDrawEveryMoodAndSlot(t *testing.T) {
	pal := testPalette()
	now := time.Unix(1000, 0)
	rects := []image.Rectangle{
		image.Rect(0, 0, 168, 168),
		image.Rect(10, 10, 34, 20),
		image.Rect(-8, -8, 8, 8),
		image.Rect(0, 0, 1, 1),
	}
	for _, mood := range []Mood{Normal, Alert, Loading, Panic} {
		for _, since := range []time.Time{{}, now.Add(-500 * time.Millisecond)} {
			m := New(Options{Palette: pal, Mood: mood, MoodSince: since})
			for _, rect := range rects {
				m.Draw(testContext(now), rect)
			}
		}
	}
}

func TestPanicGlitchDrawsDuringBurst(t *testing.T) {
	// 100 ms into a cycle is inside the burst, so the glitch path repaints
	// displaced slices.
	now := time.Unix(0, int64(100*time.Millisecond))
	New(Options{Palette: testPalette(), Mood: Panic}).Draw(testContext(now), image.Rect(0, 0, 168, 168))
}
