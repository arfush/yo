package widgets

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/layout"

	"github.com/arfush/yoanime/ui/draw"
)

// DrawSpinner paints eight fading dots around center.
func DrawSpinner(gtx layout.Context, center image.Point, radius, dotRadius int, phase float64, col color.NRGBA) {
	if radius <= 0 || dotRadius <= 0 {
		return
	}
	phase += float64(gtx.Now.UnixNano()%int64(2*time.Second)) / float64(2*time.Second) * 2 * math.Pi
	for i := 0; i < 8; i++ {
		angle := phase + float64(i)*math.Pi/4
		p := image.Pt(
			center.X+int(math.Cos(angle)*float64(radius)),
			center.Y+int(math.Sin(angle)*float64(radius)),
		)
		c := col
		c.A = uint8(35 + int(float64(190)*float64(i)/7))
		draw.FillCircle(gtx, p, dotRadius, c)
	}
}
