// Command icongen turns yoanime's original painted icon geometry into IconVG
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

func (s *shape) strokeRoundLine(a, b vec, width float64) {
	s.strokeLine(a, b, width)
	s.circle(a, width/2)
	s.circle(b, width/2)
}

// strokeRoundOpen strokes a polyline and rounds both ends. Drawing the two arms
// of a corner as separate round-capped lines also produces a round join.
func (s *shape) strokeRoundOpen(points []vec, width float64) {
	s.strokeOpen(points, width)
	if len(points) == 0 {
		return
	}
	s.circle(points[0], width/2)
	s.circle(points[len(points)-1], width/2)
}

func (s *shape) circle(center vec, radius float64) {
	if radius <= 0 {
		return
	}
	s.polygon(sampleCircle(center.x, center.y, radius, 32))
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
	d1 := p.sub(prev)
	d2 := next.sub(p)
	// sampleCubic includes its start point, so callers that append its result to
	// an existing polyline end up with duplicate points. A zero-length direction
	// would otherwise make the miter scale 1/max(0.25, 0) = 4 and paint a spike
	// four times the half-width.
	if d1.len() < 0.000001 {
		if d2.len() < 0.000001 {
			return vec{}
		}
		return d2.normal().unit()
	}
	if d2.len() < 0.000001 {
		return d1.normal().unit()
	}
	n1 := d1.normal().unit()
	n2 := d2.normal().unit()
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

// appendCubic appends a sampled cubic to an existing polyline. sampleCubic
// returns the starting point as its first element, which is already the last
// point in dst; dropping it keeps the polyline free of duplicate points.
func appendCubic(dst []vec, p0, c1, c2, p3 vec, steps int) []vec {
	points := sampleCubic(p0, c1, c2, p3, steps)
	return append(dst, points[1:]...)
}

func sampleCircle(cx, cy, radius float64, steps int) []vec {
	out := make([]vec, steps)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(steps)
		out[i] = vec{cx + radius*math.Cos(a), cy + radius*math.Sin(a)}
	}
	return out
}

// sampleArc returns a flattened circular arc. Angles are in degrees and grow
// clockwise on screen because y points down.
func sampleArc(cx, cy, radius, startDeg, endDeg float64, steps int) []vec {
	if steps < 1 {
		steps = 1
	}
	out := make([]vec, steps+1)
	for i := range out {
		a := (startDeg + (endDeg-startDeg)*float64(i)/float64(steps)) * math.Pi / 180
		out[i] = vec{cx + radius*math.Cos(a), cy + radius*math.Sin(a)}
	}
	return out
}

// sampleRoundedPolygon replaces every corner with a quadratic-like cubic. It
// lets small glyphs keep soft corners without hand-writing each curve.
func sampleRoundedPolygon(points []vec, radius float64, steps int) []vec {
	if len(points) < 3 || radius <= 0 {
		return append([]vec(nil), points...)
	}
	if steps < 2 {
		steps = 2
	}
	const kappa = 0.5522847498
	out := make([]vec, 0, len(points)*(steps+1))
	for i, p := range points {
		prev := points[(i+len(points)-1)%len(points)]
		next := points[(i+1)%len(points)]
		in := prev.sub(p)
		nextDir := next.sub(p)
		if in.len() < 0.000001 || nextDir.len() < 0.000001 {
			continue
		}
		cut := math.Min(radius, math.Min(in.len(), nextDir.len())/2)
		a := p.add(in.unit().mul(cut))
		b := p.add(nextDir.unit().mul(cut))
		if len(out) == 0 {
			out = append(out, a)
		}
		c1 := a.add(p.sub(a).mul(kappa))
		c2 := b.add(p.sub(b).mul(kappa))
		out = append(out, sampleCubic(a, c1, c2, b, steps)[1:]...)
	}
	if len(out) == 0 {
		return append([]vec(nil), points...)
	}
	return out
}

