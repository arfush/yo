package animelib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrRateLimited reports that a request hit AnimeLIB's limit while the caller
// asked not to wait for the pause to end. Callers can show the pause and let
// the user retry instead of keeping a spinner alive for up to a minute.
var ErrRateLimited = errors.New("animelib: rate limited")

type failFastRateLimitKey struct{}

// FailFastOnRateLimit marks ctx so requests return ErrRateLimited instead of
// sleeping through an active pause. Playback start uses it to stop the spinner
// and show the header countdown; background loads keep waiting as before.
func FailFastOnRateLimit(ctx context.Context) context.Context {
	return context.WithValue(ctx, failFastRateLimitKey{}, true)
}

func failFastOnRateLimit(ctx context.Context) bool {
	value, _ := ctx.Value(failFastRateLimitKey{}).(bool)
	return value
}

// AnimeLIB allows 100 requests per fixed 60-second window. The window starts
// with the first request after the previous one ended: X-Ratelimit-Remaining
// jumps back to 99 about 60s after that request. A 429 carries no reset
// timestamp (the Date/Set-Cookie headers from ddos-guard are useless here), so
// the client tracks the current window itself and waits only for its end plus
// a small margin instead of always sleeping a full minute.
var (
	rateLimitWindow = 60 * time.Second
	rateLimitMargin = 2 * time.Second
)

// maxRateLimitPause bounds a server-supplied Retry-After so a broken header
// cannot freeze the client for too long.
const maxRateLimitPause = 10 * time.Minute

var (
	rateLimitMu          sync.Mutex
	rateLimitUntil       time.Time
	rateLimitWindowStart time.Time
	// rateLimitPauseStart is when the current pause began. Successful answers
	// from requests that were already in flight when the 429 arrived must not
	// hide the notice.
	rateLimitPauseStart time.Time
	// rateLimitCleared stays false from the moment a 429 shows the header
	// notice until the first successful answer (or a UI dismissal) hides it.
	rateLimitCleared = true
	rateLimitNotify  func()
)

// SetRateLimitListener registers fn to be called when a 429 starts a pause. The
// UI uses it to repaint the header immediately. Passing nil removes it.
func SetRateLimitListener(fn func()) {
	rateLimitMu.Lock()
	rateLimitNotify = fn
	rateLimitMu.Unlock()
}

