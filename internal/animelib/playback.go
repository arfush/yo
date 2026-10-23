package animelib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// EpisodePlayer is one player attached to an episode. Animelib players expose
// direct MP4 links in Video; Kodik players expose an embed link in Src instead.
type EpisodePlayer struct {
	ID              int
	Player          string
	TranslationType int
	TeamID          int64
	TeamName        string
	Src             string
	Qualities       []QualityLink
}

// QualityLink is one quality offered by the Animelib player.
type QualityLink struct {
	Quality int
	Href    string
}

// EpisodeDetail is the data needed to start playback of one episode.
type EpisodeDetail struct {
	ID      int
	Number  string
	Season  string
	Name    string
	Players []EpisodePlayer
}

type episodeDetailResponse struct {
	Data struct {
		ID      int    `json:"id"`
		Number  string `json:"number"`
		Season  string `json:"season"`
		Name    string `json:"name"`
		Players []struct {
			ID              int    `json:"id"`
			Player          string `json:"player"`
			TranslationType struct {
				ID int `json:"id"`
			} `json:"translation_type"`
			Team struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"team"`
			Src   string `json:"src"`
			Video struct {
				Quality []struct {
					// AnimeLIB has returned this both as a JSON string and as
					// a number. Decode both forms.
					Quality any    `json:"quality"`
					Href    string `json:"href"`
				} `json:"quality"`
			} `json:"video"`
		} `json:"players"`
	} `json:"data"`
}

// ResolveTitle fetches only the title metadata. Playback uses it to recover the
// Shikimori link even for titles that were saved before that field was needed.
func (c *Client) ResolveTitle(ctx context.Context, input string) (*Title, error) {
	slug, err := ParseInput(input)
	if err != nil {
		return nil, err
	}
	return c.resolve(ctx, slug)
}

// Episodes returns the title's flat episode list with AnimeLIB episode ids.
func (c *Client) Episodes(ctx context.Context, slugURL string) ([]EpisodeListItem, error) {
	return c.fetchEpisodeList(ctx, slugURL)
}

// EpisodeDetail fetches the players and direct links for one episode.
func (c *Client) EpisodeDetail(ctx context.Context, episodeID int) (*EpisodeDetail, error) {
	if episodeID <= 0 {
		return nil, fmt.Errorf("некорректный id серии")
	}
	endpoint := c.baseURL + "/api/episodes/" + strconv.Itoa(episodeID) + "?"
	body, err := c.getWithRetry(ctx, endpoint, 1, 3)
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить серию: %w", err)
	}
	var response episodeDetailResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("некорректный ответ серии: %w", err)
	}
	data := response.Data
	detail := &EpisodeDetail{
		ID:      data.ID,
		Number:  NormalizeEpisodeNumber(data.Number),
		Season:  strings.TrimSpace(data.Season),
		Name:    data.Name,
		Players: make([]EpisodePlayer, 0, len(data.Players)),
	}
	for _, player := range data.Players {
		// translation_type 2 is a voiceover. Subtitle teams must not be
		// offered as a voiceover in the title window.
		if player.TranslationType.ID == 1 {
			continue
		}
		playerType := strings.TrimSpace(player.Player)
		if playerType == "" {
			continue
		}
		item := EpisodePlayer{
			ID:              player.ID,
			Player:          playerType,
			TranslationType: player.TranslationType.ID,
			TeamID:          player.Team.ID,
			TeamName:        strings.TrimSpace(player.Team.Name),
			Src:             normalizeEmbedURL(player.Src),
			Qualities:       make([]QualityLink, 0, len(player.Video.Quality)),
		}
		for _, quality := range player.Video.Quality {
			number, ok := qualityNumber(quality.Quality)
			if !ok || number <= 0 || strings.TrimSpace(quality.Href) == "" {
				continue
			}
			item.Qualities = append(item.Qualities, QualityLink{
				Quality: number,
				Href:    strings.TrimSpace(quality.Href),
			})
		}
		detail.Players = append(detail.Players, item)
	}
	return detail, nil
}

// VideoBases returns the CDN bases used by Animelib's direct MP4 links.
func (c *Client) VideoBases(ctx context.Context) ([]string, error) {
	endpoint := c.baseURL + "/api/constants?fields%5B%5D=videoServers"
	body, err := c.get(ctx, endpoint, 1)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить видео-серверы: %w", err)
	}
	var response struct {
		Data struct {
			VideoServers []struct {
				URL string `json:"url"`
			} `json:"videoServers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("некорректный ответ видео-серверов: %w", err)
	}
	bases := make([]string, 0, len(response.Data.VideoServers))
	seen := make(map[string]bool)
	for _, server := range response.Data.VideoServers {
		base := strings.TrimSpace(server.URL)
		if base == "" || seen[base] {
			continue
		}
		seen[base] = true
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		bases = append(bases, base)
	}
	return bases, nil
}

// JoinVideoBase joins a CDN base and the relative href returned by the API.
func JoinVideoBase(base, href string) string {
	base = strings.TrimSpace(base)
	href = strings.TrimSpace(href)
	if base == "" || href == "" {
		return ""
	}
	if parsed, err := url.Parse(href); err == nil && parsed.IsAbs() {
		return href
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(href, "/")
}

func qualityNumber(value any) (int, bool) {
	switch typed := value.(type) {
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		return number, err == nil
	case float64:
		return int(typed), typed > 0
	case json.Number:
		number, err := typed.Int64()
		return int(number), err == nil
	default:
		return 0, false
	}
}

func normalizeEmbedURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}
