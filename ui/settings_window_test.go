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

// TestSettingsWindowProviderMenuOpensInsideTheContentArea keeps the dropdown
// inside the area the shell does not clip away.
func TestSettingsWindowProviderMenuOpensInsideTheContentArea(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	settings := newSettingsWindow(typo, settingsValues{
		Notifications:  true,
		SharedControl:  true,
		Provider:       watchparty.Cloudflare,
		ProviderChoice: true,
	})
	settings.providerOpen = true

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 442)),
	}
	gtx = gtx.Disabled()
	settings.layout(gtx)

	if settings.providerAnchor.Empty() {
		t.Fatal("provider trigger was not laid out")
	}
	bounds := image.Rectangle{Max: image.Pt(833, 442)}
	menu := placeProviderMenu(
		settings.providerAnchor,
		providerMenuHeight(gtx, len(watchparty.Providers())),
		gtx.Dp(settingsProviderMenuGap),
		bounds,
	)
	if !menu.In(bounds) {
		t.Fatalf("provider menu %v leaves the content area %v", menu, bounds)
	}
}

func TestPlaceProviderMenu(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 400)
	const height, gap = 72, 6

	below := placeProviderMenu(image.Rect(500, 100, 624, 130), height, gap, bounds)
	if want := image.Rect(500, 136, 624, 208); below != want {
		t.Fatalf("menu below = %v, want %v", below, want)
	}

	above := placeProviderMenu(image.Rect(500, 350, 624, 380), height, gap, bounds)
	if want := image.Rect(500, 272, 624, 344); above != want {
		t.Fatalf("menu above = %v, want %v", above, want)
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
