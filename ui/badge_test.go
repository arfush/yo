package ui

import (
	"image"
	"math"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// badgeVersions covers the shapes a release version can take: short, long,
// multi-dot and wide digit runs. Every one of them has to grow the pill and
// keep its label centered.
var badgeVersions = []string{
	"v1.1", "v1.2", "v1.10", "v1.32", "v1.1.10", "v2.100", "v1.2.11", "v10.0.100",
}

func badgeTestContext(scale float32) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(image.Pt(833, 545)),
	}
}

func TestVersionBadgeFollowsTheLabelWidth(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}

	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		gtx := badgeTestContext(scale)
		g := makeGeometry(gtx, false, true, true, true)
		border := g.leastPx(1)
		pad := g.px(badgePaddingX)
		height := g.px(badgeHeight)

		widths := make(map[string]int, len(badgeVersions))
		for _, v := range badgeVersions {
			badge, ok := measureVersionBadge(gtx, typo, v, g)
			if !ok {
				t.Fatalf("scale %v: %q produced no ink", scale, v)
			}
			widths[v] = badge.pill.Dx()

			if got, want := badge.pill.Dx(), int(math.Round(float64(badge.textWidth)))+2*pad+2*border; got != want {
				t.Errorf("scale %v %q: pill width %d, want text %.2f + padding %d", scale, v, got, badge.textWidth, want)
			}
			if badge.textWidth+1 < float32(badge.ink.Dx()) {
				t.Errorf("scale %v %q: text width %.2f is much narrower than its ink %d", scale, v, badge.textWidth, badge.ink.Dx())
			}
			if got := badge.pill.Dy(); got != height {
				t.Errorf("scale %v %q: pill height %d, want %d", scale, v, got, height)
			}
			if badge.pill.Min.X != g.badgeX {
				t.Errorf("scale %v %q: pill starts at %d, want %d", scale, v, badge.pill.Min.X, g.badgeX)
			}
			if badge.pill.Min.Y < 0 || badge.pill.Max.Y > g.headerClickEnd {
				t.Errorf("scale %v %q: pill %v leaves the header", scale, v, badge.pill)
			}
		}

		// Longer versions must get wider pills, otherwise the text would be
		// squeezed into the same space as a short one.
		ordered := []string{"v1.1", "v1.10", "v1.1.10", "v2.100"}
		for i := 1; i < len(ordered); i++ {
			if widths[ordered[i]] <= widths[ordered[i-1]] {
				t.Errorf("scale %v: %q (%d) is not wider than %q (%d)",
					scale, ordered[i], widths[ordered[i]], ordered[i-1], widths[ordered[i-1]])
			}
		}
	}
}

func TestVersionBadgeCentersItsLabel(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}

	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		gtx := badgeTestContext(scale)
		g := makeGeometry(gtx, false, true, true, true)
		for _, v := range badgeVersions {
			badge, ok := measureVersionBadge(gtx, typo, v, g)
			if !ok {
				t.Fatalf("scale %v: %q produced no ink", scale, v)
			}

			inner := badge.pill.Inset(g.leastPx(1))

			// Replay the painter: it centers the ink horizontally and the cap
			// block vertically on badge.center. The gaps that remain inside the
			// pill have to be equal, otherwise the label looks shifted even
			// though the numbers are close.
			dx := badge.center.X - float32(badge.ink.Min.X+badge.ink.Max.X)/2
			dy := badge.center.Y - float32(badge.ink.Min.Y) - float32(badge.cap)/2

			leftGap := float32(badge.ink.Min.X) + dx - float32(inner.Min.X)
			rightGap := float32(inner.Max.X) - (float32(badge.ink.Max.X) + dx)
			if d := math.Abs(float64(leftGap - rightGap)); d > 0.5 {
				t.Errorf("scale %v %q: left gap %.2f, right gap %.2f", scale, v, leftGap, rightGap)
			}
			if d := math.Abs(float64(leftGap - float32(g.px(badgePaddingX)))); d > 1 {
				t.Errorf("scale %v %q: left gap %.2f, want padding %d", scale, v, leftGap, g.px(badgePaddingX))
			}

			capTop := float32(badge.ink.Min.Y) + dy
			topGap := capTop - float32(inner.Min.Y)
			bottomGap := float32(inner.Max.Y) - (capTop + float32(badge.cap))
			if d := math.Abs(float64(topGap - bottomGap)); d > 0.5 {
				t.Errorf("scale %v %q: top gap %.2f, bottom gap %.2f (ink %v, cap %d)",
					scale, v, topGap, bottomGap, badge.ink, badge.cap)
			}

			// The label must not be lifted by the dot below the baseline: the
			// cap block, not the full ink, defines the vertical middle.
			if badge.cap <= 0 || badge.cap >= badge.ink.Dy()+1 {
				t.Errorf("scale %v %q: cap %d does not describe the label ink %v", scale, v, badge.cap, badge.ink)
			}
		}
	}
}

// TestVersionBadgeMatchesMPVHeader keeps the width handed to the mpv overlay in
// sync with the pill the application draws: the Lua script rebuilds the pill as
// text width + 2*pad + 2*border, so a mismatch would show up as two different
// badges.
func TestVersionBadgeMatchesMPVHeader(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}

	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		gtx := badgeTestContext(scale)
		g := makeGeometry(gtx, false, true, true, true)
		s := &shell{typo: typo}
		right := s.layoutBadge(gtx, g)

		border := g.leastPx(1)
		pad := g.px(badgePaddingX)
		textPx := s.badgeWidth * scale
		if got, want := float64(textPx)+float64(2*pad+2*border), float64(right-g.badgeX); math.Abs(got-want) > 0.5 {
			t.Errorf("scale %v: mpv would draw a %.2fpx pill, the application draws %dpx", scale, got, right-g.badgeX)
		}
		if s.badgeWidth <= 0 {
			t.Errorf("scale %v: badge width handed to mpv is %v", scale, s.badgeWidth)
		}
	}
}

func TestVersionBadgeShapesKnownVersions(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	gtx := badgeTestContext(1)
	g := makeGeometry(gtx, false, true, true, true)

	// Guard rails for the common case: at 100% DPI the pill stays compact but
	// wide enough for its text.
	for _, v := range []string{"v1.1", "v1.2", "v1.10", "v1.32", "v1.1.10", "v2.100", "v1.2.11"} {
		badge, ok := measureVersionBadge(gtx, typo, v, g)
		if !ok {
			t.Fatalf("%q produced no ink", v)
		}
		if badge.pill.Dx() < 30 || badge.pill.Dx() > 80 {
			t.Errorf("%q: pill width %d is outside the designed range", v, badge.pill.Dx())
		}
		if badge.cap < 7 || badge.cap > 9 {
			t.Errorf("%q: cap height %d does not match the 11sp badge text", v, badge.cap)
		}
	}
}
