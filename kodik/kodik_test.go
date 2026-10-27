package kodik

import "testing"

func TestPickQuality(t *testing.T) {
	links := map[int]string{
		360: "360",
		480: "480",
		720: "720",
	}

	tests := []struct {
		name        string
		preferred   int
		wantURL     string
		wantQuality int
	}{
		{name: "exact", preferred: 480, wantURL: "480", wantQuality: 480},
		{name: "nearest lower", preferred: 1080, wantURL: "720", wantQuality: 720},
		{name: "nearest lower between", preferred: 600, wantURL: "480", wantQuality: 480},
		{name: "nearest higher", preferred: 300, wantURL: "360", wantQuality: 360},
		{name: "default is 1080", preferred: 0, wantURL: "720", wantQuality: 720},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			url, quality := PickQuality(links, test.preferred)
			if url != test.wantURL || quality != test.wantQuality {
				t.Fatalf("got (%q, %d), want (%q, %d)", url, quality, test.wantURL, test.wantQuality)
			}
		})
	}

	if url, quality := PickQuality(nil, 1080); url != "" || quality != 0 {
		t.Fatalf("empty links: got (%q, %d), want empty", url, quality)
	}
	if url, quality := PickQuality(map[int]string{720: ""}, 1080); url != "" || quality != 0 {
		t.Fatalf("empty URLs: got (%q, %d), want empty", url, quality)
	}
}
