package playback

import (
	"errors"
	"image"
	"math"
)

// ErrNotRunning is returned by the control methods when mpv is gone.
var ErrNotRunning = errors.New("mpv не запущен")

// ErrEpisodeNotLoaded is returned by JumpTo when the episode is not part of
// the already-resolved playlist, so the caller has to start a fresh session.
var ErrEpisodeNotLoaded = errors.New("серия ещё не загружена")

// Position returns the last playback position reported by mpv, in seconds.
func (s *Session) Position() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPos
}

// Duration returns the media duration of the current file, in seconds.
func (s *Session) Duration() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.duration
}

// Paused reports the last known pause state.
func (s *Session) Paused() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// Episode returns the episode number that is currently loaded and its index in
// the playback plan.
func (s *Session) Episode() (string, int) {
	if s == nil {
		return "", -1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentNumber, s.currentEp
}

// Slug returns the slug of the running plan. The title page compares it with
// its own title so a live playback update cannot leak into another screen.
func (s *Session) Slug() string {
	if s == nil || s.plan == nil {
		return ""
	}
	return s.plan.Slug
}

// MpvWindowRect returns the current rectangle of the mpv window in physical
// pixels. Playback persistence uses it to remember where the user parked the
// player, independently of the application window.
func (s *Session) MpvWindowRect() (image.Rectangle, bool) {
	if s == nil {
		return image.Rectangle{}, false
	}
	s.mu.Lock()
	mpv := s.mpv
	s.mu.Unlock()
	if mpv == nil {
		return image.Rectangle{}, false
	}
	return mpv.windowRect()
}

// LivePosition asks mpv for the current position instead of returning the last
// observed property event. A published watch-party anchor must not be a second
// behind the picture.
func (s *Session) LivePosition() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	mpv := s.mpv
	fallback := s.lastPos
	s.mu.Unlock()
	if mpv == nil {
		return fallback
	}
	value, err := mpv.call("get_property", "time-pos")
	if err != nil {
		return fallback
	}
	if number, ok := value.(float64); ok && number >= 0 {
		return number
	}
	return fallback
}

// SeekTo jumps to an absolute position inside the current file. Watch-party
// guests use it for the large corrections that speed changes cannot fix.
func (s *Session) SeekTo(seconds float64) error {
	if s == nil || seconds < 0 {
		return nil
	}
	s.mu.Lock()
	mpv := s.mpv
	// Assume the jump succeeded so the correction loop does not fire again
	// before mpv reports the new time-pos.
	s.lastPos = seconds
	s.mu.Unlock()
	if mpv == nil {
		return ErrNotRunning
	}
	_, err := mpv.call("seek", seconds, "absolute+exact")
	return err
}

// SetSpeed sets the playback rate. mpv keeps the audio pitch, so a gentle rate
// change is inaudible and removes small drift without a visible jump.
func (s *Session) SetSpeed(rate float64) error {
	if s == nil {
		return nil
	}
	rate = math.Max(0.85, math.Min(1.15, rate))
	s.mu.Lock()
	mpv := s.mpv
	s.mu.Unlock()
	if mpv == nil {
		return ErrNotRunning
	}
	_, err := mpv.call("set_property", "speed", rate)
	return err
}

// SetPaused pauses or resumes playback.
func (s *Session) SetPaused(paused bool) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	mpv := s.mpv
	s.paused = paused
	s.mu.Unlock()
	if mpv == nil {
		return ErrNotRunning
	}
	_, err := mpv.call("set_property", "pause", paused)
	return err
}

// Matches reports whether the running session plays the given title and voice.
// The watch-party guest uses it to decide whether a host episode can be
// followed inside the current player instead of replacing the session.
func (s *Session) Matches(slug, voice string) bool {
	if s == nil || s.plan == nil {
		return false
	}
	if s.plan.Slug != slug {
		return false
	}
	return voice == "" || s.plan.Voice == voice
}

// JumpTo moves the running player to an episode that already sits in its
// resolved playlist. Resolving a stream can take seconds (Kodik, and every
// Kinopoisk episode), so when the host rolls to the next episode of the same
// title the guest should jump inside its own preloaded window instead of
// killing mpv and re-resolving. ErrEpisodeNotLoaded tells the caller to fall
// back to a fresh session for a jump outside the window.
func (s *Session) JumpTo(episode string) error {
	if s == nil {
		return ErrNotRunning
	}
	s.mu.Lock()
	mpv := s.mpv
	index := resolveEntryIndex(s.entries, episode)
	var entry *playlistEntry
	if index >= 0 {
		entry = s.entries[index]
	}
	s.mu.Unlock()
	if mpv == nil {
		return ErrNotRunning
	}
	if index < 0 || entry == nil {
		return ErrEpisodeNotLoaded
	}
	// entries[i] always sits at playlist position i: resolveAndAdd inserts in
	// episode order and mirrors every insert into mpv. The entry carries its
	// own per-file headers from the loadfile that added it, so selecting it
	// brings the Kinopoisk Referer of that episode along.
	return mpv.jump(index)
}

// resolveEntryIndex finds the playlist index of a resolved episode, or -1.
func resolveEntryIndex(entries []*playlistEntry, episode string) int {
	for index, entry := range entries {
		if entry != nil && entry.Episode.Number == episode {
			return index
		}
	}
	return -1
}
