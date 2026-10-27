package ui

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/arfush/yoanime/internal/kinopoisk"
	"github.com/arfush/yoanime/internal/library"
)

type titleStatus int

const (
	statusCompleted titleStatus = iota
	statusOngoing
	statusAnnounced
)

type voiceover struct {
	name     string
	episodes []string
	missing  []string
	hasKodik bool
}

func (v voiceover) complete() bool {
	return len(v.missing) == 0
}

// titleSeason groups the episodes of a Kinopoisk series. AnimeLIB titles keep
// their flat episode list and have no seasons at all, so the season strip is
// only drawn when a title carries more than one season.
type titleSeason struct {
	number   int
	episodes []string
}

type historyTitle struct {
	id              int64
	animeID         int64
	source          string
	externalID      string
	slug            string
	title           string
	original        string
	status          titleStatus
	userStatus      library.UserStatus
	statusOrder     int64
	movie           bool // hides the episode grid: a film has no series to pick from.
	poster1         color.NRGBA
	poster2         color.NRGBA
	initial         string
	coverURL        string
	coverPath       string
	coverNeedsRetry bool
	episodes        []string
	seasons         []titleSeason
	// episodesTotal is the cached count used by history rows, where the episode
	// list itself is not loaded.
	episodesTotal int
	// episodeLabel is the saved episode number for a history row that has no
	// episode list in memory.
	episodeLabel string
	voices       []voiceover
	imdbID       string
	season       int
	resumeAt     float64
	duration     float64

	selectedVoice   int
	selectedEpisode int
}

func (t historyTitle) episodeCount() int {
	return len(t.episodes)
}

// totalEpisodes prefers the freshly loaded list and falls back to the cached
// count so history rows can draw the progress bar without opening the title.
func (t historyTitle) totalEpisodes() int {
	if len(t.episodes) > 0 {
		return len(t.episodes)
	}
	return t.episodesTotal
}

// currentEpisodeLabel returns the episode the progress bar refers to.
func (t historyTitle) currentEpisodeLabel() string {
	if t.selectedEpisode >= 0 && t.selectedEpisode < len(t.episodes) {
		return t.episodes[t.selectedEpisode]
	}
	return t.episodeLabel
}

// progressLabel is the "current / total эп." caption next to the progress bar.
// A film has a single playable file, so the episode counter would be noise.
func (t historyTitle) progressLabel() string {
	if t.movie {
		return ""
	}
	return episodeProgressLabel(t.currentEpisodeLabel(), t.totalEpisodes())
}

// episodesSectionLabel names the episode block. A series with several seasons
// is labelled by its season range ("СЕЗОНЫ (1-3)"); everything else keeps the
// episode range it already had.
func (t historyTitle) episodesSectionLabel() string {
	if len(t.seasons) > 1 {
		first := t.seasons[0].number
		last := t.seasons[len(t.seasons)-1].number
		if first == last {
			return fmt.Sprintf("СЕЗОНЫ (%d)", first)
		}
		return fmt.Sprintf("СЕЗОНЫ (%d-%d)", first, last)
	}
	return episodeRangeLabel(t.episodes)
}

// seasonIndexForEpisode finds the season that contains the episode at index. It
// returns 0 when the title has no seasons.
func (t historyTitle) seasonIndexForEpisode(index int) int {
	if len(t.seasons) == 0 || index < 0 || index >= len(t.episodes) {
		return 0
	}
	key := t.episodes[index]
	for seasonIndex, season := range t.seasons {
		for _, episode := range season.episodes {
			if episode == key {
				return seasonIndex
			}
		}
	}
	return 0
}

