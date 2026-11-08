package mascot

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
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

func TestMoodStyleColors(t *testing.T) {
	pal := testPalette()
	for _, mood := range []Mood{Normal, Loading, Panic} {
		if got := mood.style(pal).colors; got != pal.Normal {
			t.Fatalf("%v colors = %+v, want normal set", mood, got)
		}
	}
	if got := Alert.style(pal).colors; got != pal.Alert {
		t.Fatalf("Alert colors = %+v, want alert set", got)
	}
}

func TestAlertPoseBringsHandsTogether(t *testing.T) {
	if pose := Alert.pose(0); pose.together != 0 || !pose.displeased {
		t.Fatalf("pose at start = %+v, want hands apart and displeased", pose)
	}
	if pose := Alert.pose(alertTogetherSeconds / 2); pose.together <= 0 || pose.together >= 1 {
		t.Fatalf("pose halfway = %+v, want hands moving together", pose)
	}
	if pose := Alert.pose(alertTogetherSeconds); pose.together != 1 {
		t.Fatalf("pose after one second = %+v, want hands together", pose)
	}
	if pose := Alert.pose(2 * alertTogetherSeconds); pose.together != 1 {
		t.Fatalf("pose after hold = %+v, want hands together", pose)
	}
	for _, mood := range []Mood{Normal, Loading} {
		if pose := mood.pose(0); pose != (moodPose{}) {
			t.Fatalf("%v pose = %+v, want zero", mood, pose)
		}
	}
}

func TestAlertHandsMoveSymmetrically(t *testing.T) {
	pt := func(x, y float32) f32.Point { return f32.Pt(x, y) }
	for _, together := range []float32{0, 0.25, 0.5, 1} {
		left, right := alertHands(pt, together)
		if left.X != -right.X || left.Y != right.Y {
			t.Fatalf("together=%v: hands are not mirrored: left=%v right=%v", together, left, right)
		}
		if together == 0 && (left.X != -alertHandRestX || left.Y != alertHandRestY) {
			t.Fatalf("together=0: left hand = %v, want resting position", left)
		}
		if together == 1 && (left.X != -alertHandTogetherX || left.Y != alertHandTogetherY) {
			t.Fatalf("together=1: left hand = %v, want together position", left)
		}
	}
}

func TestAlertSettlesWithoutSince(t *testing.T) {
	m := New(Options{Palette: testPalette(), Mood: Alert})
	if got := m.elapsed(time.Unix(3, 0)); got != alertTogetherSeconds {
		t.Fatalf("elapsed without since = %v, want settled", got)
	}
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

func TestSetMoodDrawsNextFrame(t *testing.T) {
	m := New(Options{Palette: testPalette()})
	since := time.Unix(5, 0)
	m.SetMood(Alert, since)
	if m.mood != Alert || !m.since.Equal(since) {
		t.Fatalf("mood = %v, since = %v, want Alert at %v", m.mood, m.since, since)
	}
	m.Draw(testContext(since.Add(500*time.Millisecond)), image.Rect(0, 0, 96, 96))
}

func TestPanicGlitchDrawsDuringBurst(t *testing.T) {
	// 100 ms into a cycle is inside the burst, so the glitch path repaints
	// displaced slices.
	now := time.Unix(0, int64(100*time.Millisecond))
	New(Options{Palette: testPalette(), Mood: Panic}).Draw(testContext(now), image.Rect(0, 0, 168, 168))
}

func TestGlitchNoiseIsStable(t *testing.T) {
	if glitchNoise(7) != glitchNoise(7) {
		t.Fatal("glitchNoise is not deterministic")
	}
	if glitchNoise(7) == glitchNoise(8) {
		t.Fatal("glitchNoise returns the same value for different steps")
	}
}
