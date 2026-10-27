// Command icongen turns Yonime's original painted icon geometry into IconVG
// assets.
//
// The desktop UI used to rasterise these glyphs with Gio clip paths on every
// frame. The shapes in this file are a direct translation of those painting
// functions: same proportions, same stroke widths, same joins. IconVG is only
// the storage format, so the visual design does not change.
//
// Run from the repository root:
//
//	go run ./tools/icongen
package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/exp/shiny/iconvg"
	"golang.org/x/image/math/f32"
)

// The assets use a unit viewBox centered on (0.5, 0.5). design converts the
// old design-pixel coordinates into that box. Each icon's nominal size is the
// same number the old painting function used as its nominal width/height, so
// callers can keep passing the old sizes.
type design struct {
	size float64
}

func (d design) pt(x, y float64) vec {
	return vec{x: 0.5 + x/d.size, y: 0.5 + y/d.size}
}

func (d design) length(v float64) float64 {
	return v / d.size
}

type vec struct {
	x, y float64
}

func (v vec) add(o vec) vec     { return vec{v.x + o.x, v.y + o.y} }
func (v vec) sub(o vec) vec     { return vec{v.x - o.x, v.y - o.y} }
func (v vec) mul(f float64) vec { return vec{v.x * f, v.y * f} }
func (v vec) len() float64      { return math.Hypot(v.x, v.y) }
func (v vec) unit() vec {
	l := v.len()
	if l == 0 {
		return vec{}
	}
	return vec{v.x / l, v.y / l}
}
func (v vec) normal() vec { return vec{-v.y, v.x} }

type segment struct {
	kind byte // 'L' or 'C'
	c1   vec
	c2   vec
	to   vec
}

type contour struct {
	start vec
	segs  []segment
}

type shape struct {
	contours []contour
}

func (s *shape) move(p vec) {
	s.contours = append(s.contours, contour{start: p})
}

func (s *shape) line(p vec) {
	c := &s.contours[len(s.contours)-1]
	c.segs = append(c.segs, segment{kind: 'L', to: p})
}

func (s *shape) cube(c1, c2, p vec) {
	c := &s.contours[len(s.contours)-1]
	c.segs = append(c.segs, segment{kind: 'C', c1: c1, c2: c2, to: p})
}

func (s *shape) polygon(points []vec) {
	if len(points) < 2 {
		return
	}
	if signedArea(points) < 0 {
		reversed := make([]vec, len(points))
		for i := range points {
			reversed[i] = points[len(points)-1-i]
		}
		points = reversed
	}
	s.polygonRaw(points)
}

func (s *shape) polygonRaw(points []vec) {
	if len(points) < 2 {
		return
	}
	s.move(points[0])
	for _, p := range points[1:] {
		s.line(p)
	}
}

func (s *shape) rect(x0, y0, x1, y1 float64) {
	s.polygon([]vec{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}})
}

func (s *shape) strokeLine(a, b vec, width float64) {
	s.strokeOpen([]vec{a, b}, width)
}

func (s *shape) strokeOpen(points []vec, width float64) {
	if len(points) < 2 || width <= 0 {
		return
	}
	s.polygon(openStrokePolygon(points, width))
}

func openStrokePolygon(points []vec, width float64) []vec {
	left, right := offsetPolyline(points, width/2, false)
	outline := append([]vec{}, left...)
	for i := len(right) - 1; i >= 0; i-- {
		outline = append(outline, right[i])
	}
	return outline
}

func (s *shape) strokeClosed(points []vec, width float64) {
	if len(points) < 2 || width <= 0 {
		return
	}
	if len(points) > 2 && points[0].sub(points[len(points)-1]).len() < 0.000001 {
		points = points[:len(points)-1]
	}
	left, right := offsetPolyline(points, width/2, true)
	if signedArea(left) < 0 {
		reversePoints(left)
	}
	// The outer contour has positive winding and the inner one negative. That
	// is the IconVG equivalent of a stroked closed path: the middle stays empty.
	if signedArea(right) > 0 {
		reversePoints(right)
	}
	s.polygonRaw(left)
	s.polygonRaw(right)
}

