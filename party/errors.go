package party

import "errors"

// Sentinels returned by the room manager and the relay transports.
var (
	// ErrNotConfigured means the selected provider has no relay URL wired in
	// at build time or through party.json.
	ErrNotConfigured = errors.New("party provider is not configured")
	// ErrWrongSecret means the guest submitted a wrong room secret.
	ErrWrongSecret = errors.New("wrong room secret")
	// ErrNoCard means the host has not published a room card yet.
	ErrNoCard = errors.New("room has not answered yet")
	// ErrInvalidKey means the room key format is invalid.
	ErrInvalidKey = errors.New("invalid room key")
	// ErrCorruptedMessage means a room payload is too short or fails
	// authentication.
	ErrCorruptedMessage = errors.New("corrupted room message")
	// ErrNotConnected means the transport is not connected to the relay.
	ErrNotConnected = errors.New("not connected to the room")
	// ErrRoomClosed means the relay dropped the room channel.
	ErrRoomClosed = errors.New("room connection is closed")
	// ErrServerUnreachable means the relay did not answer in time.
	ErrServerUnreachable = errors.New("room server is unreachable")
	// ErrSupabaseKey means a secret key was passed instead of a publishable
	// one.
	ErrSupabaseKey = errors.New("Supabase requires a publishable key, not a secret key")
)
