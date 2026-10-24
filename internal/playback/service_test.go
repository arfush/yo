package playback

import (
	"strings"
	"testing"
	"unicode"

	"github.com/arfush/yonime/internal/library"
)

func TestUnopenedErrorDescribesKodik(t *testing.T) {
	err := unopenedError(&playlistEntry{
		Episode: library.PlaybackEpisode{Number: "7"},
		Kind:    "hls",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	if !strings.Contains(message, "Kodik") {
		t.Fatalf("message %q should mention Kodik", message)
	}
	if strings.Contains(message, "серия") {
		t.Fatalf("message %q should not carry an episode prefix", message)
	}
	if !unicode.IsUpper([]rune(message)[0]) {
		t.Fatalf("message %q should start with a capital letter", message)
	}
}

func TestUnopenedErrorFallsBackToGenericText(t *testing.T) {
	err := unopenedError(nil)
	if err == nil || err.Error() == "" {
		t.Fatal("expected a non-empty error")
	}
}