func offsetPolyline(points []vec, half float64, closed bool) (left, right []vec) {
	left = make([]vec, len(points))
	right = make([]vec, len(points))
	for i, p := range points {
		var normal vec
		switch {
		case !closed && i == 0:
			normal = points[1].sub(points[0]).normal().unit()
		case !closed && i == len(points)-1:
			normal = points[i].sub(points[i-1]).normal().unit()
		case closed && i == 0:
			normal = offsetNormal(points[len(points)-1], p, points[1])
		case i == len(points)-1:
			normal = offsetNormal(points[i-1], p, points[0])
		default:
			normal = offsetNormal(points[i-1], p, points[i+1])
		}
		left[i] = p.add(normal.mul(half))
		right[i] = p.sub(normal.mul(half))
	}
	return left, right
}

func offsetNormal(prev, p, next vec) vec {
	n1 := p.sub(prev).normal().unit()
	n2 := next.sub(p).normal().unit()
	sum := n1.add(n2)
	if sum.len() < 0.0001 {
		return n1
	}
	sum = sum.unit()
	scale := 1 / math.Max(0.25, n1.x*sum.x+n1.y*sum.y)
	scale = math.Min(scale, 4)
	return sum.mul(scale)
}

func reversePoints(points []vec) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}

func signedArea(points []vec) float64 {
	area := 0.0
	for i, p := range points {
		q := points[(i+1)%len(points)]
		area += p.x*q.y - q.x*p.y
	}
	return area / 2
}

func sampleCubic(p0, c1, c2, p3 vec, steps int) []vec {
	if steps < 2 {
		steps = 2
	}
	out := make([]vec, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		u := 1 - t
		out = append(out, vec{
			x: u*u*u*p0.x + 3*u*u*t*c1.x + 3*u*t*t*c2.x + t*t*t*p3.x,
			y: u*u*u*p0.y + 3*u*u*t*c1.y + 3*u*t*t*c2.y + t*t*t*p3.y,
		})
	}
	return out
}

func sampleCircle(cx, cy, radius float64, steps int) []vec {
	out := make([]vec, steps)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(steps)
		out[i] = vec{cx + radius*math.Cos(a), cy + radius*math.Sin(a)}
	}
	return out
}

func sampleRoundedRect(x0, y0, x1, y1, radius float64) []vec {
	const cornerSteps = 10
	const kappa = 0.5522847498
	k := radius * kappa
	var out []vec
	add := func(points []vec) {
		if len(out) > 0 {
			points = points[1:]
		}
		out = append(out, points...)
	}
	add([]vec{{x0 + radius, y0}})
	add([]vec{{x1 - radius, y0}})
	add(sampleCubic(vec{x1 - radius, y0}, vec{x1 - radius + k, y0}, vec{x1, y0 + radius - k}, vec{x1, y0 + radius}, cornerSteps))
	add([]vec{{x1, y1 - radius}})
	add(sampleCubic(vec{x1, y1 - radius}, vec{x1, y1 - radius + k}, vec{x1 - radius + k, y1}, vec{x1 - radius, y1}, cornerSteps))
	add([]vec{{x0 + radius, y1}})
	add(sampleCubic(vec{x0 + radius, y1}, vec{x0 + radius - k, y1}, vec{x0, y1 - radius + k}, vec{x0, y1 - radius}, cornerSteps))
	add([]vec{{x0, y0 + radius}})
	add(sampleCubic(vec{x0, y0 + radius}, vec{x0, y0 + radius - k}, vec{x0 + radius - k, y0}, vec{x0 + radius, y0}, cornerSteps))
	return out
}

func emitShape(enc *iconvg.Encoder, s shape) {
	if len(s.contours) == 0 {
		return
	}
	for ci, c := range s.contours {
		if len(c.segs) == 0 {
			continue
		}
		if ci == 0 {
			enc.StartPath(0, float32(c.start.x), float32(c.start.y))
		} else {
			enc.ClosePathAbsMoveTo(float32(c.start.x), float32(c.start.y))
		}
		for _, seg := range c.segs {
			if seg.kind == 'C' {
				enc.AbsCubeTo(
					float32(seg.c1.x), float32(seg.c1.y),
					float32(seg.c2.x), float32(seg.c2.y),
					float32(seg.to.x), float32(seg.to.y),
				)
			} else {
				enc.AbsLineTo(float32(seg.to.x), float32(seg.to.y))
			}
		}
	}
	enc.ClosePathEndPath()
}

