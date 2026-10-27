package ui

import (
	"testing"
	"time"

	"gioui.org/font"
)

func TestShellErrorExpiresAfterFiveSeconds(t *testing.T) {
	s := &shell{}
	start := time.Unix(100, 0)
	s.setError("Не удалось загрузить тайтл", start)
	wantUntil := start.Add(5 * time.Second)
	if !s.errorUntil.Equal(wantUntil) || !s.logAlertUntil.Equal(wantUntil) {
		t.Fatalf("deadlines = %v / %v, want %v", s.errorUntil, s.logAlertUntil, wantUntil)
	}
	if s.expireError(wantUntil.Add(-time.Millisecond)) {
		t.Fatal("error expired too early")
	}
	if s.errorText == "" {
		t.Fatal("error disappeared before five seconds")
	}
	if !s.expireError(wantUntil) {
		t.Fatal("error did not expire at five seconds")
	}
	if s.errorText != "" || !s.errorUntil.IsZero() {
		t.Fatalf("error state survived expiry: %q %v", s.errorText, s.errorUntil)
	}
}

func TestHeaderSectionNamesShareBaseline(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	gtx := readerTestContext()
	face := font.Font{Typeface: fontFamily, Weight: font.Bold}
	centerY := 32

	var want int
	for index, section := range []librarySection{sectionAnime, sectionManga, sectionRanobe} {
		name := section.appName()
		top := headerTitleTop(gtx, typo, name, centerY, face, titleSize)
		glyphs, ink := typo.measure(gtx, name, face, titleSize)
		if len(glyphs) == 0 || ink.Empty() {
			t.Fatalf("%s measured empty", name)
		}
		baseline := top - ink.Min.Y + int(glyphs[0].Y)
		if index == 0 {
			want = baseline
			continue
		}
		if baseline != want {
			t.Fatalf("%s baseline = %d, want %d", name, baseline, want)
		}
	}
}
