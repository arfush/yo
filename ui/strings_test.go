package ui

import "testing"

func TestUpperFirst(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "сервер Kodik не отвечает", want: "Сервер Kodik не отвечает"},
		{in: "mpv не найден", want: "Mpv не найден"},
		{in: "\"серия\" недоступна", want: "\"Серия\" недоступна"},
		{in: "", want: ""},
		{in: "123 ошибка", want: "123 Ошибка"},
	}
	for _, tc := range cases {
		if got := upperFirst(tc.in); got != tc.want {
			t.Errorf("upperFirst(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
