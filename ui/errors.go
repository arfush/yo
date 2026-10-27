package ui

import (
	"context"
	"errors"
	"time"
)

// userError is a failure that already carries the Russian sentence shown to
// the user. cause keeps the lower-level error for the English journal; it is
// nil for expected UI feedback such as a validation message.
type userError struct {
	message string
	cause   error
}

func (e *userError) Error() string { return e.message }
func (e *userError) Unwrap() error { return e.cause }

// uiErrorKind is a reusable UI-level error with a fixed user message. Backend
// calls wrap their technical error with the kind that matches the operation, so
// screens never translate raw feature-package errors themselves.
type uiErrorKind struct {
	message string
}

func (k uiErrorKind) new() error {
	return &userError{message: k.message}
}

func (k uiErrorKind) wrap(cause error) error {
	if cause == nil {
		return k.new()
	}
	return &userError{message: k.message, cause: cause}
}

var (
	errLoadTitle        = uiErrorKind{message: "Не удалось загрузить тайтл. Проверьте ссылку и попробуйте позже."}
	errOpenTitle        = uiErrorKind{message: "Не удалось открыть тайтл. Попробуйте ещё раз."}
	errHistoryLoad      = uiErrorKind{message: "Не удалось загрузить историю. Попробуйте позже."}
	errHistorySave      = uiErrorKind{message: "Не удалось сохранить порядок истории. Попробуйте ещё раз."}
	errHistoryDelete    = uiErrorKind{message: "Не удалось удалить тайтл из истории. Попробуйте ещё раз."}
	errSaveTitle        = uiErrorKind{message: "Не удалось сохранить тайтл. Попробуйте ещё раз."}
	errUserStatus       = uiErrorKind{message: "Не удалось изменить статус тайтла. Попробуйте ещё раз."}
	errPlaybackStart    = uiErrorKind{message: "Не удалось начать воспроизведение. Попробуйте ещё раз."}
	errPlaybackFinished = uiErrorKind{message: "Не удалось воспроизвести видео. Попробуйте ещё раз."}
	errReaderChapter    = uiErrorKind{message: "Не удалось загрузить главу. Попробуйте другую или повторите позже."}
	errSettingsSave     = uiErrorKind{message: "Не удалось сохранить настройки. Попробуйте ещё раз."}
	errPartyCreate      = uiErrorKind{message: "Не удалось создать комнату. Попробуйте позже."}
	errPartyJoin        = uiErrorKind{message: "Не удалось войти в комнату. Проверьте ключ и попробуйте снова."}
	errPartyRoom        = uiErrorKind{message: "Не удалось выполнить действие с комнатой. Попробуйте ещё раз."}
	errPartySecret      = uiErrorKind{message: "Не удалось подтвердить секрет комнаты. Попробуйте снова."}
	errPartyName        = uiErrorKind{message: "Никнейм — до 24 символов, без служебных знаков и не похож на ключ комнаты."}
	errRoomSync         = uiErrorKind{message: "Не удалось синхронизировать комнату. Проверьте подключение."}
	errUpdate           = uiErrorKind{message: "Не удалось обновить приложение. Попробуйте позже."}
	errInstallBrowser   = uiErrorKind{message: "Не удалось установить Chromium. Проверьте подключение и повторите."}
	errInstallMPV       = uiErrorKind{message: "Не удалось установить mpv. Проверьте подключение и повторите."}
	errAuthorize        = uiErrorKind{message: "Не удалось войти в AnimeLIB. Попробуйте ещё раз."}
	errUnexpected       = uiErrorKind{message: "Что-то пошло не так. Попробуйте ещё раз."}
)

// userMessage creates UI feedback that has no lower-level error to log.
func userMessage(message string) error {
	return (&uiErrorKind{message: message}).new()
}

// userErrorText returns the sentence prepared by the UI. A raw error should not
// reach a screen; it falls back to a neutral sentence instead of leaking
// English technical text into the interface.
func userErrorText(err error) string {
	var ui *userError
	if errors.As(err, &ui) {
		return ui.message
	}
	return errUnexpected.message
}

// userErrorCause returns the backend error carried by a UI error, if any. This
// is the value that belongs in the English application journal.
func userErrorCause(err error) error {
	var ui *userError
	if errors.As(err, &ui) {
		return ui.cause
	}
	return err
}

func isUserError(err error) bool {
	var ui *userError
	return errors.As(err, &ui)
}

// reportError shows the UI sentence and logs only the lower-level cause. A UI
// validation message has no cause and therefore produces no journal entry.
func (a *application) reportError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	if cause := userErrorCause(err); cause != nil && !errors.Is(cause, context.Canceled) {
		logger.Error("ui operation failed", "error", cause)
	}
	message := userErrorText(err)
	if message == "" {
		return
	}
	a.shell.setError(message, time.Now())
	if a.window != nil {
		a.window.Invalidate()
	}
}
