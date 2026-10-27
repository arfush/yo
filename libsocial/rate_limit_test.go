package libsocial

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	applog "github.com/arfush/yo/log"
)

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("7"); got != 7*time.Second {
		t.Fatalf("parseRetryAfter(\"7\") = %v, want 7s", got)
	}
	if got := parseRetryAfter(""); got != 0 {
		t.Fatalf("parseRetryAfter(\"\") = %v, want 0", got)
	}
	if got := parseRetryAfter("garbage"); got != 0 {
		t.Fatalf("parseRetryAfter(garbage) = %v, want 0", got)
	}
	if got := parseRetryAfter("-5"); got != 0 {
		t.Fatalf("parseRetryAfter(-5) = %v, want 0", got)
	}

	date := time.Now().Add(4 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(date); got < 3*time.Second || got > 4*time.Second {
		t.Fatalf("parseRetryAfter(date) = %v, want about 4s", got)
	}
}

func TestNoteRequestSentStartsNewWindow(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	originalWindow := rateLimitWindow
	rateLimitWindow = 100 * time.Millisecond
	defer func() { rateLimitWindow = originalWindow }()

	first := time.Now()
	noteRequestSent(first)
	if got := windowStart(); !got.Equal(first) {
		t.Fatalf("window start = %v, want %v", got, first)
	}

	// A request inside the same window keeps its start.
	noteRequestSent(first.Add(50 * time.Millisecond))
	if got := windowStart(); !got.Equal(first) {
		t.Fatalf("window start after an in-window request = %v, want %v", got, first)
	}

	// The first request after the window has ended opens the next window.
	next := first.Add(150 * time.Millisecond)
	noteRequestSent(next)
	if got := windowStart(); !got.Equal(next) {
		t.Fatalf("window start after the window ended = %v, want %v", got, next)
	}
}

func TestRateLimitWaitsOnlyForTheRestOfTheWindow(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	originalWindow, originalMargin := rateLimitWindow, rateLimitMargin
	rateLimitWindow = 60 * time.Second
	rateLimitMargin = 2 * time.Second
	defer func() {
		rateLimitWindow, rateLimitMargin = originalWindow, originalMargin
	}()

	// The window started 50s ago, so a 429 only needs the remaining ~10s.
	rateLimitMu.Lock()
	rateLimitWindowStart = time.Now().Add(-50 * time.Second)
	rateLimitMu.Unlock()

	if got := noteRateLimit(time.Time{}, 0); got < 11*time.Second || got > 13*time.Second {
		t.Fatalf("pause = %v, want about 12s (the rest of the window plus margin)", got)
	}
}

func TestRateLimitFallsBackToFullWindowWithoutWindowStart(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	originalWindow, originalMargin := rateLimitWindow, rateLimitMargin
	rateLimitWindow = 60 * time.Second
	rateLimitMargin = 2 * time.Second
	defer func() {
		rateLimitWindow, rateLimitMargin = originalWindow, originalMargin
	}()

	rateLimitMu.Lock()
	rateLimitWindowStart = time.Time{}
	rateLimitMu.Unlock()

	if got, want := noteRateLimit(time.Time{}, 0), rateLimitWindow+rateLimitMargin; got != want {
		t.Fatalf("pause without a window start = %v, want %v", got, want)
	}
}

func TestRateLimitAfterWindowEndUsesShortSettle(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	originalMargin := rateLimitMargin
	rateLimitMargin = 2 * time.Second
	defer func() { rateLimitMargin = originalMargin }()

	rateLimitMu.Lock()
	rateLimitWindowStart = time.Now().Add(-2 * rateLimitWindow)
	rateLimitMu.Unlock()

	if got := noteRateLimit(time.Time{}, 0); got != rateLimitMargin {
		t.Fatalf("pause after the window ended = %v, want %v", got, rateLimitMargin)
	}
}

func TestRateLimitNoticeLifecycle(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	if RateLimitNoticePending() {
		t.Fatal("notice pending before any 429, want clear")
	}

	noteRateLimit(time.Time{}, 0)
	if !RateLimitNoticePending() {
		t.Fatal("notice not pending during pause, want pending")
	}

	// The pause expires but nothing got through yet: the notice stays.
	rateLimitMu.Lock()
	rateLimitUntil = time.Now().Add(-time.Millisecond)
	rateLimitMu.Unlock()
	if !RateLimitNoticePending() {
		t.Fatal("notice cleared before a request got through, want pending")
	}

	noteRateLimitSuccess(time.Now())
	if RateLimitNoticePending() {
		t.Fatal("notice pending after a successful request, want clear")
	}

	// A UI-side dismissal also clears the notice without touching the throttle.
	noteRateLimit(time.Time{}, 0)
	DismissRateLimitNotice()
	if RateLimitNoticePending() {
		t.Fatal("notice pending after dismissal, want clear")
	}
}

func TestSuccessHidesNoticeOnlyAfterThePause(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	noteRateLimit(time.Time{}, 0)
	if !RateLimitNoticePending() {
		t.Fatal("notice not pending after a 429, want pending")
	}

	// A request that was already in flight when the 429 arrived may still
	// succeed. It must not hide the countdown: queued requests are still
	// parked, and the user needs to see why nothing is happening.
	noteRateLimitSuccess(time.Now().Add(-time.Second))
	if !RateLimitNoticePending() {
		t.Fatal("notice hidden by a success from before the pause, want pending")
	}

	// A success during the pause is not proof the window rolled over either.
	noteRateLimitSuccess(time.Now())
	if !RateLimitNoticePending() {
		t.Fatal("notice hidden while the pause is active, want pending")
	}

	// Once the pause is over, a successful answer hides the notice.
	rateLimitMu.Lock()
	rateLimitUntil = time.Now().Add(-time.Millisecond)
	rateLimitMu.Unlock()
	noteRateLimitSuccess(time.Now())
	if RateLimitNoticePending() {
		t.Fatal("notice pending after the pause ended, want clear")
	}
	if RateLimitRemaining() != 0 {
		t.Fatalf("RateLimitRemaining() = %v after the pause, want 0", RateLimitRemaining())
	}
}

