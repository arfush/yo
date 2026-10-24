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

// Label is the short name shown in the provider chip.
func (p Provider) Label() string {
	switch p {
	case Cloudflare:
		return "Cloudflare"
	case Supabase:
		return "Supabase"
	}
	return string(p)
}

// Next returns the backend reached by clicking the provider chip.
func (p Provider) Next() Provider {
	if p == Cloudflare {
		return Supabase
	}
	return Cloudflare
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
}

// Started reports whether the host has opened a title.
func (s State) Started() bool { return s.TitleSlug != "" }

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

// UpdateKind classifies a room notification.
type UpdateKind int

const (
	// UpdateState is a new authoritative anchor from the host.
	UpdateState UpdateKind = iota
	// UpdateHostSeen fires the first time the host answers.
	UpdateHostSeen
	// UpdateHostLeft fires when the relay reports the host is gone.
	UpdateHostLeft
	// UpdateError reports a relay problem that the UI may want to show.
	UpdateError
	// UpdateClosed reports that the room was closed locally.
	UpdateClosed
)

// Update is delivered to the UI through Session.Updates.
type Update struct {
	Kind  UpdateKind
	State State
	Err   error
}