type iconSpec struct {
	name  string
	build func() []shape
}

var icons = []iconSpec{
	{"log", buildLog},
	{"update", buildUpdate},
	{"settings", buildSettings},
	{"people", buildPeople},
	{"copy", buildCopy},
	{"play", buildPlay},
	{"trash", buildTrash},
	{"chevron_left", chevronBuilder(chevronLeftPoints, chevronWidthBase)},
	{"chevron_right", chevronBuilder(chevronRightPoints, chevronWidthBase)},
	{"chevron_up", chevronBuilder(chevronUpPoints, chevronWidthBase)},
	{"chevron_down", chevronBuilder(chevronDownPoints, chevronWidthBase)},
	{"chevron_left_thin", chevronBuilder(chevronLeftPoints, chevronWidthThin)},
	{"chevron_right_thin", chevronBuilder(chevronRightPoints, chevronWidthThin)},
	{"chevron_up_thin", chevronBuilder(chevronUpPoints, chevronWidthThin)},
	{"chevron_down_thin", chevronBuilder(chevronDownPoints, chevronWidthThin)},
	{"chevron_left_thick", chevronBuilder(chevronLeftPoints, chevronWidthThick)},
	{"chevron_right_thick", chevronBuilder(chevronRightPoints, chevronWidthThick)},
	{"chevron_up_thick", chevronBuilder(chevronUpPoints, chevronWidthThick)},
	{"chevron_down_thick", chevronBuilder(chevronDownPoints, chevronWidthThick)},
	{"chevron_left_heavy", chevronBuilder(chevronLeftPoints, chevronWidthHeavy)},
	{"chevron_right_heavy", chevronBuilder(chevronRightPoints, chevronWidthHeavy)},
	{"chevron_up_heavy", chevronBuilder(chevronUpPoints, chevronWidthHeavy)},
	{"chevron_down_heavy", chevronBuilder(chevronDownPoints, chevronWidthHeavy)},
	{"undo", buildUndo},
	{"minimize", buildMinimize},
	{"maximize", buildMaximize},
	{"restore", buildRestore},
	{"close", buildClose},
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fail(err)
	}
	outDir := filepath.Join(root, "ui", "assets", "icons")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}
	for _, icon := range icons {
		data, err := encode(icon.build())
		if err != nil {
			fail(fmt.Errorf("%s: %w", icon.name, err))
		}
		out := filepath.Join(outDir, icon.name+".iconvg")
		if err := os.WriteFile(out, data, 0o644); err != nil {
			fail(fmt.Errorf("write %s: %w", out, err))
		}
		fmt.Printf("%s (%d bytes)\n", icon.name, len(data))
	}
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "icongen:", err)
	os.Exit(1)
}

func encode(shapes []shape) ([]byte, error) {
	var enc iconvg.Encoder
	enc.Reset(iconvg.Metadata{
		ViewBox: iconvg.Rectangle{Min: f32.Vec2{0, 0}, Max: f32.Vec2{1, 1}},
		Palette: iconvg.DefaultPalette,
	})
	enc.HighResolutionCoordinates = true
	for _, shape := range shapes {
		emitShape(&enc, shape)
	}
	return enc.Bytes()
}

// --- Icon geometry -------------------------------------------------------
//
// Each build function below is a direct translation of the painting function
// that used to live in ui/icons.go, ui/main_window.go, ui/settings_window.go,
// ui/title_window.go, ui/undo_delete.go and ui/window_controls.go.

