package ui

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestMascotHatForIsStable(t *testing.T) {
	for _, key := range []string{"", "YOCBQOPGZ", "YOSZZZZZZ"} {
		first := mascotHatFor(key)
		if first < 0 || first >= mascotHatCount {
			t.Fatalf("mascotHatFor(%q) = %d, want a hat in [0, %d)", key, first, mascotHatCount)
		}
		if again := mascotHatFor(key); again != first {
			t.Fatalf("mascotHatFor(%q) changed from %d to %d", key, first, again)
		}
	}
}

func TestMascotHatForVariesAcrossRooms(t *testing.T) {
	seen := make(map[int]bool)
	for index := 0; index < 100; index++ {
		seen[mascotHatFor(fmt.Sprintf("YOC%06d", index))] = true
	}
	if len(seen) < 5 {
		t.Fatalf("only %d hats were selected across 100 rooms, the hash is too flat", len(seen))
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
		drawCatgirl(gtx, rect, gtx.Now, time.Time{}, hat)
	}
}
