package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

type textRenderer struct {
	shaper *text.Shaper
}

// measure shapes txt and reports the glyphs and their visual (ink) bounds.
func (r *textRenderer) measure(gtx layout.Context, txt string, face font.Font, size unit.Sp) ([]text.Glyph, image.Rectangle) {
	r.shaper.LayoutString(text.Parameters{
		Font:     face,
		PxPerEm:  fixed.I(gtx.Sp(size)),
		MaxWidth: 1 << 20,
		Locale:   gtx.Locale,
	}, txt)

	var glyphs []text.Glyph
	var ink image.Rectangle
	first := true
	for {
		g, ok := r.shaper.NextGlyph()
		if !ok {
			break
		}
		glyphs = append(glyphs, g)
		if g.Flags&text.FlagParagraphBreak != 0 {
			continue
		}

		b := image.Rect(
			(g.X + g.Bounds.Min.X).Floor(),
			int(g.Y)+g.Bounds.Min.Y.Floor(),
			(g.X + g.Bounds.Max.X).Ceil(),
			int(g.Y)+g.Bounds.Max.Y.Ceil(),
		)
		if b.Empty() {
			continue
		}
		if first {
			ink = b
			first = false
		} else {
			ink = ink.Union(b)
		}
	}
	if first {
		ink = image.Rectangle{}
	}
	return glyphs, ink
}

// draw paints txt so that its visual top-left corner lands on at. It returns
// the visual bounds of the rendered text.
func (r *textRenderer) draw(gtx layout.Context, txt string, face font.Font, size unit.Sp, col color.NRGBA, at image.Point) image.Rectangle {
	glyphs, ink := r.measure(gtx, txt, face, size)
	if len(glyphs) == 0 || ink.Empty() {
		return image.Rectangle{}
	}

	offset := at.Sub(ink.Min)
	stack := op.Offset(offset).Push(gtx.Ops)
	r.paintGlyphs(gtx, glyphs, col)
	stack.Pop()
	return ink.Add(offset)
}

func (r *textRenderer) drawCentered(gtx layout.Context, txt string, face font.Font, size unit.Sp, col color.NRGBA, center image.Point) image.Rectangle {
	_, ink := r.measure(gtx, txt, face, size)
	if ink.Empty() {
		return image.Rectangle{}
	}
	r.paintCentered(gtx, txt, face, size, col, f32.Pt(float32(center.X), float32(center.Y)), 0)
	offset := image.Pt(
		center.X-(ink.Min.X+ink.Max.X)/2,
		center.Y-(ink.Min.Y+ink.Max.Y)/2,
	)
	return ink.Add(offset)
}

// paintCentered paints txt with its ink center at center. A positive capHeight
// centers only the box from the ink top to the baseline, so descenders do not
// make the text look too high inside a control. The fractional offset keeps
// odd-width text truly centered instead of half a pixel to the right.
func (r *textRenderer) paintCentered(gtx layout.Context, txt string, face font.Font, size unit.Sp, col color.NRGBA, center f32.Point, capHeight int) {
	glyphs, ink := r.measure(gtx, txt, face, size)
	if len(glyphs) == 0 || ink.Empty() {
		return
	}
	dy := center.Y - float32(ink.Min.Y+ink.Max.Y)/2
	if capHeight > 0 {
		dy = center.Y - float32(ink.Min.Y) - float32(capHeight)/2
	}
	dx := center.X - float32(ink.Min.X+ink.Max.X)/2
	stack := op.Affine(f32.Affine2D{}.Offset(f32.Pt(dx, dy))).Push(gtx.Ops)
	r.paintGlyphs(gtx, glyphs, col)
	stack.Pop()
}

func (r *textRenderer) paintGlyphs(gtx layout.Context, glyphs []text.Glyph, col color.NRGBA) {
	material := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	mat := material.Stop()

	var line []text.Glyph
	for _, g := range glyphs {
		line = append(line, g)
		if g.Flags&text.FlagLineBreak != 0 {
			r.paintLine(gtx, line, mat)
			line = line[:0]
		}
	}
	if len(line) > 0 {
		r.paintLine(gtx, line, mat)
	}
}

func (r *textRenderer) paintLine(gtx layout.Context, line []text.Glyph, mat op.CallOp) {
	first := line[0]
	origin := f32.Pt(
		float32(first.X)/64,
		float32(first.Y),
	)
	trans := op.Affine(f32.Affine2D{}.Offset(origin)).Push(gtx.Ops)
	path := r.shaper.Shape(line)
	outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
	mat.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	outline.Pop()
	if call := r.shaper.Bitmaps(line); call != (op.CallOp{}) {
		call.Add(gtx.Ops)
	}
	trans.Pop()
}
