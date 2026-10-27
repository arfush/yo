package ui

import (
	"image"
	"image/color"
	"math"
	"testing"

	"gioui.org/font"
)

func TestSectionThemeChangesEveryAccent(t *testing.T) {
	t.Cleanup(func() { applySectionTheme(sectionAnime) })

	anime := sectionAnime.palette()
	applySectionTheme(sectionAnime)
	if mainAccentColor1 != anime.accent1 || badgeTextColor != anime.ink {
		t.Fatalf("anime accent = %v/%v, want %v/%v", mainAccentColor1, badgeTextColor, anime.accent1, anime.ink)
	}

	manga := sectionManga.palette()
	applySectionTheme(sectionManga)
	if mainAccentColor1 != manga.accent1 || mainAccentColor2 != manga.accent2 {
		t.Fatalf("manga accent = %v/%v, want %v/%v", mainAccentColor1, mainAccentColor2, manga.accent1, manga.accent2)
	}
	if badgeFillColor != manga.badge || badgeBorderColor != manga.edge || badgeTextColor != manga.ink {
		t.Fatalf("manga badge colors = %v/%v/%v, want %v/%v/%v", badgeFillColor, badgeBorderColor, badgeTextColor, manga.badge, manga.edge, manga.ink)
	}

	ranobe := sectionRanobe.palette()
	applySectionTheme(sectionRanobe)
	if mainAccentColor1 != ranobe.accent1 || mainAccentColor2 != ranobe.accent2 {
		t.Fatalf("ranobe accent = %v/%v, want %v/%v", mainAccentColor1, mainAccentColor2, ranobe.accent1, ranobe.accent2)
	}
	if footerControlHoverIconColor != mixColor(ranobe.accent1, titleColor, 0.55) {
		t.Fatal("footer hover accent did not follow the section")
	}
}

func TestRecolorPurpleIconKeepsShapeAndAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff})
	src.SetNRGBA(1, 0, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	src.SetNRGBA(2, 0, color.NRGBA{A: 0})

	orange := sectionManga.palette().accent1
	dst := recolorPurpleIcon(src, orange)
	purple := color.NRGBAModel.Convert(dst.At(0, 0)).(color.NRGBA)
	if purple.R <= purple.B {
		t.Fatalf("recolored purple pixel = %v, want a warm hue", purple)
	}
	if got := color.NRGBAModel.Convert(dst.At(1, 0)).(color.NRGBA); got != (color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Fatalf("white artwork changed to %v", got)
	}
	if got := color.NRGBAModel.Convert(dst.At(2, 0)).(color.NRGBA); got.A != 0 {
		t.Fatalf("transparent pixel alpha = %d, want 0", got.A)
	}
}

func TestSectionBrandKeepsFixedGapsForEveryName(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	s := &shell{typo: typo}
	gtx := badgeTestContext(1)
	g := makeGeometry(gtx, false, true, true, true)
	brand := s.brandLayout(gtx, g)

	if brand.titleX <= g.logo.Max.X {
		t.Fatalf("title starts at %d, logo ends at %d", brand.titleX, g.logo.Max.X)
	}
	if brand.badgeX <= brand.right {
		t.Fatalf("badge at %d overlaps brand ending at %d", brand.badgeX, brand.right)
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Bold}
	for _, section := range []librarySection{sectionAnime, sectionManga, sectionRanobe} {
		_, ink := typo.measure(gtx, section.appName(), face, titleSize)
		if end := brand.titleX + ink.Dx(); end >= brand.chevron.X-4 {
			t.Fatalf("%s ends at %d, chevron is at %d", section.appName(), end, brand.chevron.X)
		}
	}
}

func TestHueRoundTrip(t *testing.T) {
	for _, c := range []color.NRGBA{
		{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff},
		{R: 0xf0, G: 0x8a, B: 0x2c, A: 0xff},
		{R: 0x4c, G: 0x8d, B: 0xf6, A: 0xff},
	} {
		h, s, v := rgbToHSV(c)
		got := hsvToRGB(h, s, v, c.A)
		if math.Abs(float64(got.R)-float64(c.R)) > 1 ||
			math.Abs(float64(got.G)-float64(c.G)) > 1 ||
			math.Abs(float64(got.B)-float64(c.B)) > 1 {
			t.Fatalf("hsv round trip %v -> %v", c, got)
		}
	}
}
