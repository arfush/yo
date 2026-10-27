package library

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arfush/yo/filmlib"
)

// FilmlibPlan is everything the Filmlib resolver needs besides the episode
// number. The iframe already carries the translation and the access token the
// theatre page expects.
type FilmlibPlan struct {
	Iframe        string
	TranslationID int
}

// filmlibPlan builds the playlist for a Filmlib title. It is deliberately
// separate from the AnimeLIB plan: episode keys are prefixed with their season,
// an episode id does not exist, and the voice list comes from the mirror rather
// than from episode payloads.
func (l *Library) filmlibPlan(ctx context.Context, title Title, voiceName, episode string) (*PlaybackPlan, error) {
	id, err := filmlibTitleID(title)
	if err != nil {
		return nil, err
	}
	client := l.filmlib
	if client == nil {
		client = filmlib.NewClient()
	}
	voices, err := client.Voices(ctx, id)
	if err != nil {
		return nil, err
	}

	voice := findVoiceover(title.Voiceovers, voiceName)
	if voice == nil {
		return nil, ErrNoVoiceovers
	}
	apiVoice, ok := matchFilmlibVoice(voices, voice.Name)
	if !ok {
		if len(voices) == 0 {
			return nil, ErrNotServed
		}
		apiVoice = voices[0]
		logger.Warn("filmlib voice not found, using the first available", "voice", voice.Name, "fallback", apiVoice.Name)
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
		return nil, fmt.Errorf("the selected voiceover has no available episodes")
	}

	selectedIndex := filmlibEpisodeIndex(episodes, episode)
	if selectedIndex < 0 {
		selectedIndex = nextFilmlibIndex(title.Episodes, episodes, episode)
	}
	if selectedIndex < 0 {
		return nil, fmt.Errorf("the selected voiceover has no next available episode")
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

	logger.Info("filmlib plan ready", "title", title.Slug, "voice", voice.Name, "episode", chosen.Number, "episodes", len(episodes))
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
		CompleteOnFinish: filmlibPlanCompletes(title, episodes),
		Source:           filmlib.Source,
		Filmlib: &FilmlibPlan{
			Iframe:        apiVoice.Iframe,
			TranslationID: apiVoice.ID,
		},
	}, nil
}

// filmlibTitleID reads the Filmlib id from the title row.
func filmlibTitleID(title Title) (int, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(title.ExternalID)); err == nil && id > 0 {
		return id, nil
	}
	if link, ok := filmlib.ParseInput(title.Slug); ok {
		return link.ID, nil
	}
	return 0, ErrNoFilmlibID
}

// matchFilmlibVoice finds the mirror voice with the same display name.
func matchFilmlibVoice(voices []filmlib.Voice, name string) (filmlib.Voice, bool) {
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
	return filmlib.Voice{}, false
}

// filmlibEpisodeIndex finds the exact episode key in the plan.
func filmlibEpisodeIndex(episodes []PlaybackEpisode, key string) int {
	key = strings.TrimSpace(key)
	for index, episode := range episodes {
		if episode.Number == key {
			return index
		}
	}
	return -1
}

// nextFilmlibIndex walks the displayed episode order forward from the
// selected episode, exactly like the AnimeLIB plan does.
func nextFilmlibIndex(displayOrder []string, episodes []PlaybackEpisode, selected string) int {
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
		if index := filmlibEpisodeIndex(episodes, key); index >= 0 {
			return index
		}
	}
	return -1
}

// filmlibPlanCompletes reports whether the last playable episode is the final
// episode of the title.
func filmlibPlanCompletes(title Title, episodes []PlaybackEpisode) bool {
	if title.Status != StatusCompleted || len(episodes) == 0 || len(title.Episodes) == 0 {
		return false
	}
	return episodes[len(episodes)-1].Number == title.Episodes[len(title.Episodes)-1]
}
