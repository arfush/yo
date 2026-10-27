package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// The icon sheet is authored in a 24×24 glyph box with 4.25 units of padding
// inside every 32.5-unit button tile. Keeping the SVG as the source of truth
// lets the generator consume the exact curves from the design instead of
// approximating them with hand-written primitives.
const (
	svgTilePadding = 4.25
	svgGlyphBox    = 24.0
	svgTrashBox    = 26.5
)

type svgSheet struct {
	boxes  []svgBox
	small  []svgBox
	glyphs []svgGlyph
}

type svgBox struct {
	x, y, w, h float64
}

func (b svgBox) iconRect() (x, y, size float64) {
	return b.x + svgTilePadding, b.y + svgTilePadding, b.w - 2*svgTilePadding
}

type svgGlyph struct {
	width float64
	subs  []svgSubpath
}

type svgSubpath struct {
	points []vec
	closed bool
}

func loadIconSheet(path string) ([]iconSpec, error) {
	sheet, err := readIconSheet(path)
	if err != nil {
		return nil, err
	}
	if len(sheet.boxes) != len(svgIconNames) {
		return nil, fmt.Errorf("icon sheet has %d large boxes, want %d", len(sheet.boxes), len(svgIconNames))
	}
	assignments := make([][]svgGlyph, len(sheet.boxes))
	smallGlyphs := make([]svgGlyph, len(sheet.small))
	for _, glyph := range sheet.glyphs {
		box, ok := glyphBox(glyph, sheet.small)
		if ok {
			for i := range sheet.small {
				if sheet.small[i] == box {
					smallGlyphs[i] = glyph
				}
			}
			continue
		}
		index := -1
		for i, candidate := range sheet.boxes {
			if glyphInsideBox(glyph, candidate) {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("glyph at %v is outside every icon box", glyph.center())
		}
		assignments[index] = append(assignments[index], glyph)
	}

	specs := make([]iconSpec, 0, len(sheet.boxes)+len(sheet.small))
	for i, box := range sheet.boxes {
		if len(assignments[i]) == 0 {
			return nil, fmt.Errorf("icon %q has no paths", svgIconNames[i])
		}
		box := box
		glyphs := assignments[i]
		specs = append(specs, iconSpec{
			name: svgIconNames[i],
			build: func() []shape {
				return []shape{buildSVGGlyphs(glyphs, box)}
			},
		})
	}
	for i, box := range sheet.small {
		if len(smallGlyphs) == 0 || len(smallGlyphs[i].subs) == 0 {
			continue
		}
		box := box
		glyph := smallGlyphs[i]
		specs = append(specs, iconSpec{
			name: "trash_small",
			build: func() []shape {
				return []shape{buildSVGGlyphs([]svgGlyph{glyph}, box)}
			},
		})
	}
	return specs, nil
}

func readIconSheet(path string) (svgSheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return svgSheet{}, fmt.Errorf("read icon sheet: %w", err)
	}
	var sheet svgSheet
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return svgSheet{}, fmt.Errorf("parse icon sheet: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "rect":
			fill := svgAttr(start, "fill")
			if fill != "#1A1822" {
				continue
			}
			box := svgBox{
				x: svgFloat(start, "x"),
				y: svgFloat(start, "y"),
				w: svgFloat(start, "width"),
				h: svgFloat(start, "height"),
			}
			if box.w <= 0 || box.h <= 0 {
				continue
			}
			if box.w < 30 {
				sheet.small = append(sheet.small, box)
			} else {
				sheet.boxes = append(sheet.boxes, box)
			}
		case "path":
			d := svgAttr(start, "d")
			width := svgFloat(start, "stroke-width")
			if d == "" || width <= 0 {
				continue
			}
			// The masked maximize glyph uses a doubled stroke so only its inner
			// half remains visible. Flatten it to the half-width the eye sees.
			if width > 2 {
				width = 1.6
			}
			subs, err := parseSVGPath(d)
			if err != nil {
				return svgSheet{}, fmt.Errorf("parse path %q: %w", d, err)
			}
			sheet.glyphs = append(sheet.glyphs, svgGlyph{width: width, subs: subs})
		}
	}
	sort.SliceStable(sheet.boxes, func(i, j int) bool {
		if sheet.boxes[i].y == sheet.boxes[j].y {
			return sheet.boxes[i].x < sheet.boxes[j].x
		}
		return sheet.boxes[i].y < sheet.boxes[j].y
	})
	sort.SliceStable(sheet.small, func(i, j int) bool {
		if sheet.small[i].y == sheet.small[j].y {
			return sheet.small[i].x < sheet.small[j].x
		}
		return sheet.small[i].y < sheet.small[j].y
	})
	return sheet, nil
}

