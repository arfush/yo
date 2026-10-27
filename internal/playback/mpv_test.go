package playback

import (
	"strings"
	"testing"

	"github.com/dexterlb/mpvipc"
)

func TestStreamOptionsKeepHeadersPerFile(t *testing.T) {
	options := streamOptions([]string{"Referer: https://example.test/", "Origin: https://example.test"})
	got, ok := options["http-header-fields"]
	if !ok {
		t.Fatal("stream options have no http-header-fields")
	}
	want := "Referer: https://example.test/,Origin: https://example.test"
	if got != want {
		t.Fatalf("headers = %q, want %q", got, want)
	}
	if len(streamOptions(nil)) != 0 {
		t.Fatal("empty headers must not add a per-file option")
	}
}

func TestPlaylistEntryID(t *testing.T) {
	if got := playlistEntryID(nil); got != 0 {
		t.Fatalf("nil event entry id = %d", got)
	}
	event := &mpvipc.Event{ExtraData: map[string]interface{}{"playlist_entry_id": float64(7)}}
	if got := playlistEntryID(event); got != 7 {
		t.Fatalf("entry id = %d, want 7", got)
	}
	if got := playlistEntryID(&mpvipc.Event{}); got != 0 {
		t.Fatalf("missing entry id = %d, want 0", got)
	}
}

func TestGeometryKeepsPositionButLetsMPVChooseSize(t *testing.T) {
	got := geometryString(120, -80, 1600, 900)
	if got != "+120-80" {
		t.Fatalf("geometry = %q, want position only", got)
	}
	if strings.ContainsAny(got, "xX") {
		t.Fatalf("geometry %q must not carry a size", got)
	}
	if got := geometryString(10, 20, 0, 900); got != "" {
		t.Fatalf("geometry without a full rectangle = %q, want empty", got)
	}
}
