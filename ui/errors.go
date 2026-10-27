package ui

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/arfush/yoanime/internal/animelib"
	"github.com/arfush/yoanime/internal/token"
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

// friendlyError turns a technical error into a short Russian sentence for the
// header. The original error is logged by the caller and never shown: messages
// from HTTP clients and browsers contain paths, URLs and status codes that mean
// nothing to the user.
func friendlyError(err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	var friendly userMessageError
	if errors.As(err, &friendly) {
		return friendly.message
	}
	if errors.Is(err, animelib.ErrRateLimited) {
		return "AnimeLIB просит подождать. Попробуйте через минуту."
	}
	if errors.Is(err, token.ErrNotFound) {
		return "Нужно войти в AnimeLIB."
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
		strings.Contains(message, "токен"), strings.Contains(message, "token"):
		return "Сессия AnimeLIB истекла. Войдите на сайт заново."
	case strings.Contains(message, "не удалось найти"), strings.Contains(message, "not found"),
		strings.Contains(message, "404"):
		return "Тайтл не найден. Проверьте ссылку или slug."
	case strings.Contains(message, "chromium"):
		return "Не удалось запустить браузер. Попробуйте перезапустить приложение."
	case strings.Contains(message, "mpv"):
		return "Не удалось запустить плеер. Проверьте установку mpv."
	case strings.Contains(message, "kodik"):
		return "Не удалось получить видео с Kodik. Попробуйте другую озвучку."
	case strings.Contains(message, "alloha"), strings.Contains(message, "kinopoisk"),
		strings.Contains(message, "плеер"):
		return "Не удалось получить видео с Кинопоиска. Попробуйте позже."
	case strings.Contains(message, "комнат"), strings.Contains(message, "реле"),
		strings.Contains(message, "party"), strings.Contains(message, "websocket"):
		return "Не удалось подключиться к комнате совместного просмотра."
	case strings.Contains(message, "обновлен"), strings.Contains(message, "release"),
		strings.Contains(message, "update"):
		return "Не удалось проверить обновления. Попробуйте позже."
	case strings.Contains(message, "http 429"), strings.Contains(message, "too many requests"):
		return "AnimeLIB просит подождать. Попробуйте через минуту."
	case strings.Contains(message, "no such host"), strings.Contains(message, "connection"),
		strings.Contains(message, "network"), strings.Contains(message, "dial"):
		return "Нет соединения с интернетом. Проверьте сеть и повторите."
	case strings.Contains(message, "база данных"), strings.Contains(message, "sqlite"),
		strings.Contains(message, "database"):
		return "Не удалось сохранить данные. Попробуйте ещё раз."
	case strings.Contains(message, "ссылк"), strings.Contains(message, "slug"):
		return "Не удалось разобрать ссылку. Проверьте её и повторите."
	}
	return "Что-то пошло не так. Попробуйте ещё раз."
}
