package kit

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

// MeasureText returns the size of one line of value in the face and size of
// role, in logical pixels.
func MeasureText(app appearance.Appearance, role appearance.TextRole, value string) image.Point {
	if value == "" {
		return image.Point{}
	}
	f := face(app.Resource(), app.Typography().Font(role), app.Typography().TextSize(role))
	fm := qt6.NewQFontMetricsF(f)
	width, height := fm.HorizontalAdvance(value), fm.Height()
	fm.Delete()
	return image.Pt(int(math.Ceil(width)), int(math.Ceil(height)))
}

// Text draws one line of value at the painter origin and returns its size.
func Text(p *qt6.QPainter, app appearance.Appearance, role appearance.TextRole, value string, col color.NRGBA) image.Point {
	if value == "" {
		return image.Point{}
	}
	f := face(app.Resource(), app.Typography().Font(role), app.Typography().TextSize(role))
	fm := qt6.NewQFontMetricsF(f)
	width, ascent, height := fm.HorizontalAdvance(value), fm.Ascent(), fm.Height()
	fm.Delete()

	p.Save()
	p.SetFont(f)
	pen := qcolor(col)
	p.SetPen(pen)
	p.DrawText3(0, int(math.Round(ascent)), value)
	pen.Delete()
	p.Restore()

	return image.Pt(int(math.Ceil(width)), int(math.Ceil(height)))
}

// TextBlock draws value with the typography role and color, wrapping long text
// into as many lines as maxWidth allows. It returns the drawn size.
func TextBlock(p *qt6.QPainter, app appearance.Appearance, role appearance.TextRole, value string, col color.NRGBA, maxWidth int) image.Point {
	if value == "" {
		return image.Point{}
	}
	f := face(app.Resource(), app.Typography().Font(role), app.Typography().TextSize(role))
	fm := qt6.NewQFontMetricsF(f)
	lines := wrapLines(fm, value, float64(maxWidth))
	width, ascent, height := 0.0, fm.Ascent(), fm.Height()
	for _, line := range lines {
		width = math.Max(width, fm.HorizontalAdvance(line))
	}
	fm.Delete()

	p.Save()
	p.SetFont(f)
	pen := qcolor(col)
	p.SetPen(pen)
	for i, line := range lines {
		baseline := float64(i)*height + ascent
		p.DrawText3(0, int(math.Round(baseline)), line)
	}
	pen.Delete()
	p.Restore()

	return image.Pt(int(math.Ceil(width)), int(math.Ceil(height*float64(len(lines)))))
}

// wrapLines splits value into lines no wider than maxWidth. A non-positive
// width disables wrapping and only honors explicit newlines.
func wrapLines(fm *qt6.QFontMetricsF, value string, maxWidth float64) []string {
	paragraphs := strings.Split(value, "\n")
	if maxWidth <= 0 {
		return paragraphs
	}
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			candidate := line + " " + word
			if fm.HorizontalAdvance(candidate) <= maxWidth {
				line = candidate
				continue
			}
			lines = append(lines, line)
			line = word
		}
		lines = append(lines, line)
	}
	return lines
}
