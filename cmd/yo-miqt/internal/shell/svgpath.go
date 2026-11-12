package shell

import (
	"image"
	"image/color"
	"math"
	"strconv"

	qt "github.com/mappu/miqt/qt6"
)

// svgIcon is one glyph lifted from home.svg. The bounds keep the original
// design position and size; the renderer centers that box in its target.
type svgIcon struct {
	d      string
	x, y   float64
	w, h   float64
	stroke float64
	filled bool
}

// svgIcons are the paths of home.svg. Every glyph is drawn at its design size
// instead of being stretched to the target box, so strokes keep their weight.
var svgIcons = map[string]svgIcon{
	"torii": {
		d: "M25.7305 17.5H40.2695C40.9495 17.5 41.5 18.04 41.5 18.7V19.7C41.5 19.87 41.4102 19.95 41.2402 19.95H38.9902V23.35H39.3496C39.7696 23.35 40.1201 23.69 40.1201 24.1V24.8C40.1201 25.21 39.7696 25.55 39.3496 25.55H38.9902V29.3C38.9902 29.96 38.4398 30.5 37.7598 30.5H37.5098C36.8298 30.5 36.2803 29.96 36.2803 29.3V25.55H29.7197V29.3C29.7197 29.96 29.1702 30.5 28.4902 30.5H28.2402C27.5602 30.5 27.0098 29.96 27.0098 29.3V25.55H26.6504C26.2304 25.55 25.8799 25.21 25.8799 24.8V24.1C25.8799 23.69 26.2304 23.35 26.6504 23.35H27.0098V19.95H24.7598C24.5898 19.95 24.5 19.87 24.5 19.7V18.7C24.5 18.04 25.0505 17.5 25.7305 17.5ZM29.7197 19.95V23.35H31.8701V19.95H29.7197ZM34.1299 19.95V23.35H36.2803V19.95H34.1299Z",
		x: 24.5, y: 17.5, w: 17, h: 13, filled: true,
	},
	"minimize": {
		d: "M732.9 24H741.1",
		x: 732.9, y: 23.2, w: 8.2, h: 1.6, stroke: 1.6,
	},
	"maximize": {
		d: "M770.9 18.75H779.1C779.7 18.75 780.2 19.27 780.2 19.92V28.0801C780.2 28.7301 779.7 29.25 779.1 29.25H770.9C770.3 29.25 769.8 28.7301 769.8 28.0801V19.92C769.8 19.27 770.3 18.75 770.9 18.75Z",
		x: 769.8, y: 18.75, w: 10.4, h: 10.5, stroke: 1.6,
	},
	"unmaximize": {
		d: "M4.3 1.3H10.63C11.28 1.3 11.8 1.82 11.8 2.47V8.8M8.8 4.3V10.63C8.8 11.28 8.28 11.8 7.63 11.8H2.47C1.82 11.8 1.3 11.28 1.3 10.63V5.47C1.3 4.82 1.82 4.3 2.47 4.3H8.8",
		x: 0, y: 0, w: 12.1, h: 12.1, stroke: 1.6,
	},
	"close": {
		d: "M816.5 20.5L813 24M813 24L809.5 27.5M813 24L809.5 20.5M813 24L816.5 27.5",
		x: 809.5, y: 20.5, w: 7, h: 7, stroke: 1.6,
	},
	"link": {
		d: "M36.6701 111.67C36.9501 112.05 37.3204 112.37 37.7404 112.6C38.1604 112.82 38.6198 112.96 39.0998 112.99C39.5698 113.03 40.0502 112.96 40.5002 112.79C40.9502 112.63 41.3606 112.36 41.6906 112.03L43.6906 110.03C44.3006 109.4 44.6401 108.56 44.6301 107.68C44.6201 106.81 44.2706 105.97 43.6506 105.35C43.0306 104.74 42.2005 104.39 41.3205 104.38C40.4505 104.37 39.6107 104.71 38.9807 105.31L37.8303 106.45M39.3303 110.33C39.0503 109.95 38.68 109.63 38.26 109.4C37.84 109.18 37.3806 109.04 36.9006 109.01C36.4306 108.97 35.9502 109.04 35.5002 109.21C35.0502 109.37 34.6398 109.64 34.3098 109.97L32.3098 111.97C31.6998 112.6 31.3603 113.44 31.3703 114.32C31.3803 115.19 31.7298 116.03 32.3498 116.65C32.9698 117.26 33.7999 117.61 34.6799 117.62C35.5499 117.63 36.3897 117.29 37.0197 116.69L38.1604 115.55",
		x: 31.37, y: 104.37, w: 13.26, h: 13.25, stroke: 1.6,
	},
	"arrow": {
		d: "M729.3 111H738.7M734 106.33L738.7 111L734 115.67",
		x: 729.3, y: 106.33, w: 9.4, h: 9.34, stroke: 1.6,
	},
	"refresh": {
		d: "M724 529.5C724 527.91 724.6 526.38 725.8 525.26C726.9 524.13 728.4 523.5 730 523.5C731.7 523.51 733.3 524.16 734.5 525.33L736 526.83M736 523.5V526.83H732.7M736 529.5C736 531.09 735.4 532.62 734.2 533.74C733.1 534.87 731.6 535.5 730 535.5C728.3 535.49 726.7 534.84 725.5 533.67L724 532.17M727.3 532.17H724V535.5",
		x: 724, y: 523.5, w: 12, h: 12, stroke: 1.6,
	},
	"user": {
		d: "M768.7 535.5V534.17C768.7 533.46 768.4 532.78 767.9 532.28C767.4 531.78 766.7 531.5 766 531.5H762C761.3 531.5 760.6 531.78 760.1 532.28C759.6 532.78 759.3 533.46 759.3 534.17V535.5M768.7 523.59C769.2 523.73 769.7 524.07 770.1 524.53C770.5 525 770.7 525.58 770.7 526.17C770.7 526.76 770.5 527.33 770.1 527.8C769.7 528.27 769.2 528.6 768.7 528.75M772.7 535.5V534.17C772.7 533.58 772.5 533 772.1 532.53C771.7 532.07 771.2 531.73 770.7 531.59M766.7 526.17C766.7 527.64 765.5 528.83 764 528.83C762.5 528.83 761.3 527.64 761.3 526.17C761.3 524.69 762.5 523.5 764 523.5C765.5 523.5 766.7 524.69 766.7 526.17Z",
		x: 759.3, y: 523.5, w: 13.4, h: 12, stroke: 1.6,
	},
	"filters": {
		d: "M800.7 524.83H796M802 534.17H796M803.3 523.5V524.83M803.3 524.83V526.17M803.3 524.83H808M804.7 532.83V534.17M804.7 534.17V535.5M804.7 534.17H808M808 529.5H802M799.3 528.17V529.5M799.3 529.5V530.83M799.3 529.5H796",
		x: 796, y: 523.5, w: 12, h: 12, stroke: 1.6,
	},
	"play": {
		d: "M753.2 227.931C753.2 227.261 753.5 227.131 754.2 227.531L761.5 232.031C762.2 232.431 762.2 232.831 761.5 233.231L754.2 237.731C753.5 238.131 753.2 237.991 753.2 237.331V227.931Z",
		x: 753.2, y: 227, w: 9, h: 11, filled: true,
	},
	"trash": {
		d: "M789 229.5H790.5M790.5 229.5H793M790.5 229.5L791.2 238C791.3 238.67 791.6 239 792.3 239H797.7C798.4 239 798.7 238.67 798.8 238L799.5 229.5M793 229.5H797M793 229.5V228C793 227.33 793.3 227 794 227H796C796.7 227 797 227.33 797 228V229.5M799.5 229.5H797M799.5 229.5H801M793.5 232V236.5M796.5 232V236.5",
		x: 789, y: 227, w: 12, h: 12, stroke: 1.25,
	},
}

