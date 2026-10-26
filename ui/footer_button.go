package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// footerButtonIcon selects the small glyph drawn before the label. The base
// button has no icon; callers choose one explicitly.
type footerButtonIcon int

const (
	footerButtonNoIcon footerButtonIcon = iota
	footerButtonBackIcon
	footerButtonPlayIcon
)

// footerButtonStyle describes the colours of one button. The zero value draws
// nothing, so a button never inherits a label or a colour by accident.
type footerButtonStyle struct {
	fill        color.NRGBA
	fillHover   color.NRGBA
	border      color.NRGBA
	borderHover color.NRGBA
	text        color.NRGBA
	gradient    bool
}

func (s footerButtonStyle) palette(progress float32) (fill, border, text color.NRGBA) {
	return mixColor(s.fill, s.fillHover, progress),
		mixColor(s.border, s.borderHover, progress),
		s.text
}

// footerNamedButton is the shared labelled footer control: an 8 dp rounded
// rect with a 1 dp inner stroke, optional leading icon and a cap-height-centred
// label. The caller supplies the text, icon, size and palette; the control
// itself has no defaults.
type footerNamedButton struct {
	click widget.Clickable
	hover hoverAnim

	label   string
	reserve string
	icon    footerButtonIcon
	style   footerButtonStyle
	size    unit.Sp
}

// layout draws the button into rect and reports a click. reserve optionally
// names the widest label so an icon does not jump when the text changes (for
// example "Смотреть" -> "Запуск...").
func (b *footerNamedButton) layout(gtx layout.Context, typo *typography, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	_, clicked := b.click.Update(gtx)
	progress := b.hover.update(gtx, b.click.Hovered())

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	b.click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(8))
		fill, border, _ := b.style.palette(progress)
		if b.style.gradient {
			fillRoundedGradient(gtx, local, local, radius, fill, border)
		} else {
			fillRounded(gtx, border, local, radius)
			fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
			strokeRoundedRect(
				gtx,
				local.Inset(gtx.Dp(unit.Dp(1))),
				float32(radius),
				float32(gtx.Dp(unit.Dp(1))),
				border,
			)
		}
		b.drawContent(gtx, typo, local)
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}

func (b *footerNamedButton) drawContent(gtx layout.Context, typo *typography, local image.Rectangle) {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, labelInk := typo.measure(gtx, b.label, face, b.size)
	labelWidth := labelInk.Dx()
	if b.reserve != "" {
		if _, reserveInk := typo.measure(gtx, b.reserve, face, b.size); reserveInk.Dx() > labelWidth {
			labelWidth = reserveInk.Dx()
		}
	}
	_, capInk := typo.measure(gtx, "Н", face, b.size)
	centerY := float32(local.Dy()) / 2

	if b.icon == footerButtonNoIcon {
		if b.label == "" {
			return
		}
		typo.paintCentered(
			gtx,
			b.label,
			face,
			b.size,
			b.style.text,
			f32.Pt(float32(local.Dx())/2, centerY),
			capInk.Dy(),
		)
		return
	}

	iconSize := gtx.Dp(unit.Dp(11))
	if b.icon == footerButtonPlayIcon {
		iconSize = gtx.Dp(unit.Dp(12))
	}
	gap := gtx.Dp(unit.Dp(8))
	groupWidth := float32(iconSize + gap + labelWidth)
	start := (float32(local.Dx()) - groupWidth) / 2
	iconCenter := image.Pt(
		int(math.Round(float64(start+float32(iconSize)/2))),
		int(math.Round(float64(centerY))),
	)
	switch b.icon {
	case footerButtonBackIcon:
		drawChevronLeft(gtx, iconCenter, iconSize, gtx.Dp(unit.Dp(1.5)), b.style.text)
	case footerButtonPlayIcon:
		drawPlay(gtx, iconCenter, iconSize, b.style.text)
	}
	if b.label == "" {
		return
	}
	typo.paintCentered(
		gtx,
		b.label,
		face,
		b.size,
		b.style.text,
		f32.Pt(start+float32(iconSize+gap)+float32(labelWidth)/2, centerY),
		capInk.Dy(),
	)
}

var (
	footerSecondaryStyle = footerButtonStyle{
		fill:        titleButtonFill,
		fillHover:   titleButtonHover,
		border:      titleButtonBorder,
		borderHover: titleButtonHoverBorder,
		text:        titleButtonText,
	}
	footerSavedStyle = footerButtonStyle{
		fill:        titleSavedFill,
		fillHover:   titleSavedHover,
		border:      titleSavedBorder,
		borderHover: titleSavedHoverBorder,
		text:        titleSavedText,
	}
	footerPrimaryStyle = footerButtonStyle{
		fill:        mainAccentColor1,
		fillHover:   color.NRGBA{R: 0xae, G: 0x8b, B: 0xff, A: 0xff},
		border:      mainAccentColor2,
		borderHover: color.NRGBA{R: 0x82, G: 0x3e, B: 0xee, A: 0xff},
		text:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		gradient:    true,
	}
	footerCopyStyle = footerButtonStyle{
		fill:        color.NRGBA{R: 0x4a, G: 0x35, B: 0x72, A: 0xff},
		fillHover:   color.NRGBA{R: 0x5d, G: 0x45, B: 0x8a, A: 0xff},
		border:      color.NRGBA{R: 0x6d, G: 0x52, B: 0xa0, A: 0xff},
		borderHover: footerControlHoverBorder,
		text:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	}
)

// newBackFooterButton returns the standard "Вернуться" action. The palette is
// the shared secondary one; a caller that needs another colour can replace it.
func newBackFooterButton() footerNamedButton {
	return footerNamedButton{
		label: "Вернуться",
		icon:  footerButtonBackIcon,
		style: footerSecondaryStyle,
		size:  titlePageButtonSize,
	}
}

// newWatchFooterButton returns the primary "Смотреть" action. It uses the same
// shape as the back button with a play icon, a primary palette and a reserved
// width for the loading label.
func newWatchFooterButton() footerNamedButton {
	return footerNamedButton{
		label:   "Смотреть",
		reserve: "Запуск...",
		icon:    footerButtonPlayIcon,
		style:   footerPrimaryStyle,
		size:    titlePageButtonSize,
	}
}
