package catalog

import "errors"

// Sentinels shared across the application layers. Feature packages return these
// errors instead of raw network or database messages so screens and the
// friendlyError mapper can recognise them without importing a client.
var (
	// ErrRateLimited means the shared AnimeLIB rate-limit window is still
	// cooling down, and the request was parked instead of sent.
	ErrRateLimited = errors.New("animelib: rate limited")
	// ErrTokenMissing means the AnimeLIB token file could not be read.
	ErrTokenMissing = errors.New("token file not found")
	// ErrChapterNotFound is returned by the reader when a chapter key does not
	// exist for the selected translation.
	ErrChapterNotFound = errors.New("глава не найдена")
	// ErrUnsupportedSource is returned when a resolver gets a title whose
	// source it does not handle.
	ErrUnsupportedSource = errors.New("неизвестный источник тайтла")
)
