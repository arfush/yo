package playback

import "errors"

// Sentinels returned by the playback service. The UI maps them to short
// Russian sentences; messages here stay technical English.
var (
	// ErrNotRunning is returned by the control methods when mpv is gone.
	ErrNotRunning = errors.New("mpv is not running")
	// ErrEpisodeNotLoaded means the episode is not part of the current
	// playlist, so the guest has to restart playback instead of jumping.
	ErrEpisodeNotLoaded = errors.New("episode is not loaded")
	// ErrUnavailable means the service was not wired with a library.
	ErrUnavailable = errors.New("playback is not available")
	// ErrNoEpisodes means the plan has nothing to play.
	ErrNoEpisodes = errors.New("no episodes to play")
	// ErrInvalidEpisode means the selected episode index is out of range.
	ErrInvalidEpisode = errors.New("invalid episode")
	// ErrClosing means the application is shutting down.
	ErrClosing = errors.New("application is closing")
	// ErrAlreadyPlaying means another session is starting or running.
	ErrAlreadyPlaying = errors.New("video is already playing")
	// ErrVideoOpen means mpv refused the resolved stream.
	ErrVideoOpen = errors.New("cannot open the video stream")
	// ErrKodikUnavailable means the Kodik fallback did not answer.
	ErrKodikUnavailable = errors.New("Kodik stream is unavailable")
	// ErrPlaybackFailed is the generic refusal to start a stream.
	ErrPlaybackFailed = errors.New("cannot play the video")
)
