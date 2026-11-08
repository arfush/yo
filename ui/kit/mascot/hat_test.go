package mascot

import (
	"image"
	"testing"
	"time"
)

func TestHatsCatalog(t *testing.T) {
	hats := Hats()
	if len(hats) != len(allHats) {
		t.Fatalf("Hats() returned %d outfits, want %d", len(hats), len(allHats))
	}
	for _, hat := range hats {
		if hat == HatNone {
			t.Fatal("Hats() contains HatNone")
		}
		hat.validate()
	}
	hats[0] = Hat("changed")
	if Hats()[0] != allHats[0] {
		t.Fatal("Hats() did not return a copy")
	}
}

func TestRandomHatStaysInRange(t *testing.T) {
	seen := map[Hat]bool{}
	calls := len(allHats) * 20
	for index := 0; index < calls; index++ {
		hat := RandomHat()
		if hat == HatNone {
			t.Fatal("RandomHat() returned HatNone")
		}
		seen[hat] = true
	}
	if len(seen) != len(allHats) {
		t.Fatalf("%d calls covered %d outfits, want all %d", calls, len(seen), len(allHats))
	}
}

func TestSeasonalHatHolidayWindows(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want Hat
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
			if got := SeasonalHat(test.date); got != test.want {
				t.Fatalf("SeasonalHat(%s) = %q, want %q", test.date.Format("2006-01-02"), got, test.want)
			}
		})
	}
}

func TestNewRejectsUnknownHat(t *testing.T) {
	assertPanics(t, func() {
		New(Options{Palette: testPalette(), Hat: Hat("unknown")})
	})
}

func TestDrawUnknownHatPanics(t *testing.T) {
	m := Mascot{palette: testPalette(), hat: Hat("unknown")}
	assertPanics(t, func() {
		m.Draw(testContext(time.Unix(1, 0)), image.Rect(0, 0, 42, 42))
	})
}

func TestDrawEveryHatAndMood(t *testing.T) {
	pal := testPalette()
	now := time.Unix(1000, 0)
	for _, hat := range append([]Hat{HatNone}, allHats...) {
		for _, mood := range []Mood{Normal, Alert, Loading, Panic} {
			m := New(Options{Palette: pal, Hat: hat, Mood: mood})
			m.Draw(testContext(now), image.Rect(0, 0, 168, 168))
			m = New(Options{
				Palette:   pal,
				Hat:       hat,
				Mood:      mood,
				MoodSince: now.Add(-500 * time.Millisecond),
			})
			m.Draw(testContext(now), image.Rect(0, 0, 168, 168))
		}
	}
}

func TestDrawCypherSmallSlot(t *testing.T) {
	m := New(Options{Palette: testPalette(), Hat: HatCypher})
	for _, rect := range []image.Rectangle{
		image.Rect(0, 0, 1, 1),
		image.Rect(10, 10, 34, 20),
	} {
		m.Draw(testContext(time.Unix(2, 0)), rect)
	}
}
