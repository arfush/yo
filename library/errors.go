package library

import (
	"errors"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/store"
)

// Sentinels returned by the library layer. Messages are technical English:
// the UI maps them to Russian sentences instead of showing them directly.
var (
	// ErrDatabaseUnavailable means the store was not opened.
	ErrDatabaseUnavailable = errors.New("database is not available")
	// ErrDatabaseCorrupt re-exports the store sentinel so UI code does not
	// import the data layer directly.
	ErrDatabaseCorrupt = store.ErrCorruptDatabase
	// ErrUnknownUserStatus is returned by SetUserStatus for an invalid value.
	ErrUnknownUserStatus = errors.New("unknown title status")
	// ErrTokenMissing means the AnimeLIB token file was not found.
	ErrTokenMissing = errors.New("AnimeLIB token not found; restart yoanime to sign in")
	// ErrTokenExpired means the saved token is past its expiry margin.
	ErrTokenExpired = errors.New("AnimeLIB token expired; restart yoanime to sign in")
	// ErrNoVoiceovers means a Kinopoisk plan request found no translations.
	ErrNoVoiceovers = errors.New("title has no voiceovers")
	// ErrNotServed re-exports the Kinopoisk sentinel wording.
	ErrNotServed = errors.New("the provider does not serve this title")
	// ErrNoKinopoiskID means the cached title has no Kinopoisk id.
	ErrNoKinopoiskID = errors.New("title has no Kinopoisk id")

	// ErrChapterNotFound and ErrUnsupportedSource are aliases of the catalog
	// sentinels used by the reader screen.
	ErrChapterNotFound   = catalog.ErrChapterNotFound
	ErrUnsupportedSource = catalog.ErrUnsupportedSource
	// ErrBadPageURL means a page link was not an http(s) URL.
	ErrBadPageURL = errors.New("invalid page URL")

	// Cover errors.
	ErrCoversClosed = errors.New("cover store is not open")
	ErrNoCoverURL   = errors.New("title has no cover URL")
	ErrNoTitleID    = errors.New("title has no local id")
	ErrBadCoverFile = errors.New("cover: empty or oversized file")
)
