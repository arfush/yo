package ui

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/arfush/yoanime/library"
)

func TestFormatEpisodeNotification(t *testing.T) {
	tests := []struct {
		name      string
		event     library.EpisodeNotification
		wantTitle string
		wantBody  string
	}{
		{
			name: "new episode names voiceover",
			event: library.EpisodeNotification{
				Title:   "Ван-Пис",
				Episode: "1080",
				Voiceovers: []library.EpisodeVoiceover{
					{Name: "AniLibria.TV"},
				},
			},
			wantTitle: "Ван-Пис",
			wantBody:  "1080 эп. в озвучке AniLibria.TV уже добавлен!",
		},
		{
			name: "kodik fallback is mentioned",
			event: library.EpisodeNotification{
				Title:      "Ван-Пис",
				Episode:    "5",
				Voiceovers: []library.EpisodeVoiceover{{Name: "AniLibria.TV", KodikOnly: true}},
			},
			wantTitle: "Ван-Пис",
			wantBody:  "5 эп. в озвучке AniLibria.TV уже добавлен! Доступно в Kodik.",
		},
		{
			name: "announced title has been released",
			event: library.EpisodeNotification{
				Title:         "Анонс",
				Episode:       "1",
				Voiceovers:    []library.EpisodeVoiceover{{Name: "AniLibria.TV"}},
				TitleReleased: true,
			},
			wantTitle: "Анонс",
			wantBody:  "Тайтл вышел! 1 эп. в озвучке AniLibria.TV уже добавлен!",
		},
		{
			name: "preferred voice follow-up",
			event: library.EpisodeNotification{
				Title:      "Ван-Пис",
				Episode:    "5",
				Voiceovers: []library.EpisodeVoiceover{{Name: "AniLibria.TV"}},
				Preferred:  true,
			},
			wantTitle: "Ван-Пис",
			wantBody:  "5 эп. в твоей озвучке AniLibria.TV уже добавлен!",
		},
		{
			name: "preferred voice after announced release",
			event: library.EpisodeNotification{
				Title:         "Анонс",
				Episode:       "1",
				Voiceovers:    []library.EpisodeVoiceover{{Name: "AniLibria.TV"}},
				TitleReleased: true,
				Preferred:     true,
			},
			wantTitle: "Анонс",
			wantBody:  "Тайтл вышел! 1 эп. в твоей озвучке AniLibria.TV уже добавлен!",
		},
		{
			name: "several voiceovers are listed",
			event: library.EpisodeNotification{
				Title:   "Тайтл",
				Episode: "2",
				Voiceovers: []library.EpisodeVoiceover{
					{Name: "A"},
					{Name: "B", KodikOnly: true},
				},
			},
			wantTitle: "Тайтл",
			wantBody:  "2 эп. в озвучках A, B уже добавлен! Доступно в Kodik.",
		},
		{
			name: "missing title falls back to a generic one",
			event: library.EpisodeNotification{
				Episode:    "1",
				Voiceovers: []library.EpisodeVoiceover{{Name: "A"}},
			},
			wantTitle: "Новая серия",
			wantBody:  "1 эп. в озвучке A уже добавлен!",
		},
		{
			name: "missing voiceover still reports the episode",
			event: library.EpisodeNotification{
				Title:   "Тайтл",
				Episode: "5",
			},
			wantTitle: "Тайтл",
			wantBody:  "5 эп. уже добавлен!",
		},
		{
			name: "missing voiceover after announced release",
			event: library.EpisodeNotification{
				Title:         "Анонс",
				Episode:       "3",
				TitleReleased: true,
			},
			wantTitle: "Анонс",
			wantBody:  "Тайтл вышел! 3 эп. уже добавлен!",
		},
		{
			name: "missing episode still names the voiceover",
			event: library.EpisodeNotification{
				Title:      "Тайтл",
				Voiceovers: []library.EpisodeVoiceover{{Name: "A"}},
			},
			wantTitle: "Тайтл",
			wantBody:  "Новая серия в озвучке A уже добавлена!",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			title, body := formatEpisodeNotification(test.event)
			if title != test.wantTitle {
				t.Fatalf("title = %q, want %q", title, test.wantTitle)
			}
			if body != test.wantBody {
				t.Fatalf("body = %q, want %q", body, test.wantBody)
			}
		})
	}
}

func TestFormatEpisodeNotificationTruncatesToWindowsLimits(t *testing.T) {
	event := library.EpisodeNotification{
		Title:   strings.Repeat("Я", 100),
		Episode: "1",
		Voiceovers: []library.EpisodeVoiceover{
			{Name: strings.Repeat("О", 400)},
		},
	}
	title, body := formatEpisodeNotification(event)
	if units := len(utf16.Encode([]rune(title))); units > maxNotificationTitle {
		t.Fatalf("title has %d UTF-16 units, limit is %d", units, maxNotificationTitle)
	}
	if units := len(utf16.Encode([]rune(body))); units > maxNotificationBody {
		t.Fatalf("body has %d UTF-16 units, limit is %d", units, maxNotificationBody)
	}
	if !strings.HasSuffix(title, "…") || !strings.HasSuffix(body, "…") {
		t.Fatalf("truncated values do not end with an ellipsis: %q / %q", title, body)
	}
}

func TestTrimNotificationTextKeepsSurrogatePairs(t *testing.T) {
	value := strings.Repeat("😀", 10)
	trimmed := trimNotificationText(value, 5)
	if want := strings.Repeat("😀", 2) + "…"; trimmed != want {
		t.Fatalf("trimmed value = %q, want %q", trimmed, want)
	}
	if units := len(utf16.Encode([]rune(trimmed))); units > 5 {
		t.Fatalf("trimmed value has %d UTF-16 units, limit is 5", units)
	}
}
