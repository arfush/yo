package ui

import (
	"fmt"
	"strings"

	"github.com/arfush/yoanime/catalog"
)

// formatEpisodeNotification turns a checker event into the two strings a
// Windows toast shows. The title stays just the series name so long titles do
// not crowd out the sentence that names the episode and voiceover.
func formatEpisodeNotification(event catalog.EpisodeNotification) (string, string) {
	title := event.Title
	if strings.TrimSpace(title) == "" {
		if event.TitleReleased {
			title = "Тайтл вышел"
		} else {
			title = "Новая серия"
		}
	}

	voices, voiceCount, kodik := notificationVoiceovers(event.Voiceovers)
	episode := strings.TrimSpace(event.Episode)
	body := ""
	switch {
	case voiceCount == 0 && episode != "":
		body = fmt.Sprintf("%s эп. уже добавлен!", episode)
	case voiceCount == 0:
		body = "Новая серия уже добавлена!"
	case event.Preferred && episode != "":
		body = fmt.Sprintf("%s эп. в твоей озвучке %s уже добавлен!", episode, voices)
	case event.Preferred:
		body = fmt.Sprintf("Новая серия в твоей озвучке %s уже добавлена!", voices)
	case episode != "" && voiceCount > 1:
		body = fmt.Sprintf("%s эп. в озвучках %s уже добавлен!", episode, voices)
	case episode != "":
		body = fmt.Sprintf("%s эп. в озвучке %s уже добавлен!", episode, voices)
	case voiceCount > 1:
		body = fmt.Sprintf("Новая серия в озвучках %s уже добавлена!", voices)
	default:
		body = fmt.Sprintf("Новая серия в озвучке %s уже добавлена!", voices)
	}
	if event.TitleReleased {
		body = "Тайтл вышел! " + body
	}
	if kodik {
		body += " Доступно в Kodik."
	}

	return trimNotificationText(title, maxNotificationTitle), trimNotificationText(body, maxNotificationBody)
}

// notificationVoiceovers returns the comma-separated team names, their count
// and whether Kodik is needed for any of them. Kodik is mentioned separately
// instead of being glued to the name, so the voiceover list stays readable.
func notificationVoiceovers(voices []catalog.EpisodeVoiceover) (string, int, bool) {
	labels := make([]string, 0, len(voices))
	kodik := false
	for _, voice := range voices {
		name := strings.TrimSpace(voice.Name)
		if name == "" {
			continue
		}
		if voice.KodikOnly {
			kodik = true
		}
		labels = append(labels, name)
	}
	return strings.Join(labels, ", "), len(labels), kodik
}
