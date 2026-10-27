package kinopoisk

import "errors"

// Sentinels returned by the Kinopoisk/Alloha client. Messages are technical
// English; the UI maps them to Russian sentences without naming the source.
var (
	// ErrInvalidID means the film/series id was not a number.
	ErrInvalidID = errors.New("invalid Kinopoisk id")
	// ErrNotServed means Alloha does not carry this title.
	ErrNotServed = errors.New("the provider does not serve this title")
	// ErrNoMirrors means every mirror was unreachable.
	ErrNoMirrors = errors.New("no available mirrors")
	// ErrNoPlayerLink means a voiceover has no player URL.
	ErrNoPlayerLink = errors.New("voiceover has no player URL")
	// ErrNoEpisodeList means the player answered without a series list.
	ErrNoEpisodeList = errors.New("player returned no episode list")
	// ErrEmptyEpisodeList means the player returned an empty list.
	ErrEmptyEpisodeList = errors.New("empty episode list")
	// ErrBadEpisodeList means the list format was not understood.
	ErrBadEpisodeList = errors.New("invalid episode list")
	// ErrCorruptedEpisodeList means the list JSON failed to parse.
	ErrCorruptedEpisodeList = errors.New("episode list is corrupted")
	// ErrNoEpisodes means the season has no episodes.
	ErrNoEpisodes = errors.New("player has no episodes")
	// ErrNoMovieFile means the film has no playable file.
	ErrNoMovieFile = errors.New("player has no movie file")
	// ErrBrowserClosed means the shared browser exited mid-resolve.
	ErrBrowserClosed = errors.New("Kinopoisk browser is closed")
	// ErrBrowserUnavailable means the browser is busy or unresponsive.
	ErrBrowserUnavailable = errors.New("Kinopoisk browser is unavailable")
	// ErrNoVideoSources means the player page had no stream links.
	ErrNoVideoSources = errors.New("player returned no video links")
	// ErrNoQualityLinks means the stream response had no quality ladder.
	ErrNoQualityLinks = errors.New("player returned an empty quality list")
	// ErrNoStreamURL means the response had no video URL.
	ErrNoStreamURL = errors.New("player returned no video URL")
	// ErrNoAccessKey means the key to access the stream was missing.
	ErrNoAccessKey = errors.New("player returned no stream access key")
)
