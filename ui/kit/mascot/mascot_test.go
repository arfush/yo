package mascot

import (
	"image"
	"image/color"
	"testing"
	"time"

	qt "github.com/mappu/miqt/qt6"

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

// testPainter returns a painter over an offscreen image and cleans it up.
func testPainter(t *testing.T) *qt.QPainter {
	t.Helper()
	img := qt.NewQImage3(168, 168, qt.QImage__Format_ARGB32_Premultiplied)
	p := qt.NewQPainter2(img.QPaintDevice)
	t.Cleanup(func() {
		p.End()
		p.Delete()
		img.Delete()
	})
	return p
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

func TestZeroMascotPaintPanics(t *testing.T) {
	p := testPainter(t)
	assertPanics(t, func() {
		(Mascot{}).Paint(p, image.Rect(0, 0, 42, 42), time.Unix(1, 0))
	})
}

func TestZeroMascotEmptyRectIsIgnored(t *testing.T) {
	(Mascot{}).Paint(testPainter(t), image.Rectangle{}, time.Unix(1, 0))
}

func TestPaintEveryMoodAndSlot(t *testing.T) {
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
			p := testPainter(t)
			for _, rect := range rects {
				m.Paint(p, rect, now)
			}
		}
	}
}

func TestPanicGlitchDrawsDuringBurst(t *testing.T) {
	// 100 ms into a cycle is inside the burst, so the glitch path repaints
	// displaced slices.
	now := time.Unix(0, int64(100*time.Millisecond))
	New(Options{Palette: testPalette(), Mood: Panic}).
		Paint(testPainter(t), image.Rect(0, 0, 168, 168), now)
}
