package ui

import "testing"

func TestTitleQualityOptionsWithoutLadder(t *testing.T) {
	options, selected := titleQualityOptions(nil, 1080)
	if len(options) != 4 {
		t.Fatalf("len = %d, want 4", len(options))
	}
	for index, option := range options {
		if !option.enabled {
			t.Fatalf("option %d is disabled without a known ladder", index)
		}
	}
	if options[selected].value != 1080 {
		t.Fatalf("selected value = %d, want 1080", options[selected].value)
	}
	// A huge preference clamps to 2160 instead of returning nothing.
	_, selected = titleQualityOptions(nil, 5000)
	if options[selected].value != 2160 {
		t.Fatalf("clamped selection = %d, want 2160", options[selected].value)
	}
}

func TestTitleQualityOptionsBlockMissingSteps(t *testing.T) {
	// A 1080p title: 1440 and 2160 stay visible but disabled.
	options, selected := titleQualityOptions([]int{360, 480, 720, 1080}, 2160)
	wantValues := []int{720, 1080, 1440, 2160}
	for index, option := range options {
		if option.value != wantValues[index] {
			t.Fatalf("option %d value = %d, want %d", index, option.value, wantValues[index])
		}
	}
	if options[0].enabled != options[1].enabled || !options[1].enabled {
		t.Fatal("720/1080 must stay enabled")
	}
	if options[2].enabled || options[3].enabled {
		t.Fatal("1440/2160 must be blocked for a 1080p source")
	}
	if options[selected].value != 1080 {
		t.Fatalf("preferred 2160 selected %d, want the nearest enabled 1080", options[selected].value)
	}
}

func TestTitleQualityOptionsFallbackToRealLadder(t *testing.T) {
	// A voice with only 360/480 offers none of the standard steps: the block
	// switches to its real ladder instead of four dead buttons.
	options, selected := titleQualityOptions([]int{360, 480}, 1080)
	if len(options) != 2 {
		t.Fatalf("len = %d, want 2", len(options))
	}
	if options[0].value != 360 || options[1].value != 480 {
		t.Fatalf("options = %+v", options)
	}
	for _, option := range options {
		if !option.enabled {
			t.Fatal("fallback ladder must be fully enabled")
		}
	}
	if options[selected].value != 480 {
		t.Fatalf("selection = %d, want the nearest lower 480", options[selected].value)
	}
}

func TestTitleQualityLabels(t *testing.T) {
	labels := map[int]string{
		360: "360 SD", 480: "480 SD", 720: "720 HD", 1080: "1080 FHD",
		1440: "1440 2K", 2160: "2160 4K", 777: "777",
	}
	for value, want := range labels {
		if got := titleQualityLabel(value); got != want {
			t.Fatalf("label(%d) = %q, want %q", value, got, want)
		}
	}
}

func TestQualitySectionLabel(t *testing.T) {
	for _, item := range []historyTitle{
		{source: "animelib", movie: false},
		{source: "kinopoisk", movie: false},
		{source: "kinopoisk", movie: true},
	} {
		if got := (&titleWindow{item: item}).qualitySectionLabel(); got != "КАЧЕСТВО" {
			t.Fatalf("label for %+v = %q, want КАЧЕСТВО", item, got)
		}
	}
}