var svgPaths = map[string]*qt.QPainterPath{}

// svgPath parses and caches one path. The supported commands cover every
// glyph in home.svg: M, L, H, V, C and Z, absolute and relative.
func svgPath(d string) *qt.QPainterPath {
	if path, ok := svgPaths[d]; ok {
		return path
	}
	path := parseSVGPath(d)
	svgPaths[d] = path
	return path
}

// drawSVGIcon centers a glyph from home.svg in r without stretching it.
func drawSVGIcon(p *qt.QPainter, name string, r image.Rectangle, col color.NRGBA) {
	icon, ok := svgIcons[name]
	if !ok || icon.w <= 0 || icon.h <= 0 || r.Empty() {
		return
	}
	scale := math.Min(1, math.Min(float64(r.Dx())/icon.w, float64(r.Dy())/icon.h))
	x := float64(r.Min.X) + (float64(r.Dx())-icon.w*scale)/2 - icon.x*scale
	y := float64(r.Min.Y) + (float64(r.Dy())-icon.h*scale)/2 - icon.y*scale

	p.Save()
	p.Translate2(x, y)
	if scale != 1 {
		p.Scale(scale, scale)
	}
	path := svgPath(icon.d)
	if icon.filled {
		brush := qt.NewQBrush3(qcolor(col))
		p.SetPenWithStyle(qt.NoPen)
		p.SetBrush(brush)
		p.DrawPath(path)
		brush.Delete()
	} else {
		pen := qt.NewQPen3(qcolor(col))
		pen.SetWidthF(icon.stroke)
		pen.SetCapStyle(qt.RoundCap)
		pen.SetJoinStyle(qt.RoundJoin)
		p.SetPenWithPen(pen)
		p.SetBrushWithStyle(qt.NoBrush)
		p.DrawPath(path)
		pen.Delete()
	}
	p.Restore()
}

// parseSVGPath turns SVG path data into a QPainterPath.
func parseSVGPath(d string) *qt.QPainterPath {
	tokens := tokenizeSVG(d)
	path := qt.NewQPainterPath()
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

type svgToken struct {
	cmd byte
	num float64
}

// tokenizeSVG splits path data into command letters and numbers.
func tokenizeSVG(d string) []svgToken {
	tokens := make([]svgToken, 0, len(d)/3)
	for i := 0; i < len(d); {
		c := d[i]
		switch {
		case c == ' ' || c == ',' || c == '\n' || c == '\r' || c == '\t':
			i++
		case isSVGCommand(c):
			tokens = append(tokens, svgToken{cmd: c})
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
				tokens = append(tokens, svgToken{num: v})
			}
		}
	}
	return tokens
}

func isSVGCommand(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'Z', 'z':
		return true
	}
	return false
}
