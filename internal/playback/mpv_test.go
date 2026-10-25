package playback

import (
	"reflect"
	"strings"
	"testing"
)

func TestInputLockArgs(t *testing.T) {
	if got := inputLockArgs(false); got != nil {
		t.Fatalf("inputLockArgs(false) = %v, want no extra arguments", got)
	}
	want := []string{"--osc=no"}
	if got := inputLockArgs(true); !reflect.DeepEqual(got, want) {
		t.Fatalf("inputLockArgs(true) = %v, want %v", got, want)
	}
}

// TestGuestInputSectionBlocksOnlyTheTimeline guards the input.conf contract:
// the section must exist with the name the launcher enables, and every default
// pause/seek binding must be overridden there. Dropping one line would let a
// locked guest move the host's playback again.
func TestGuestInputSectionBlocksOnlyTheTimeline(t *testing.T) {
	conf := string(guestInputConf)
	if !strings.Contains(conf, "["+guestInputSection+"]") {
		t.Fatalf("input.conf does not declare [%s]", guestInputSection)
	}
	blocked := []string{
		"LEFT ignore",
		"RIGHT ignore",
		"Shift+LEFT ignore",
		"Shift+RIGHT ignore",
		"Ctrl+LEFT ignore",
		"Ctrl+RIGHT ignore",
		"PGUP ignore",
		"PGDN ignore",
		"Shift+PGUP ignore",
		"Shift+PGDN ignore",
		", ignore",
		". ignore",
		"SPACE ignore",
		"p ignore",
		"MBTN_LEFT ignore",
		"MBTN_RIGHT ignore",
		"WHEEL_UP ignore",
		"WHEEL_DOWN ignore",
	}
	for _, binding := range blocked {
		if !strings.Contains(conf, binding) {
			t.Errorf("input.conf is missing %q", binding)
		}
	}
}
