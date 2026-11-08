package appearance

import (
	"time"

	"gioui.org/unit"
)

// Spacing scale in density-independent pixels.
const (
	SpaceXS      unit.Dp = 4
	SpaceS       unit.Dp = 8
	SpaceM       unit.Dp = 12
	SpaceL       unit.Dp = 16
	SpaceXL      unit.Dp = 24
	SpaceXXL     unit.Dp = 32
	SpaceContent unit.Dp = 28
)

// Corner radii in density-independent pixels.
const (
	RadiusS unit.Dp = 8
	RadiusM unit.Dp = 9
	RadiusL unit.Dp = 20
)

// Motion durations.
const (
	MotionFast   = 100 * time.Millisecond
	MotionNormal = 320 * time.Millisecond
	MotionSlow   = 900 * time.Millisecond
)
