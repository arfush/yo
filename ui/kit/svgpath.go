package kit

import (
	"strconv"
	"sync"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/resource"
)

// iconBox is the coordinate box every icon in resource.Path uses.
const iconBox = 24

var (
	iconPathsMu sync.Mutex
	iconPaths   = map[resource.Path]*qt.QPainterPath{}
)

// iconPath parses and caches one path. The supported commands cover every
// glyph in the icon sprite: M, L, H, V, C and Z, absolute and relative.
func iconPath(d resource.Path) *qt.QPainterPath {
	iconPathsMu.Lock()
	defer iconPathsMu.Unlock()
	if path, ok := iconPaths[d]; ok {
		return path
	}
	path := parsePath(string(d))
	iconPaths[d] = path
	return path
}

// parsePath turns SVG path data into a QPainterPath.
func parsePath(d string) *qt.QPainterPath {
	tokens := tokenizePath(d)
	path := qt.NewQPainterPath()
	// SVG fills with the nonzero rule, which keeps overlapping subpaths solid.
	// The default Qt rule is even-odd and punches holes where they overlap.
	path.SetFillRule(qt.WindingFill)
	var cx, cy, sx, sy float64
	var cmd byte
	i := 0

	next := func() (float64, bool) {
		if i >= len(tokens) || tokens[i].cmd != 0 {
			return 0, false
		}
		v := tokens[i].num
		i++
		return v, true
	}
	hasNum := func() bool {
		return i < len(tokens) && tokens[i].cmd == 0
	}

	for i < len(tokens) {
		if tokens[i].cmd != 0 {
			cmd = tokens[i].cmd
			i++
		}
		switch cmd {
		case 'M', 'm':
			relative := cmd == 'm'
			x, ok := next()
			if !ok {
				return path
			}
			y, ok := next()
			if !ok {
				return path
			}
			if relative {
				cx += x
				cy += y
			} else {
				cx, cy = x, y
			}
			path.MoveTo2(cx, cy)
			sx, sy = cx, cy
			if relative {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L', 'l':
			for hasNum() {
				x, _ := next()
				y, _ := next()
				if cmd == 'l' {
					cx += x
					cy += y
				} else {
					cx, cy = x, y
				}
				path.LineTo2(cx, cy)
			}
		case 'H', 'h':
			for hasNum() {
				x, _ := next()
				if cmd == 'h' {
					cx += x
				} else {
					cx = x
				}
				path.LineTo2(cx, cy)
			}
		case 'V', 'v':
			for hasNum() {
				y, _ := next()
				if cmd == 'v' {
					cy += y
				} else {
					cy = y
				}
				path.LineTo2(cx, cy)
			}
		case 'C', 'c':
			for hasNum() {
				x1, _ := next()
				y1, _ := next()
				x2, _ := next()
				y2, _ := next()
				x, _ := next()
				y, _ := next()
				if cmd == 'c' {
					x1, y1 = cx+x1, cy+y1
					x2, y2 = cx+x2, cy+y2
					x, y = cx+x, cy+y
				}
				path.CubicTo2(x1, y1, x2, y2, x, y)
				cx, cy = x, y
			}
		case 'Z', 'z':
			path.CloseSubpath()
			cx, cy = sx, sy
		default:
			return path
		}
	}
	return path
}

type pathToken struct {
	cmd byte
	num float64
}

// tokenizePath splits path data into command letters and numbers.
func tokenizePath(d string) []pathToken {
	tokens := make([]pathToken, 0, len(d)/3)
	for i := 0; i < len(d); {
		c := d[i]
		switch {
		case c == ' ' || c == ',' || c == '\n' || c == '\r' || c == '\t':
			i++
		case isPathCommand(c):
			tokens = append(tokens, pathToken{cmd: c})
			i++
		default:
			start := i
			if i < len(d) && (d[i] == '-' || d[i] == '+') {
				i++
			}
			for i < len(d) && d[i] >= '0' && d[i] <= '9' {
				i++
			}
			if i < len(d) && d[i] == '.' {
				i++
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
				}
			}
			if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
				i++
				if i < len(d) && (d[i] == '-' || d[i] == '+') {
					i++
				}
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
				}
			}
			if i == start {
				i++
				continue
			}
			v, err := strconv.ParseFloat(d[start:i], 64)
			if err == nil {
				tokens = append(tokens, pathToken{num: v})
			}
		}
	}
	return tokens
}

func isPathCommand(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'Z', 'z':
		return true
	}
	return false
}
