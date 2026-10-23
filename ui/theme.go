package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
	"gioui.org/unit"
)

const (
	initialWidth  = 833
	initialHeight = 545
	// The designed size is also the smallest usable size. All layouts were
	// verified at this exact width and height.
	minimumWidth  = initialWidth
	minimumHeight = initialHeight
)

const (
	headerHeight     = 52
	headerBackground = headerHeight - 1
	footerHeight     = 49
	controlWidth     = 30
	controlGap       = 12
	controlsPadding  = 12
	innerRadius      = 8
)

const (
	titleSize = unit.Sp(13.2)
	badgeSize = unit.Sp(11)
)

const (
	badgeHeight   = 20
	badgePaddingX = 7
)

const fontFamily = "Inter"

var (
	contentColor = color.NRGBA{R: 0x0d, G: 0x0e, B: 0x14, A: 0xff}
	chromeColor  = color.NRGBA{R: 0x11, G: 0x12, B: 0x19, A: 0xff}
	borderColor  = color.NRGBA{R: 0x20, G: 0x21, B: 0x27, A: 0xff}
	dividerColor = color.NRGBA{R: 0x1b, G: 0x1c, B: 0x23, A: 0xff}

	iconColor = color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff}

	titleColor = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}

	badgeFillColor   = color.NRGBA{R: 0x29, G: 0x22, B: 0x3e, A: 0xff}
	badgeBorderColor = color.NRGBA{R: 0x39, G: 0x30, B: 0x4f, A: 0xff}
	badgeTextColor   = color.NRGBA{R: 0x96, G: 0x83, B: 0xe7, A: 0xff}

	controlHoverColor = color.NRGBA{R: 0x9a, G: 0x9a, B: 0x9f, A: 0xff}
	closeHoverColor   = color.NRGBA{R: 0xff, G: 0x64, B: 0x73, A: 0xff}

	footerControlFillColor      = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	footerControlBorderColor    = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
	footerControlHoverFill      = color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}
	footerControlHoverBorder    = color.NRGBA{R: 0x4a, G: 0x3c, B: 0x69, A: 0xff}
	footerControlIconColor      = color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff}
	footerControlHoverIconColor = color.NRGBA{R: 0xd7, G: 0xc9, B: 0xff, A: 0xff}
)

// geometry holds every chrome measurement converted from device-independent
// units to pixels for the current frame.
type geometry struct {
	size  image.Point
	scale float32

	border         int
	inner          image.Rectangle
	headerDivider  int
	contentTop     int
	contentBottom  int
	footerDivider  int
	footerTop      int
	footerBottom   int
	stripTop       int
	cornerRadius   int
	headerClickEnd int

	titleX        int
	headerCenterY int
	badgeX        int
	logo          image.Rectangle

	footerCenterY int
	updateRect    image.Rectangle

	// iconCenter and the hover measurements are local to a control hitbox.
	iconCenter  image.Point
	hoverSize   int
	hoverRadius int

	controls controlGeometry
}

type controlGeometry struct {
	minimize, maximize, close image.Rectangle
}

func (g geometry) px(v float32) int {
	return int(math.Round(float64(v) * float64(g.scale)))
}

func (g geometry) leastPx(v float32) int {
	n := g.px(v)
	if n < 1 {
		return 1
	}
	return n
}

func (g geometry) vec(v float32) float32 {
	return v * g.scale
}

// iconOffsetRect converts a rectangle expressed in design pixels relative to
// the center of a control hitbox into device pixels.
func (g geometry) iconOffsetRect(minX, minY, maxX, maxY float32) image.Rectangle {
	return image.Rect(
		g.iconCenter.X+g.px(minX),
		g.iconCenter.Y+g.px(minY),
		g.iconCenter.X+g.px(maxX),
		g.iconCenter.Y+g.px(maxY),
	)
}

// hoverRect is the rounded highlight behind a control icon, local to the
// control hitbox.
func (g geometry) hoverRect() image.Rectangle {
	c := g.iconCenter
	return image.Rect(
		c.X-g.hoverSize/2,
		c.Y-g.hoverSize/2,
		c.X+g.hoverSize/2,
		c.Y+g.hoverSize/2,
	)
}

func makeGeometry(gtx layout.Context, maximized, updateVisible bool) geometry {
	size := gtx.Constraints.Max
	scale := gtx.Metric.PxPerDp
	if scale == 0 {
		scale = 1
	}

	g := geometry{size: size, scale: scale}
	g.border = g.leastPx(1)
	divider := g.leastPx(1)
	g.inner = image.Rect(g.border, g.border, size.X-g.border, size.Y-g.border)

	g.headerDivider = g.border + g.px(headerBackground)
	g.contentTop = g.headerDivider + divider

	strip := g.px(1)
	footer := g.px(footerHeight)
	g.footerBottom = size.Y - g.border - strip
	g.footerTop = g.footerBottom - footer
	g.footerDivider = g.footerTop - divider
	g.contentBottom = g.footerDivider
	g.stripTop = g.footerBottom
	g.headerClickEnd = g.headerDivider

	radius := g.px(innerRadius)
	if maximized {
		radius = 0
	}
	g.cornerRadius = radius

	g.titleX = g.px(49)
	g.headerCenterY = (g.border + g.headerDivider) / 2
	g.badgeX = g.px(110)

	g.footerCenterY = (g.footerTop + g.footerBottom) / 2
	controlSize := g.px(float32(mainControlSize))
	controlRight := size.X - g.border - g.px(15)
	// The update control is the only footer button and sits at the right edge.
	if updateVisible {
		g.updateRect = image.Rect(
			controlRight-controlSize,
			g.footerCenterY-controlSize/2,
			controlRight,
			g.footerCenterY+controlSize/2,
		)
	}

	logoSize := g.px(24)
	logoTop := g.headerCenterY - logoSize/2
	g.logo = image.Rect(
		g.px(19),
		logoTop,
		g.px(19)+logoSize,
		logoTop+logoSize,
	)

	g.iconCenter = image.Pt(g.px(controlWidth)/2, g.headerClickEnd/2)
	g.hoverSize = g.px(28)
	g.hoverRadius = g.px(7)

	right := size.X - g.border - g.px(controlsPadding)
	width := g.px(controlWidth)
	gap := g.px(controlGap)
	g.controls.close = image.Rect(right-width, 0, right, g.headerClickEnd)
	g.controls.maximize = g.controls.close.Add(image.Pt(-(width + gap), 0))
	g.controls.minimize = g.controls.maximize.Add(image.Pt(-(width + gap), 0))

	return g
}

func (g geometry) contentRect() image.Rectangle {
	return image.Rect(
		g.border,
		g.contentTop,
		g.size.X-g.border,
		g.contentBottom,
	)
}
