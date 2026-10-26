package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestNextMascotHatStaysInRange(t *testing.T) {
	seen := map[int]bool{}
	for index := 0; index < mascotHatCount*20; index++ {
		hat := nextMascotHat()
		if hat < 0 || hat >= mascotHatCount {
			t.Fatalf("nextMascotHat() = %d, want a hat in [0, %d)", hat, mascotHatCount)
		}
		seen[hat] = true
	}
	if len(seen) != mascotHatCount {
		t.Fatalf("%d calls covered %d hats, want all %d", mascotHatCount*20, len(seen), mascotHatCount)
	}
}

func TestSeasonalMascotHatBoundaries(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want int
	}{
		{"halloween eve", time.Date(2026, time.October, 23, 12, 0, 0, 0, time.Local), mascotHatBeanie},
		{"halloween week starts", time.Date(2026, time.October, 24, 0, 0, 0, 0, time.Local), mascotHatWitch},
		{"halloween", time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local), mascotHatWitch},
		{"halloween week ends", time.Date(2026, time.November, 7, 23, 59, 0, 0, time.Local), mascotHatWitch},
		{"after halloween", time.Date(2026, time.November, 8, 0, 0, 0, 0, time.Local), mascotHatBeanie},
		{"late autumn", time.Date(2026, time.December, 17, 12, 0, 0, 0, time.Local), mascotHatBeanie},
		{"holidays start", time.Date(2026, time.December, 18, 0, 0, 0, 0, time.Local), mascotHatSanta},
		{"christmas", time.Date(2026, time.December, 25, 12, 0, 0, 0, time.Local), mascotHatSanta},
		{"new year", time.Date(2027, time.January, 1, 12, 0, 0, 0, time.Local), mascotHatSanta},
		{"holidays end", time.Date(2027, time.January, 8, 23, 59, 0, 0, time.Local), mascotHatSanta},
		{"winter starts", time.Date(2027, time.January, 9, 0, 0, 0, 0, time.Local), mascotHatUshanka},
		{"leap day", time.Date(2024, time.February, 29, 12, 0, 0, 0, time.Local), mascotHatUshanka},
		{"spring starts", time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), mascotHatFlowers},
		{"spring ends", time.Date(2026, time.May, 31, 23, 59, 0, 0, time.Local), mascotHatFlowers},
		{"summer starts", time.Date(2026, time.June, 1, 0, 0, 0, 0, time.Local), mascotHatCap},
		{"summer ends", time.Date(2026, time.August, 31, 23, 59, 0, 0, time.Local), mascotHatCap},
		{"autumn starts", time.Date(2026, time.September, 1, 0, 0, 0, 0, time.Local), mascotHatBeanie},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := seasonalMascotHat(test.date); got != test.want {
				t.Fatalf("seasonalMascotHat(%s) = %d, want %d", test.date.Format("2006-01-02"), got, test.want)
			}
		})
	}
}

// TestSeasonalMascotHatCoversEveryDay keeps a future edit from leaving a gap
// where the mascot unexpectedly loses her hat.
func TestSeasonalMascotHatCoversEveryDay(t *testing.T) {
	day := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.Local)
	end := time.Date(2028, time.January, 1, 12, 0, 0, 0, time.Local)
	for ; day.Before(end); day = day.AddDate(0, 0, 1) {
		hat := seasonalMascotHat(day)
		if hat < 0 || hat >= mascotHatCount {
			t.Fatalf("seasonalMascotHat(%s) = %d, want a seasonal hat", day.Format("2006-01-02"), hat)
		}
	}
}

// TestDrawCatgirlWithEveryHat exercises every branch of the hat switch. It
// cannot judge the picture, but a nil rectangle or a broken polygon would
// panic here before the waiting screen does it for the user.
func TestDrawCatgirlWithEveryHat(t *testing.T) {
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Now:    time.Unix(1000, 0),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	rect := image.Rect(0, 0, 168, 168)
	for hat := mascotHatNone; hat < mascotHatCount; hat++ {
		drawCatgirl(gtx, rect, gtx.Now, hat, false)
		drawCatgirl(gtx, rect, gtx.Now, hat, true)
	}
}
