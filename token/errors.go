package token

import (
	"errors"

	"github.com/arfush/yo/catalog"
)

// Sentinels returned by the token store. ErrNotFound is an alias of the shared
// catalog sentinel so library and UI code can compare it without importing the
// platform-specific token package.
var (
	// ErrNotFound means the token file does not exist or cannot be read.
	ErrNotFound = catalog.ErrTokenMissing

	// ErrEmptyTokenFile means the DPAPI container held no data.
	ErrEmptyTokenFile = errors.New("token file is empty")
	// ErrMissingAccessToken means the JSON container has no access_token.
	ErrMissingAccessToken = errors.New("token file has no access_token")
	// ErrEmptySave means a caller tried to persist an empty token.
	ErrEmptySave = errors.New("cannot save an empty token")
	// ErrEmptyPayload means DPAPI protected an empty buffer.
	ErrEmptyPayload = errors.New("token payload is empty")
	// ErrEmptyDPAPIResult means Windows returned an empty ciphertext.
	ErrEmptyDPAPIResult = errors.New("DPAPI returned an empty payload")
)
