package kodik

import "errors"

// Sentinels returned by the Kodik resolver. They keep their Russian wording
// because the UI shows them directly after the friendlyError mapping.
var (
	// ErrEmptyLink means the embed URL was empty.
	ErrEmptyLink = errors.New("пустая ссылка Kodik")
	// ErrNoVideoLinks means the resolved page had no <video> sources.
	ErrNoVideoLinks = errors.New("Kodik не вернул ссылки на видео")
	// ErrEmptyLinks means the m3u8 fetch produced an empty list.
	ErrEmptyLinks = errors.New("Kodik вернул пустой список ссылок")
	// ErrUnknownFormat means the quality/format object was not understood.
	ErrUnknownFormat = errors.New("Kodik вернул ссылки в неизвестном формате")
	// ErrEmptySource means the link entry carried no URL.
	ErrEmptySource = errors.New("пустая ссылка")
	// ErrCannotDecodeLink means the encrypted page link could not be
	// decrypted.
	ErrCannotDecodeLink = errors.New("не удалось декодировать ссылку Kodik")
	// ErrBrowserClosed means the shared browser exited mid-resolve.
	ErrBrowserClosed = errors.New("браузер Kodik закрыт")
	// ErrBrowserUnavailable means the browser is busy or unresponsive.
	ErrBrowserUnavailable = errors.New("браузер Kodik недоступен")
)
