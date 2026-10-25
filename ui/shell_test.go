package ui

import (
	"testing"
	"time"
)

func TestShellErrorExpiresAfterFiveSeconds(t *testing.T) {
	s := &shell{}
	start := time.Unix(100, 0)
	s.setError("Не удалось загрузить тайтл", start)
	wantUntil := start.Add(5 * time.Second)
	if !s.errorUntil.Equal(wantUntil) || !s.logAlertUntil.Equal(wantUntil) {
		t.Fatalf("deadlines = %v / %v, want %v", s.errorUntil, s.logAlertUntil, wantUntil)
	}
	if s.expireError(wantUntil.Add(-time.Millisecond)) {
		t.Fatal("error expired too early")
	}
	if s.errorText == "" {
		t.Fatal("error disappeared before five seconds")
	}
	if !s.expireError(wantUntil) {
		t.Fatal("error did not expire at five seconds")
	}
	if s.errorText != "" || !s.errorUntil.IsZero() {
		t.Fatalf("error state survived expiry: %q %v", s.errorText, s.errorUntil)
	}
}
