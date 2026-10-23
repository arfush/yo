package ui

import (
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestTokenCardMatchesFirstRunSize(t *testing.T) {
	if tokenCardWidth != firstRunCardWidth || tokenCardHeight != firstRunCardHeight {
		t.Fatalf(
			"token card is %dx%d, first-run card is %dx%d",
			tokenCardWidth, tokenCardHeight, firstRunCardWidth, firstRunCardHeight,
		)
	}
}

func TestTokenButtonMatchesFirstRunButton(t *testing.T) {
	if tokenButtonTop != firstRunButtonTop || tokenButtonHeight != firstRunButtonHeight {
		t.Fatalf(
			"token button top/height %d/%d, first-run button %d/%d",
			tokenButtonTop, tokenButtonHeight, firstRunButtonTop, firstRunButtonHeight,
		)
	}
	if tokenButtonTop+tokenButtonHeight > tokenCardHeight {
		t.Fatalf("button bottom %d exceeds card height %d", tokenButtonTop+tokenButtonHeight, tokenCardHeight)
	}
}

func TestDomainFieldFitsCard(t *testing.T) {
	bottom := tokenDomainFieldTop + tokenDomainFieldHeight
	if bottom > tokenCardHeight {
		t.Fatalf("domain field bottom %d exceeds card height %d", bottom, tokenCardHeight)
	}
	// The field replaces the GO button, so both share a center line.
	fieldCenter := tokenDomainFieldTop + tokenDomainFieldHeight/2
	buttonCenter := tokenButtonTop + tokenButtonHeight/2
	if fieldCenter != buttonCenter {
		t.Fatalf("domain field center %d, button center %d", fieldCenter, buttonCenter)
	}
}

func TestDomainEditorTexts(t *testing.T) {
	if tokenDomainTitle != "AnimeLIB сломался?" {
		t.Fatalf("domain title = %q", tokenDomainTitle)
	}
	if tokenDomainDescription != "Введите домен, который у вас открывается" {
		t.Fatalf("domain description = %q", tokenDomainDescription)
	}
}

func TestTokenTextsFitCard(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatalf("newTypography: %v", err)
	}
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	normal := font.Font{Typeface: fontFamily, Weight: font.Normal}
	bold := font.Font{Typeface: fontFamily, Weight: font.Bold}
	width := tokenCardWidth - 2*tokenCardPadding

	measure := func(text string, face font.Font, size unit.Sp) {
		t.Helper()
		_, ink := typo.measure(gtx, text, face, size)
		if ink.Dx() > width {
			t.Errorf("%q is %dpx wide, card allows %dpx", text, ink.Dx(), width)
		}
	}

	measure(tokenTitle, bold, tokenTitleSize)
	measure(tokenDescription, normal, tokenDescSize)
	measure(tokenDomainTitle, bold, tokenTitleSize)
	measure(tokenDomainDescription, normal, tokenDescSize)
}

func TestTokenLabelShakeEndsAtZero(t *testing.T) {
	start := time.Now()
	if shake := tokenLabelShake(start, start); shake != 0 {
		t.Fatalf("shake at start = %v, want 0", shake)
	}
	if shake := tokenLabelShake(start.Add(tokenShakeDuration), start); shake != 0 {
		t.Fatalf("shake at end = %v, want 0", shake)
	}
	if shake := tokenLabelShake(start.Add(tokenShakeDuration/4), start); shake == 0 {
		t.Fatal("shake should move during the animation")
	}
}

func TestTokenErrorFeedbackRestoresAfterThreeSeconds(t *testing.T) {
	start := time.Now()
	errored, _ := tokenErrorFeedback(start, start)
	if !errored {
		t.Fatal("error feedback should start immediately")
	}
	errored, _ = tokenErrorFeedback(start.Add(tokenErrorDuration), start)
	if errored {
		t.Fatal("error feedback should end after three seconds")
	}
}
