package app

import (
	"time"

	"github.com/arfush/yo/animelib"
)

// animeLibRateLimit exposes only the cooldown state the header renders. The
// limiter itself stays inside animelib, which owns the request window.
type animeLibRateLimit struct{}

func (animeLibRateLimit) Remaining() time.Duration {
	return animelib.RateLimitRemaining()
}

func (animeLibRateLimit) NoticePending() bool {
	return animelib.RateLimitNoticePending()
}

func (animeLibRateLimit) Dismiss() {
	animelib.DismissRateLimitNotice()
}

func (animeLibRateLimit) SetListener(fn func()) {
	animelib.SetRateLimitListener(fn)
}
