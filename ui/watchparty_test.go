package ui

import (
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yonime/internal/watchparty"
)

func TestButtonErrorFeedbackWindow(t *testing.T) {
	start := time.Unix(1000, 0)

	if errored, _ := buttonErrorFeedback(start.Add(-time.Second), start, true); errored {
		t.Fatal("feedback must not appear before the failure")
	}
	if errored, _ := buttonErrorFeedback(start.Add(time.Second), start, true); !errored {
		t.Fatal("feedback must appear for three seconds after the failure")
	}
	if errored, _ := buttonErrorFeedback(start.Add(wpFeedbackDelay), start, true); errored {
		t.Fatal("feedback must end exactly at the three-second mark")
	}
	if errored, _ := buttonErrorFeedback(start, time.Time{}, true); errored {
		t.Fatal("a zero failure time must not report an error")
	}
}

func TestLabelShakeStartsAndEndsCalm(t *testing.T) {
	start := time.Unix(2000, 0)

	if got := labelShake(start, start); got != 0 {
		t.Fatalf("labelShake at the start = %v, want 0", got)
	}
	mid := labelShake(start.Add(wpShakeDuration/2), start)
	if mid > 1 || mid < -1 {
		t.Fatalf("labelShake in the middle = %v, want within [-1, 1]", mid)
	}
	if got := labelShake(start.Add(wpShakeDuration), start); got != 0 {
		t.Fatalf("labelShake after the shake = %v, want 0", got)
	}
}

func TestWatchPartyOpensOnChoiceWithoutRoom(t *testing.T) {
	party := &partyManager{provider: watchparty.Cloudflare}
	screen := newWatchPartyWindow(nil, nil, party)
	if screen.mode != wpChoice {
		t.Fatalf("mode = %v, want wpChoice without a room", screen.mode)
	}
}

func TestWatchPartyReopensCreatedFormForHost(t *testing.T) {
	party := &partyManager{
		provider: watchparty.Cloudflare,
		session:  &watchparty.Session{Key: "YOFBQOPGZ", Provider: watchparty.Cloudflare, Role: watchparty.Host},
	}
	screen := newWatchPartyWindow(nil, nil, party)
	if screen.mode != wpCreated {
		t.Fatalf("mode = %v, want wpCreated for a host", screen.mode)
	}
	snapshot := party.Snapshot()
	if snapshot.Key != "YOFBQOPGZ" || !snapshot.Active {
		t.Fatalf("snapshot = %+v, want the retained key", snapshot)
	}
}

func TestWatchPartyReopensWaitingScreenForGuest(t *testing.T) {
	party := &partyManager{
		provider: watchparty.Cloudflare,
		session:  &watchparty.Session{Key: "YOFBQOPGZ", Provider: watchparty.Cloudflare, Role: watchparty.Guest},
	}
	screen := newWatchPartyWindow(nil, nil, party)
	if screen.mode != wpWaiting {
		t.Fatalf("mode = %v, want wpWaiting for a guest", screen.mode)
	}
}

func TestPartySnapshotFallsBackToSelectedProvider(t *testing.T) {
	party := &partyManager{provider: watchparty.Supabase}
	snapshot := party.Snapshot()
	if snapshot.Provider != watchparty.Supabase {
		t.Fatalf("provider = %q, want supabase", snapshot.Provider)
	}
	if snapshot.Active {
		t.Fatal("a manager without a session must not report an active room")
	}
	if party.HasRoom() {
		t.Fatal("a manager without a session must not report a room")
	}
}

// TestWatchPartyCardFitsText keeps the manually placed card from overflowing:
// the provider chip has to fit next to the title and every description has to
// stay above the action row.
func TestWatchPartyCardFitsText(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatalf("newTypography: %v", err)
	}
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	bold := font.Font{Typeface: fontFamily, Weight: font.Bold}
	medium := font.Font{Typeface: fontFamily, Weight: font.Medium}
	normal := font.Font{Typeface: fontFamily, Weight: font.Normal}
	width := wpCardWidth - 2*wpCardPadding

	_, titleInk := typo.measure(gtx, wpTitleChoice, bold, wpTitleSize)
	_, chipInk := typo.measure(gtx, "Cloudflare", medium, wpChipSize)
	row := titleInk.Dx() + 2*wpChipPadX + chipInk.Dx() + 12
	if row > width {
		t.Fatalf("title and provider chip need %dpx, card content allows %dpx", row, width)
	}

	advance := int(float64(gtx.Sp(wpDescSize))*1.25) + 1
	for _, description := range []string{wpDescChoice, wpDescCreated, wpDescJoin} {
		lines := wrapFirstRunText(gtx, typo, description, normal, wpDescSize, width)
		for _, line := range lines {
			if _, ink := typo.measure(gtx, line, normal, wpDescSize); ink.Dx() > width {
				t.Errorf("description line %q is %dpx wide, card allows %dpx", line, ink.Dx(), width)
			}
		}
		bottom := wpDescTop + len(lines)*advance
		if bottom > wpActionTop {
			t.Errorf("description %q ends at %dpx, actions start at %dpx", description, bottom, wpActionTop)
		}
	}

	if wpActionTop+wpFieldHeight+14+16 > wpCardHeight {
		t.Fatalf("key field and delete link do not fit the %dpx card", wpCardHeight)
	}
	if wpErrorTop+16 > wpCardHeight {
		t.Fatalf("error line at %dpx does not fit the %dpx card", wpErrorTop, wpCardHeight)
	}

	// The round submit button carries the word "Далее", so the label has to fit
	// inside the circle with a little breathing room.
	semiBold := font.Font{Typeface: fontFamily, Weight: font.SemiBold}
	_, nextInk := typo.measure(gtx, wpNextLabel, semiBold, wpRoundLabelSize)
	if nextInk.Dx() > wpRoundSize-8 {
		t.Fatalf("%q is %dpx wide, the %dpx circle leaves %dpx", wpNextLabel, nextInk.Dx(), wpRoundSize, wpRoundSize-8)
	}
}
