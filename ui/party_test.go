package ui

import (
	"errors"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yo/party"
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
	if _, shake := buttonErrorFeedback(start.Add(wpShakeDuration/2), start, true); shake == 0 {
		t.Fatal("a join error must shake the icon as well as turn the button red")
	}
	if _, shake := buttonErrorFeedback(start.Add(wpShakeDuration/2), start, false); shake != 0 {
		t.Fatal("a caller that only needs the red state must not receive a shake")
	}
	screen := &partyScreen{errorAt: start}
	if !screen.shakeRunning(start) {
		t.Fatal("the first shake frame must still request the next one")
	}
	if screen.shakeRunning(start.Add(wpShakeDuration)) {
		t.Fatal("animation frames must stop when the shake ends")
	}
}

func TestJoinFailureClearsTheKeyAndStartsRedFeedback(t *testing.T) {
	screen := newPartyScreen(nil, nil, &partyManager{provider: party.Cloudflare})
	screen.mode = wpJoin
	screen.busy = true
	screen.editor.SetText("YOC4K7Q2M")

	screen.handleResult(partyResult{kind: partyJoined, err: errors.New("комната не найдена — проверьте ключ")})

	if screen.busy {
		t.Fatal("a failed join must stop the button spinner")
	}
	if got := screen.editor.Text(); got != "" {
		t.Fatalf("key editor = %q, want it cleared", got)
	}
	if !screen.joinFeedback {
		t.Fatal("a join error must use the auto-clearing red feedback")
	}
	if screen.errorText == "" || screen.errorAt.IsZero() {
		t.Fatalf("error = %q at %v, want text and a start time", screen.errorText, screen.errorAt)
	}
	if !screen.focusField {
		t.Fatal("the cleared key field should take focus again")
	}
}

func TestCreateFailureKeepsItsOwnFeedback(t *testing.T) {
	screen := newPartyScreen(nil, nil, &partyManager{provider: party.Cloudflare})
	screen.busy = true
	screen.editor.SetText("YOC4K7Q2M")

	screen.handleResult(partyResult{kind: partyCreated, err: errors.New("реле не настроено")})

	if screen.busy {
		t.Fatal("a failed create must stop the button spinner")
	}
	if got := screen.editor.Text(); got != "YOC4K7Q2M" {
		t.Fatalf("key editor = %q, create errors must not clear the join field", got)
	}
	if screen.joinFeedback {
		t.Fatal("a create error must stay until the next attempt")
	}
}

func TestPartyOpensOnChoiceWithoutRoom(t *testing.T) {
	manager := &partyManager{provider: party.Cloudflare}
	screen := newPartyScreen(nil, nil, manager)
	if screen.mode != wpChoice {
		t.Fatalf("mode = %v, want wpChoice without a room", screen.mode)
	}
}

func TestPartyReopensCreatedFormForHost(t *testing.T) {
	manager := &partyManager{
		provider: party.Cloudflare,
		session:  &party.Session{Key: "YOC4K7Q2M", Provider: party.Cloudflare, Role: party.Host},
	}
	screen := newPartyScreen(nil, nil, manager)
	if screen.mode != wpCreated {
		t.Fatalf("mode = %v, want wpCreated for a host", screen.mode)
	}
	snapshot := manager.Snapshot()
	if snapshot.Key != "YOC4K7Q2M" || !snapshot.Active {
		t.Fatalf("snapshot = %+v, want the retained key", snapshot)
	}
}

func TestPartyReopensWaitingScreenWithAHat(t *testing.T) {
	manager := &partyManager{
		provider: party.Cloudflare,
		session:  &party.Session{Key: "YOC4K7Q2M", Provider: party.Cloudflare, Role: party.Guest},
	}
	screen := newPartyScreen(nil, nil, manager)
	if screen.mode != wpWaiting {
		t.Fatalf("mode = %v, want wpWaiting for a guest", screen.mode)
	}
	if screen.waitHat < 0 || screen.waitHat >= mascotHatCount {
		t.Fatalf("first wait hat = %d, want a hat in [0, %d)", screen.waitHat, mascotHatCount)
	}
	reopened := newPartyScreen(nil, nil, manager)
	if reopened.mode != wpWaiting {
		t.Fatalf("reopened mode = %v, want wpWaiting for a guest", reopened.mode)
	}
	if reopened.waitHat < 0 || reopened.waitHat >= mascotHatCount {
		t.Fatalf("reopened wait hat = %d, want a hat in [0, %d)", reopened.waitHat, mascotHatCount)
	}
}

