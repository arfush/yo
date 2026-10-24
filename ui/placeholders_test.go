package ui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gioui.org/layout"
)

func TestAnimeLinkPromptList(t *testing.T) {
	if len(animeLinkPlaceholders) < 2 {
		t.Fatalf("got %d prompt phrases, need at least two to rotate", len(animeLinkPlaceholders))
	}
	seen := make(map[string]bool, len(animeLinkPlaceholders))
	for i, phrase := range animeLinkPlaceholders {
		if strings.TrimSpace(phrase) == "" {
			t.Fatalf("prompt %d is empty", i)
		}
		if !strings.Contains(phrase, "AnimeLIB") {
			t.Fatalf("prompt %d does not mention AnimeLIB: %q", i, phrase)
		}
		if seen[phrase] {
			t.Fatalf("prompt %d is duplicated: %q", i, phrase)
		}
		seen[phrase] = true
	}
}

func TestPlaceholderClearingEditorShowsFullPhrase(t *testing.T) {
	now := time.Unix(100, 0)
	p := placeholderState{
		text:          "Старая фраза AnimeLIB",
		visible:       3,
		phase:         placeholderErasing,
		idleAt:        now.Add(-time.Minute),
		lastEditorLen: 7,
	}

	p.update(layout.Context{Now: now}, 0, false)
	if p.lastEditorLen != 0 {
		t.Fatalf("last editor length = %d, want 0", p.lastEditorLen)
	}
	if p.visible != utf8.RuneCountInString(p.text) {
		t.Fatalf("visible = %d, want the full prompt (%d runes)", p.visible, utf8.RuneCountInString(p.text))
	}
	if p.phase != placeholderIdle {
		t.Fatalf("phase = %d, want idle", p.phase)
	}
}

func TestPlaceholderFocusRevealsFullPrompt(t *testing.T) {
	now := time.Unix(100, 0)
	p := newPlaceholderState()
	p.visible = 2
	p.phase = placeholderTyping

	p.update(layout.Context{Now: now}, 0, true)
	if p.phase != placeholderIdle {
		t.Fatalf("phase = %d, want idle while focused", p.phase)
	}
	if p.visible != utf8.RuneCountInString(p.text) {
		t.Fatalf("visible = %d, want full prompt %d", p.visible, utf8.RuneCountInString(p.text))
	}
}

func TestPlaceholderStartsRewriteAfterIdleDelay(t *testing.T) {
	start := time.Unix(100, 0)
	p := newPlaceholderState()
	p.update(layout.Context{Now: start}, 0, false)
	if p.phase != placeholderIdle {
		t.Fatalf("phase = %d, want idle", p.phase)
	}
	p.update(layout.Context{Now: start.Add(placeholderIdleDelay)}, 0, false)
	if p.phase != placeholderErasing {
		t.Fatalf("phase = %d, want erasing after idle delay", p.phase)
	}
}
