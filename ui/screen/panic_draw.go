package screen

import (
	"image"

	"gioui.org/layout"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/kit/mascot"
)

// layoutRow places the mascot beside the text block. The mascot takes the
// smaller of the available side and its design cap; the text gets the rest.
func (p Panic) layoutRow(gtx layout.Context, app appearance.Appearance) layout.Dimensions {
	size := min(min(gtx.Constraints.Max.X, gtx.Constraints.Max.Y), gtx.Dp(panicMascotMax))
	gap := gtx.Dp(appearance.SpaceL)
	textWidth := min(max(0, gtx.Constraints.Max.X-size-gap), gtx.Dp(panicTextMax))
	head := size * panicHeadOffsetNum / panicHeadOffsetDen

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.drawMascot(gtx, app.Palette(), size)
		}),
		layout.Rigid(layout.Spacer{Width: appearance.SpaceL}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutText(gtx, app, textWidth, head)
		}),
	)
}

// drawMascot paints the panic mood mascot into a square of the given pixel
// size and reports that square as the layout size.
func (p Panic) drawMascot(gtx layout.Context, palette appearance.Palette, size int) layout.Dimensions {
	m := mascot.New(mascot.Options{Palette: palette.Mascot, Mood: mascot.Panic})
	m.Draw(gtx, image.Rect(0, 0, size, size))
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// layoutText wraps the title and the panic value into the text column. The
// column starts at the mascot's visible head, not at its slot top.
func (p Panic) layoutText(gtx layout.Context, app appearance.Appearance, width, head int) layout.Dimensions {
	gtx.Constraints.Max.X = width
	typo := app.Typography()
	palette := app.Palette()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(0, head)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Text(gtx, typo, appearance.RoleSubtitle, p.title, palette.Text)
		}),
		layout.Rigid(layout.Spacer{Height: appearance.SpaceM}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.TextBlock(gtx, typo, appearance.RoleBody, p.detail, palette.TextMuted)
		}),
	)
}
