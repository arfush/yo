package kit

import (
	"image"
	"testing"
	"time"
)

func TestHatsCatalog(t *testing.T) {
	hats := MascotHats()
	if len(hats) == 0 {
		t.Fatal("MascotHats() is empty")
	}
	seen := make(map[MascotHat]bool, len(hats))
	for _, hat := range hats {
		if hat == HatNone {
			t.Fatal("MascotHats() contains HatNone")
		}
		if seen[hat] {
			t.Fatalf("MascotHats() contains %q twice", hat)
		}
		seen[hat] = true
	}

	first := hats[0]
	hats[0] = MascotHat("changed")
	if MascotHats()[0] != first {
		t.Fatal("MascotHats() did not return a copy")
	}
}

func TestRandomHatReturnsKnownHat(t *testing.T) {
	known := make(map[MascotHat]bool)
	for _, hat := range MascotHats() {
		known[hat] = true
	}
	for i := 0; i < len(known)*8; i++ {
		hat := RandomMascotHat()
		if hat == HatNone || !known[hat] {
			t.Fatalf("RandomMascotHat() = %q; want a known outfit", hat)
		}
	}
}

func TestSeasonalHatHolidayWindows(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want MascotHat
	}{
		{"christmas eve", time.Date(2026, time.December, 17, 12, 0, 0, 0, time.Local), HatNone},
		{"holidays start", time.Date(2026, time.December, 18, 0, 0, 0, 0, time.Local), HatSanta},
		{"new year", time.Date(2027, time.January, 1, 12, 0, 0, 0, time.Local), HatSanta},
		{"holidays end", time.Date(2027, time.January, 8, 23, 59, 0, 0, time.Local), HatSanta},
		{"after holidays", time.Date(2027, time.January, 9, 0, 0, 0, 0, time.Local), HatNone},
		{"women's day week starts", time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), HatFlowers},
		{"women's day week ends", time.Date(2026, time.March, 15, 23, 59, 0, 0, time.Local), HatFlowers},
		{"after women's day", time.Date(2026, time.March, 16, 0, 0, 0, 0, time.Local), HatNone},
		{"before knowledge day", time.Date(2026, time.August, 24, 23, 59, 0, 0, time.Local), HatNone},
		{"knowledge day week starts", time.Date(2026, time.August, 25, 0, 0, 0, 0, time.Local), HatGraduation},
		{"knowledge day week ends", time.Date(2026, time.September, 8, 23, 59, 0, 0, time.Local), HatGraduation},
		{"after knowledge day", time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local), HatNone},
		{"halloween eve", time.Date(2026, time.October, 23, 12, 0, 0, 0, time.Local), HatNone},
		{"halloween week starts", time.Date(2026, time.October, 24, 0, 0, 0, 0, time.Local), HatWitch},
		{"halloween week ends", time.Date(2026, time.November, 7, 23, 59, 0, 0, time.Local), HatWitch},
		{"after halloween", time.Date(2026, time.November, 8, 0, 0, 0, 0, time.Local), HatNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SeasonalMascotHat(test.date); got != test.want {
				t.Fatalf("SeasonalMascotHat(%s) = %q, want %q", test.date.Format("2006-01-02"), got, test.want)
			}
		})
	}
}

func TestNewRejectsUnknownHat(t *testing.T) {
	assertPanics(t, func() {
		NewMascot(MascotOptions{Palette: testPalette(), Hat: MascotHat("unknown")})
	})
}

func TestPaintEveryHatAndMood(t *testing.T) {
	pal := testPalette()
	now := time.Unix(1000, 0)
	p := testPainter(t)
	for _, hat := range append([]MascotHat{HatNone}, MascotHats()...) {
		for _, mood := range []MascotMood{MascotNormal, MascotAlert, MascotLoading, MascotPanic} {
			m := NewMascot(MascotOptions{Palette: pal, Hat: hat, Mood: mood})
			m.Paint(p, image.Rect(0, 0, 168, 168), now)
			m = NewMascot(MascotOptions{
				Palette:   pal,
				Hat:       hat,
				Mood:      mood,
				MoodSince: now.Add(-500 * time.Millisecond),
			})
			m.Paint(p, image.Rect(0, 0, 168, 168), now)
		}
	}
}

func TestPaintCypherSmallSlot(t *testing.T) {
	m := NewMascot(MascotOptions{Palette: testPalette(), Hat: HatCypher})
	p := testPainter(t)
	for _, rect := range []image.Rectangle{
		image.Rect(0, 0, 1, 1),
		image.Rect(10, 10, 34, 20),
	} {
		m.Paint(p, rect, time.Unix(2, 0))
	}
}
