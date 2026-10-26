package ui

import "testing"

func TestSavedTitleButtonUsesMutedGreen(t *testing.T) {
	fill, border, text := titleFooterButtonPalette(titleFooterSaved, 0)
	for name, col := range map[string]struct{ r, g, b uint8 }{
		"fill":   {fill.R, fill.G, fill.B},
		"border": {border.R, border.G, border.B},
		"text":   {text.R, text.G, text.B},
	} {
		if col.g <= col.r || col.g <= col.b {
			t.Fatalf("saved %s is not green: r=%#x g=%#x b=%#x", name, col.r, col.g, col.b)
		}
	}
	if text.G > 0xd0 {
		t.Fatalf("saved text is too bright for a muted state: %#x", text.G)
	}

	fill, border, text = titleFooterButtonPalette(titleFooterSecondary, 0)
	if fill.G > fill.R || border.G > border.R || text.G > text.R {
		t.Fatal("the ordinary footer button gained the saved green tone")
	}
}
