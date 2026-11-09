package shell

import (
	"image"
	"testing"

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
		Resources: resource.New(),
		Language:  "ru",
	}))
	sites := stdx.NewHolder(catalog.SiteYoanime)
	shell := New(Options{Screen: testScreen{}, Site: sites, Appearance: apps})

	shell.switchSite(catalog.SiteYomanga)
	if got := sites.Value(); got != catalog.SiteYomanga {
		t.Fatalf("site holder = %q; want %q", got, catalog.SiteYomanga)
	}
	want := appearance.PaletteFor(catalog.SiteYomanga)
	if got := apps.Value().Palette().Accent; got != want.Accent {
		t.Fatal("switchSite did not sync the site palette")
	}
}

func TestPlaceSiteMenuBelow(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(16, 0, 120, 52)
	got := placeSiteMenu(anchor, 200, 90, 6, 8, bounds)
	want := image.Rect(16, 58, 216, 148)
	if got != want {
		t.Fatalf("placeSiteMenu() = %v; want %v", got, want)
	}
}

func TestPlaceSiteMenuFlipsAbove(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(16, 500, 120, 552)
	got := placeSiteMenu(anchor, 200, 90, 6, 8, bounds)
	want := image.Rect(16, 404, 216, 494)
	if got != want {
		t.Fatalf("placeSiteMenu() = %v; want %v", got, want)
	}
}

func TestPlaceSiteMenuClampsRight(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(700, 0, 760, 52)
	got := placeSiteMenu(anchor, 200, 90, 6, 8, bounds)
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
			if got := placeSiteMenu(anchor, tt.width, tt.height, 6, 8, tt.bounds); !got.Empty() {
				t.Fatalf("placeSiteMenu() = %v; want empty", got)
			}
		})
	}
}
