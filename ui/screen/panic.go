package screen

import (
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yo/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/navigator"
)

// Panic screen geometry, in device-independent dp.
const (
	panicMascotMax unit.Dp = 253
	panicTextMax   unit.Dp = 460

	// panicHeadOffsetNum and panicHeadOffsetDen place the visible head inside
	// the mascot slot: (21 - 18.5) / 42 = 5 / 84 of the slot height.
	panicHeadOffsetNum = 5
	panicHeadOffsetDen = 84
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
	layout.UniformInset(appearance.SpaceXXL).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return p.layoutRow(gtx, app)
		})
	})
}