// titleFromLibrary converts the API/storage model into the compact model used
// by the existing widgets.
func titleFromLibrary(item library.Title) historyTitle {
	title := historyTitle{
		id:              item.ID,
		animeID:         item.AnimeID,
		source:          item.Source,
		externalID:      item.ExternalID,
		slug:            item.Slug,
		title:           item.Russian,
		original:        item.Original,
		status:          titleStatus(item.Status),
		userStatus:      library.NormalizeUserStatus(item.UserStatus),
		statusOrder:     item.StatusOrder,
		movie:           item.IsMovie,
		coverURL:        item.CoverURL,
		coverPath:       item.CoverPath,
		coverNeedsRetry: item.CoverNeedsRetry,
		episodes:        append([]string(nil), item.Episodes...),
		episodesTotal:   item.EpisodesTotal,
		episodeLabel:    item.SelectedEpisode,
		voices:          make([]voiceover, 0, len(item.Voiceovers)),
		imdbID:          item.IMDbID,
		season:          item.Season,
		resumeAt:        item.ResumeAt,
		duration:        item.Duration,
	}
	title.poster1, title.poster2 = generatedPosterColors(item.Slug + "|" + item.Russian)
	title.initial = posterInitial(item.Russian)
	for _, season := range item.Seasons {
		title.seasons = append(title.seasons, titleSeason{
			number:   season.Number,
			episodes: append([]string(nil), season.Episodes...),
		})
	}
	for _, voice := range item.Voiceovers {
		title.voices = append(title.voices, voiceover{
			name:     voice.Name,
			episodes: append([]string(nil), voice.Episodes...),
			missing:  append([]string(nil), voice.Missing...),
			hasKodik: voice.HasKodik,
		})
	}
	title.selectedVoice = voiceIndex(title, item.SelectedVoice)
	title.selectedEpisode = episodeIndex(title, item.SelectedEpisode)
	if title.selectedEpisode < 0 {
		title.selectedEpisode = firstAvailableEpisode(title, title.selectedVoice)
	} else if title.episodeMissing(title.selectedVoice, title.episodes[title.selectedEpisode]) {
		// A saved episode can be absent from the currently selected voice. Move
		// forward only, matching playback: never silently switch to an earlier
		// episode.
		if next := nextAvailableEpisode(title, title.selectedVoice, title.selectedEpisode); next >= 0 {
			title.selectedEpisode = next
		}
	}
	return title
}

// libraryTitleFromHistory converts the compact UI model back into the library
// model so the title window can persist it when Save is pressed.
func libraryTitleFromHistory(item historyTitle) library.Title {
	title := library.Title{
		ID:            item.id,
		AnimeID:       item.animeID,
		Source:        item.source,
		ExternalID:    item.externalID,
		Slug:          item.slug,
		Russian:       item.title,
		Original:      item.original,
		Status:        library.Status(item.status),
		UserStatus:    library.NormalizeUserStatus(item.userStatus),
		StatusOrder:   item.statusOrder,
		IsMovie:       item.movie,
		CoverURL:      item.coverURL,
		CoverPath:     item.coverPath,
		Episodes:      append([]string(nil), item.episodes...),
		EpisodesTotal: item.totalEpisodes(),
		IMDbID:        item.imdbID,
		Season:        item.season,
		ResumeAt:      item.resumeAt,
		Duration:      item.duration,
		Voiceovers:    make([]library.Voiceover, 0, len(item.voices)),
	}
	for _, season := range item.seasons {
		title.Seasons = append(title.Seasons, library.Season{
			Number:   season.number,
			Episodes: append([]string(nil), season.episodes...),
		})
	}
	for _, voice := range item.voices {
		title.Voiceovers = append(title.Voiceovers, library.Voiceover{
			Name:     voice.name,
			Episodes: append([]string(nil), voice.episodes...),
			Missing:  append([]string(nil), voice.missing...),
			HasKodik: voice.hasKodik,
		})
	}
	return title
}

func voiceIndex(title historyTitle, name string) int {
	for index, voice := range title.voices {
		if voice.name == name {
			return index
		}
	}
	return 0
}

func episodeIndex(title historyTitle, episode string) int {
	for index, candidate := range title.episodes {
		if candidate == episode {
			return index
		}
	}
	return -1
}

// nextAvailableEpisode finds the first episode after from that the selected
// voice actually has. It never looks backwards.
func nextAvailableEpisode(title historyTitle, voice, from int) int {
	for index := from + 1; index < len(title.episodes); index++ {
		if !title.episodeMissing(voice, title.episodes[index]) {
			return index
		}
	}
	return -1
}

func firstAvailableEpisode(title historyTitle, voice int) int {
	if len(title.episodes) == 0 {
		return -1
	}
	if voice < 0 || voice >= len(title.voices) {
		return 0
	}
	for index, episode := range title.episodes {
		if !episodeMissingForVoice(title.voices[voice], episode) {
			return index
		}
	}
	return 0
}

func (t historyTitle) episodeMissing(voiceIndex int, episode string) bool {
	if voiceIndex < 0 || voiceIndex >= len(t.voices) {
		return false
	}
	return episodeMissingForVoice(t.voices[voiceIndex], episode)
}

func episodeMissingForVoice(voice voiceover, episode string) bool {
	for _, missing := range voice.missing {
		if missing == episode {
			return true
		}
	}
	return false
}