func buildLog() []shape {
	d := design{size: 16}
	var out []shape
	// Rounded document outline.
	var frame shape
	frame.strokeClosed(mapPoints(d, sampleRoundedRect(-6, -7, 6, 7, 2.2)), d.length(1.3))
	out = append(out, frame)

	// Four text lines, the last one deliberately short.
	var lines shape
	for i := 0; i < 4; i++ {
		y := -3.4 + 2.5*float64(i)
		right := 3.5
		if i == 3 {
			right = 1.0
		}
		lines.polygon(mapPoints(d, []vec{
			{-3.5, y}, {right, y}, {right, y + 1.3}, {-3.5, y + 1.3},
		}))
	}
	out = append(out, lines)
	return out
}

// buildUpdate paints two circular arrows. The geometry follows the reference
// refresh glyph: a clockwise arc on top and a counter-clockwise arc below, each
// ending in a square arrowhead.
func buildUpdate() []shape {
	d := design{size: 24}
	pt := func(x, y float64) vec {
		return d.pt(x-12, y-12)
	}
	// Scale the original 18-unit glyph to fill the 14 dp slot. The thin stroke
	// and small arrowheads disappeared at that size; the reference reads as a
	// full-box icon with heavy corners.
	const radius = 10.35
	width := d.length(2.8)
	var out shape

	// Top arc: left edge, over the top, down to the right shoulder.
	top := []vec{pt(12-radius, 12)}
	const steps = 32
	for i := 1; i <= steps; i++ {
		angle := math.Pi + (math.Pi-0.62)*float64(i)/steps
		top = append(top, pt(12+radius*math.Cos(angle), 12+radius*math.Sin(angle)))
	}
	top = append(top, pt(22.35, 7.4))
	out.strokeOpen(top, width)

	// Bottom arc: right edge, under the bottom, up to the left shoulder.
	bottom := []vec{pt(12+radius, 12)}
	for i := 1; i <= steps; i++ {
		angle := (math.Pi + 0.62) * float64(i) / steps
		bottom = append(bottom, pt(12+radius*math.Cos(angle), 12+radius*math.Sin(angle)))
	}
	bottom = append(bottom, pt(1.65, 16.6))
	out.strokeOpen(bottom, width)

	// The reference's arrowheads read as solid triangular wedges at 14 dp.
	out.polygon([]vec{pt(22.35, 1.65), pt(22.35, 7.4), pt(16.6, 7.4)})
	out.polygon([]vec{pt(7.4, 16.6), pt(1.65, 16.6), pt(1.65, 22.35)})
	return []shape{out}
}

func buildSettings() []shape {
	d := design{size: 16}
	var out []shape
	toothWidth := d.length(1.8)
	var teeth shape
	for i := 0; i < 8; i++ {
		angle := float64(i) * math.Pi / 4
		dx, dy := math.Cos(angle), math.Sin(angle)
		teeth.strokeLine(d.pt(dx*5.0, dy*5.0), d.pt(dx*6.8, dy*6.8), toothWidth)
	}
	out = append(out, teeth)

	var rings shape
	ringWidth := d.length(1.25)
	rings.strokeClosed(sampleCircle(0.5, 0.5, d.length(4.4), 64), ringWidth)
	rings.strokeClosed(sampleCircle(0.5, 0.5, d.length(1.8), 64), ringWidth)
	out = append(out, rings)
	return out
}

// buildPeople follows the reference "users" glyph: one complete figure in front
// and a second head and shoulder arc behind it. The earlier knockout silhouette
// read as a single blob at footer size, so the two figures now stay separate.
func buildPeople() []shape {
	d := design{size: 24}
	pt := func(x, y float64) vec {
		return d.pt(x-12, y-12)
	}
	width := d.length(2)
	var out []shape

	var front shape
	front.strokeClosed(sampleCircle(
		0.5+d.length(9-12),
		0.5+d.length(7-12),
		d.length(4),
		64,
	), width)
	body := []vec{pt(2, 21), pt(2, 19)}
	body = append(body, sampleCubic(
		pt(2, 19), pt(2, 16.79), pt(4.21, 15), pt(6, 15), 16,
	)...)
	body = append(body, pt(12, 15))
	body = append(body, sampleCubic(
		pt(12, 15), pt(13.79, 15), pt(16, 16.79), pt(16, 19), 16,
	)...)
	body = append(body, pt(16, 21))
	front.strokeOpen(body, width)
	out = append(out, front)

	var back shape
	head := make([]vec, 0, 33)
	for i := 0; i <= 32; i++ {
		angle := -math.Pi/2 + math.Pi*float64(i)/32
		head = append(head, pt(16+4*math.Cos(angle), 7+4*math.Sin(angle)))
	}
	back.strokeOpen(head, width)

	shoulder := []vec{pt(22, 21), pt(22, 19)}
	for i := 1; i <= 16; i++ {
		angle := -math.Pi / 2 * float64(i) / 16
		shoulder = append(shoulder, pt(18+4*math.Cos(angle), 19+4*math.Sin(angle)))
	}
	back.strokeOpen(shoulder, width)
	out = append(out, back)
	return out
}

