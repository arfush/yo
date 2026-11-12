package screen

import (
	"time"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yo/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/resource"
)

// Dialog demo geometry, in device-independent dp.
const (
	demoWidth         unit.Dp = 464
	demoControlWidth  unit.Dp = 408
	demoControlHeight unit.Dp = 36
)

// Progress cycle: one percent per second, then a five-second hold at 100%.
const (
	demoFillPeriod = 100 * time.Second
	demoHoldPeriod = 5 * time.Second
)

// DialogDemo is a temporary screen that mirrors the dialog reference: an
// authorization button above, the dialog in the middle and a progress bar
// below. It is not part of the product.
type DialogDemo struct {
	navigator.ScreenBase
	app    stdx.ReadonlyHolder[appearance.Appearance]
	since  time.Time
	auth   kit.Button
	dialog kit.Dialog
	bar    kit.ProgressBar
}

// NewDialogDemo builds the demo screen with the reference texts. The strings
// are literals on purpose: the screen is a temporary local harness.
func NewDialogDemo(app stdx.ReadonlyHolder[appearance.Appearance], res *resource.Resource) *DialogDemo {
	if res == nil {
		panic("screen: nil resources")
	}
	download := res.Icon(resource.IconDownload)
	return &DialogDemo{
		ScreenBase: navigator.ScreenBase{ScreenTitle: "yo"},
		app:        app,
		since:      time.Now(),
		auth: kit.NewButton(kit.ButtonOptions{
			Width:  demoControlWidth,
			Height: demoControlHeight,
			Icon:   res.Icon(resource.IconArrowUpRight),
			Text:   "Авторизоваться",
		}),
		dialog: kit.NewDialog(kit.DialogOptions{
			Width:       demoWidth,
			Height:      255,
			Icon:        download,
			Title:       "Требуется Chromium",
			Description: "Для генерации API-токена необходимо скачать Chromium.\nПосле установки можно будет перейти к авторизации.",
			ShowClose:   true,
			CloseIcon:   res.Icon(resource.IconCloseDialog),
			OnClose:     func() {},
			Button: kit.ButtonOptions{
				Width:  demoControlWidth,
				Height: demoControlHeight,
				Icon:   res.Icon(resource.IconDownloadButton),
				Text:   "Скачать",
				Accent: true,
			},
		}),
		bar: kit.NewProgressBar(kit.ProgressBarOptions{
			Width:  demoControlWidth,
			Height: demoControlHeight,
			Text:   "0%",
		}),
	}
}

// Layout draws the column and advances the progress from the frame clock.
func (d *DialogDemo) Layout(gtx layout.Context) {
	app := d.app.Value()
	gtx.Locale = system.Locale{Language: app.Locale().Tag().String()}
	d.advance(gtx, app)
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.auth.Layout(gtx, app)
			}),
			layout.Rigid(layout.Spacer{Height: appearance.SpaceXL}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.dialog.Layout(gtx, app)
			}),
			layout.Rigid(layout.Spacer{Height: appearance.SpaceXL}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.bar.Layout(gtx, app)
			}),
		)
	})
}

// advance updates the progress bar and asks for the next whole second.
func (d *DialogDemo) advance(gtx layout.Context, app appearance.Appearance) {
	cycle := demoFillPeriod + demoHoldPeriod
	elapsed := gtx.Now.Sub(d.since)
	if elapsed < 0 {
		elapsed = 0
	}
	phase := elapsed % cycle
	percent := 100
	if phase < demoFillPeriod {
		percent = int(phase / time.Second)
	}
	d.bar.SetPercent(percent)
	d.bar.SetText(app.Locale().Percent(float64(percent)/100, 0))
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second - phase%time.Second)})
}