func TestRateLimitHonorsRetryAfterAndCapsIt(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	rateLimitMu.Lock()
	rateLimitWindowStart = time.Now()
	rateLimitMu.Unlock()

	// A fresh window means the window-end pause is longer than the header.
	if got, want := noteRateLimit(time.Time{}, 9*time.Second), rateLimitWindow+rateLimitMargin; got != want {
		t.Fatalf("pause with Retry-After = %v, want %v", got, want)
	}
	rateLimitMu.Lock()
	rateLimitUntil = time.Now().Add(-time.Millisecond)
	rateLimitMu.Unlock()

	if got := noteRateLimit(time.Time{}, 10*time.Minute); got != maxRateLimitPause {
		t.Fatalf("pause with huge Retry-After = %v, want %v", got, maxRateLimitPause)
	}
}

func TestWaitForRateLimitStopsOnContext(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	noteRateLimit(time.Time{}, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := waitForRateLimit(ctx); err == nil {
		t.Fatal("waitForRateLimit returned nil, want context error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waitForRateLimit took %v, want it to stop with the context", elapsed)
	}
}

func TestWaitForRateLimitFailsFastWhenRequested(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	noteRateLimit(time.Time{}, time.Hour)
	ctx := FailFastOnRateLimit(context.Background())

	start := time.Now()
	err := waitForRateLimit(ctx)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("waitForRateLimit = %v, want ErrRateLimited", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waitForRateLimit took %v, want an immediate error", elapsed)
	}

	// Without an active pause the marker changes nothing.
	resetRateLimit()
	if err := waitForRateLimit(ctx); err != nil {
		t.Fatalf("waitForRateLimit without a pause = %v, want nil", err)
	}
}

// TestGetFailsFastOnRateLimitWhenRequested keeps playback start from hiding in
// a retry loop: the spinner has to stop and the header shows the countdown.
func TestGetFailsFastOnRateLimitWhenRequested(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	ctx := FailFastOnRateLimit(context.Background())
	start := time.Now()
	_, err := client.get(ctx, server.URL, 1)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("get = %v, want ErrRateLimited", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("get took %v, want an immediate error", elapsed)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("server saw %d requests, want 1", got)
	}
}

// TestResolveKeepsRateLimitCause covers the Play-from-history path: the title
// loader tries several variants, and a fail-fast 429 must not be reported as
// "title not found", which used to open an empty title page instead of showing
// the header countdown.
func TestResolveKeepsRateLimitCause(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	ctx := FailFastOnRateLimit(context.Background())
	_, err := client.resolve(ctx, "some-title")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("resolve = %v, want ErrRateLimited", err)
	}
	if strings.Contains(err.Error(), "не удалось найти") {
		t.Fatalf("resolve = %v, must not mask the pause as a missing title", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("server saw %d requests, want 1", got)
	}
}

func TestHTTP429IsRetriedUntilSuccess(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	// Keep the test quick while exercising the same code path.
	originalWindow, originalMargin := rateLimitWindow, rateLimitMargin
	rateLimitWindow = 250 * time.Millisecond
	rateLimitMargin = 100 * time.Millisecond
	defer func() {
		rateLimitWindow, rateLimitMargin = originalWindow, originalMargin
	}()

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	// A 429 no longer fails the request: it pauses and retries until the
	// server answers without a rate limit.
	start := time.Now()
	body, err := client.get(context.Background(), server.URL, 1)
	if err != nil {
		t.Fatalf("get returned %v, want it to retry until success", err)
	}
	if string(body) != `{"data":{}}` {
		t.Fatalf("body = %q, want a successful response", body)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("server saw %d requests, want 2", got)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("retry waited %v, want it to honor Retry-After", elapsed)
	}
	if RateLimitNoticePending() {
		t.Fatal("notice is still pending after a successful request")
	}
	if remaining := RateLimitRemaining(); remaining != 0 {
		t.Fatalf("RateLimitRemaining() = %v after success, want 0", remaining)
	}
	var warning *applog.Entry
	for _, entry := range applog.Recent() {
		if entry.Message == "libsocial rate limit reached" {
			entry := entry
			warning = &entry
		}
	}
	if warning == nil {
		t.Fatal("rate-limit warning was not logged")
	}
	const marker = "retry_after="
	index := strings.Index(warning.Detail, marker)
	if index < 0 || strings.TrimSpace(warning.Detail[index+len(marker):]) == "" {
		t.Fatalf("rate-limit warning has an empty retry_after: %q", warning.Detail)
	}
}

func TestRepeated429KeepsRetryingUntilAllRequestsSucceed(t *testing.T) {
	resetRateLimit()
	defer resetRateLimit()

	originalWindow, originalMargin := rateLimitWindow, rateLimitMargin
	rateLimitWindow = 20 * time.Millisecond
	rateLimitMargin = 10 * time.Millisecond
	defer func() {
		rateLimitWindow, rateLimitMargin = originalWindow, originalMargin
	}()

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) <= 3 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	if _, err := client.get(context.Background(), server.URL, 1); err != nil {
		t.Fatalf("get returned %v, want success after retries", err)
	}
	if got := atomic.LoadInt32(&requests); got != 4 {
		t.Fatalf("server saw %d requests, want 4", got)
	}
	if RateLimitNoticePending() {
		t.Fatal("notice is still pending after a successful request")
	}
}

func windowStart() time.Time {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	return rateLimitWindowStart
}
