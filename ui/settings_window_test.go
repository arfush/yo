package ui

import (
	"errors"
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
		{Notifications: true, SharedControl: true, Provider: watchparty.Cloudflare},
		{Notifications: false, SharedControl: false, Provider: watchparty.Supabase},
		{
			Notifications: true,
			SharedControl: true,
			Provider:      watchparty.Supabase,
			CloudflareURL: "wss://yonime-watchparty.example.workers.dev",
			SupabaseURL:   "wss://abcdefghijkl.supabase.co",
			SupabaseKey:   "sb_publishable_example",
		},
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
		Notifications: true,
		SharedControl: true,
		Provider:      watchparty.Cloudflare,
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
		Notifications: true,
		SharedControl: true,
		Provider:      watchparty.Supabase,
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
		5*gtx.Dp(settingsPanelHeight) +
		3*gtx.Dp(settingsRowGap)
	if bottom > contentHeight {
		t.Fatalf("settings need %dpx, content area allows %dpx", bottom, contentHeight)
	}
	_, noteInk := typo.measure(gtx, "Ошибка", font.Font{Typeface: fontFamily, Weight: font.Normal}, settingsNoteSize)
	errorBottom := bottom + gtx.Dp(settingsErrorGap) + noteInk.Dy()
	if errorBottom > contentHeight {
		t.Fatalf("settings with an error need %dpx, content area allows %dpx", errorBottom, contentHeight)
	}
}

// TestRelayFieldRectKeepsTheInputBesideTheLabel pins the row layout: the editor
// sits on the right of the same line as the setting name, not under it.
func TestRelayFieldRectKeepsTheInputBesideTheLabel(t *testing.T) {
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(777, 60)),
	}
	panel := image.Rect(0, 100, 777, 160)
	field := relayFieldRect(gtx, panel)

	if want := panel.Max.X - gtx.Dp(settingsPanelPadX); field.Max.X != want {
		t.Fatalf("field right = %d, want %d", field.Max.X, want)
	}
	if field.Dy() != gtx.Dp(settingsFieldHeight) {
		t.Fatalf("field height = %d, want %d", field.Dy(), gtx.Dp(settingsFieldHeight))
	}
	if got, want := field.Min.Y+field.Dy()/2, panel.Min.Y+panel.Dy()/2; got != want {
		t.Fatalf("field center = %d, want %d", got, want)
	}
	if field.Min.X <= panel.Min.X+gtx.Dp(settingsPanelPadX) {
		t.Fatalf("field %v leaves no room for the label", field)
	}

	narrow := relayFieldRect(gtx, image.Rect(0, 0, 400, 60))
	if max := (400 - 2*gtx.Dp(settingsPanelPadX)) / 2; narrow.Dx() > max {
		t.Fatalf("narrow field width = %d, want at most %d", narrow.Dx(), max)
	}
}

func TestSettingsWindowSavesTheRelayBlock(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	settings := newSettingsWindow(typo, settingsValues{Provider: watchparty.Supabase})
	settings.cloudflareEditor.SetText("https://party.example.workers.dev")
	settings.supabaseURLEditor.SetText("https://abcd.supabase.co")
	settings.supabaseKeyEditor.SetText("sb_publishable_example")
	settings.relayDirty = true

	var saved watchparty.Config
	settings.onRelay = func(cfg watchparty.Config) error {
		saved = cfg
		return nil
	}
	settings.saveRelays()

	if saved.CloudflareURL != "https://party.example.workers.dev" ||
		saved.SupabaseURL != "https://abcd.supabase.co" ||
		saved.SupabaseKey != "sb_publishable_example" {
		t.Fatalf("saved config = %+v", saved)
	}
	if settings.relayDirty {
		t.Fatal("a successful save must clear the dirty flag")
	}
	if settings.errorText != "" {
		t.Fatalf("error = %q, want none", settings.errorText)
	}

	settings.onRelay = func(watchparty.Config) error {
		return errors.New("адрес Cloudflare указан неверно")
	}
	settings.relayDirty = true
	settings.saveRelays()
	if !settings.relayDirty {
		t.Fatal("a failed save must stay dirty for the next attempt")
	}
	if settings.errorText == "" {
		t.Fatal("a failed save must surface the error")
	}
}
