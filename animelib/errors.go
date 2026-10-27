package animelib

import (
	"errors"

	"github.com/arfush/yo/catalog"
)

// Sentinels returned by the AnimeLIB and LibSocial API client. Messages are
// technical English: the UI maps the exported errors to Russian sentences.
var (
	// ErrNoToken means the client was created without a token file.
	ErrNoToken = errors.New("AnimeLIB token not found (animelib_token.txt in the yoanime data directory)")
	// ErrEmptySlug means a reading URL carried no title slug.
	ErrEmptySlug = errors.New("reading link has no title slug")
	// ErrUnknownSite means a reading URL belongs to a LibSocial site the
	// client does not know.
	ErrUnknownSite = errors.New("unknown title source")
	// ErrInvalidInput means a user-supplied title link or slug is not usable.
	ErrInvalidInput = errors.New("invalid title link or slug")
	// ErrTitleNotFound means the API answered without a usable slug_url.
	ErrTitleNotFound = errors.New("title not found on this site")
	// ErrRateLimited re-exports the shared sentinel so callers that only
	// import animelib can recognise the throttle pause.
	ErrRateLimited = catalog.ErrRateLimited
)
