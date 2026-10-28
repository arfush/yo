package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"gioui.org/app"

	"github.com/arfush/yo/library"
)

func (a *application) startEpisodeLoop(w *app.Window) {
	a.episodeMu.Lock()
	if a.episodeStarted {
		a.episodeMu.Unlock()
		return
	}
	a.episodeStarted = true
	a.episodeMu.Unlock()

	go func() {
		defer close(a.episodeDone)
		// History saved before episode totals were cached needs one slow
		// backfill pass. It shares the checker goroutine and its context, so a
		// disabled feature or a closed window stops it before the database is
		// closed.
		a.backfillEpisodeTotals(w)
		timer := time.NewTimer(episodeCheckStartupDelay)
		defer timer.Stop()
		for {
			select {
			case <-a.episodeStop:
				return
			case <-timer.C:
			case <-a.episodeWake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			a.checkEpisodeNotifications(w)
			timer.Reset(episodeCheckInterval)
		}
	}()
}

// stopEpisodeLoop cancels an in-flight AnimeLIB pass and waits for the checker
// goroutine before close() destroys the database.
func (a *application) stopEpisodeLoop() {
	a.episodeMu.Lock()
	started := a.episodeStarted
	a.episodeStarted = false
	a.episodeMu.Unlock()
	if !started {
		return
	}
	a.episodeCancel()
	close(a.episodeStop)
	<-a.episodeDone
}

func (a *application) triggerEpisodeCheck() {
	select {
	case a.episodeWake <- struct{}{}:
	default:
	}
}

func (a *application) cancelEpisodePass() {
	a.episodePassMu.Lock()
	cancel := a.episodePassCancel
	a.episodePassMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// backfillEpisodeTotals fills the cached episode count for older history rows.
// It runs before the regular checker so a fresh install of this build draws its
// progress bars without the user opening every title first.
func (a *application) backfillEpisodeTotals(w *app.Window) {
	if a.library == nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.episodeCtx, 30*time.Minute)
	defer cancel()
	refresh := func() {
		if a.main != nil {
			a.main.requestHistoryRefresh()
		}
		if w != nil {
			w.Invalidate()
		}
	}
	if err := a.library.RefreshMissingEpisodeTotals(ctx, refresh); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "yo: подсчёт серий:", err)
	}
}

func (a *application) checkEpisodeNotifications(w *app.Window) {
	if a.library == nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.episodeCtx, 30*time.Minute)
	a.episodePassMu.Lock()
	a.episodePassCancel = cancel
	a.episodePassMu.Unlock()
	defer func() {
		cancel()
		a.episodePassMu.Lock()
		a.episodePassCancel = nil
		a.episodePassMu.Unlock()
	}()

	if err := a.library.CheckEpisodeNotifications(ctx, a.notifyEpisode); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "yo: проверка новых серий:", err)
	}
	if a.main != nil {
		// A new episode also updates the cached total shown in history.
		a.main.requestHistoryRefresh()
	}
	if w != nil {
		// Cached statuses may have changed from ongoing to completed.
		w.Invalidate()
	}
}

// notifyEpisode is the delivery callback shared by the checker and the
// temporary test button.
func (a *application) notifyEpisode(event library.EpisodeNotification) error {
	if a.tray == nil {
		return userMessage("Уведомления Windows недоступны.")
	}
	title, body := formatEpisodeNotification(event)
	return a.tray.notifyMessage(title, body)
}
