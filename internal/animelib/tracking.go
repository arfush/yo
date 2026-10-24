package animelib

import (
	"context"
	"strings"
)

// ReleaseVoiceover is one translation team that has published the latest
// episode. KodikOnly is true when AnimeLIB's own player has no copy and the
// episode can only be watched through Kodik.
type ReleaseVoiceover struct {
	Name      string
	KodikOnly bool
}

// EpisodeRelease is the lightweight answer used by the new-episode checker: the
// title status, its latest episode and the voiceovers that already carry it.
type EpisodeRelease struct {
	TitleID     int64
	Slug        string
	StatusID    int
	StatusLabel string

	Episode    string
	EpisodeID  int
	Voiceovers []ReleaseVoiceover
}

// LatestEpisode resolves a title and asks only for the latest episode's player
// list. Three requests per title are enough to learn whether a new episode is
// available in any voiceover, which voice it belongs to and whether it needs
// Kodik. This deliberately avoids the per-team availability burst used by
// Fetch.
func (c *Client) LatestEpisode(ctx context.Context, input string) (*EpisodeRelease, error) {
	slug, err := ParseInput(input)
	if err != nil {
		return nil, err
	}
	title, err := c.resolve(ctx, slug)
	if err != nil {
		return nil, err
	}
	release := &EpisodeRelease{
		TitleID:     title.ID,
		Slug:        title.SlugURL,
		StatusID:    title.StatusID,
		StatusLabel: title.StatusLabel,
	}

	list, err := c.fetchEpisodeList(ctx, title.SlugURL)
	if err != nil {
		return nil, err
	}
	item, number, ok := latestEpisode(list)
	if !ok {
		return release, nil
	}
	release.Episode = number
	release.EpisodeID = item.ID

	detail, err := c.EpisodeDetail(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	release.Voiceovers = releaseVoiceovers(detail.Players)
	return release, nil
}

// latestEpisode returns the last episode in display order. AnimeLIB's flat list
// is normally ascending, but sorting a copy keeps this correct even when a
// special or a late-added episode arrives out of order.
func latestEpisode(list []EpisodeListItem) (EpisodeListItem, string, bool) {
	numbers := make([]string, 0, len(list))
	lastByNumber := make(map[string]EpisodeListItem, len(list))
	for _, item := range list {
		number := NormalizeEpisodeNumber(item.Number)
		if number == "" || item.ID == 0 {
			continue
		}
		numbers = append(numbers, number)
		lastByNumber[number] = item
	}
	if len(numbers) == 0 {
		return EpisodeListItem{}, "", false
	}
	SortEpisodes(numbers)
	number := numbers[len(numbers)-1]
	return lastByNumber[number], number, true
}

// EpisodeNewer reports whether candidate should replace previous as the last
// seen episode. It uses the same ordering as the title page, so a temporarily
// shorter API list or a removed episode cannot rewind tracking and fire a
// bogus "new episode" notification.
func EpisodeNewer(candidate, previous string) bool {
	candidate = strings.TrimSpace(candidate)
	previous = strings.TrimSpace(previous)
	if candidate == "" || candidate == previous {
		return false
	}
	if previous == "" {
		return true
	}
	values := []string{previous, candidate}
	SortEpisodes(values)
	return values[1] == candidate
}

func releaseVoiceovers(players []EpisodePlayer) []ReleaseVoiceover {
	type availability struct {
		name        string
		hasAnimelib bool
		hasKodik    bool
	}
	byName := make(map[string]*availability)
	order := make([]string, 0, len(players))
	for _, player := range players {
		name := strings.TrimSpace(player.TeamName)
		if name == "" {
			continue
		}
		item := byName[name]
		if item == nil {
			item = &availability{name: name}
			byName[name] = item
			order = append(order, name)
		}
		switch strings.ToLower(strings.TrimSpace(player.Player)) {
		case "animelib":
			item.hasAnimelib = true
		case "kodik":
			item.hasKodik = true
		}
	}

	out := make([]ReleaseVoiceover, 0, len(order))
	for _, name := range order {
		item := byName[name]
		if !item.hasAnimelib && !item.hasKodik {
			continue
		}
		out = append(out, ReleaseVoiceover{
			Name:      item.name,
			KodikOnly: item.hasKodik && !item.hasAnimelib,
		})
	}
	return out
}
