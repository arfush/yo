package ui

import (
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
		{library.UserStatusCompleted, "Просмотрено"},
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
	want := []string{"Все", "Смотрю", "В планах", "Обожаю", "Просмотрено", "Брошено"}
	if len(userStatusFilterOptions) != len(want) {
		t.Fatalf("filter count = %d, want %d", len(userStatusFilterOptions), len(want))
	}
	for index, label := range want {
		if got := userStatusFilterOptions[index].label; got != label {
			t.Fatalf("filter %d = %q, want %q", index, got, label)
		}
	}
}