func generatedPosterColors(seed string) (color.NRGBA, color.NRGBA) {
	palette := []struct {
		first  color.NRGBA
		second color.NRGBA
	}{
		{color.NRGBA{R: 0x3b, G: 0x6b, B: 0xa8, A: 0xff}, color.NRGBA{R: 0x18, G: 0x2b, B: 0x4c, A: 0xff}},
		{color.NRGBA{R: 0x75, G: 0x3f, B: 0xa8, A: 0xff}, color.NRGBA{R: 0x2c, G: 0x1c, B: 0x4d, A: 0xff}},
		{color.NRGBA{R: 0xb1, G: 0x5b, B: 0x4d, A: 0xff}, color.NRGBA{R: 0x4a, G: 0x20, B: 0x2d, A: 0xff}},
		{color.NRGBA{R: 0x46, G: 0x73, B: 0x77, A: 0xff}, color.NRGBA{R: 0x1c, G: 0x31, B: 0x3b, A: 0xff}},
		{color.NRGBA{R: 0x8d, G: 0x3c, B: 0x58, A: 0xff}, color.NRGBA{R: 0x36, G: 0x1d, B: 0x34, A: 0xff}},
		{color.NRGBA{R: 0x5f, G: 0x5d, B: 0x52, A: 0xff}, color.NRGBA{R: 0x29, G: 0x28, B: 0x24, A: 0xff}},
		{color.NRGBA{R: 0x2f, G: 0x79, B: 0x9f, A: 0xff}, color.NRGBA{R: 0x16, G: 0x34, B: 0x50, A: 0xff}},
		{color.NRGBA{R: 0x9a, G: 0x55, B: 0x3f, A: 0xff}, color.NRGBA{R: 0x3f, G: 0x22, B: 0x20, A: 0xff}},
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(seed))
	entry := palette[int(hash.Sum32())%len(palette)]
	return entry.first, entry.second
}

func posterInitial(title string) string {
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return "?"
}

// episodeDisplayLabel strips the season prefix from a Kinopoisk episode key, so
// the grid shows the plain episode number inside its season.
func episodeDisplayLabel(key string) string {
	return kinopoisk.EpisodeLabel(key)
}

func episodeRangeLabel(episodes []string) string {
	if len(episodes) == 0 {
		return "СЕРИИ"
	}
	numbers := make([]int, 0, len(episodes))
	for _, episode := range episodes {
		number, err := strconv.Atoi(episodeDisplayLabel(episode))
		if err != nil {
			numbers = nil
			break
		}
		numbers = append(numbers, number)
	}
	if len(numbers) == len(episodes) && len(numbers) > 0 && numbers[0] == 1 {
		contiguous := true
		for index, number := range numbers {
			if number != index+1 {
				contiguous = false
				break
			}
		}
		if contiguous {
			return fmt.Sprintf("СЕРИИ (1-%d)", numbers[len(numbers)-1])
		}
	}
	return fmt.Sprintf("СЕРИИ (%d)", len(episodes))
}

func voiceMissingLabel(voice voiceover) string {
	if len(voice.missing) == 0 {
		return ""
	}
	episodes := uniqueSortedEpisodes(voice.missing)
	parts := make([]string, 0, len(episodes))
	for index := 0; index < len(episodes); {
		start, startErr := strconv.Atoi(episodes[index])
		if startErr != nil {
			parts = append(parts, episodes[index])
			index++
			continue
		}
		end := start
		next := index + 1
		for next < len(episodes) {
			number, err := strconv.Atoi(episodes[next])
			if err != nil || number != end+1 {
				break
			}
			end = number
			next++
		}
		if end > start {
			parts = append(parts, fmt.Sprintf("%d-%d", start, end))
		} else {
			parts = append(parts, strconv.Itoa(start))
		}
		index = next
	}
	return "Нет серий: " + strings.Join(parts, ", ")
}

func uniqueSortedEpisodes(episodes []string) []string {
	seen := make(map[string]bool, len(episodes))
	out := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		if episode == "" || seen[episode] {
			continue
		}
		seen[episode] = true
		out = append(out, episode)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, leftErr := strconv.ParseFloat(out[i], 64)
		right, rightErr := strconv.ParseFloat(out[j], 64)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		if leftErr == nil {
			return true
		}
		if rightErr == nil {
			return false
		}
		return out[i] < out[j]
	})
	return out
}