func svgAttr(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func svgFloat(start xml.StartElement, name string) float64 {
	value, _ := strconv.ParseFloat(strings.TrimSpace(svgAttr(start, name)), 64)
	return value
}

func (g svgGlyph) center() vec {
	min, max := g.bounds()
	return vec{x: (min.x + max.x) / 2, y: (min.y + max.y) / 2}
}

func (g svgGlyph) bounds() (vec, vec) {
	min := vec{x: math.Inf(1), y: math.Inf(1)}
	max := vec{x: math.Inf(-1), y: math.Inf(-1)}
	for _, sub := range g.subs {
		for _, p := range sub.points {
			min.x = math.Min(min.x, p.x)
			min.y = math.Min(min.y, p.y)
			max.x = math.Max(max.x, p.x)
			max.y = math.Max(max.y, p.y)
		}
	}
	return min, max
}

func glyphBox(glyph svgGlyph, boxes []svgBox) (svgBox, bool) {
	c := glyph.center()
	for _, box := range boxes {
		if c.x >= box.x && c.x <= box.x+box.w && c.y >= box.y && c.y <= box.y+box.h {
			return box, true
		}
	}
	return svgBox{}, false
}

func glyphInsideBox(glyph svgGlyph, box svgBox) bool {
	c := glyph.center()
	return c.x >= box.x && c.x <= box.x+box.w && c.y >= box.y && c.y <= box.y+box.h
}

func buildSVGGlyphs(glyphs []svgGlyph, box svgBox) shape {
	var out shape
	originX, originY, size := box.iconRect()
	for _, glyph := range glyphs {
		width := glyph.width / size
		for _, sub := range glyph.subs {
			points := make([]vec, 0, len(sub.points))
			for _, p := range sub.points {
				points = append(points, vec{
					x: (p.x - originX) / size,
					y: (p.y - originY) / size,
				})
			}
			points = trimDuplicatePoints(points)
			if len(points) == 0 {
				continue
			}
			if sub.closed {
				out.strokeClosed(points, width)
			} else {
				out.strokeRoundOpen(points, width)
			}
		}
	}
	return out
}

func trimDuplicatePoints(points []vec) []vec {
	if len(points) < 2 {
		return points
	}
	out := points[:1]
	for _, p := range points[1:] {
		if p.sub(out[len(out)-1]).len() < 0.00001 {
			continue
		}
		out = append(out, p)
	}
	if len(out) > 2 && out[0].sub(out[len(out)-1]).len() < 0.00001 {
		out = out[:len(out)-1]
	}
	return out
}

var svgIconNames = []string{
	"update",
	"people",
	"log",
	"settings",
	"bell",
	"play",
	"link",
	"keyboard",
	"trash",
	"pause",
	"autoscroll_on",
	"autoscroll_off",
	"minimize",
	"maximize",
	"restore",
	"close",
	"minimize_active",
	"maximize_active",
	"restore_active",
	"close_active",
}

// --- SVG path parsing ----------------------------------------------------

type svgPathToken struct {
	command bool
	letter  byte
	number  float64
}

func parseSVGPath(d string) ([]svgSubpath, error) {
	tokens, err := tokenizeSVGPath(d)
	if err != nil {
		return nil, err
	}
	var (
		subs      []svgSubpath
		current   []vec
		start     vec
		pos       vec
		prevCubic vec
		prevQuad  vec
		lastKind  byte
		index     int
		closePath func(bool)
	)
	flush := func(closed bool) {
		if len(current) == 0 {
			return
		}
		points := append([]vec(nil), current...)
		if closed && len(points) > 2 && points[0].sub(points[len(points)-1]).len() > 0.00001 {
			points = append(points, points[0])
		}
		subs = append(subs, svgSubpath{points: points, closed: closed})
		current = nil
	}
	closePath = func(closed bool) {
		flush(closed)
	}

	read := func() (float64, error) {
		if index >= len(tokens) || tokens[index].command {
			return 0, fmt.Errorf("expected number")
		}
		n := tokens[index].number
		index++
		return n, nil
	}
	readPoint := func() (vec, error) {
		x, err := read()
		if err != nil {
			return vec{}, err
		}
		y, err := read()
		if err != nil {
			return vec{}, err
		}
		return vec{x: x, y: y}, nil
	}

	for index < len(tokens) {
		token := tokens[index]
		if token.command {
			index++
			switch token.letter {
			case 'Z', 'z':
				closePath(true)
				pos = start
				lastKind = 'Z'
				continue
			}
			if token.letter == 'M' || token.letter == 'm' {
				p, err := readPoint()
				if err != nil {
					return nil, err
				}
				if token.letter == 'm' {
					p = pos.add(p)
				}
				flush(false)
				start = p
				pos = p
				current = []vec{p}
				lastKind = 'M'
				// Additional coordinate pairs after a move are implicit lines.
				for index < len(tokens) && !tokens[index].command {
					p, err = readPoint()
					if err != nil {
						return nil, err
					}
					if token.letter == 'm' {
						p = pos.add(p)
					}
					current = append(current, p)
					pos = p
					lastKind = 'L'
				}
				continue
			}
			for index < len(tokens) && !tokens[index].command {
				switch token.letter {
				case 'L', 'l':
					p, err := readPoint()
					if err != nil {
						return nil, err
					}
					if token.letter == 'l' {
						p = pos.add(p)
					}
					current = append(current, p)
					pos = p
					lastKind = 'L'
				case 'H', 'h':
					x, err := read()
					if err != nil {
						return nil, err
					}
					p := vec{x: x, y: pos.y}
					if token.letter == 'h' {
						p.x = pos.x + x
					}
					current = append(current, p)
					pos = p
					lastKind = 'L'
				case 'V', 'v':
					y, err := read()
					if err != nil {
						return nil, err
					}
					p := vec{x: pos.x, y: y}
					if token.letter == 'v' {
						p.y = pos.y + y
					}
					current = append(current, p)
					pos = p
					lastKind = 'L'
				case 'C', 'c':
					c1, err := readPoint()
					if err != nil {
						return nil, err
					}
					c2, err := readPoint()
					if err != nil {
						return nil, err
					}
					p, err := readPoint()
					if err != nil {
						return nil, err
					}
					if token.letter == 'c' {
						c1 = pos.add(c1)
						c2 = pos.add(c2)
						p = pos.add(p)
					}
					steps := curveSteps(pos, c1, c2, p)
					sampled := sampleCubic(pos, c1, c2, p, steps)
					current = append(current, sampled[1:]...)
					pos = p
					prevCubic = c2
					lastKind = 'C'
				case 'S', 's':
					c1 := pos
					if lastKind == 'C' || lastKind == 'S' {
						c1 = pos.mul(2).sub(prevCubic)
					}
					c2, err := readPoint()
					if err != nil {
						return nil, err
					}
					p, err := readPoint()
					if err != nil {
						return nil, err
					}
					if token.letter == 's' {
						c2 = pos.add(c2)
						p = pos.add(p)
					}
					steps := curveSteps(pos, c1, c2, p)
					sampled := sampleCubic(pos, c1, c2, p, steps)
					current = append(current, sampled[1:]...)
					pos = p
					prevCubic = c2
					lastKind = 'S'
				case 'Q', 'q':
					c, err := readPoint()
					if err != nil {
						return nil, err
					}
					p, err := readPoint()
					if err != nil {
						return nil, err
					}
					if token.letter == 'q' {
						c = pos.add(c)
						p = pos.add(p)
					}
					sampled := sampleQuadratic(pos, c, p, curveSteps(pos, c, c, p))
					current = append(current, sampled[1:]...)
					pos = p
					prevQuad = c
					lastKind = 'Q'
				case 'T', 't':
					c := pos
					if lastKind == 'Q' || lastKind == 'T' {
						c = pos.mul(2).sub(prevQuad)
					}
					p, err := readPoint()
					if err != nil {
						return nil, err
					}
					if token.letter == 't' {
						p = pos.add(p)
					}
					sampled := sampleQuadratic(pos, c, p, curveSteps(pos, c, c, p))
					current = append(current, sampled[1:]...)
					pos = p
					prevQuad = c
					lastKind = 'T'
				default:
					return nil, fmt.Errorf("unsupported command %q", token.letter)
				}
			}
			continue
		}
		return nil, fmt.Errorf("unexpected number %v", token.number)
	}
	flush(false)
	return subs, nil
}

func tokenizeSVGPath(d string) ([]svgPathToken, error) {
	var tokens []svgPathToken
	for i := 0; i < len(d); {
		c := d[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',':
			i++
		case isSVGCommand(c):
			tokens = append(tokens, svgPathToken{command: true, letter: c})
			i++
		default:
			start := i
			if d[i] == '+' || d[i] == '-' {
				i++
			}
			digits := false
			for i < len(d) && d[i] >= '0' && d[i] <= '9' {
				i++
				digits = true
			}
			if i < len(d) && d[i] == '.' {
				i++
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
					digits = true
				}
			}
			if !digits {
				return nil, fmt.Errorf("invalid number at byte %d", start)
			}
			if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
				i++
				if i < len(d) && (d[i] == '+' || d[i] == '-') {
					i++
				}
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
				}
			}
			value, err := strconv.ParseFloat(d[start:i], 64)
			if err != nil {
				return nil, fmt.Errorf("parse number %q: %w", d[start:i], err)
			}
			tokens = append(tokens, svgPathToken{number: value})
		}
	}
	return tokens, nil
}

func isSVGCommand(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'Z', 'z':
		return true
	}
	return false
}

func curveSteps(p0, c1, c2, p3 vec) int {
	length := p0.sub(c1).len() + c1.sub(c2).len() + c2.sub(p3).len()
	steps := int(math.Ceil(length * 1.5))
	if steps < 8 {
		steps = 8
	}
	if steps > 28 {
		steps = 28
	}
	return steps
}

func sampleQuadratic(p0, c, p1 vec, steps int) []vec {
	if steps < 2 {
		steps = 2
	}
	out := make([]vec, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		u := 1 - t
		out = append(out, vec{
			x: u*u*p0.x + 2*u*t*c.x + t*t*p1.x,
			y: u*u*p0.y + 2*u*t*c.y + t*t*p1.y,
		})
	}
	return out
}
