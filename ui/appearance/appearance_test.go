package appearance_test

import (
	"testing"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

func TestNew(t *testing.T) {
	t.Parallel()

	app := appearance.New(appearance.Options{Resource: resource.New(), Language: "ru"})
	if app.Resource() == nil {
		t.Fatal("Resource() = nil")
	}
	if got := app.Locale().Code(); got != "ru" {
		t.Fatalf("Locale().Code() = %q; want ru", got)
	}
	if got := app.Typography().Font(appearance.RoleBody); got != resource.FontRegular {
		t.Fatalf("Typography().Font(RoleBody) = %q; want %q", got, resource.FontRegular)
	}
	if got := app.Palette().Canvas.A; got == 0 {
		t.Fatal("Palette().Canvas is fully transparent")
	}
}

func TestSetLanguage(t *testing.T) {
	t.Parallel()

	app := appearance.New(appearance.Options{Resource: resource.New(), Language: "ru"})
	app.SetLanguage("en")

	if got := app.Locale().Code(); got != "en" {
		t.Fatalf("Locale().Code() = %q; want en", got)
	}
}

func TestPaletteFor(t *testing.T) {
	t.Parallel()

	base := appearance.PaletteFor(catalog.SiteYoanime)
	if base == (appearance.Palette{}) {
		t.Fatal("PaletteFor(yoanime) returned the zero palette")
	}
	if base.Badge.A == 0 || base.Edge.A == 0 || base.Ink.A == 0 {
		t.Fatal("PaletteFor(yoanime) returned an empty badge chrome")
	}
	if base.ControlIcon.A == 0 || base.ControlIconHover.A == 0 {
		t.Fatal("PaletteFor(yoanime) returned empty control icons")
	}
	if got := appearance.BasePalette(); got.ControlIcon.A == 0 || got.Accent == base.Accent {
		t.Fatal("BasePalette did not return the neutral chrome")
	}
	seen := []appearance.Palette{base}
	for _, site := range catalog.Sites()[1:] {
		p := appearance.PaletteFor(site)
		for _, prev := range seen {
			if p.Accent == prev.Accent {
				t.Fatalf("PaletteFor(%q) reuses an accent", site)
			}
			if p.Badge == prev.Badge || p.Ink == prev.Ink {
				t.Fatalf("PaletteFor(%q) reuses badge chrome", site)
			}
		}
		seen = append(seen, p)
	}
	assertPanics(t, func() { appearance.PaletteFor(catalog.Site("unknown")) })
}

func TestSetPalette(t *testing.T) {
	t.Parallel()

	app := appearance.New(appearance.Options{Resource: resource.New(), Language: "ru"})
	want := appearance.PaletteFor(catalog.SiteYomanga)
	app.SetPalette(want)
	if got := app.Palette(); got != want {
		t.Fatal("SetPalette did not replace the palette")
	}

	var nilApp *appearance.Appearance
	assertPanics(t, func() { nilApp.SetPalette(want) })
	assertPanics(t, func() { app.SetPalette(appearance.Palette{}) })
}

func TestAppIcon(t *testing.T) {
	t.Parallel()

	for site, want := range map[catalog.Site]resource.AppIconKey{
		catalog.SiteYoanime:  resource.AppIconYoanime,
		catalog.SiteYomanga:  resource.AppIconYomanga,
		catalog.SiteYoranobe: resource.AppIconYoranobe,
	} {
		if got := appearance.AppIconFor(site); got != want {
			t.Fatalf("AppIconFor(%q) = %q; want %q", site, got, want)
		}
	}
	assertPanics(t, func() { appearance.AppIconFor(catalog.Site("unknown")) })
}

func TestSetAppIcon(t *testing.T) {
	t.Parallel()

	app := appearance.New(appearance.Options{Resource: resource.New(), Language: "ru"})
	if got := app.AppIcon(); got != resource.AppIconYobase {
		t.Fatalf("AppIcon() = %q; want %q", got, resource.AppIconYobase)
	}
	app.SetAppIcon(resource.AppIconYomanga)
	if got := app.AppIcon(); got != resource.AppIconYomanga {
		t.Fatalf("AppIcon() = %q; want %q", got, resource.AppIconYomanga)
	}

	var nilApp *appearance.Appearance
	assertPanics(t, func() { nilApp.SetAppIcon(resource.AppIconYobase) })
	assertPanics(t, func() { app.SetAppIcon("") })
}

func TestTypographyRoles(t *testing.T) {
	t.Parallel()

	app := appearance.New(appearance.Options{
		Resource: resource.New(),
		Language: "en",
	})
	typo := app.Typography()

	tests := []struct {
		name string
		role appearance.TextRole
		font resource.FontKey
		size float32
	}{
		{"display", appearance.RoleDisplay, resource.FontBold, 26},
		{"title", appearance.RoleTitle, resource.FontSemiBold, 23.5},
		{"subtitle", appearance.RoleSubtitle, resource.FontMedium, 15},
		{"body", appearance.RoleBody, resource.FontRegular, 12.5},
		{"label", appearance.RoleLabel, resource.FontBold, 13.2},
		{"caption", appearance.RoleCaption, resource.FontMedium, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := typo.Font(tt.role); got != tt.font {
				t.Fatalf("Font(%s) = %v; want %v", tt.name, got, tt.font)
			}
			if got := typo.TextSize(tt.role); got != tt.size {
				t.Fatalf("TextSize(%s) = %v; want %v", tt.name, got, tt.size)
			}
		})
	}
}

func TestNewPanics(t *testing.T) {
	t.Parallel()

	res := resource.New()
	tests := []struct {
		name string
		opts appearance.Options
	}{
		{"nil resources", appearance.Options{Language: "ru"}},
		{"empty language", appearance.Options{Resource: res}},
		{"unknown language", appearance.Options{Resource: res, Language: "xx"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertPanics(t, func() { appearance.New(tt.opts) })
		})
	}
}

func TestSetLanguagePanics(t *testing.T) {
	t.Parallel()

	res := resource.New()
	app := appearance.New(appearance.Options{Resource: res, Language: "ru"})
	tests := []struct {
		name string
		call func()
	}{
		{"nil appearance", func() {
			var nilApp *appearance.Appearance
			nilApp.SetLanguage("en")
		}},
		{"empty language", func() {
			app.SetLanguage("")
		}},
		{"unknown language", func() {
			app.SetLanguage("xx")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertPanics(t, tt.call)
		})
	}
}

func TestUnknownTextRolePanics(t *testing.T) {
	t.Parallel()

	app := appearance.New(appearance.Options{
		Resource: resource.New(),
		Language: "ru",
	})
	typo := app.Typography()
	for _, role := range []appearance.TextRole{appearance.RoleCaption + 1, 255} {
		assertPanics(t, func() { typo.Font(role) })
		assertPanics(t, func() { typo.TextSize(role) })
	}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("call did not panic")
		}
	}()
	fn()
}
