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
