package library

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arfush/yo/kinopoisk"
)

// KinopoiskPlan is everything the Kinopoisk resolver needs besides the episode
// number. The iframe already carries the translation and the access token the
// theatre page expects.
type KinopoiskPlan struct {
	Iframe        string
	TranslationID int
}

// kinopoiskPlan builds the playlist for a Kinopoisk title. It is deliberately
// separate from the AnimeLIB plan: episode keys are prefixed with their season,
// an episode id does not exist, and the voice list comes from the mirror rather
// than from episode payloads.
func (l *Library) kinopoiskPlan(ctx context.Context, title Title, voiceName, episode string) (*PlaybackPlan, error) {
	id, err := kinopoiskTitleID(title)
	if err != nil {
		return nil, err
	}
	client := l.kinopoisk
	if client == nil {
		client = kinopoisk.NewClient()
	}
	voices, err := client.Voices(ctx, id)
	if err != nil {
		return nil, err
	}

	voice := findVoiceover(title.Voiceovers, voiceName)
	if voice == nil {
		return nil, ErrNoVoiceovers
	}
	apiVoice, ok := matchKinopoiskVoice(voices, voice.Name)
	if !ok {
		if len(voices) == 0 {
			return nil, ErrNotServed
		}
		apiVoice = voices[0]
		logger.Warn("Kinopoisk voice not found, using the first available", "voice", voice.Name, "fallback", apiVoice.Name)
	}

	available := make(map[string]bool, len(voice.Episodes))
	for _, key := range voice.Episodes {
		available[key] = true
	}
	episodes := make([]PlaybackEpisode, 0, len(voice.Episodes))
	ordered := make([]string, 0, len(voice.Episodes))
	for _, key := range title.Episodes {
		if !available[key] {
			continue
		}
		if contains(ordered, key) {
			continue
		}
		ordered = append(ordered, key)
		episodes = append(episodes, PlaybackEpisode{Number: key})
	}
	if len(episodes) == 0 {
		return nil, fmt.Errorf("в выбранной озвучке нет доступных серий")
	}

	selectedIndex := kinopoiskEpisodeIndex(episodes, episode)
	if selectedIndex < 0 {
		selectedIndex = nextKinopoiskIndex(title.Episodes, episodes, episode)
	}
	if selectedIndex < 0 {
		return nil, fmt.Errorf("в выбранной озвучке нет следующей доступной серии")
	}
	chosen := episodes[selectedIndex]

	resumeAt, duration := 0.0, 0.0
	if title.ID != 0 {
		if entry, historyErr := l.store.GetHistory(title.ID); historyErr == nil && entry != nil && entry.Episode == chosen.Number {
			resumeAt, duration = entry.ResumeAt, entry.Duration
			if duration > 0 && resumeAt >= duration-5 {
				resumeAt = 0
			}
			if resumeAt < 5 {
				resumeAt = 0
			}
		}
	}

	logger.Info("Kinopoisk plan ready", "title", title.Slug, "voice", voice.Name, "episode", chosen.Number, "episodes", len(episodes))
	return &PlaybackPlan{
		TitleID:          title.ID,
		Slug:             title.Slug,
		Russian:          title.Russian,
		English:          title.Original,
		Voice:            voice.Name,
		Episode:          chosen.Number,
		EpisodeIndex:     selectedIndex,
		Episodes:         episodes,
		IMDbID:           title.IMDbID,
		ResumeAt:         resumeAt,
		Duration:         duration,
		CompleteOnFinish: kinopoiskPlanCompletes(title, episodes),
		Source:           kinopoisk.Source,
		Kinopoisk: &KinopoiskPlan{
			Iframe:        apiVoice.Iframe,
			TranslationID: apiVoice.ID,
		},
	}, nil
}

// kinopoiskTitleID reads the Kinopoisk id from the title row.
func kinopoiskTitleID(title Title) (int, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(title.ExternalID)); err == nil && id > 0 {
		return id, nil
	}
	if link, ok := kinopoisk.ParseInput(title.Slug); ok {
		return link.ID, nil
	}
	return 0, ErrNoKinopoiskID
}

// matchKinopoiskVoice finds the mirror voice with the same display name.
func matchKinopoiskVoice(voices []kinopoisk.Voice, name string) (kinopoisk.Voice, bool) {
	name = strings.TrimSpace(name)
	for _, voice := range voices {
		if strings.TrimSpace(voice.Name) == name {
			return voice, true
		}
	}
	for _, voice := range voices {
		if strings.EqualFold(strings.TrimSpace(voice.Name), name) {
			return voice, true
		}
	}
	return kinopoisk.Voice{}, false
}

// kinopoiskEpisodeIndex finds the exact episode key in the plan.
func kinopoiskEpisodeIndex(episodes []PlaybackEpisode, key string) int {
	key = strings.TrimSpace(key)
	for index, episode := range episodes {
		if episode.Number == key {
			return index
		}
	}
	return -1
}

// nextKinopoiskIndex walks the displayed episode order forward from the
// selected episode, exactly like the AnimeLIB plan does.
func nextKinopoiskIndex(displayOrder []string, episodes []PlaybackEpisode, selected string) int {
	selected = strings.TrimSpace(selected)
	start := 0
	if selected != "" {
		found := false
		for index, key := range displayOrder {
			if key == selected {
				start = index + 1
				found = true
				break
			}
		}
		if !found {
			return -1
		}
	}
	for _, key := range displayOrder[start:] {
		if index := kinopoiskEpisodeIndex(episodes, key); index >= 0 {
			return index
		}
	}
	return -1
}

// kinopoiskPlanCompletes reports whether the last playable episode is the final
// episode of the title.
func kinopoiskPlanCompletes(title Title, episodes []PlaybackEpisode) bool {
	if title.Status != StatusCompleted || len(episodes) == 0 || len(title.Episodes) == 0 {
		return false
	}
	return episodes[len(episodes)-1].Number == title.Episodes[len(title.Episodes)-1]
}
