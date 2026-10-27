package ui

import (
	"image"
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
	settingsRect  image.Rectangle
	logRect       image.Rectangle
	partyRect     image.Rectangle
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

func makeGeometry(gtx layout.Context, maximized, settingsVisible, partyVisible, updateVisible bool) geometry {
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
	controlTop := g.footerCenterY - controlSize/2
	controlBottom := g.footerCenterY + controlSize/2
	// Settings is the rightmost footer button. The log control sits between it
	// and the watch-party control; the update control appears left of the party
	// button only while a newer release is available.
	if settingsVisible {
		g.settingsRect = image.Rect(controlRight-controlSize, controlTop, controlRight, controlBottom)
		nextRight := g.settingsRect.Min.X - g.px(float32(mainControlGap))
		g.logRect = image.Rect(nextRight-controlSize, controlTop, nextRight, controlBottom)
		nextRight = g.logRect.Min.X - g.px(float32(mainControlGap))
		if partyVisible {
			g.partyRect = image.Rect(nextRight-controlSize, controlTop, nextRight, controlBottom)
			nextRight = g.partyRect.Min.X - g.px(float32(mainControlGap))
		}
		if updateVisible {
			g.updateRect = image.Rect(nextRight-controlSize, controlTop, nextRight, controlBottom)
		}
	}

	logoSize := g.px(24)
	logoTop := g.headerCenterY - logoSize/2
	// The section menu starts at the old trigger position, while the visible
	// brand block is shifted right so the menu icon and label line up exactly
	// under the active section icon and label.
	g.logo = image.Rect(
		g.px(29),
		logoTop,
		g.px(29)+logoSize,
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
