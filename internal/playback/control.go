package playback

import (
	"errors"
	"math"
)

// ErrNotRunning is returned by the control methods when mpv is gone.
var ErrNotRunning = errors.New("mpv не запущен")

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

// SetInputLocked blocks or restores timeline control in mpv. It is used for a
// watch-party guest whose host does not allow shared control: the drift loop
// alone only undoes a seek after the fact, while this makes pausing and seeking
// impossible in the first place. Volume, speed, subtitles and fullscreen stay
// available.
func (s *Session) SetInputLocked(locked bool) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	mpv := s.mpv
	changed := s.inputLocked != locked
	s.inputLocked = locked
	s.mu.Unlock()
	if mpv == nil {
		return ErrNotRunning
	}
	if !changed {
		return nil
	}
	command := "disable-section"
	if locked {
		command = "enable-section"
	}
	var firstErr error
	if _, err := mpv.call(command, guestInputSection); err != nil {
		firstErr = err
	}
	// The osc property loads and unloads the built-in OSC script. Hiding it
	// with a visibility message would leave its seek bar and pause button
	// alive, and they bypass the input section above.
	if _, err := mpv.call("set_property", "osc", !locked); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
