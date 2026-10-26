package watchparty

import "time"

// Provider identifies the relay backend.
type Provider string

const (
	Cloudflare Provider = "cloudflare"
	Supabase   Provider = "supabase"
)

// Providers lists the backends in toggle order.
func Providers() []Provider { return []Provider{Cloudflare, Supabase} }

// Valid reports whether p is a known backend.
func (p Provider) Valid() bool {
	return p == Cloudflare || p == Supabase
}

// Label is the short name shown in the settings.
func (p Provider) Label() string {
	switch p {
	case Cloudflare:
		return "Cloudflare"
	case Supabase:
		return "Supabase"
	}
	return string(p)
}

// Role tells the relay who owns the playback state.
type Role string

const (
	Host  Role = "host"
	Guest Role = "guest"
)

// State is the host's authoritative playback state. It is an anchor, not a
// stream of positions: guests extrapolate from Position and Anchor with their
// own monotonic clock, so a lost packet never desynchronises the room.
type State struct {
	TitleSlug    string  `json:"slug"`
	TitleName    string  `json:"title"`
	Voice        string  `json:"voice"`
	Episode      string  `json:"episode"`
	EpisodeIndex int     `json:"index"`
	Playing      bool    `json:"playing"`
	Position     float64 `json:"position"`
	// Anchor is the host wall clock in unix milliseconds at which Position
	// was sampled.
	Anchor int64 `json:"anchor"`
	// ControlEnabled carries the host's guest-control decision. It is a
	// pointer so an anchor from an older host, which has no such field, can be
	// told apart from an explicit "guests may not control playback".
	ControlEnabled *bool `json:"control,omitempty"`
}

// Started reports whether the host has opened a title.
func (s State) Started() bool { return s.TitleSlug != "" }

// GuestsControl returns the host's guest-control setting and whether the
// anchor carried one at all. The second value is false for rooms created by
// older builds.
func (s State) GuestsControl() (bool, bool) {
	if s.ControlEnabled == nil {
		return false, false
	}
	return *s.ControlEnabled, true
}

// SamePlayback reports whether two states point at the same media, so a guest
// does not relaunch mpv for a position-only change.
func (s State) SamePlayback(other State) bool {
	return s.TitleSlug == other.TitleSlug &&
		s.Voice == other.Voice &&
		s.Episode == other.Episode
}

// PositionAt returns the position the host expects at the given host-clock
// instant, in seconds.
func (s State) PositionAt(hostNow time.Time) float64 {
	position := s.Position
	if !s.Playing || s.Anchor <= 0 {
		return position
	}
	elapsed := hostNow.UnixMilli() - s.Anchor
	if elapsed <= 0 {
		return position
	}
	return position + float64(elapsed)/1000
}

// Control actions a guest may send to the host when shared control is on.
const (
	ControlPause = "pause"
	ControlPlay  = "play"
	ControlSeek  = "seek"
)

// Control is one playback command from a guest. The host applies it and then
// publishes a fresh anchor, so the other guests follow the same command through
// the regular state channel.
type Control struct {
	Action   string  `json:"action"`
	Position float64 `json:"position,omitempty"`
}

// Valid reports whether the host can apply the action.
func (c Control) Valid() bool {
	switch c.Action {
	case ControlPause, ControlPlay:
		return true
	case ControlSeek:
		return c.Position >= 0
	}
	return false
}

// UpdateKind classifies a room notification.
type UpdateKind int

const (
	// UpdateState is a new authoritative anchor from the host.
	UpdateState UpdateKind = iota
	// UpdateHostSeen fires the first time the host answers.
	UpdateHostSeen
	// UpdateHostLeft fires when the reconnect grace window expires without the
	// host coming back.
	UpdateHostLeft
	// UpdateHostClosed fires when the host explicitly closes the room, which
	// ends it immediately.
	UpdateHostClosed
	// UpdateControl is a guest playback command the host should apply.
	UpdateControl
	// UpdateCard reports a new room header: the host locked, unlocked or
	// rotated the secret, or a guest found a secret that opens the room.
	UpdateCard
	// UpdateError reports a relay problem that the UI may want to show.
	UpdateError
	// UpdateClosed reports that the room was closed locally.
	UpdateClosed
)

// Update is delivered to the UI through Session.Updates.
type Update struct {
	Kind    UpdateKind
	State   State
	Control Control
	Err     error
}
