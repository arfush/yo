package shell

import (
	"image"
	"testing"

	"gioui.org/app"
	"gioui.org/layout"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

type testScreen struct{}

func (testScreen) Layout(layout.Context)       {}
func (testScreen) LayoutFooter(layout.Context) {}

func TestSwitchSite(t *testing.T) {
	t.Parallel()

	apps := stdx.NewHolder(appearance.New(appearance.Options{
		Resource: resource.New(),
		Language: "ru",
	}))
	site := stdx.NewHolder(catalog.SiteYoanime)
	shell := New(Options{Screen: testScreen{}, Site: site, Appearance: apps})

	shell.switchSite(catalog.SiteYomanga)
	if got := site.Value(); got != catalog.SiteYomanga {
		t.Fatalf("site holder = %q; want %q", got, catalog.SiteYomanga)
	}
	want := appearance.PaletteFor(catalog.SiteYomanga)
	if got := apps.Value().Palette().Accent; got != want.Accent {
		t.Fatal("switchSite did not sync the site palette")
	}
}

func TestSiteMenuDismissFallback(t *testing.T) {
	t.Parallel()

	apps := stdx.NewHolder(appearance.New(appearance.Options{
		Resource: resource.New(),
		Language: "ru",
	}))
	site := stdx.NewHolder(catalog.SiteYoanime)
	shell := New(Options{Screen: testScreen{}, Site: site, Appearance: apps})
	shell.syncPalette()

	for range siteMenuBasePaletteAfter {
		shell.menu.toggle(image.Rect(0, 0, 10, 10))
		shell.dismissSiteMenu()
	}
	if got := apps.Value().Palette(); got != appearance.BasePalette() {
		t.Fatal("repeated dismissals did not fall back to yobase")
	}

	shell.switchSite(catalog.SiteYoanime)
	if got := apps.Value().Palette(); got != appearance.PaletteFor(catalog.SiteYoanime) {
		t.Fatal("selecting a site did not restore its palette")
	}
	if shell.siteDismissals != 0 {
		t.Fatalf("dismissal counter = %d; want 0", shell.siteDismissals)
	}
}

func TestSetConfigFocusLossClosesSiteMenu(t *testing.T) {
	t.Parallel()

	apps := stdx.NewHolder(appearance.New(appearance.Options{
		Resource: resource.New(),
		Language: "ru",
	}))
	site := stdx.NewHolder(catalog.SiteYoanime)
	shell := New(Options{Screen: testScreen{}, Site: site, Appearance: apps})
	shell.menu.toggle(image.Rect(0, 0, 10, 10))

	shell.SetConfig(app.Config{Focused: false})

	if shell.menu.open {
		t.Fatal("focus loss left the site menu open")
	}
	if shell.siteDismissals != 1 {
		t.Fatalf("dismissal counter = %d; want 1", shell.siteDismissals)
	}
}

func TestSiteMenuAnchor(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 833, 545)
	tests := []struct {
		name    string
		trigger image.Rectangle
		min     int
		want    image.Rectangle
	}{
		{"aligns with the brand", image.Rect(16, 0, 123, 48), 120, image.Rect(4, 0, 123, 48)},
		{"keeps the window margin", image.Rect(10, 0, 80, 48), 40, image.Rect(4, 0, 80, 48)},
		{"widens left for the minimum", image.Rect(100, 0, 180, 48), 120, image.Rect(60, 0, 180, 48)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := siteMenuAnchor(tt.trigger, 12, tt.min, 4, bounds)
			if got != tt.want {
				t.Fatalf("siteMenuAnchor() = %v; want %v", got, tt.want)
			}
			if got.Max.X != tt.trigger.Max.X {
				t.Fatalf("menu right = %d; want chevron right %d", got.Max.X, tt.trigger.Max.X)
			}
		})
	}
}

func menuBox(anchor image.Rectangle, width, height, gap, margin int, bounds image.Rectangle) siteMenuBox {
	return siteMenuBox{
		anchor: anchor,
		width:  width,
		height: height,
		gap:    gap,
		margin: margin,
		bounds: bounds,
	}
}

func TestPlaceSiteMenuBelow(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(16, 0, 120, 52)
	got := placeSiteMenu(menuBox(anchor, 200, 90, 6, 8, bounds))
	want := image.Rect(16, 58, 216, 148)
	if got != want {
		t.Fatalf("placeSiteMenu() = %v; want %v", got, want)
	}
}

func TestPlaceSiteMenuFlipsAbove(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(16, 500, 120, 552)
	got := placeSiteMenu(menuBox(anchor, 200, 90, 6, 8, bounds))
	want := image.Rect(16, 404, 216, 494)
	if got != want {
		t.Fatalf("placeSiteMenu() = %v; want %v", got, want)
	}
}

func TestPlaceSiteMenuClampsRight(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(700, 0, 760, 52)
	got := placeSiteMenu(menuBox(anchor, 200, 90, 6, 8, bounds))
	want := image.Rect(592, 58, 792, 148)
	if got != want {
		t.Fatalf("placeSiteMenu() = %v; want %v", got, want)
	}
}

func TestPlaceSiteMenuEmpty(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(16, 0, 120, 52)
	tests := []struct {
		name   string
		width  int
		height int
		bounds image.Rectangle
	}{
		{"zero width", 0, 90, bounds},
		{"zero height", 200, 0, bounds},
		{"empty bounds", 200, 90, image.Rectangle{}},
		{"too tall", 200, 700, bounds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := placeSiteMenu(menuBox(anchor, tt.width, tt.height, 6, 8, tt.bounds)); !got.Empty() {
				t.Fatalf("placeSiteMenu() = %v; want empty", got)
			}
		})
	}
}
