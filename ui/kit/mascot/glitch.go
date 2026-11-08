package mascot

import (
	"image"
	"math"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/arfush/yo/ui/appearance"
)

// Panic glitch timing, in seconds. The character stays intact for most of the
// cycle, then a short burst tears a few horizontal slices sideways.
const (
	glitchCycle = 1.35
	glitchBurst = 0.3
	glitchStep  = 1.0 / 18
	glitchBands = 3
	glitchShift = 3.0
)

// glitchScene repaints shifted slices of an already painted Panic frame.
type glitchScene struct {
	bounds image.Rectangle
	center image.Point
	phase  float64
	px     func(float32) int
	colors appearance.MascotGlitch
	paint  func(layout.Context, float32)
}

func (g glitchScene) draw(gtx layout.Context) {
	if math.Mod(g.phase, glitchCycle) > glitchBurst {
		return
	}
	step := uint64(g.phase / glitchStep)
	for index := uint64(0); index < glitchBands; index++ {
		noise := glitchNoise(step*glitchBands + index)
		top := -16 + float32(noise&0xff)/255*30
		height := 1.2 + float32((noise>>8)&3)*0.9
		band := image.Rect(
			g.bounds.Min.X,
			g.center.Y+g.px(top),
			g.bounds.Max.X,
			g.center.Y+g.px(top+height),
		).Intersect(g.bounds)
		if band.Empty() {
			continue
		}

		stack := clip.Rect(band).Push(gtx.Ops)
		g.paint(gtx, glitchShiftAmount(noise))
		stack.Pop()

		tint := g.colors.Cyan
		if index%2 == 1 {
			tint = g.colors.Magenta
		}
		line := image.Rect(
			g.center.X+g.px(-14),
			band.Min.Y,
			g.center.X+g.px(14),
			band.Min.Y+max(1, g.px(0.35)),
		)
		paint.FillShape(gtx.Ops, tint, clip.Rect(line).Op())
	}
}

// glitchShiftAmount turns hash bits into a signed sideways shift in design
// units, biased away from zero so each slice visibly moves.
func glitchShiftAmount(noise uint64) float32 {
	sign := float32(1)
	if noise&0x10 == 0 {
		sign = -1
	}
	return sign * (1.2 + float32((noise>>5)&0xff)/255*(glitchShift-1.2))
}

// glitchNoise hashes a frame step so a still frame and its redraw agree.
func glitchNoise(seed uint64) uint64 {
	seed ^= seed >> 33
	seed *= 0xff51afd7ed558ccd
	seed ^= seed >> 33
	seed *= 0xc4ceb9fe1a85ec53
	seed ^= seed >> 33
	return seed
}
