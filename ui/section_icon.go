package ui

import (
	"image"
	"image/color"
	"math"
)

// recolorPurpleIcon keeps the application icon's shape and shading and rotates
// its violet hue to the target section color. White artwork and transparent
// pixels are left untouched, so all sections share exactly the same icon.
func recolorPurpleIcon(src image.Image, target color.NRGBA) image.Image {
	if src == nil {
		return nil
	}
	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	targetHue, _, _ := rgbToHSV(color.NRGBAModel.Convert(target).(color.NRGBA))

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			hue, saturation, value := rgbToHSV(c)
			if saturation >= 0.10 && hue >= 220 && hue <= 300 {
				c = hsvToRGB(targetHue, saturation, value, c.A)
			}
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

func rgbToHSV(c color.NRGBA) (hue, saturation, value float64) {
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	value = max
	delta := max - min
	if max == 0 || delta == 0 {
		return 0, 0, value
	}
	saturation = delta / max
	switch max {
	case r:
		hue = math.Mod((g-b)/delta, 6)
	case g:
		hue = (b-r)/delta + 2
	default:
		hue = (r-g)/delta + 4
	}
	hue *= 60
	if hue < 0 {
		hue += 360
	}
	return hue, saturation, value
}

func hsvToRGB(hue, saturation, value float64, alpha uint8) color.NRGBA {
	hue = math.Mod(hue, 360)
	if hue < 0 {
		hue += 360
	}
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	m := value - chroma
	var r, g, b float64
	switch {
	case hue < 60:
		r, g, b = chroma, x, 0
	case hue < 120:
		r, g, b = x, chroma, 0
	case hue < 180:
		r, g, b = 0, chroma, x
	case hue < 240:
		r, g, b = 0, x, chroma
	case hue < 300:
		r, g, b = x, 0, chroma
	default:
		r, g, b = chroma, 0, x
	}
	return color.NRGBA{
		R: uint8(math.Round((r + m) * 255)),
		G: uint8(math.Round((g + m) * 255)),
		B: uint8(math.Round((b + m) * 255)),
		A: alpha,
	}
}
