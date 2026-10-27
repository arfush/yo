package library

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/arfush/yo/libsocial"
	"github.com/arfush/yo/metadata"
	"github.com/arfush/yo/storage"
)

// PlaybackEpisode is one episode that can be added to the mpv playlist.
type PlaybackEpisode struct {
	Number string
	ID     int
	Season string
}

// PlaybackPlan is everything the playback service needs before resolving media
// URLs. It deliberately contains only episode ids and display metadata; the
// actual links are fetched lazily for the playlist window.
type PlaybackPlan struct {
	TitleID      int64
	AnimeID      int64
	Slug         string
	Russian      string
	English      string
	Voice        string
	Episode      string
	EpisodeIndex int
	Episodes     []PlaybackEpisode
	Season       int
	IMDbID       string
	ResumeAt     float64
	Duration     float64
	// Source names the catalogue the plan belongs to. Filmlib plans carry the
	// iframe of the selected translation instead of AnimeLIB episode ids.
	Source  string
	Filmlib *FilmlibPlan
	// CompleteOnFinish is true when the last episode in Episodes is the final
	// released episode of a completed title. Playback uses it to move the title
	// to "Completed" only after that episode reaches EOF.
	CompleteOnFinish bool
}

// PlaybackPlan resolves the selected voice, the next available episode and the
// external season/IMDb metadata. When the selected episode is missing from the
// voice it picks the next available episode forward, as requested.
func (l *Library) PlaybackPlan(ctx context.Context, title Title, voiceName, episode string) (*PlaybackPlan, error) {
	if title.IsFilmlib() {
		return l.filmlibPlan(ctx, title, voiceName, episode)
	}
	if strings.TrimSpace(title.Slug) == "" {
		return nil, fmt.Errorf("title has no AnimeLIB link")
	}
	token, err := l.readToken()
	if err != nil {
		return nil, err
	}
	client := libsocial.New(token)

	meta, err := client.ResolveTitle(ctx, title.Slug)
	if err != nil {
		return nil, err
	}
	list, err := client.Episodes(ctx, meta.SlugURL)
	if err != nil {
		return nil, err
	}

	byNumber := make(map[string]libsocial.EpisodeListItem, len(list))
	for _, item := range list {
		number := libsocial.NormalizeEpisodeNumber(item.Number)
		if number == "" {
			continue
		}
		if _, exists := byNumber[number]; !exists {
			byNumber[number] = item
		}
	}

	voice := findVoiceover(title.Voiceovers, voiceName)
	if voice == nil {
		return nil, fmt.Errorf("title has no voiceovers")
	}
	voiceSet := make(map[string]bool)
	for _, number := range voice.Episodes {
		voiceSet[libsocial.NormalizeEpisodeNumber(number)] = true
	}
	if len(voiceSet) == 0 {
		return nil, fmt.Errorf("the selected voiceover has no available episodes")
	}

	orderedNumbers := make([]string, 0, len(voiceSet))
	seen := make(map[string]bool, len(voiceSet))
	// Prefer the title's episode order, which is also the order displayed in
	// the title window. If the local list is incomplete, fall back to the fresh
	// AnimeLIB list.
	for _, number := range title.Episodes {
		number = libsocial.NormalizeEpisodeNumber(number)
		if number == "" || seen[number] || !voiceSet[number] || byNumber[number].ID == 0 {
			continue
		}
		seen[number] = true
		orderedNumbers = append(orderedNumbers, number)
	}
	for _, item := range list {
		number := libsocial.NormalizeEpisodeNumber(item.Number)
		if number == "" || seen[number] || !voiceSet[number] {
			continue
		}
		seen[number] = true
		orderedNumbers = append(orderedNumbers, number)
	}
	if len(orderedNumbers) == 0 {
		return nil, fmt.Errorf("the selected voiceover has no available episodes")
	}

	episodes := make([]PlaybackEpisode, 0, len(orderedNumbers))
	for _, number := range orderedNumbers {
		item := byNumber[number]
		episodes = append(episodes, PlaybackEpisode{
			Number: number,
			ID:     item.ID,
			Season: item.Season,
		})
	}

	selectedIndex := indexOfEpisode(episodes, episode)
	if selectedIndex < 0 {
		selectedIndex = nextAvailableIndex(title.Episodes, episodes, episode)
	}
	if selectedIndex < 0 {
		return nil, fmt.Errorf("the selected voiceover has no next available episode")
	}
	chosen := episodes[selectedIndex]

	malID := metadata.MALID(meta.Shikimori)
	english := strings.TrimSpace(meta.EngName)
	if english == "" {
		english = strings.TrimSpace(meta.Name)
	}
	if english == "" {
		english = title.Original
	}
	russian := strings.TrimSpace(meta.RusName)
	if russian == "" {
		russian = strings.TrimSpace(meta.Name)
	}
	if russian == "" {
		russian = title.Russian
	}

	var (
		season int
		imdbID string
		wg     sync.WaitGroup
	)
	if malID > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			season = metadata.AniListTitle(ctx, malID)
		}()
	}
	imdbID = strings.TrimSpace(title.IMDbID)
	if imdbID == "" && english != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			imdbID = metadata.IMDbID(ctx, english)
		}()
	}
	wg.Wait()

	if season <= 0 {
		season = parseSeason(chosen.Season)
	}
	if season <= 0 {
		season = title.Season
	}

	resumeAt, duration := 0.0, 0.0
	if title.ID != 0 {
		if entry, historyErr := l.storage.GetHistory(title.ID); historyErr == nil && entry != nil && entry.Episode == chosen.Number {
			resumeAt, duration = entry.ResumeAt, entry.Duration
			// Treat the episode as finished if playback stopped at its very end,
			// and ignore a few leading seconds so a title does not reopen there.
			if duration > 0 && resumeAt >= duration-5 {
				resumeAt = 0
			}
			if resumeAt < 5 {
				resumeAt = 0
			}
		}
		// Do not persist display fallbacks: an empty rus_name can fall back to
		// the English name in the plan, but that must not overwrite the stored
		// Russian title.
		storedEnglish := strings.TrimSpace(meta.EngName)
		if storedEnglish == "" {
			storedEnglish = strings.TrimSpace(meta.Name)
		}
		storedRussian := strings.TrimSpace(meta.RusName)
		if storedEnglish != "" || storedRussian != "" {
			_ = l.storage.UpdateNames(title.ID, storedEnglish, storedRussian)
		}
		if imdbID != "" || season > 0 {
			_ = l.storage.UpdateMetadata(title.ID, imdbID, season)
		}
	}

	voiceLabel := voiceName
	if voice != nil {
		voiceLabel = voice.Name
	}
	return &PlaybackPlan{
		TitleID:          title.ID,
		AnimeID:          title.AnimeID,
		Slug:             meta.SlugURL,
		Russian:          russian,
		English:          english,
		Voice:            voiceLabel,
		Episode:          chosen.Number,
		EpisodeIndex:     selectedIndex,
		Episodes:         episodes,
		Season:           season,
		IMDbID:           imdbID,
		ResumeAt:         resumeAt,
		Duration:         duration,
		CompleteOnFinish: planCompletesTitle(title, episodes),
		Source:           storage.SourceLibSocial,
	}, nil
}

