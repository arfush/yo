package ui

import "testing"

func TestWindowRectEncodingRoundTrip(t *testing.T) {
	rect := windowRect{X: 120, Y: 40, Width: 960, Height: 540}
	raw := encodeRect(rect)
	if raw != "120,40,960,540" {
		t.Fatalf("encodeRect = %q", raw)
	}
	got, ok := decodeRect(raw)
	if !ok || got != rect {
		t.Fatalf("decodeRect(%q) = %+v, %v; want %+v", raw, got, ok, rect)
	}
}

func TestWindowRectDecodeRejectsGarbage(t *testing.T) {
	for _, raw := range []string{"", "1,2,3", "1,2,3,x", "1,2,0,10", "1,2,-5,10"} {
		if got, ok := decodeRect(raw); ok {
			t.Fatalf("decodeRect(%q) accepted %+v", raw, got)
		}
	}
}
