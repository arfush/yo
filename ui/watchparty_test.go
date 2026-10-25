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
		session:  &watchparty.Session{Key: "YOCBQOPGZ", Provider: watchparty.Cloudflare, Role: watchparty.Host},
	}
	screen := newWatchPartyWindow(nil, nil, party)
	if screen.mode != wpCreated {
		t.Fatalf("mode = %v, want wpCreated for a host", screen.mode)
	}
	snapshot := party.Snapshot()
	if snapshot.Key != "YOCBQOPGZ" || !snapshot.Active {
		t.Fatalf("snapshot = %+v, want the retained key", snapshot)
	}
}

func TestWatchPartyReopensWaitingScreenForGuest(t *testing.T) {
	party := &partyManager{
		provider: watchparty.Cloudflare,
		session:  &watchparty.Session{Key: "YOCBQOPGZ", Provider: watchparty.Cloudflare, Role: watchparty.Guest},
	}
	screen := newWatchPartyWindow(nil, nil, party)
	if screen.mode != wpWaiting {
		t.Fatalf("mode = %v, want wpWaiting for a guest", screen.mode)
	}
}

func TestPartyProviderChoiceRequiresTwoRelays(t *testing.T) {
	single := &partyManager{cfg: watchparty.Config{CloudflareURL: "wss://relay.example.workers.dev"}}
	if single.HasProviderChoice() {
		t.Fatal("a build with one relay must not offer a provider switch")
	}
	both := &partyManager{cfg: watchparty.Config{
		CloudflareURL: "wss://relay.example.workers.dev",
		SupabaseURL:   "wss://project.supabase.co",
		SupabaseKey:   "sb_publishable_example",
	}}
	if !both.HasProviderChoice() {
		t.Fatal("a build with both relays must allow switching")
	}
	var missing *partyManager
	if missing.HasProviderChoice() {
		t.Fatal("a nil manager must not report a choice")
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
// the title has to fit the card and every description has to stay above the
// action row.
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
	normal := font.Font{Typeface: fontFamily, Weight: font.Normal}
	width := wpCardWidth - 2*wpCardPadding

	_, titleInk := typo.measure(gtx, wpTitleChoice, bold, wpTitleSize)
	if titleInk.Dx() > width {
		t.Fatalf("title needs %dpx, card content allows %dpx", titleInk.Dx(), width)
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

	// The round submit button is an icon now. Its chevron has to stay inside
	// the circle with the same breathing room the token form uses.
	if chevron := wpRoundSize / 3; chevron+8 > wpRoundSize {
		t.Fatalf("%dpx chevron does not fit the %dpx circle", chevron, wpRoundSize)
	}
}

func TestWaitingTimeoutOnlyAppliesToUnknownHost(t *testing.T) {
	now := time.Now()
	stale := now.Add(-wpWaitTimeout - time.Second)

	if !waitingTimedOut(now, stale, partySnapshot{}) {
		t.Fatal("a key that never reached a host should time out")
	}
	if waitingTimedOut(now, stale, partySnapshot{HostEverSeen: true}) {
		t.Fatal("the grace window must not look like a wrong room key")
	}
	if waitingTimedOut(now, stale, partySnapshot{HostSeen: true}) {
		t.Fatal("a present host must keep the waiting screen alive")
	}
	if waitingTimedOut(now, now, partySnapshot{}) {
		t.Fatal("the timeout fired before the window elapsed")
	}
}

func TestRoomClosedShowsJoinFormWithReason(t *testing.T) {
	screen := newWatchPartyWindow(nil, nil, &partyManager{provider: watchparty.Cloudflare})
	screen.mode = wpWaiting
	screen.busy = true
	screen.editor.SetText("YOCBQOPGZ")

	screen.roomClosed(hostClosedNotice)

	if screen.mode != wpJoin {
		t.Fatalf("mode = %d, want the join form", screen.mode)
	}
	if screen.errorText != hostClosedNotice {
		t.Fatalf("error = %q, want the host-left reason", screen.errorText)
	}
	if screen.joinFeedback {
		t.Fatal("the host-left reason must not auto-clear like a mistyped key")
	}
	if screen.busy {
		t.Fatal("a closed room must not leave the screen busy")
	}
	if got := screen.editor.Text(); got != "" {
		t.Fatalf("key editor = %q, want it cleared", got)
	}
}

func TestHostGoneLeavesANotice(t *testing.T) {
	party := &partyManager{provider: watchparty.Cloudflare}
	party.hostGone(hostLostNotice)

	if got := party.takeClosedNotice(); got != hostLostNotice {
		t.Fatalf("notice = %q, want the host-lost reason", got)
	}
	if got := party.takeClosedNotice(); got != "" {
		t.Fatalf("notice repeated: %q", got)
	}
}

func TestPartyWindowStartsWithTheHostLeftReason(t *testing.T) {
	party := &partyManager{provider: watchparty.Cloudflare}
	party.closedNotice = hostClosedNotice

	screen := newWatchPartyWindow(nil, nil, party)
	if screen.mode != wpJoin {
		t.Fatalf("mode = %d, want the join form", screen.mode)
	}
	if screen.errorText != hostClosedNotice {
		t.Fatalf("error = %q, want the host-left reason", screen.errorText)
	}
	if got := party.takeClosedNotice(); got != "" {
		t.Fatalf("the notice was not consumed: %q", got)
	}
}

func TestPreferredProviderFollowsTheSetting(t *testing.T) {
	both := watchparty.Config{
		CloudflareURL: "wss://relay.example",
		SupabaseURL:   "wss://supabase.example",
		SupabaseKey:   "sb_publishable_x",
	}
	onlySupabase := watchparty.Config{
		SupabaseURL: "wss://supabase.example",
		SupabaseKey: "sb_publishable_x",
	}

	cases := []struct {
		name  string
		cfg   watchparty.Config
		saved string
		want  watchparty.Provider
	}{
		{"saved supabase", both, string(watchparty.Supabase), watchparty.Supabase},
		{"saved cloudflare", both, string(watchparty.Cloudflare), watchparty.Cloudflare},
		{"auto without a choice", both, "", watchparty.Cloudflare},
		{"unknown value", both, "bogus", watchparty.Cloudflare},
		{"saved provider missing from the build", onlySupabase, string(watchparty.Cloudflare), watchparty.Supabase},
		{"single relay build", onlySupabase, "", watchparty.Supabase},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := preferredProvider(test.cfg, test.saved); got != test.want {
				t.Fatalf("provider = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGuestControlUsesTheHostDecision(t *testing.T) {
	off := false
	on := true

	if guestControlAllowed(watchparty.State{ControlEnabled: &off}, true) {
		t.Fatal("the guest's local setting must not override the host's refusal")
	}
	if !guestControlAllowed(watchparty.State{ControlEnabled: &on}, false) {
		t.Fatal("the host's permission must work even when the guest's local setting is off")
	}
	if !guestControlAllowed(watchparty.State{}, true) {
		t.Fatal("an older host without the field must fall back to the local setting")
	}
	if guestControlAllowed(watchparty.State{}, false) {
		t.Fatal("the local setting must still be able to refuse control")
	}
}
