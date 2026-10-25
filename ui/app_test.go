package ui

import "testing"

func TestAutoUpdateSafe(t *testing.T) {
	tests := []struct {
		name                       string
		playing, party, main, quit bool
		want                       bool
	}{
		{name: "idle main screen", main: true, want: true},
		{name: "playback blocks", playing: true, main: true, want: false},
		{name: "watch party blocks", party: true, main: true, want: false},
		{name: "settings screen blocks", want: false},
		{name: "quitting blocks", main: true, quit: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := autoUpdateSafe(test.playing, test.party, test.main, test.quit); got != test.want {
				t.Fatalf("autoUpdateSafe() = %v, want %v", got, test.want)
			}
		})
	}
}
