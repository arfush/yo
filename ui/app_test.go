package ui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/arfush/yonime/internal/library"
)

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
	lib, err := library.Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
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
	lib, err := library.Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
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
