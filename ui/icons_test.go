package ui

import "testing"

func TestChevronIconSelectsStrokeWeight(t *testing.T) {
	tests := []struct {
		name  string
		size  int
		width int
		want  string
	}{
		{"footer at 1.25x", 14, 2, "chevron_right_thin"},
		{"submit and room button", 13, 2, "chevron_right"},
		{"footer at 1x", 11, 2, "chevron_right_thick"},
		{"provider at 1x", 9, 2, "chevron_right_heavy"},
		{"missing size", 0, 2, "chevron_right"},
	}
	for _, test := range tests {
		if got := chevronIcon("chevron_right", test.size, test.width); got != test.want {
			t.Errorf("%s: chevronIcon(%d, %d) = %q, want %q", test.name, test.size, test.width, got, test.want)
		}
	}
}
