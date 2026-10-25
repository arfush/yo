package ui

import (
	"image"
	"testing"

	"github.com/arfush/yonime/internal/library"
)

func TestUserStatusLabel(t *testing.T) {
	tests := []struct {
		status library.UserStatus
		want   string
	}{
		{0, "В планах"},
		{library.UserStatusWatching, "Смотрю"},
		{library.UserStatusPlanned, "В планах"},
		{library.UserStatusFavorite, "Обожаю"},
		{library.UserStatusCompleted, "Досмотрел"},
		{library.UserStatusDropped, "Брошено"},
		{library.UserStatus(99), "В планах"},
	}
	for _, test := range tests {
		if got := userStatusLabel(test.status); got != test.want {
			t.Fatalf("userStatusLabel(%d) = %q, want %q", test.status, got, test.want)
		}
	}
}

func TestUserStatusFilterMatches(t *testing.T) {
	if !filterAll.matches(0) || !filterAll.matches(library.UserStatusDropped) {
		t.Fatal("filterAll must match every status")
	}
	if !filterPlanned.matches(0) {
		t.Fatal("filterPlanned must match the default status")
	}
	if filterWatching.matches(library.UserStatusPlanned) {
		t.Fatal("filterWatching matched a planned title")
	}
	if !filterCompleted.matches(library.UserStatusCompleted) {
		t.Fatal("filterCompleted did not match a completed title")
	}
}

func TestUserStatusFilterOrder(t *testing.T) {
	want := []string{"Все", "Смотрю", "В планах", "Обожаю", "Досмотрел", "Брошено"}
	if len(userStatusFilterOptions) != len(want) {
		t.Fatalf("filter count = %d, want %d", len(userStatusFilterOptions), len(want))
	}
	for index, label := range want {
		if got := userStatusFilterOptions[index].label; got != label {
			t.Fatalf("filter %d = %q, want %q", index, got, label)
		}
	}
}

func TestUserStatusMenuOptionsSkipCurrent(t *testing.T) {
	tests := []struct {
		name    string
		current library.UserStatus
		want    []library.UserStatus
	}{
		{
			name:    "watching",
			current: library.UserStatusWatching,
			want: []library.UserStatus{
				library.UserStatusPlanned,
				library.UserStatusFavorite,
				library.UserStatusCompleted,
				library.UserStatusDropped,
			},
		},
		{
			name:    "unknown normalizes to planned",
			current: library.UserStatus(99),
			want: []library.UserStatus{
				library.UserStatusWatching,
				library.UserStatusFavorite,
				library.UserStatusCompleted,
				library.UserStatusDropped,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := userStatusMenuOptionsFor(test.current)
			if len(got) != len(test.want) {
				t.Fatalf("option count = %d, want %d", len(got), len(test.want))
			}
			for index := range test.want {
				if got[index] != test.want[index] {
					t.Fatalf("option %d = %d, want %d", index, got[index], test.want[index])
				}
			}
		})
	}
}

func TestPlaceUserStatusMenuFlipsAbove(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(100, 520, 180, 543)
	menu := placeUserStatusMenu(anchor, 120, 100, 7, 8, bounds)
	if menu.Max.Y > bounds.Max.Y-8 {
		t.Fatalf("menu = %v, want it inside the bottom margin", menu)
	}
	if menu.Max.Y >= anchor.Min.Y {
		t.Fatalf("menu = %v, want it above anchor %v", menu, anchor)
	}
}
