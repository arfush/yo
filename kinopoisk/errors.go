package kinopoisk

import "errors"

// Sentinels returned by the Kinopoisk/Alloha client. The messages are already
// user-facing Russian, so the UI shows them directly; the names make them
// comparable in tests and in the resolver.
var (
	// ErrInvalidID means the film/series id was not a number.
	ErrInvalidID = errors.New("некорректный id Кинопоиска")
	// ErrNotServed means Alloha does not carry this title.
	ErrNotServed = errors.New("Alloha не раздаёт этот тайтл")
	// ErrNoMirrors means every mirror was unreachable.
	ErrNoMirrors = errors.New("нет доступных зеркал")
	// ErrNoPlayerLink means a voiceover has no player URL.
	ErrNoPlayerLink = errors.New("у озвучки нет ссылки плеера")
	// ErrNoEpisodeList means the player answered without a series list.
	ErrNoEpisodeList = errors.New("плеер не вернул список серий")
	// ErrEmptyEpisodeList means the player returned an empty list.
	ErrEmptyEpisodeList = errors.New("пустой список серий")
	// ErrBadEpisodeList means the list format was not understood.
	ErrBadEpisodeList = errors.New("некорректный список серий")
	// ErrCorruptedEpisodeList means the list JSON failed to parse.
	ErrCorruptedEpisodeList = errors.New("список серий повреждён")
	// ErrNoEpisodes means the season has no episodes.
	ErrNoEpisodes = errors.New("в плеере нет серий")
	// ErrNoMovieFile means the film has no playable file.
	ErrNoMovieFile = errors.New("в плеере нет файла фильма")
	// ErrBrowserClosed means the shared browser exited mid-resolve.
	ErrBrowserClosed = errors.New("браузер Kinopoisk закрыт")
	// ErrBrowserUnavailable means the browser is busy or unresponsive.
	ErrBrowserUnavailable = errors.New("браузер Kinopoisk недоступен")
	// ErrNoVideoSources means the player page had no stream links.
	ErrNoVideoSources = errors.New("плеер не вернул ссылки на видео")
	// ErrNoQualityLinks means the stream response had no quality ladder.
	ErrNoQualityLinks = errors.New("плеер вернул пустой список качеств")
	// ErrNoStreamURL means the response had no video URL.
	ErrNoStreamURL = errors.New("плеер не вернул ссылку на видео")
	// ErrNoAccessKey means the key to access the stream was missing.
	ErrNoAccessKey = errors.New("плеер не вернул ключ доступа к видео")
)
