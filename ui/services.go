package ui

import (
	"time"

	"github.com/arfush/yo/browser"
	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/mpv"
	"github.com/arfush/yo/playback"
	"github.com/arfush/yo/update"
)

// RateLimitState is the part of AnimeLIB's cooldown the header needs. The
// composition root adapts the API client to it, so screens do not import the
// network client just to draw a countdown.
type RateLimitState interface {
	Remaining() time.Duration
	NoticePending() bool
	Dismiss()
	SetListener(func())
}

// Services is the object graph the composition root hands to the screens. The
// UI never opens the database, browser profile or update store itself; it only
// wires the already constructed services into the window controllers.
type Services struct {
	Library           *library.Library
	Browser           *browser.Manager
	MPV               *mpv.Manager
	Playback          *playback.Service
	UpdateStore       *update.Store
	RateLimit         RateLimitState
	History           []catalog.Title
	TokenPath         string
	BrowserProfileDir string
	CurrentSHA        string
}
