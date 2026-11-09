package screen

import (
	"image"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/kit/mascot"
	"github.com/arfush/yo/ui/navigator"
)

// Panic screen geometry, in device-independent dp.
const (
	panicInset     unit.Dp = 32
	panicMascotMax unit.Dp = 253
	panicMascotGap unit.Dp = 16
	panicTextMax   unit.Dp = 460
	panicTitleGap  unit.Dp = 12
)

// Panic is the fallback screen shown after a recovered UI panic. It draws the
// mascot in the panic mood next to the title and the recovered value.
type Panic struct {
	navigator.ScreenBase
	app    stdx.ReadonlyHolder[appearance.Appearance]
	title  string
	detail string
}

// NewPanic builds the fallback screen from the live appearance. Reading the
// holder every frame lets the startup placeholder pick up the site palette once
// the shell applies it, and keeps the mascot in sync with later changes.
func NewPanic(app stdx.ReadonlyHolder[appearance.Appearance], title, detail string) Panic {
	return Panic{
		ScreenBase: navigator.ScreenBase{ScreenTitle: title},
		app:        app,
		title:      title,
		detail:     detail,
	}
}

// Layout centers the mascot and the text block in the content area. The title
// starts at the top of the mascot's head; the panic value wraps below it.
func (p Panic) Layout(gtx layout.Context) {
	app := p.app.Value()
	gtx.Locale = system.Locale{Language: app.Locale().Tag().String()}
	palette := app.Palette()
	typo := app.Typography()
	m := mascot.New(mascot.Options{Palette: palette.Mascot, Mood: mascot.Panic})

	layout.UniformInset(panicInset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := min(min(gtx.Constraints.Max.X, gtx.Constraints.Max.Y), gtx.Dp(panicMascotMax))
			gap := gtx.Dp(panicMascotGap)
			textWidth := min(max(0, gtx.Constraints.Max.X-size-gap), gtx.Dp(panicTextMax))
			// The visible head starts 2.5 of 42 design units below the mascot
			// slot top: (21 - 18.5) / 42 = 5 / 84.
			head := size * 5 / 84

			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					m.Draw(gtx, image.Rect(0, 0, size, size))
					return layout.Dimensions{Size: image.Pt(size, size)}
				}),
				layout.Rigid(layout.Spacer{Width: panicMascotGap}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = textWidth
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(0, head)}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.Text(gtx, typo, appearance.RoleSubtitle, p.title, palette.Text)
						}),
						layout.Rigid(layout.Spacer{Height: panicTitleGap}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.TextBlock(gtx, typo, appearance.RoleBody, p.detail, palette.TextMuted)
						}),
					)
				}),
			)
		})
	})
}
