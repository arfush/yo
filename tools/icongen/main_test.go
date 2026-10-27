package main

import (
	"math"
	"testing"
)

func TestPlayGlyphStaysCentered(t *testing.T) {
	b := iconBounds(buildPlay())
	assertCentered(t, "play", b)
}

func TestUpdateGlyphStaysCentered(t *testing.T) {
	b := iconBounds(buildUpdate())
	assertCentered(t, "update", b)
}

func TestAllGlyphsStayInsideViewBox(t *testing.T) {
	for _, icon := range icons {
		b := iconBounds(icon.build())
		if b.minX < 0 || b.minY < 0 || b.maxX > 1 || b.maxY > 1 {
			t.Errorf("%s bounds %+v leave the unit view box", icon.name, b)
		}
	}
}

func TestIconSheetProducesEveryGlyph(t *testing.T) {
	specs, err := loadIconSheet("icons.svg")
	if err != nil {
		t.Fatal(err)
	}
	if want := len(svgIconNames) + 1; len(specs) != want {
		t.Fatalf("generated %d sheet glyphs, want %d", len(specs), want)
	}
	for _, icon := range specs {
		b := iconBounds(icon.build())
		if b.minX < 0 || b.minY < 0 || b.maxX > 1 || b.maxY > 1 {
			t.Errorf("%s bounds %+v leave the unit view box", icon.name, b)
		}
	}
}

func TestGlyphPolylinesHaveNoDuplicatePoints(t *testing.T) {
	for _, icon := range icons {
		for si, s := range icon.build() {
			for ci, c := range s.contours {
				prev := c.start
				for _, seg := range c.segs {
					if seg.to.sub(prev).len() < 0.000001 {
						t.Fatalf(
							"%s shape %d contour %d repeats point %+v",
							icon.name, si, ci, seg.to,
						)
					}
					prev = seg.to
				}
			}
		}
	}
}

func TestOffsetNormalHandlesZeroLengthSegments(t *testing.T) {
	p := vec{0.5, 0.5}
	next := vec{0.6, 0.5}

	if got := offsetNormal(p, p, next); math.Abs(got.len()-1) > 1e-9 {
		t.Fatalf("duplicate previous: normal length = %v, want 1", got.len())
	}
	if got := offsetNormal(next, p, p); math.Abs(got.len()-1) > 1e-9 {
		t.Fatalf("duplicate next: normal length = %v, want 1", got.len())
	}
	if got := offsetNormal(p, p, p); got.len() != 0 {
		t.Fatalf("duplicate both: normal = %+v, want zero", got)
	}
}

func assertCentered(t *testing.T, name string, b iconBox) {
	t.Helper()
	centerX := (b.minX + b.maxX) / 2
	centerY := (b.minY + b.maxY) / 2
	if math.Abs(centerX-0.5) > 0.02 || math.Abs(centerY-0.5) > 0.02 {
		t.Fatalf("%s centre = (%.3f, %.3f), want (0.5, 0.5)", name, centerX, centerY)
	}
	if b.minX < 0 || b.minY < 0 || b.maxX > 1 || b.maxY > 1 {
		t.Fatalf("%s bounds %+v leave the unit view box", name, b)
	}
}

type iconBox struct {
	minX, minY float64
	maxX, maxY float64
}

func iconBounds(shapes []shape) iconBox {
	b := iconBox{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for _, s := range shapes {
		for _, c := range s.contours {
			b.include(c.start)
			for _, seg := range c.segs {
				if seg.kind == 'C' {
					b.include(seg.c1)
					b.include(seg.c2)
				}
				b.include(seg.to)
			}
		}
	}
	return b
}

func (b *iconBox) include(p vec) {
	b.minX = math.Min(b.minX, p.x)
	b.minY = math.Min(b.minY, p.y)
	b.maxX = math.Max(b.maxX, p.x)
	b.maxY = math.Max(b.maxY, p.y)
}
