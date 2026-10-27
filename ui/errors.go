package ui

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/arfush/yo/animelib"
	"github.com/arfush/yo/browser"
	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/kinopoisk"
	"github.com/arfush/yo/kodik"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/mpv"
	"github.com/arfush/yo/party"
	"github.com/arfush/yo/playback"
	"github.com/arfush/yo/token"
	"github.com/arfush/yo/update"
)

// userMessageError carries a message that is already written for the user.
type userMessageError struct {
	message string
}

func (e userMessageError) Error() string {
	return e.message
}

func userMessage(message string) error {
	return userMessageError{message: message}
}

// userFacing returns the Russian sentence of a UI-generated message. Such
// errors are expected feedback, not technical failures, so they are shown but
// never written to the English log.
func userFacing(err error) (string, bool) {
	var friendly userMessageError
	if errors.As(err, &friendly) {
		return friendly.message, true
	}
	return "", false
}

// friendlyError turns a technical error into a short Russian sentence for the
// header. Feature packages keep their messages in English; the Russian wording
// lives here, and the original error is logged by the caller and never shown.
func friendlyError(err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	var friendly userMessageError
	if errors.As(err, &friendly) {
		return friendly.message
	}

	switch {
	case errors.Is(err, catalog.ErrRateLimited):
		return "AnimeLIB просит подождать. Попробуйте через минуту."
	case errors.Is(err, catalog.ErrTokenMissing),
		errors.Is(err, animelib.ErrNoToken),
		errors.Is(err, token.ErrNotFound):
		return "Нужно войти в AnimeLIB."
	case errors.Is(err, library.ErrTokenExpired):
		return "Сессия AnimeLIB истекла. Войдите на сайт заново."
	case errors.Is(err, animelib.ErrInvalidInput),
		errors.Is(err, animelib.ErrEmptySlug),
		errors.Is(err, browser.ErrInvalidLink):
		return "Не удалось разобрать ссылку. Проверьте её и повторите."
	case errors.Is(err, animelib.ErrUnknownSite),
		errors.Is(err, catalog.ErrUnsupportedSource):
		return "Этот источник не поддерживается."
	case errors.Is(err, animelib.ErrTitleNotFound),
		errors.Is(err, kinopoisk.ErrInvalidID):
		return "Тайтл не найден. Проверьте ссылку или slug."
	case errors.Is(err, catalog.ErrChapterNotFound):
		return "Глава не найдена. Выберите другую главу."
	case errors.Is(err, library.ErrNoVoiceovers),
		errors.Is(err, library.ErrNoKinopoiskID),
		errors.Is(err, library.ErrNotServed),
		errors.Is(err, kinopoisk.ErrNotServed),
		errors.Is(err, kinopoisk.ErrNoMirrors),
		errors.Is(err, kinopoisk.ErrNoPlayerLink):
		return "Не удалось загрузить тайтл. Попробуйте позже."
	case errors.Is(err, kinopoisk.ErrNoEpisodeList),
		errors.Is(err, kinopoisk.ErrEmptyEpisodeList),
		errors.Is(err, kinopoisk.ErrBadEpisodeList),
		errors.Is(err, kinopoisk.ErrCorruptedEpisodeList),
		errors.Is(err, kinopoisk.ErrNoEpisodes),
		errors.Is(err, kinopoisk.ErrNoMovieFile),
		errors.Is(err, kinopoisk.ErrNoVideoSources),
		errors.Is(err, kinopoisk.ErrNoQualityLinks),
		errors.Is(err, kinopoisk.ErrNoStreamURL),
		errors.Is(err, kinopoisk.ErrNoAccessKey):
		return "Не удалось загрузить видео. Попробуйте позже."
	case errors.Is(err, playback.ErrVideoOpen):
		return "Не удалось открыть видео. Проверьте подключение и повторите."
	case errors.Is(err, playback.ErrKodikUnavailable):
		return "Сервер видео не отвечает. Проверьте подключение."
	case errors.Is(err, playback.ErrPlaybackFailed):
		return "Не удалось воспроизвести видео. Попробуйте ещё раз."
	case errors.Is(err, playback.ErrNoEpisodes), errors.Is(err, playback.ErrInvalidEpisode):
		return "Не удалось выбрать серию. Попробуйте другую."
	case errors.Is(err, playback.ErrAlreadyPlaying):
		return "Видео уже воспроизводится."
	case errors.Is(err, playback.ErrClosing):
		return "Приложение закрывается."
	case errors.Is(err, playback.ErrNotRunning), errors.Is(err, mpv.ErrNotFound),
		errors.Is(err, mpv.ErrNotInstalled):
		return "Не удалось запустить плеер. Проверьте установку mpv."
	case errors.Is(err, browser.ErrSiteUnreachable):
		return "Сайт не открылся. Попробуйте другое зеркало."
	case errors.Is(err, browser.ErrWindowClosed):
		return "Окно авторизации закрыто."
	case errors.Is(err, browser.ErrChromiumMissing),
		errors.Is(err, browser.ErrHeadlessClosed),
		errors.Is(err, browser.ErrHeadlessNil),
		errors.Is(err, kinopoisk.ErrBrowserClosed),
		errors.Is(err, kinopoisk.ErrBrowserUnavailable),
		errors.Is(err, kodik.ErrBrowserClosed),
		errors.Is(err, kodik.ErrBrowserUnavailable):
		return "Не удалось запустить браузер. Попробуйте перезапустить приложение."
	case errors.Is(err, party.ErrWrongSecret):
		return "Неверный секрет комнаты."
	case errors.Is(err, party.ErrRoomNotFound), errors.Is(err, party.ErrNoCard):
		return "Комната не найдена. Проверьте ключ."
	case errors.Is(err, party.ErrInvalidKey):
		return "Некорректный ключ комнаты."
	case errors.Is(err, party.ErrInvalidSecret):
		return "Некорректный секрет комнаты."
	case errors.Is(err, party.ErrInvalidName):
		return "Некорректный никнейм."
	case errors.Is(err, party.ErrVersionMismatch):
		return "Разные версии приложения. Обновите yoanime у обоих участников."
	case errors.Is(err, party.ErrNotConfigured), errors.Is(err, party.ErrSupabaseKey):
		return "Совместный просмотр не настроен."
	case errors.Is(err, party.ErrNotConnected), errors.Is(err, party.ErrRoomClosed),
		errors.Is(err, party.ErrServerUnreachable), errors.Is(err, party.ErrCorruptedMessage):
		return "Не удалось подключиться к комнате. Попробуйте ещё раз."
	case errors.Is(err, library.ErrDatabaseCorrupt):
		return "Данные приложения повреждены. Перезапустите приложение."
	case errors.Is(err, library.ErrDatabaseUnavailable):
		return "Не удалось сохранить данные. Попробуйте ещё раз."
	case errors.Is(err, update.ErrBadAsset):
		return "Скачанное обновление повреждено. Попробуйте позже."
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return "Нет соединения с интернетом. Проверьте сеть и повторите."
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "context deadline"), strings.Contains(message, "timeout"):
		return "Сервер не ответил вовремя. Попробуйте ещё раз."
	case strings.Contains(message, "401"), strings.Contains(message, "unauthorized"),
		strings.Contains(message, "token"):
		return "Сессия AnimeLIB истекла. Войдите на сайт заново."
	case strings.Contains(message, "not found"), strings.Contains(message, "404"):
		return "Тайтл не найден. Проверьте ссылку или slug."
	case strings.Contains(message, "chromium"), strings.Contains(message, "playwright"):
		return "Не удалось запустить браузер. Попробуйте перезапустить приложение."
	case strings.Contains(message, "mpv"):
		return "Не удалось запустить плеер. Проверьте установку mpv."
	case strings.Contains(message, "kodik"):
		return "Не удалось получить видео. Попробуйте другую озвучку."
	case strings.Contains(message, "kinopoisk"), strings.Contains(message, "alloha"),
		strings.Contains(message, "player"), strings.Contains(message, "stream"):
		return "Не удалось загрузить видео. Попробуйте позже."
	case strings.Contains(message, "room"), strings.Contains(message, "party"),
		strings.Contains(message, "relay"), strings.Contains(message, "websocket"):
		return "Не удалось подключиться к комнате совместного просмотра."
	case strings.Contains(message, "update"), strings.Contains(message, "release"):
		return "Не удалось проверить обновления. Попробуйте позже."
	case strings.Contains(message, "rate limited"), strings.Contains(message, "http 429"),
		strings.Contains(message, "too many requests"):
		return "AnimeLIB просит подождать. Попробуйте через минуту."
	case strings.Contains(message, "no such host"), strings.Contains(message, "connection"),
		strings.Contains(message, "network"), strings.Contains(message, "dial"):
		return "Нет соединения с интернетом. Проверьте сеть и повторите."
	case strings.Contains(message, "sqlite"), strings.Contains(message, "database"):
		return "Не удалось сохранить данные. Попробуйте ещё раз."
	case strings.Contains(message, "link"), strings.Contains(message, "slug"):
		return "Не удалось разобрать ссылку. Проверьте её и повторите."
	}
	return "Что-то пошло не так. Попробуйте ещё раз."
}
