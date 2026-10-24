package playback

import (
	"strings"
	"testing"

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
	if !strings.Contains(message, "7") || !strings.Contains(message, "Kodik") {
		t.Fatalf("message %q should mention the episode and Kodik", message)
	}
}

func TestUnopenedErrorFallsBackToGenericText(t *testing.T) {
	err := unopenedError(nil)
	if err == nil || err.Error() == "" {
		t.Fatal("expected a non-empty error")
	}
}
