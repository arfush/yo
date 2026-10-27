package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arfush/yo/library"
	"github.com/arfush/yo/libsocial"
	applog "github.com/arfush/yo/log"
	"github.com/arfush/yo/update"
)

func TestRecoverPanicLogsAtPanicLevelAndShowsUserError(t *testing.T) {
	a := &application{shell: &shell{}, main: &mainScreen{}}
	a.recoverPanic("boom")

	if !strings.Contains(a.shell.errorText, "Что-то пошло не так") {
		t.Fatalf("user-facing panic message = %q", a.shell.errorText)
	}
	found := false
	for _, entry := range applog.Recent() {
		if entry.Level == applog.LevelPanic && entry.Message == "ui panic" && strings.Contains(entry.Detail, "boom") {
			found = true
		}
	}
	if !found {
		t.Fatal("panic was not written to the journal at the panic level")
	}
}

func TestStartFailureResultKeepsRateLimitOnMainScreen(t *testing.T) {
	rateErr := fmt.Errorf("load: %w", libsocial.ErrRateLimited)
	item := &historyTitle{id: 7}

	result := startFailureResult(rateErr, item, 7, false)
	if !result.historyFailure {
		t.Fatal("rate-limited history play must stay on the main window")
	}
	if result.historyItem != nil {
		t.Fatal("history item must be dropped so the title page does not open")
	}
	if result.err == nil || !strings.Contains(result.err.Error(), "AnimeLIB") {
		t.Fatalf("err = %v, want a user-facing rate-limit message", result.err)
	}

	result = startFailureResult(rateErr, nil, 0, false)
	if result.historyFailure || result.historyItem != nil {
		t.Fatal("title-page playback must keep its normal error path")
	}

	result = startFailureResult(errors.New("boom"), item, 7, false)
	if result.historyFailure || result.historyItem != item {
		t.Fatal("unrelated errors must keep the history item")
	}
}

func TestHandleWatchResultRefreshesTitleUserStatus(t *testing.T) {
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "yoanime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	title := library.Title{
		Slug:     "refresh-status",
		Russian:  "Статус",
		Status:   library.StatusCompleted,
		Episodes: []string{"1"},
		Voiceovers: []library.Voiceover{
			{Name: "Озвучка", Episodes: []string{"1"}},
		},
	}
	saved, err := lib.Save(title, "Озвучка", "1")
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.SetUserStatus(saved.ID, library.UserStatusWatching); err != nil {
		t.Fatal(err)
	}

	window := &titleScreen{item: historyTitle{
		id:         saved.ID,
		userStatus: library.UserStatusPlanned,
	}}
	app := &application{library: lib, active: window}
	app.handleWatchResult(watchResult{titleID: saved.ID})
	if window.item.userStatus != library.UserStatusWatching {
		t.Fatalf("user status = %d, want watching", window.item.userStatus)
	}

	if err := lib.MarkCompleted(saved.ID); err != nil {
		t.Fatal(err)
	}
	app.handleWatchResult(watchResult{titleID: saved.ID})
	if window.item.userStatus != library.UserStatusCompleted {
		t.Fatalf("user status = %d, want completed", window.item.userStatus)
	}
}

func TestAutoUpdateSafe(t *testing.T) {
	tests := []struct {
		name                         string
		playing, party, hidden, quit bool
		want                         bool
	}{
		{name: "idle in tray", hidden: true, want: true},
		{name: "visible window blocks", want: false},
		{name: "playback blocks", playing: true, hidden: true, want: false},
		{name: "watch party blocks", party: true, hidden: true, want: false},
		{name: "quitting blocks", hidden: true, quit: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := autoUpdateSafe(test.playing, test.party, test.hidden, test.quit); got != test.want {
				t.Fatalf("autoUpdateSafe() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestWaitForAutoUpdateWakesOnTray covers the regression where a pending install
// kept polling after the window had already gone to the tray.
func TestWaitForAutoUpdateWakesOnTray(t *testing.T) {
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "yoanime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	if err := lib.SetAutoUpdateEnabled(true); err != nil {
		t.Fatalf("enable auto-update: %v", err)
	}

	a := &application{library: lib, autoUpdateWake: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool, 1)
	go func() { done <- a.waitForAutoUpdate(ctx) }()

	// The waiter is blocked while the window is visible. Hiding it must release
	// the install without waiting for the poll ticker.
	time.Sleep(20 * time.Millisecond)
	a.hidden.Store(true)
	a.wakeAutoUpdate()

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("waitForAutoUpdate() = false, want true after the window went to the tray")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForAutoUpdate did not react to autoUpdateWake")
	}
}

// TestWaitForAutoUpdateStopsWhenDisabled makes sure turning auto-update off
// still aborts a waiting install on the next wake.
func TestWaitForAutoUpdateStopsWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "yoanime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	if err := lib.SetAutoUpdateEnabled(true); err != nil {
		t.Fatalf("enable auto-update: %v", err)
	}

	a := &application{library: lib, autoUpdateWake: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool, 1)
	go func() { done <- a.waitForAutoUpdate(ctx) }()

	time.Sleep(20 * time.Millisecond)
	if err := lib.SetAutoUpdateEnabled(false); err != nil {
		t.Fatalf("disable auto-update: %v", err)
	}
	a.wakeAutoUpdate()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("waitForAutoUpdate() = true, want false after auto-update was disabled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForAutoUpdate did not react to autoUpdateWake")
	}
}

// TestAutoUpdateIdleDuringPlaybackStart keeps the silent update away from the
// window between a Play click and Service.Start registering the session.
func TestAutoUpdateIdleDuringPlaybackStart(t *testing.T) {
	a := &application{}
	a.hidden.Store(true)
	if !a.autoUpdateIdle() {
		t.Fatal("autoUpdateIdle() = false, want true with the window in the tray")
	}
	a.playbackStarting.Store(true)
	if a.autoUpdateIdle() {
		t.Fatal("autoUpdateIdle() = true while playback is starting")
	}
}

// TestMaybeAutoUpdateWaitsForMain covers the first-install flow: the launch
// check can finish while authorization, Chromium or mpv still occupy the app.
// The release must wait for the first main screen instead of falling back to
// the tray-only path.
func TestMaybeAutoUpdateWaitsForMain(t *testing.T) {
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "yoanime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	if !lib.AutoUpdateEnabled() {
		t.Fatal("auto-update must be enabled by default")
	}

	a := &application{library: lib, autoUpdateCard: make(chan update.Release, 1)}
	a.autoUpdateStartup.Store(true)
	release := &update.Release{Version: "9.9", SHA256: "0123456789abcdef"}

	a.maybeAutoUpdate(release)
	if !a.autoUpdateStartup.Load() {
		t.Fatal("launch window closed before the first main screen")
	}
	select {
	case got := <-a.autoUpdateCard:
		t.Fatalf("card queued before main: %+v", got)
	default:
	}

	a.mainShown.Store(true)
	a.maybeAutoUpdate(release)
	if a.autoUpdateStartup.Load() {
		t.Fatal("launch window stayed open after main")
	}
	select {
	case got := <-a.autoUpdateCard:
		if got.Version != release.Version {
			t.Fatalf("card release = %q, want %q", got.Version, release.Version)
		}
	default:
		t.Fatal("card was not queued after main")
	}
}
