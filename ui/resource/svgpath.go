package resource

import (
	"fmt"
	"strconv"

	qt "github.com/mappu/miqt/qt6"
)

// pathToken is one command letter or numeric operand from SVG path data. cmd
// is zero for numbers. pos is the byte offset in the source, for error
// messages.
type pathToken struct {
	cmd byte
	num float64
	pos int
}

// parsePath turns SVG path data into a QPainterPath. Supported commands are
// M/m, L/l, H/h, V/v, C/c and Z/z; the icon sprite uses only M, L, C and Z.
// Unsupported commands and malformed data are errors, so a broken icon fails
// at startup instead of producing a partial glyph.
func parsePath(d string) (*qt.QPainterPath, error) {
	tokens, err := tokenizePath(d)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty path data")
	}
	if tokens[0].cmd == 0 {
		return nil, fmt.Errorf("path must start with M or m")
	}
	if tokens[0].cmd != 'M' && tokens[0].cmd != 'm' {
		return nil, fmt.Errorf("path must start with M or m, got %q at byte %d",
			tokens[0].cmd, tokens[0].pos)
	}

	path := qt.NewQPainterPath()
	// SVG fills with the nonzero rule, which keeps overlapping subpaths solid.
	// The default Qt rule is even-odd and punches holes where they overlap.
	path.SetFillRule(qt.WindingFill)

	var cx, cy, sx, sy float64
	var cmd byte
	var cmdPos int
	i := 0

	hasNum := func() bool {
		return i < len(tokens) && tokens[i].cmd == 0
	}
	next := func() (float64, error) {
		if i >= len(tokens) || tokens[i].cmd != 0 {
			return 0, fmt.Errorf("command %q at byte %d: missing argument", cmd, cmdPos)
		}
		v := tokens[i].num
		i++
		return v, nil
	}
	fail := func(err error) (*qt.QPainterPath, error) {
		path.Delete()
		return nil, err
	}

	for i < len(tokens) {
		if tokens[i].cmd != 0 {
			cmd = tokens[i].cmd
			cmdPos = tokens[i].pos
			i++
			if cmd != 'Z' && cmd != 'z' && !hasNum() {
				return fail(fmt.Errorf("command %q at byte %d has no arguments", cmd, cmdPos))
			}
		} else if cmd == 0 {
			return fail(fmt.Errorf("unexpected number at byte %d", tokens[i].pos))
		}

		switch cmd {
		case 'M', 'm':
			x, err := next()
			if err != nil {
				return fail(err)
			}
			y, err := next()
			if err != nil {
				return fail(err)
			}
			if cmd == 'm' {
				cx += x
				cy += y
			} else {
				cx, cy = x, y
			}
			path.MoveTo2(cx, cy)
			sx, sy = cx, cy
			// Further coordinate pairs after M are implicit linetos.
			if cmd == 'm' {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L', 'l':
			for hasNum() {
				x, err := next()
				if err != nil {
					return fail(err)
				}
				y, err := next()
				if err != nil {
					return fail(err)
				}
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
				x, err := next()
				if err != nil {
					return fail(err)
				}
				if cmd == 'h' {
					cx += x
				} else {
					cx = x
				}
				path.LineTo2(cx, cy)
			}
		case 'V', 'v':
			for hasNum() {
				y, err := next()
				if err != nil {
					return fail(err)
				}
				if cmd == 'v' {
					cy += y
				} else {
					cy = y
				}
				path.LineTo2(cx, cy)
			}
		case 'C', 'c':
			for hasNum() {
				x1, err := next()
				if err != nil {
					return fail(err)
				}
				y1, err := next()
				if err != nil {
					return fail(err)
				}
				x2, err := next()
				if err != nil {
					return fail(err)
				}
				y2, err := next()
				if err != nil {
					return fail(err)
				}
				x, err := next()
				if err != nil {
					return fail(err)
				}
				y, err := next()
				if err != nil {
					return fail(err)
				}
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
			if hasNum() {
				return fail(fmt.Errorf("unexpected number at byte %d after Z", tokens[i].pos))
			}
		default:
			return fail(fmt.Errorf("unsupported command %q at byte %d", cmd, cmdPos))
		}
	}
	return path, nil
}

// tokenizePath splits path data into command letters and numbers. Every ASCII
// letter becomes a command token; parsePath rejects the unsupported ones.
// Malformed numbers are errors rather than silently skipped text.
func tokenizePath(d string) ([]pathToken, error) {
	tokens := make([]pathToken, 0, len(d)/3)
	for i := 0; i < len(d); {
		c := d[i]
		switch {
		case c == ' ' || c == ',' || c == '\n' || c == '\r' || c == '\t':
			i++
		case isPathLetter(c):
			tokens = append(tokens, pathToken{cmd: c, pos: i})
			i++
		default:
			start := i
			if c == '-' || c == '+' {
				i++
			}
			digits := 0
			for i < len(d) && d[i] >= '0' && d[i] <= '9' {
				i++
				digits++
			}
			if i < len(d) && d[i] == '.' {
				i++
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
					digits++
				}
			}
			if digits == 0 {
				return nil, fmt.Errorf("unexpected %q at byte %d", c, start)
			}
			if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
				exp := i
				i++
				if i < len(d) && (d[i] == '-' || d[i] == '+') {
					i++
				}
				expDigits := 0
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
					expDigits++
				}
				if expDigits == 0 {
					return nil, fmt.Errorf("malformed exponent at byte %d", exp)
				}
			}
			v, err := strconv.ParseFloat(d[start:i], 64)
			if err != nil {
				return nil, fmt.Errorf("malformed number %q at byte %d", d[start:i], start)
			}
			tokens = append(tokens, pathToken{num: v, pos: start})
		}
	}
	return tokens, nil
}

func isPathLetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
