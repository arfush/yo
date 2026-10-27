package app

import (
	"time"

	"github.com/arfush/yo/libsocial"
)

// libsocialRateLimit exposes only the cooldown state the header renders. The
// limiter itself stays inside libsocial, which owns the request window.
type libsocialRateLimit struct{}

func (libsocialRateLimit) Remaining() time.Duration {
	return libsocial.RateLimitRemaining()
}

func (libsocialRateLimit) NoticePending() bool {
	return libsocial.RateLimitNoticePending()
}

func (libsocialRateLimit) Dismiss() {
	libsocial.DismissRateLimitNotice()
}

func (libsocialRateLimit) SetListener(fn func()) {
	libsocial.SetRateLimitListener(fn)
}
