package playback

import (
	"reflect"
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

func TestInputLockArgs(t *testing.T) {
	if got := inputLockArgs(false); got != nil {
		t.Fatalf("inputLockArgs(false) = %v, want no extra arguments", got)
	}
	want := []string{"--osc=no"}
	if got := inputLockArgs(true); !reflect.DeepEqual(got, want) {
		t.Fatalf("inputLockArgs(true) = %v, want %v", got, want)
	}
}

// TestGuestInputSectionBlocksOnlyTheTimeline guards the input.conf contract.
// mpv assigns sections with "key {section} command", not with a [section]
// header, and a malformed file silently puts the bindings into the default
// section. That would leave every playback locked, so check the syntax for
// every line and then the bindings that matter.
func TestGuestInputSectionBlocksOnlyTheTimeline(t *testing.T) {
	conf := string(guestInputConf)
	section := "{" + guestInputSection + "}"
	binds := 0
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			t.Errorf("input.conf uses a section header, which mpv treats as a key: %q", line)
		}
		if !strings.Contains(line, section) {
			t.Errorf("binding is not in %s: %q", guestInputSection, line)
		}
		if !strings.HasSuffix(line, " ignore") {
			t.Errorf("binding does not use ignore: %q", line)
		}
		binds++
	}
	if binds == 0 {
		t.Fatal("input.conf has no bindings")
	}

	blocked := []string{
		"LEFT",
		"RIGHT",
		"UP",
		"DOWN",
		"Shift+LEFT",
		"Shift+RIGHT",
		"Shift+UP",
		"Shift+DOWN",
		"Ctrl+LEFT",
		"Ctrl+RIGHT",
		"HOME",
		"PGUP",
		"PGDWN",
		"Shift+PGUP",
		"Shift+PGDWN",
		",",
		".",
		"SPACE",
		"p",
		"PAUSE",
		"PLAY",
		"PLAYPAUSE",
		"PAUSEONLY",
		"PLAYONLY",
		"FORWARD",
		"REWIND",
		"Shift+BS",
		"Shift+Ctrl+BS",
		"l",
		"WHEEL_LEFT",
		"WHEEL_RIGHT",
		"DEL",
		"MBTN_RIGHT",
		"MBTN_BACK",
		"MBTN_FORWARD",
		"ENTER",
		"<",
		">",
		"Shift+HOME",
		"Shift+END",
	}
	for _, key := range blocked {
		binding := key + " " + section + " ignore"
		if !strings.Contains(conf, binding) {
			t.Errorf("input.conf is missing %q", binding)
		}
	}

	// These must keep working. In particular, wheel up/down is volume in mpv,
	// and MBTN_LEFT belongs to the OSC.
	allowed := []string{"WHEEL_UP", "WHEEL_DOWN", "MBTN_LEFT", "MBTN_LEFT_DBL", "f", "v", "j", "[", "]", "9", "0", "m"}
	for _, key := range allowed {
		if strings.Contains(conf, "\n"+key+" ") {
			t.Errorf("input.conf must not block %q", key)
		}
	}
}
