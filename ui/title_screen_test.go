package ui

import (
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

func TestSavedTitleButtonUsesMutedGreen(t *testing.T) {
	fill, border, text := footerSavedStyle.palette(0)
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

	fill, border, text = footerSecondaryStyle.palette(0)
	if fill.G > fill.R || border.G > border.R || text.G > text.R {
		t.Fatal("the ordinary footer button gained the saved green tone")
	}
}

func TestStepEpisodeMovesLeftAndRight(t *testing.T) {
	gtx := layout.Context{Ops: new(op.Ops), Now: time.Unix(700, 0)}
	w := &titleScreen{item: historyTitle{episodes: []string{"1", "2", "3", "4", "5"}}}

	w.stepEpisode(gtx, 1)
	if w.selectedEpisode != 1 {
		t.Fatalf("right arrow selected episode %d, want 1", w.selectedEpisode)
	}
	w.selectedEpisode = 4
	w.stepEpisode(gtx, -1)
	if w.selectedEpisode != 3 {
		t.Fatalf("left arrow selected episode %d, want 3", w.selectedEpisode)
	}
}