// arrowhead builds a filled, softly rounded triangle at the end of a stroked
// arc. dir is the tangent at the end, so the tip continues the arc's motion.
func arrowhead(end, dir vec, length, halfWidth, radius float64) []vec {
	dir = dir.unit()
	normal := dir.normal()
	base1 := end.add(normal.mul(halfWidth))
	base2 := end.sub(normal.mul(halfWidth))
	tip := end.add(dir.mul(length))
	return sampleRoundedPolygon([]vec{base1, tip, base2}, radius, 5)
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
	{"search", buildSearch},
	{"pause", buildPause},
	{"info", buildInfo},
	{"warning", buildWarning},
	{"error", buildError},
	{"check", buildCheck},
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

// buildUpdate draws two circular arrows. The heads are filled, softly rounded
// triangles that continue the arc's tangent, so the glyph reads as motion at
// footer size instead of two open right angles.
func buildUpdate() []shape {
	d := design{size: 14.6}
	const (
		radius     = 6.0
		headLength = 3.3
		headHalf   = 1.8
	)
	width := d.length(1.55)
	top := sampleArc(0, 0, radius, 190, 340, 28)
	bottom := sampleArc(0, 0, radius, 10, 160, 28)

	var out shape
	out.strokeRoundOpen(mapPoints(d, top), width)
	out.strokeRoundOpen(mapPoints(d, bottom), width)
	out.polygon(mapPoints(d, arrowhead(
		top[len(top)-1],
		top[len(top)-1].sub(top[len(top)-2]),
		headLength,
		headHalf,
		0.65,
	)))
	out.polygon(mapPoints(d, arrowhead(
		bottom[len(bottom)-1],
		bottom[len(bottom)-1].sub(bottom[len(bottom)-2]),
		headLength,
		headHalf,
		0.65,
	)))
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
	body = appendCubic(
		body,
		pt(2, 19), pt(2, 16.79), pt(4.21, 15), pt(6, 15), 16,
	)
	body = append(body, pt(12, 15))
	body = appendCubic(
		body,
		pt(12, 15), pt(13.79, 15), pt(16, 16.79), pt(16, 19), 16,
	)
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

// buildPlay is a rounded outlined triangle followed by a short stop bar. The
// triangle is about nine units wide and the bar sits one unit away, matching
// the optical rhythm of the footer controls at 14 dp.
func buildPlay() []shape {
	d := design{size: 13.6}
	width := d.length(1.9)
	triangle := sampleRoundedPolygon([]vec{
		{-5.0, -4.6},
		{2.2, 0},
		{-5.0, 4.6},
	}, 1.0, 6)

	var out shape
	out.strokeClosed(mapPoints(d, triangle), width)
	out.strokeRoundLine(d.pt(5.2, -4.6), d.pt(5.2, 4.6), d.length(2.0))
	return []shape{out}
}

// buildTrash keeps the tapered can from the reference bin: a short handle, a
// wide lid, a softly rounded body and two ribs. The lid and body are separate
// strokes so the top edge does not read as a double line.
func buildTrash() []shape {
	d := design{size: 14.6}
	width := d.length(1.8)
	var out shape

	out.strokeRoundLine(d.pt(-5.3, -4.3), d.pt(5.3, -4.3), width)
	out.strokeRoundLine(d.pt(-2.2, -6.0), d.pt(2.2, -6.0), d.length(1.5))

	body := []vec{d.pt(-4.5, -3.1), d.pt(-4.0, 4.2)}
	body = appendCubic(body, d.pt(-4.0, 4.2), d.pt(-4.0, 4.9), d.pt(-3.5, 5.4), d.pt(-2.9, 5.4), 8)
	body = append(body, d.pt(2.9, 5.4))
	body = appendCubic(body, d.pt(2.9, 5.4), d.pt(3.5, 5.4), d.pt(4.0, 4.9), d.pt(4.0, 4.2), 8)
	body = append(body, d.pt(4.5, -3.1))
	out.strokeRoundOpen(body, width)

	out.strokeRoundLine(d.pt(-1.55, -1.0), d.pt(-1.55, 4.0), d.length(1.15))
	out.strokeRoundLine(d.pt(1.55, -1.0), d.pt(1.55, 4.0), d.length(1.15))
	return []shape{out}
}

// buildSearch is the magnifier used by the log toolbar.
func buildSearch() []shape {
	d := design{size: 14.0}
	width := d.length(1.5)
	var out shape
	out.strokeClosed(sampleCircle(
		0.5+d.length(-1.0),
		0.5+d.length(-1.0),
		d.length(4.5),
		64,
	), width)
	out.strokeRoundLine(d.pt(2.3, 2.3), d.pt(5.6, 5.6), d.length(1.7))
	return []shape{out}
}

// buildPause is a pair of round-capped bars.
func buildPause() []shape {
	d := design{size: 12.0}
	width := d.length(2.1)
	var out shape
	out.strokeRoundLine(d.pt(-2.6, -4.2), d.pt(-2.6, 4.2), width)
	out.strokeRoundLine(d.pt(2.6, -4.2), d.pt(2.6, 4.2), width)
	return []shape{out}
}

// buildInfo is a circled lowercase i.
func buildInfo() []shape {
	d := design{size: 16.0}
	width := d.length(1.35)
	var out shape
	out.strokeClosed(sampleCircle(0.5, 0.5, d.length(6.8), 64), width)
	out.circle(d.pt(0, -3.4), d.length(1.15))
	out.strokeRoundLine(d.pt(0, -0.9), d.pt(0, 3.4), d.length(1.7))
	return []shape{out}
}

// buildWarning is a rounded triangle with an exclamation mark.
func buildWarning() []shape {
	d := design{size: 15.2}
	width := d.length(1.35)
	triangle := sampleRoundedPolygon([]vec{
		{0, -6.4},
		{-6.8, 5.8},
		{6.8, 5.8},
	}, 1.1, 6)

	var out shape
	out.strokeClosed(mapPoints(d, triangle), width)
	out.strokeRoundLine(d.pt(0, -2.5), d.pt(0, 1.8), d.length(1.6))
	out.circle(d.pt(0, 4.0), d.length(1.05))
	return []shape{out}
}

// buildError is a circled exclamation mark.
func buildError() []shape {
	d := design{size: 16.0}
	width := d.length(1.35)
	var out shape
	out.strokeClosed(sampleCircle(0.5, 0.5, d.length(6.8), 64), width)
	out.strokeRoundLine(d.pt(0, -3.6), d.pt(0, 1.8), d.length(1.7))
	out.circle(d.pt(0, 4.0), d.length(1.1))
	return []shape{out}
}

// buildCheck is the tick inside the autoscroll checkbox.
func buildCheck() []shape {
	d := design{size: 10.0}
	var out shape
	out.strokeRoundOpen(mapPoints(d, []vec{
		{-3.8, 0.2},
		{-1.2, 2.8},
		{3.8, -2.8},
	}), d.length(1.8))
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
