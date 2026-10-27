package party

import "errors"

// Sentinels returned by the room manager and the relay transports.
var (
	// ErrNotConfigured means the selected provider has no relay URL wired in
	// at build time or through party.json.
	ErrNotConfigured = errors.New("провайдер не настроен")
	// ErrWrongSecret means the guest submitted a wrong room secret.
	ErrWrongSecret = errors.New("неверный секрет комнаты")
	// ErrNoCard means the host has not published a room card yet.
	ErrNoCard = errors.New("комната ещё не ответила")
	// ErrInvalidKey means the room key format is invalid.
	ErrInvalidKey = errors.New("некорректный ключ комнаты")
	// ErrCorruptedMessage means a room payload is too short or fails
	// authentication.
	ErrCorruptedMessage = errors.New("повреждённое сообщение комнаты")
	// ErrNotConnected means the transport is not connected to the relay.
	ErrNotConnected = errors.New("нет соединения с комнатой")
	// ErrRoomClosed means the relay dropped the room channel.
	ErrRoomClosed = errors.New("соединение с комнатой закрыто")
	// ErrServerUnreachable means the relay did not answer in time.
	ErrServerUnreachable = errors.New("не удалось связаться с сервером комнаты")
	// ErrSupabaseKey means a secret key was passed instead of a publishable
	// one.
	ErrSupabaseKey = errors.New("для Supabase нужен publishable-ключ, а не секретный")
)
