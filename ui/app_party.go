package ui

import (
	"context"
	"time"

	"github.com/arfush/yo/party"
)

// openParty shows the room card. When a room is still open the screen
// reopens on the created (host) or waiting (guest) form with the same key.
func (a *application) openParty() {
	if a.window == nil {
		return
	}
	// Watch party belongs to anime playback only. The footer hides the button
	// for the reading sections; this guard keeps keyboard/debug entry points
	// from opening a room there as well.
	if a.main != nil && a.main.section != sectionAnime {
		return
	}
	if a.party == nil {
		a.party = newPartyManager(a)
	}
	screen := newPartyScreen(a.shell.typo, a.window, a.party)
	screen.hidden = a.windowHidden
	screen.onError = a.reportError
	screen.onBack = func() {
		if a.window == nil {
			return
		}
		a.showWindow(a.window, a.main)
		a.window.Invalidate()
	}
	a.showWindow(a.window, screen)
}

// startPartyPlayback opens the title the host is watching and starts mpv close
// to the extrapolated host position. The guest drift loop then corrects the
// remaining difference.
func (a *application) startPartyPlayback(state party.State) error {
	if a.library == nil || a.playback == nil {
		return userMessage("Воспроизведение сейчас недоступно.")
	}
	// The host may jump to another episode or voice while the guest is still
	// playing. mpv can only run one session, so the old one has to go first.
	// A duplicate party start can arrive while the first session is already
	// opening the same episode: keep it instead of stopping the picture.
	if current := a.playback.Current(); current != nil {
		number, _ := current.Episode()
		if number == state.Episode && current.Matches(state.TitleSlug, state.Voice) {
			return nil
		}
	}
	if a.playback.Active() {
		if current := a.playback.Current(); current != nil {
			current.Stop()
		}
		deadline := time.Now().Add(3 * time.Second)
		for a.playback.Active() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if a.playback.Active() {
			return userMessage("Плеер не успел завершить прошлую серию. Попробуйте ещё раз.")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), playbackStartTimeout)
	defer cancel()
	title, err := a.library.Load(ctx, state.TitleSlug)
	if err != nil {
		return errPlaybackStart.wrap(err)
	}
	quality := a.library.PreferredQuality()
	startAt := 0.0
	if a.party != nil {
		if position, ok := a.party.ExpectedPosition(); ok {
			startAt = position
		}
	}
	// startPlayback launches the resolve and mpv startup in the background.
	// A party guest must keep its "starting" guard until the actual session
	// exists: a heartbeat state arrives every few seconds, and clearing the
	// guard on return made the guest launch mpv again while the first resolve
	// was still running. Two concurrent starts fought for the single playback
	// slot and killed the episode that had just opened.
	started := make(chan error, 1)
	if err := a.startPlaybackNotify(title, state.Voice, state.Episode, quality, nil, true, startAt, func(err error) {
		started <- err
	}); err != nil {
		return err
	}
	select {
	case err := <-started:
		return err
	case <-time.After(playbackStartTimeout + 30*time.Second):
		return userMessage("Плеер не успел начать воспроизведение. Попробуйте ещё раз.")
	}
}

// processPartyState consumes a pending room invitation. The notice may arrive
// while the guest was on the main screen. The manager raises the flag from its
// consume loop; consuming it here keeps screen navigation on the UI goroutine.
func (a *application) processPartyState() {
	if a.party == nil {
		return
	}
	if a.active == a.main {
		if a.party.takeSecretNotice() {
			a.openParty()
		}
		return
	}
	// The party screen reads NeedsSecret directly. Drop the notice so it does
	// not reopen the card after the user leaves a room that was already
	// handled.
	if _, ok := a.active.(*partyScreen); ok {
		a.party.takeSecretNotice()
	}
}

// leaveParty deletes the local room and returns to the main screen. The footer
// closes the player leaves the room this way, so the party control stops
// glowing instead of keeping a room nobody follows any more.
func (a *application) leaveParty() {
	if a.party != nil {
		a.party.Delete()
	}
	if a.window == nil {
		return
	}
	a.showWindow(a.window, a.main)
	a.window.Invalidate()
}
