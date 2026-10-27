package ui

import (
	"errors"
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yoanime/party"
)

func TestSettingsWindowLayoutDoesNotOverflow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range []settingsValues{
		{Notifications: true, SharedControl: true, Provider: party.Cloudflare},
		{Notifications: false, SharedControl: false, Provider: party.Supabase},
		{
			Notifications: true,
			SharedControl: true,
			Provider:      party.Supabase,
			CloudflareURL: "wss://yoanime-party.example.workers.dev",
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
		Provider:      party.Cloudflare,
	})
	settings.providerOpen = true

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 442)),
	}
	gtx = gtx.Disabled()
	// Lay out once to learn where the provider row sits, then scroll it into
	// the viewport. Scrolling to the very bottom now leaves it far above the
	// content area because the auto-skip section follows it.
	settings.layout(gtx)
	settings.scroll = clampScroll(settings.providerAnchor.Max.Y-100, settings.contentHeight(gtx), 442)
	settings.layout(gtx)

	if settings.providerAnchor.Empty() {
		t.Fatal("provider trigger was not laid out")
	}
	bounds := image.Rectangle{Max: image.Pt(833, 442)}
	menu := placePopupList(
		settings.providerAnchor.Sub(image.Pt(0, settings.scroll)),
		providerMenuHeight(gtx, len(party.Providers())),
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

	below := placePopupList(image.Rect(500, 100, 624, 130), height, gap, bounds)
	if want := image.Rect(500, 136, 624, 208); below != want {
		t.Fatalf("menu below = %v, want %v", below, want)
	}

	above := placePopupList(image.Rect(500, 350, 624, 380), height, gap, bounds)
	if want := image.Rect(500, 272, 624, 344); above != want {
		t.Fatalf("menu above = %v, want %v", above, want)
	}
}

// TestSettingsWindowContentScrollsAtTheDefaultSize guards the wheel scrolling:
// the taller cards no longer fit the default content area, so the content has to
// report its full height and the offset has to stop at the last block.
func TestSettingsWindowContentScrollsAtTheDefaultSize(t *testing.T) {
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
	viewport := geometry.contentBottom - geometry.contentTop

	settings := newSettingsWindow(typo, settingsValues{
		Notifications: true,
		SharedControl: true,
		Provider:      party.Supabase,
	})
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      window.Metric,
		Constraints: layout.Exact(image.Pt(geometry.size.X, viewport)),
	}
	gtx = gtx.Disabled()

	full := settings.contentHeight(gtx)
	if full <= viewport {
		t.Fatalf("settings content is %dpx, want it taller than the %dpx viewport", full, viewport)
	}
	if got, want := clampScroll(full*2, full, viewport), full-viewport; got != want {
		t.Fatalf("clamped scroll = %d, want %d", got, want)
	}
}

func TestClampScroll(t *testing.T) {
	tests := []struct {
		scroll, content, viewport, want int
	}{
		{0, 400, 440, 0},
		{0, 440, 440, 0},
		{30, 500, 440, 30},
		{100, 500, 440, 60},
		{-20, 500, 440, 0},
	}
	for _, tc := range tests {
		if got := clampScroll(tc.scroll, tc.content, tc.viewport); got != tc.want {
			t.Errorf("clampScroll(%d, %d, %d) = %d, want %d", tc.scroll, tc.content, tc.viewport, got, tc.want)
		}
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

// TestSettingsWindowScrollsWithTheWheel feeds a real wheel event through the
// input router: the screen has to accept it and move the content by the delta.
func TestSettingsWindowScrollsWithTheWheel(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	settings := newSettingsWindow(typo, settingsValues{Provider: party.Supabase})
	var router input.Router
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 442)),
		Source:      router.Source(),
	}
	settings.layout(gtx)
	router.Frame(gtx.Ops)

	router.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(400, 200),
		Scroll:   f32.Pt(0, 60),
	})

	next := gtx
	next.Ops = new(op.Ops)
	settings.layout(next)

	if settings.scroll != 60 {
		t.Fatalf("scroll = %d, want 60", settings.scroll)
	}
}

func TestSettingsWindowSavesUpdateToggles(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	settings := newSettingsWindow(typo, settingsValues{Provider: party.Cloudflare})
	var auto, beta bool
	settings.onAutoUpdate = func(enabled bool) error {
		auto = enabled
		return nil
	}
	settings.onBeta = func(enabled bool) error {
		beta = enabled
		return nil
	}

	settings.setAutoUpdate(true)
	settings.setBeta(true)
	if !settings.autoUpdate || !auto {
		t.Fatal("auto-update toggle was not stored")
	}
	if !settings.beta || !beta {
		t.Fatal("beta toggle was not stored")
	}

	settings.onAutoUpdate = func(bool) error { return errors.New("база данных недоступна") }
	settings.setAutoUpdate(false)
	if !settings.autoUpdate {
		t.Fatal("a failed save must not flip the auto-update toggle")
	}
	if settings.errorText == "" {
		t.Fatal("a failed save must surface the error")
	}
}

func TestSettingsWindowSavesTheRelayBlock(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	settings := newSettingsWindow(typo, settingsValues{Provider: party.Supabase})
	settings.cloudflareEditor.SetText("https://party.example.workers.dev")
	settings.supabaseURLEditor.SetText("https://abcd.supabase.co")
	settings.supabaseKeyEditor.SetText("sb_publishable_example")
	settings.relayDirty = true

	var saved party.Config
	settings.onRelay = func(cfg party.Config) error {
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

	settings.onRelay = func(party.Config) error {
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
