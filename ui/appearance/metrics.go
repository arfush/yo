package appearance

import "time"

// Hairline is the thinnest visible line, in logical pixels.
const Hairline float32 = 1

// Spacing scale in logical pixels.
const (
	SpaceXS      float32 = 4
	SpaceS       float32 = 8
	SpaceM       float32 = 12
	SpaceL       float32 = 16
	SpaceXL      float32 = 24
	SpaceXXL     float32 = 32
	SpaceContent float32 = 28
)

// Corner radii in logical pixels.
const (
	RadiusS float32 = 8
	RadiusM float32 = 9
	RadiusL float32 = 20
)

// Motion durations.
const (
	MotionFast   = 100 * time.Millisecond
	MotionNormal = 320 * time.Millisecond
	MotionSlow   = 900 * time.Millisecond
)