func buildCopy() []shape {
	d := design{size: 12}
	width := d.length(1.3)
	var out []shape
	var back shape
	back.strokeOpen(mapPoints(d, []vec{{-4.8, 2.4}, {-4.8, -4.8}, {2.4, -4.8}}), width)
	out = append(out, back)

	var front shape
	front.strokeClosed(sampleRoundedRect(0.5+d.length(-2.4), 0.5+d.length(-2.4), 0.5+d.length(4.8), 0.5+d.length(4.8), d.length(1.4)), width)
	out = append(out, front)
	return out
}

// buildPlay is an outlined play triangle followed by a vertical stop bar, like
// the reference glyph. The old filled triangle was also visually off-centre
// because its paint bounds only covered the right half of the view box.
func buildPlay() []shape {
	d := design{size: 1}
	width := d.length(0.085)
	var out shape
	out.strokeClosed([]vec{
		d.pt(-0.42, -0.38),
		d.pt(0.20, 0),
		d.pt(-0.42, 0.38),
	}, width)
	out.strokeLine(d.pt(0.42, -0.38), d.pt(0.42, 0.38), width)
	return []shape{out}
}

// buildTrash keeps the tapered can from the original house icon, but gives the
// handle proper stems and enlarges the drawing so it stays legible at the file
// row's 16 dp size. The two ribs make the silhouette read as a bin rather than
// a plain cup.
func buildTrash() []shape {
	d := design{size: 16}
	width := d.length(1.7)
	var out shape
	out.strokeLine(d.pt(-7.2, -5.2), d.pt(7.2, -5.2), width)
	out.strokeLine(d.pt(-2.6, -6.8), d.pt(2.6, -6.8), width)
	out.strokeLine(d.pt(-2.6, -6.8), d.pt(-2.6, -5.2), width)
	out.strokeLine(d.pt(2.6, -6.8), d.pt(2.6, -5.2), width)
	out.strokeOpen([]vec{
		d.pt(-6.2, -3.6),
		d.pt(-4.8, 6.4),
		d.pt(4.8, 6.4),
		d.pt(6.2, -3.6),
	}, width)
	out.strokeLine(d.pt(-1.8, -1.2), d.pt(-1.8, 4.8), width)
	out.strokeLine(d.pt(1.8, -1.2), d.pt(1.8, 4.8), width)
	return []shape{out}
}

const (
	chevronWidthThin  = 0.136
	chevronWidthBase  = 0.154
	chevronWidthThick = 0.178
	chevronWidthHeavy = 0.222
)

var (
	chevronLeftPoints  = []vec{{0.175, -0.4}, {-0.275, 0}, {0.175, 0.4}}
	chevronRightPoints = []vec{{-0.175, -0.4}, {0.275, 0}, {-0.175, 0.4}}
	chevronUpPoints    = []vec{{-0.4, 0.175}, {0, -0.275}, {0.4, 0.175}}
	chevronDownPoints  = []vec{{-0.4, -0.175}, {0, 0.275}, {0.4, -0.175}}
)

