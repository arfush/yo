package theme

import (
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

const (
	HoverDuration     = 140 * time.Millisecond
	HoverPollInterval = 40 * time.Millisecond
)

// Hover eases a control highlight between 0 and 1 when the pointer enters or
// leaves its hitbox. It keeps requesting frames until the transition ends.
type Hover struct {
	Progress float32
	Target   float32
	from     float32
	start    time.Time
	running  bool
}

// Update advances the animation and returns its current progress.
func (a *Hover) Update(gtx layout.Context, hovered bool) float32 {
	target := float32(0)
	if hovered {
		target = 1
	}
	if target != a.Target {
		a.from = a.Progress
		a.Target = target
		a.start = gtx.Now
		a.running = true
	}
	if !a.running {
		return a.Progress
	}

	t := float32(gtx.Now.Sub(a.start)) / float32(HoverDuration)
	if t >= 1 {
		t = 1
		a.running = false
	}
	eased := t * t * (3 - 2*t)
	a.Progress = a.from + (a.Target-a.from)*eased

	if a.running {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	return a.Progress
}