// planCompletesTitle reports whether the last playable episode is the final
// released episode. An ongoing title must stay "Watching" even after the user
// catches up with every episode that exists today, and a voiceover missing the
// finale must not complete the title early.
func planCompletesTitle(title Title, episodes []PlaybackEpisode) bool {
	if title.Status != StatusCompleted || len(episodes) == 0 {
		return false
	}
	if title.IsMovie {
		return true
	}
	if len(title.Episodes) == 0 {
		return title.EpisodesTotal > 0 && len(episodes) == title.EpisodesTotal
	}
	last := libsocial.NormalizeEpisodeNumber(title.Episodes[len(title.Episodes)-1])
	return last != "" && episodes[len(episodes)-1].Number == last
}

func findVoiceover(voices []Voiceover, name string) *Voiceover {
	name = strings.TrimSpace(name)
	for index := range voices {
		if voices[index].Name == name {
			return &voices[index]
		}
	}
	for index := range voices {
		if strings.EqualFold(strings.TrimSpace(voices[index].Name), name) {
			return &voices[index]
		}
	}
	if len(voices) > 0 {
		return &voices[0]
	}
	return nil
}

func indexOfEpisode(episodes []PlaybackEpisode, number string) int {
	number = libsocial.NormalizeEpisodeNumber(number)
	for index, episode := range episodes {
		if episode.Number == number {
			return index
		}
	}
	return -1
}

// nextAvailableIndex walks the displayed episode order forward from the
// selected episode. It never falls back to an earlier episode.
func nextAvailableIndex(displayOrder []string, episodes []PlaybackEpisode, selected string) int {
	selected = libsocial.NormalizeEpisodeNumber(selected)
	start := 0
	if selected != "" {
		found := false
		for index, number := range displayOrder {
			if libsocial.NormalizeEpisodeNumber(number) == selected {
				start = index + 1
				found = true
				break
			}
		}
		// An unknown selected episode must not silently restart the title from
		// episode one. It is stale or outside this voice's current range.
		if !found {
			return -1
		}
	}
	for _, number := range displayOrder[start:] {
		if index := indexOfEpisode(episodes, number); index >= 0 {
			return index
		}
	}
	return -1
}

func parseSeason(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number <= 0 {
		return 0
	}
	return number
}
