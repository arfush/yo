package playback

import (
	"strings"
	"testing"
)

// TestHeaderScriptAcceptsTheMeasuredBadgeWidth guards the contract between the
// launcher and the Lua header. mpv matches script-opts keys verbatim, so the
// script must declare the dashed key the launcher sends; when the two drift
// apart mpv only logs "unknown key" and the header silently estimates the pill
// width from its character table, which made the two badges look different.
func TestHeaderScriptAcceptsTheMeasuredBadgeWidth(t *testing.T) {
	opts := headerScriptOpts(17.5, "Yonime")
	if !strings.Contains(opts, "yonime-header-badge-width=17.5") {
		t.Fatalf("launcher does not pass the measured width: %q", opts)
	}

	script := string(headerScript)
	if !strings.Contains(script, `["badge-width"] = 0`) {
		t.Fatal(`yonime-header.lua must declare ["badge-width"]`)
	}
	if !strings.Contains(script, `options["badge-width"]`) {
		t.Fatal(`yonime-header.lua must read options["badge-width"]`)
	}
}

func TestHeaderScriptOptsWithoutMeasurement(t *testing.T) {
	opts := headerScriptOpts(0, "Yonime")
	if strings.Contains(opts, "badge-width") {
		t.Fatalf("zero width must stay unset so the script uses its fallback: %q", opts)
	}
	if !strings.Contains(opts, "osc-windowcontrols=no") {
		t.Fatalf("header opts lost the OSC setting: %q", opts)
	}
}

func TestHeaderScriptOptsCarriesTheRole(t *testing.T) {
	opts := headerScriptOpts(20, "Хост")
	if !strings.Contains(opts, "title=Хост") {
		t.Fatalf("watch-party role is missing from the header opts: %q", opts)
	}
}
