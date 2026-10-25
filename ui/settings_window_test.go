package ui

import (
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yonime/internal/watchparty"
)

func TestSettingsWindowLayoutDoesNotOverflow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range []settingsValues{
		{Notifications: true, SharedControl: true, Provider: watchparty.Cloudflare, ProviderChoice: true},
		{Notifications: false, SharedControl: false, Provider: watchparty.Supabase, ProviderChoice: true},
		{Notifications: true, SharedControl: true, Provider: watchparty.Cloudflare, ProviderChoice: false},
	} {
		settings := newSettingsWindow(typo, values)

		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(833, 442)),
		}
		gtx = gtx.Disabled()
		settings.layout(gtx)

		footer := gtx
		footer.Constraints = layout.Exact(image.Pt(833, 49))
		settings.layoutFooter(footer)
	}
}

// TestSettingsWindowFitsTheRealContentArea guards the margin between the last
// panel and the footer at the default window size, which the panic-free layout
// test above cannot see.
func TestSettingsWindowFitsTheRealContentArea(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	window := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(initialWidth, initialHeight)),
	}
	geometry := makeGeometry(window, false, true, false, false)
	contentHeight := geometry.contentBottom - geometry.contentTop
	contentWidth := geometry.size.X

	settings := newSettingsWindow(typo, settingsValues{
		Notifications:  true,
		SharedControl:  true,
		Provider:       watchparty.Supabase,
		ProviderChoice: true,
	})
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      window.Metric,
		Constraints: layout.Exact(image.Pt(contentWidth, contentHeight)),
	}
	gtx = gtx.Disabled()
	settings.layout(gtx)

	bold := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := typo.measure(gtx, "Совместный просмотр", bold, settingsTitleSize)
	bottom := gtx.Dp(settingsPadTop) +
		2*(titleInk.Dy()+gtx.Dp(settingsTitleGap)) +
		gtx.Dp(settingsGroupGap) +
		2*gtx.Dp(settingsPanelHeight) +
		gtx.Dp(settingsRowGap) +
		gtx.Dp(settingsPanelHeight)
	if bottom > contentHeight {
		t.Fatalf("settings need %dpx, content area allows %dpx", bottom, contentHeight)
	}
}