// RateLimitRemaining reports how long requests are currently paused. Zero means
// the client is not rate limited.
func RateLimitRemaining() time.Duration {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	if rateLimitUntil.IsZero() {
		return 0
	}
	remaining := time.Until(rateLimitUntil)
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// RateLimitNoticePending reports whether the header notice should be visible.
// It turns on at a 429 and off as soon as a request answers successfully or the
// UI dismisses it. The UI keeps the header notice visible while this is true.
func RateLimitNoticePending() bool {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	return !rateLimitCleared
}

// DismissRateLimitNotice lets the UI hide the header notice early, for example
// when the user cancels the load. The throttle itself is unaffected.
func DismissRateLimitNotice() {
	rateLimitMu.Lock()
	rateLimitCleared = true
	rateLimitMu.Unlock()
}

// waitForRateLimit blocks until the current pause (if any) is over. It is
// called before every request so a single 429 throttles the whole batch,
// including requests that were queued up at the same time. A context marked
// with FailFastOnRateLimit returns ErrRateLimited instead of blocking.
func waitForRateLimit(ctx context.Context) error {
	for {
		remaining := RateLimitRemaining()
		if remaining <= 0 {
			return ctx.Err()
		}
		if failFastOnRateLimit(ctx) {
			return fmt.Errorf("%w: %s left", ErrRateLimited, remaining.Round(time.Second))
		}
		timer := time.NewTimer(remaining)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

// noteRequestSent records when the current rate-limit window started. Call it
// right before a request is sent. The first request after a window has ended
// opens the next one; concurrent requests inside the same window keep the same
// start, so a 429 can be tied to the exact budget window.
func noteRequestSent(now time.Time) {
	rateLimitMu.Lock()
	if rateLimitWindowStart.IsZero() || !now.Before(rateLimitWindowStart.Add(rateLimitWindow)) {
		rateLimitWindowStart = now
	}
	rateLimitMu.Unlock()
}

// noteRateLimit starts or keeps a pause after a 429. A rejection inside a window
// only pauses until that window ends, so the client resumes as early as the
// server accepts requests again instead of always waiting a full minute.
// Concurrent requests that all trip the limit do not extend the pause. A fresh
// 429 always brings the header notice back even if a successful response had
// already hidden it.
func noteRateLimit(sentAt time.Time, retryAfter time.Duration) time.Duration {
	rateLimitMu.Lock()
	now := time.Now()
	if now.Before(rateLimitUntil) {
		remaining := rateLimitUntil.Sub(now)
		showNotice := rateLimitCleared
		rateLimitCleared = false
		notify := rateLimitNotify
		rateLimitMu.Unlock()
		if showNotice && notify != nil {
			notify()
		}
		return remaining
	}

	// Without a tracked window the safe fallback is a whole window.
	pause := rateLimitWindow + rateLimitMargin
	if !rateLimitWindowStart.IsZero() {
		windowEnd := rateLimitWindowStart.Add(rateLimitWindow)
		switch {
		case !sentAt.IsZero() && sentAt.Before(rateLimitWindowStart):
			// This rejection belongs to a request from the previous window;
			// the current one may still have budget, so only settle briefly.
			pause = rateLimitMargin
		case now.Before(windowEnd):
			// A 429 in the middle of a window only waits for the rest of it.
			pause = windowEnd.Sub(now) + rateLimitMargin
		default:
			// The tracked window is already over; re-check after a short
			// settle. If the server still rejects, the next request opens a
			// new window and the following pause covers it fully.
			pause = rateLimitMargin
		}
	}
	if retryAfter > pause {
		pause = retryAfter
	}
	if pause > maxRateLimitPause {
		pause = maxRateLimitPause
	}
	rateLimitUntil = now.Add(pause)
	rateLimitPauseStart = now
	rateLimitCleared = false
	notify := rateLimitNotify
	rateLimitMu.Unlock()

	if notify != nil {
		notify()
	}
	return pause
}

// noteRateLimitSuccess runs when AnimeLIB answers without a rate limit. A
// success from a request sent before the 429 started, or one that arrives
// while the pause is still parking other requests, must not hide the notice:
// the user would see a hung spinner with no countdown. Once the pause has
// elapsed, a successful answer hides it as usual.
func noteRateLimitSuccess(sentAt time.Time) {
	rateLimitMu.Lock()
	if !rateLimitPauseStart.IsZero() && sentAt.Before(rateLimitPauseStart) {
		rateLimitMu.Unlock()
		return
	}
	if time.Now().Before(rateLimitUntil) {
		rateLimitMu.Unlock()
		return
	}
	hideNotice := !rateLimitCleared
	rateLimitCleared = true
	rateLimitUntil = time.Time{}
	rateLimitPauseStart = time.Time{}
	notify := rateLimitNotify
	rateLimitMu.Unlock()

	if hideNotice && notify != nil {
		notify()
	}
}

// resetRateLimit is used by tests to clear the package-level cool-down.
func resetRateLimit() {
	rateLimitMu.Lock()
	rateLimitUntil = time.Time{}
	rateLimitWindowStart = time.Time{}
	rateLimitPauseStart = time.Time{}
	rateLimitCleared = true
	rateLimitMu.Unlock()
}

// parseRetryAfter accepts both forms defined by RFC 9110: delay-seconds and an
// HTTP date. An unparsable value yields zero so the built-in pause is used.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if remaining := time.Until(when); remaining > 0 {
			return remaining
		}
	}
	return 0
}
