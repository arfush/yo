package ui

import (
	"errors"
	"testing"
)

func TestUserErrorCarriesUITextAndBackendCause(t *testing.T) {
	cause := errors.New("backend exploded")
	err := errLoadTitle.wrap(cause)
	if got, want := userErrorText(err), errLoadTitle.message; got != want {
		t.Fatalf("user text = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatal("UI error no longer unwraps to its backend cause")
	}
	if got := userErrorCause(err); got != cause {
		t.Fatalf("cause = %v, want %v", got, cause)
	}
	if !isUserError(err) {
		t.Fatal("wrapped UI error was not recognized")
	}
}

func TestUnexpectedRawErrorDoesNotLeakEnglishText(t *testing.T) {
	raw := errors.New("database is locked")
	if got := userErrorText(raw); got != errUnexpected.message {
		t.Fatalf("raw error text = %q, want %q", got, errUnexpected.message)
	}
	if got := userErrorCause(raw); got != raw {
		t.Fatalf("raw cause = %v, want the original error", got)
	}
	if isUserError(raw) {
		t.Fatal("raw backend error was treated as UI feedback")
	}
}

func TestUserMessageHasNoJournalCause(t *testing.T) {
	err := userMessage("Введите ссылку.")
	if got := userErrorText(err); got != "Введите ссылку." {
		t.Fatalf("message = %q", got)
	}
	if cause := userErrorCause(err); cause != nil {
		t.Fatalf("validation message has a cause: %v", cause)
	}
}
