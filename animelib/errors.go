package animelib

import (
	"errors"

	"github.com/arfush/yo/catalog"
)

// Sentinels returned by the AnimeLIB and LibSocial API client. Russian-message
// errors stay inline: only the errors that other layers compare are exported.
var (
	// ErrNoToken means the client was created without a token file.
	ErrNoToken = errors.New("не найден токен AnimeLIB (animelib_token.txt в папке данных yoanime)")
	// ErrEmptySlug means a reading URL carried no title slug.
	ErrEmptySlug = errors.New("пустая ссылка на тайтл")
	// ErrUnknownSite means a reading URL belongs to a LibSocial site the
	// client does not know.
	ErrUnknownSite = errors.New("неизвестный источник тайтла")
	// ErrTitleNotFound means the API answered without a usable slug_url.
	ErrTitleNotFound = errors.New("тайтл не найден на этом сайте")
	// ErrRateLimited re-exports the shared sentinel so callers that only
	// import animelib can recognise the throttle pause.
	ErrRateLimited = catalog.ErrRateLimited
)
