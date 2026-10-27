package library

import (
	"errors"

	"github.com/arfush/yo/catalog"
)

// Sentinels returned by the library layer. They carry their user-facing
// Russian wording and are named so tests and the UI can compare them.
var (
	// ErrDatabaseUnavailable means the store was not opened.
	ErrDatabaseUnavailable = errors.New("база данных недоступна")
	// ErrUnknownUserStatus is returned by SetUserStatus for an invalid value.
	ErrUnknownUserStatus = errors.New("неизвестный статус тайтла")
	// ErrTokenMissing means the AnimeLIB token file was not found.
	ErrTokenMissing = errors.New("токен AnimeLIB не найден — перезапусти yoanime, чтобы авторизоваться")
	// ErrTokenExpired means the saved token is past its expiry margin.
	ErrTokenExpired = errors.New("токен AnimeLIB истёк — перезапусти yoanime, чтобы авторизоваться")
	// ErrNoVoiceovers means a Kinopoisk plan request found no translations.
	ErrNoVoiceovers = errors.New("у тайтла нет озвучек")
	// ErrNotServed re-exports the Kinopoisk sentinel wording.
	ErrNotServed = errors.New("Alloha не раздаёт этот тайтл")
	// ErrNoKinopoiskID means the cached title has no Kinopoisk id.
	ErrNoKinopoiskID = errors.New("у тайтла нет id Кинопоиска")

	// ErrChapterNotFound and ErrUnsupportedSource are aliases of the catalog
	// sentinels used by the reader screen.
	ErrChapterNotFound   = catalog.ErrChapterNotFound
	ErrUnsupportedSource = catalog.ErrUnsupportedSource
	// ErrBadPageURL means a page link was not an http(s) URL.
	ErrBadPageURL = errors.New("некорректный адрес страницы")

	// Cover errors.
	ErrCoversClosed = errors.New("хранилище обложек не открыто")
	ErrNoCoverURL   = errors.New("у тайтла нет ссылки на обложку")
	ErrNoTitleID    = errors.New("у тайтла нет локального id")
	ErrBadCoverFile = errors.New("обложка: пустой или слишком большой файл")
)