func chevronBuilder(points []vec, width float64) func() []shape {
	return func() []shape {
		d := design{size: 1}
		mapped := mapPoints(d, points)
		var out shape
		// The old code stroked each arm separately, leaving the tip with butt
		// caps instead of a miter point. Two open polygons reproduce that.
		out.strokeOpen([]vec{mapped[0], mapped[1]}, width)
		out.strokeOpen([]vec{mapped[1], mapped[2]}, width)
		return []shape{out}
	}
}

func buildUndo() []shape {
	// Design in the same 14-unit box the header draws, so the saved geometry is
	// easy to reason about at the only size that uses this glyph.
	d := design{size: 14}
	radius := 5.0
	width := d.length(1.8)
	points := []vec{{radius, radius * 0.9}, {radius, 0}}
	const steps = 24
	for step := 1; step <= steps; step++ {
		angle := -math.Pi * float64(step) / steps
		points = append(points, vec{radius * math.Cos(angle), radius * math.Sin(angle)})
	}
	var out shape
	out.strokeOpen(mapPoints(d, points), width)

	// Keep the original return-arrow concept: the arc ends on the left and the
	// head points down. Widen it so the head stays visible at 14 dp.
	left := -radius
	head := []vec{
		{left, 4.8},
		{left - 2.0, 0.2},
		{left + 2.0, 0.2},
	}
	out.polygon(mapPoints(d, head))
	return []shape{out}
}

func buildMinimize() []shape {
	d := design{size: 11}
	var out shape
	out.rect(d.pt(-5, -0.5).x, d.pt(-5, -0.5).y, d.pt(5, 0.5).x, d.pt(5, 0.5).y)
	return []shape{out}
}

func buildMaximize() []shape {
	d := design{size: 11}
	var out shape
	// The old outline is painted inside the 10x10 box, not centred on it.
	out.rect(d.pt(-5, -5).x, d.pt(-5, -5).y, d.pt(5, -4).x, d.pt(5, -4).y)
	out.rect(d.pt(-5, 4).x, d.pt(-5, 4).y, d.pt(5, 5).x, d.pt(5, 5).y)
	out.rect(d.pt(-5, -5).x, d.pt(-5, -5).y, d.pt(-4, 5).x, d.pt(-4, 5).y)
	out.rect(d.pt(4, -5).x, d.pt(4, -5).y, d.pt(5, 5).x, d.pt(5, 5).y)
	return []shape{out}
}

func buildRestore() []shape {
	d := design{size: 13}
	var out shape
	// Rear square: only the edges that remain visible behind the front one.
	out.rect(d.pt(-3, -6).x, d.pt(-3, -6).y, d.pt(6, -5).x, d.pt(6, -5).y)
	out.rect(d.pt(5, -6).x, d.pt(5, -6).y, d.pt(6, 3).x, d.pt(6, 3).y)
	out.rect(d.pt(3, 2).x, d.pt(3, 2).y, d.pt(6, 3).x, d.pt(6, 3).y)
	out.rect(d.pt(-3, -6).x, d.pt(-3, -6).y, d.pt(-2, -3).x, d.pt(-2, -3).y)
	// Front square.
	out.rect(d.pt(-6, -3).x, d.pt(-6, -3).y, d.pt(3, -2).x, d.pt(3, -2).y)
	out.rect(d.pt(-6, 5).x, d.pt(-6, 5).y, d.pt(3, 6).x, d.pt(3, 6).y)
	out.rect(d.pt(-6, -3).x, d.pt(-6, -3).y, d.pt(-5, 6).x, d.pt(-5, 6).y)
	out.rect(d.pt(2, -3).x, d.pt(2, -3).y, d.pt(3, 6).x, d.pt(3, 6).y)
	return []shape{out}
}

func buildClose() []shape {
	d := design{size: 11}
	width := d.length(1.1)
	var out shape
	out.strokeLine(d.pt(-4.5, -4.5), d.pt(4.5, 4.5), width)
	out.strokeLine(d.pt(4.5, -4.5), d.pt(-4.5, 4.5), width)
	return []shape{out}
}

// --- small helpers -------------------------------------------------------

func mapPoints(d design, points []vec) []vec {
	out := make([]vec, len(points))
	for i, p := range points {
		out[i] = d.pt(p.x, p.y)
	}
	return out
}