func TestPartySnapshotFallsBackToSelectedProvider(t *testing.T) {
	manager := &partyManager{provider: party.Supabase}
	snapshot := manager.Snapshot()
	if snapshot.Provider != party.Supabase {
		t.Fatalf("provider = %q, want supabase", snapshot.Provider)
	}
	if snapshot.Active {
		t.Fatal("a manager without a session must not report an active room")
	}
	if manager.HasRoom() {
		t.Fatal("a manager without a session must not report a room")
	}
}

// TestPartyCardFitsText keeps the manually placed card from overflowing:
// the title has to fit the card and every description has to stay above the
// action row.
func TestPartyCardFitsText(t *testing.T) {
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
	descriptions := []struct {
		text  string
		limit int
	}{
		// The locked host card puts the secret chip under the description, so
		// its text has to stop above the chip instead of above the field.
		{wpDescChoice, wpActionTop},
		{wpDescCreated, wpActionTop},
		{wpDescLocked, wpSecretTop},
		{wpDescJoin, wpActionTop},
		{wpDescSecret, wpActionTop},
	}
	for _, description := range descriptions {
		lines := wrapFirstRunText(gtx, typo, description.text, normal, wpDescSize, width)
		for _, line := range lines {
			if _, ink := typo.measure(gtx, line, normal, wpDescSize); ink.Dx() > width {
				t.Errorf("description line %q is %dpx wide, card allows %dpx", line, ink.Dx(), width)
			}
		}
		bottom := wpDescTop + len(lines)*advance
		if bottom > description.limit {
			t.Errorf("description %q ends at %dpx, content starts at %dpx", description.text, bottom, description.limit)
		}
	}

	if wpSecretTop+wpSecretHeight+8 > wpActionTop {
		t.Fatalf("secret chip at %dpx overlaps the key field at %dpx", wpSecretTop, wpActionTop)
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
	screen := newPartyScreen(nil, nil, &partyManager{provider: party.Cloudflare})
	screen.mode = wpWaiting
	screen.busy = true
	screen.editor.SetText("YOC4K7Q2M")

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
	manager := &partyManager{provider: party.Cloudflare}
	manager.hostGone(hostLostNotice)

	if got := manager.takeClosedNotice(); got != hostLostNotice {
		t.Fatalf("notice = %q, want the host-lost reason", got)
	}
	if got := manager.takeClosedNotice(); got != "" {
		t.Fatalf("notice repeated: %q", got)
	}
}

func TestPartyScreenStartsWithTheHostLeftReason(t *testing.T) {
	manager := &partyManager{provider: party.Cloudflare}
	manager.closedNotice = hostClosedNotice

	screen := newPartyScreen(nil, nil, manager)
	if screen.mode != wpJoin {
		t.Fatalf("mode = %d, want the join form", screen.mode)
	}
	if screen.errorText != hostClosedNotice {
		t.Fatalf("error = %q, want the host-left reason", screen.errorText)
	}
	if got := manager.takeClosedNotice(); got != "" {
		t.Fatalf("the notice was not consumed: %q", got)
	}
}

func TestPreferredProviderFollowsTheSetting(t *testing.T) {
	both := party.Config{
		CloudflareURL: "wss://relay.example",
		SupabaseURL:   "wss://supabase.example",
		SupabaseKey:   "sb_publishable_x",
	}
	onlySupabase := party.Config{
		SupabaseURL: "wss://supabase.example",
		SupabaseKey: "sb_publishable_x",
	}

	cases := []struct {
		name  string
		cfg   party.Config
		saved string
		want  party.Provider
	}{
		{"saved supabase", both, string(party.Supabase), party.Supabase},
		{"saved cloudflare", both, string(party.Cloudflare), party.Cloudflare},
		{"auto without a choice", both, "", party.Cloudflare},
		{"unknown value", both, "bogus", party.Cloudflare},
		{"saved provider missing from the build", onlySupabase, string(party.Cloudflare), party.Supabase},
		{"single relay build", onlySupabase, "", party.Supabase},
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

	if guestControlAllowed(party.State{ControlEnabled: &off}, true) {
		t.Fatal("the guest's local setting must not override the host's refusal")
	}
	if !guestControlAllowed(party.State{ControlEnabled: &on}, false) {
		t.Fatal("the host's permission must work even when the guest's local setting is off")
	}
	if !guestControlAllowed(party.State{}, true) {
		t.Fatal("an older host without the field must fall back to the local setting")
	}
	if guestControlAllowed(party.State{}, false) {
		t.Fatal("the local setting must still be able to refuse control")
	}
}

func TestGuestControlDetectsInputWhilePaused(t *testing.T) {
	now := time.Unix(2000, 0)
	settled := now.Add(-time.Second)

	action, value, ok := guestControlFromInput(now, now.Add(-time.Second), 100, true, 130, true, settled)
	if !ok || action != party.ControlSeek || value != 130 {
		t.Fatalf("paused seek = (%q, %v, %v), want a seek to 130", action, value, ok)
	}
	action, _, ok = guestControlFromInput(now, now.Add(-time.Second), 100, true, 100, false, settled)
	if !ok || action != party.ControlPlay {
		t.Fatalf("paused play = (%q, %v), want play", action, ok)
	}
	action, _, ok = guestControlFromInput(now, now.Add(-time.Second), 100, false, 101, true, settled)
	if !ok || action != party.ControlPause {
		t.Fatalf("playing pause = (%q, %v), want pause", action, ok)
	}
}

func TestGuestControlIgnoresItsOwnSeekSettleAndSlowPlayback(t *testing.T) {
	now := time.Unix(3000, 0)
	settled := now.Add(-time.Second)

	if _, _, ok := guestControlFromInput(now, now.Add(-time.Second), 100, true, 130, true, now.Add(time.Second)); ok {
		t.Fatal("a seek that is still settling must not be sent back to the host")
	}
	if action, _, ok := guestControlFromInput(now, now.Add(-time.Second), 100, false, 100.2, false, settled); ok {
		t.Fatalf("a stalled picture produced %q, want no control", action)
	}
	if action, _, ok := guestControlFromInput(now, now.Add(-time.Second), 100, true, 100, true, settled); ok {
		t.Fatalf("an unchanged paused picture produced %q, want no control", action)
	}
}

func TestAcceptedSecretOpensTheWaitingScreen(t *testing.T) {
	screen := newPartyScreen(nil, nil, &partyManager{provider: party.Cloudflare})
	screen.mode = wpSecret
	screen.busy = true
	screen.secretEditor.SetText("4K7Q2M9XBT")

	screen.handleResult(partyResult{kind: partySecret})

	if screen.busy {
		t.Fatal("a finished check must clear the busy state")
	}
	if screen.mode != wpWaiting {
		t.Fatalf("mode = %d, want the waiting screen", screen.mode)
	}
	if got := screen.secretEditor.Text(); got != "" {
		t.Fatalf("secret editor = %q, want it cleared", got)
	}
	if screen.waitSince.IsZero() {
		t.Fatal("the waiting screen needs a fresh start time")
	}
}

func TestWrongSecretShakesAndClearsTheField(t *testing.T) {
	screen := newPartyScreen(nil, nil, &partyManager{provider: party.Cloudflare})
	screen.mode = wpSecret
	screen.busy = true
	screen.secretEditor.SetText("4K7Q2M9XBT")

	screen.handleResult(partyResult{kind: partySecret, err: party.ErrWrongSecret})

	if screen.busy {
		t.Fatal("a failed check must clear the busy state")
	}
	if screen.mode != wpSecret {
		t.Fatalf("mode = %d, want the secret form", screen.mode)
	}
	if screen.errorText != party.ErrWrongSecret.Error() {
		t.Fatalf("error = %q, want the wrong-secret reason", screen.errorText)
	}
	if !screen.joinFeedback {
		t.Fatal("a wrong secret must use the self-clearing error style")
	}
	if screen.errorAt.IsZero() {
		t.Fatal("a wrong secret must start the shake clock")
	}
	if !screen.focusSecret {
		t.Fatal("the secret field must take focus again")
	}
	if got := screen.secretEditor.Text(); got != "" {
		t.Fatalf("secret editor = %q, want it cleared for a retry", got)
	}
}

func TestSecretNoticeIsConsumedOnce(t *testing.T) {
	manager := &partyManager{provider: party.Cloudflare, secretNotice: true}

	if !manager.takeSecretNotice() {
		t.Fatal("the first call must report the pending secret prompt")
	}
	if manager.takeSecretNotice() {
		t.Fatal("the notice must not reopen the screen twice")
	}
}

func TestPartyNameIsGeneratedAndNormalized(t *testing.T) {
	name := loadPartyName(nil)
	if _, err := party.NormalizeName(name); err != nil {
		t.Fatalf("generated name %q is invalid: %v", name, err)
	}

	manager := &partyManager{provider: party.Cloudflare}
	if err := manager.SetName("  quiet-tanuki  "); err != nil {
		t.Fatalf("SetName: %v", err)
	}
	if got := manager.Name(); got != "quiet-tanuki" {
		t.Fatalf("name = %q, want the trimmed value", got)
	}
	if err := manager.SetName(""); err == nil {
		t.Fatal("an empty nickname must be refused")
	}
	if got := manager.Name(); got != "quiet-tanuki" {
		t.Fatalf("rejected name replaced the accepted one: %q", got)
	}
}
