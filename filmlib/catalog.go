package filmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/arfush/yo/log"
)

// Voice is one translation the mirror carries.
type Voice struct {
	ID      int
	Name    string
	Quality string
	Iframe  string
}

// Season is one season of a series with its episode numbers in order.
type Season struct {
	Number   int
	Episodes []int
}

// Catalog is the season/episode matrix of one title plus the episode keys every
// translation provides.
type Catalog struct {
	Movie   bool
	Seasons []Season
	// Episodes is the flat, display-ordered list of episode keys. Keys come
	// from EpisodeKey: "1" for a film, "S2E5" for a series episode.
	Episodes []string
	// Available maps a translation id to the episode keys it carries. A missing
	// key means the voice genuinely does not have that episode.
	Available map[int][]string
}

// Voices returns the Alloha translations of a Filmlib title, in the order the
// mirror lists them. Other players in the response are ignored: only Alloha
// exposes the season/episode catalogue this package can resolve.
func (c *Client) Voices(ctx context.Context, id int) ([]Voice, error) {
	if id <= 0 {
		return nil, ErrInvalidID
	}
	body, endpoint, err := c.fetchPlayers(ctx, id)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			Type         string `json:"type"`
			Translations []struct {
				ID      any    `json:"id"`
				Name    string `json:"name"`
				Quality string `json:"quality"`
				Iframe  string `json:"iframeUrl"`
			} `json:"translations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid player response: %w", err)
	}
	var voices []Voice
	seen := make(map[int]bool)
	for _, provider := range payload.Data {
		if !strings.EqualFold(strings.TrimSpace(provider.Type), "Alloha") {
			continue
		}
		for _, translation := range provider.Translations {
			voiceID := asInt(translation.ID)
			iframe := strings.TrimSpace(translation.Iframe)
			if voiceID <= 0 || iframe == "" || seen[voiceID] {
				continue
			}
			seen[voiceID] = true
			voices = append(voices, Voice{
				ID:      voiceID,
				Name:    cleanName(translation.Name),
				Quality: cleanName(translation.Quality),
				Iframe:  iframe,
			})
		}
	}
	if len(voices) == 0 {
		return nil, ErrNotServed
	}
	logger.Debug("filmlib voices loaded", "title", id, "voices", len(voices), "endpoint", endpoint)
	return voices, nil
}

// fetchPlayers tries the configured endpoint and the historical mirror hosts.
func (c *Client) fetchPlayers(ctx context.Context, id int) ([]byte, string, error) {
	urls := c.playersURLs()
	var lastErr error
	for _, endpoint := range urls {
		body, err := c.get(ctx, endpoint+"?kinopoisk="+strconv.Itoa(id), c.config.referer())
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, endpoint, ctx.Err()
			}
			continue
		}
		c.mu.Lock()
		c.players = endpoint
		c.mu.Unlock()
		return body, endpoint, nil
	}
	if lastErr == nil {
		lastErr = ErrNoMirrors
	}
	return nil, "", fmt.Errorf("player catalogue is unavailable: %w", lastErr)
}

// playersURLs returns the configured endpoint first, then the endpoints derived
// from the known wrapper hosts, without duplicates.
func (c *Client) playersURLs() []string {
	c.mu.Lock()
	configured := c.players
	c.mu.Unlock()
	out := make([]string, 0, len(wrapperCandidates)+1)
	seen := make(map[string]bool)
	add := func(value string) {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	add(configured)
	add(c.config.Players)
	for _, wrapper := range wrapperCandidates {
		add(playersFromWrapper(wrapper))
	}
	return out
}

// Catalog reads the complete season/episode matrix from the theatre page that
// belongs to one of the translations. The page embeds the matrix as a JSON
// literal; the browser is not needed, only the page's referer.
func (c *Client) Catalog(ctx context.Context, voice Voice) (Catalog, error) {
	iframe := strings.TrimSpace(voice.Iframe)
	if iframe == "" {
		return Catalog{}, ErrNoPlayerLink
	}
	body, err := c.get(ctx, iframe, c.config.referer())
	if err != nil {
		return Catalog{}, fmt.Errorf("cannot open the player: %w", err)
	}
	raw, err := extractFileList(body)
	if err != nil {
		return Catalog{}, err
	}
	catalog, err := parseFileList(raw)
	if err != nil {
		return Catalog{}, err
	}
	logger.Debug("filmlib catalog parsed", "voice", voice.ID, "movie", catalog.Movie, "seasons", len(catalog.Seasons), "episodes", len(catalog.Episodes))
	return catalog, nil
}

// get performs one HTTP GET with the mirror referer.
func (c *Client) get(ctx context.Context, rawURL, referer string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "*/*")
	if referer != "" {
		request.Header.Set("Referer", referer)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", response.StatusCode)
	}
	return body, nil
}

// extractFileList pulls the JSON payload out of the theatre page.
func extractFileList(html []byte) ([]byte, error) {
	text := string(html)
	const marker = "fileList = JSON.parse("
	index := strings.Index(text, marker)
	if index < 0 {
		return nil, ErrNoEpisodeList
	}
	rest := text[index+len(marker):]
	quote := byte('\'')
	if len(rest) == 0 {
		return nil, ErrEmptyEpisodeList
	}
	if rest[0] == '"' || rest[0] == '\'' {
		quote = rest[0]
		rest = rest[1:]
	}
	end := strings.IndexByte(rest, quote)
	if end < 0 {
		return nil, ErrBadEpisodeList
	}
	payload := unescapeJSString(rest[:end])
	if !json.Valid([]byte(payload)) {
		return nil, ErrCorruptedEpisodeList
	}
	return []byte(payload), nil
}

// unescapeJSString decodes the JavaScript string literal the payload is wrapped
// in. JSON.parse in the browser receives the decoded text, so decoding here
// produces the same JSON document.
func unescapeJSString(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char != '\\' || index+1 >= len(value) {
			out.WriteByte(char)
			continue
		}
		index++
		switch value[index] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'u':
			if index+4 < len(value) {
				if code, err := strconv.ParseUint(value[index+1:index+5], 16, 32); err == nil {
					out.WriteRune(rune(code))
					index += 4
					continue
				}
			}
			out.WriteByte('u')
		case 'x':
			if index+2 < len(value) {
				if code, err := strconv.ParseUint(value[index+1:index+3], 16, 8); err == nil {
					out.WriteByte(byte(code))
					index += 2
					continue
				}
			}
			out.WriteByte('x')
		default:
			out.WriteByte(value[index])
		}
	}
	return out.String()
}

type fileEntry struct {
	ID            int    `json:"id"`
	Season        int    `json:"seasons"`
	Episode       int    `json:"episode"`
	TranslationID int    `json:"id_translation"`
	Quality       string `json:"quality"`
}

func parseFileList(raw []byte) (Catalog, error) {
	var list struct {
		Type string          `json:"type"`
		All  json.RawMessage `json:"all"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return Catalog{}, fmt.Errorf("invalid episode list: %w", err)
	}
	if strings.EqualFold(list.Type, "movie") {
		return parseMovieList(list.All)
	}
	return parseSerialList(list.All)
}

// parseSerialList reads {"1":{"30":{"t82":{...}}}} — season, episode, voice.
func parseSerialList(raw json.RawMessage) (Catalog, error) {
	var seasons map[string]map[string]map[string]fileEntry
	if err := json.Unmarshal(raw, &seasons); err != nil {
		return Catalog{}, fmt.Errorf("invalid seasons: %w", err)
	}
	catalog := Catalog{Available: make(map[int][]string)}
	numbers := make([]int, 0, len(seasons))
	byNumber := make(map[int]map[string]map[string]fileEntry, len(seasons))
	for key, episodes := range seasons {
		number, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil {
			continue
		}
		numbers = append(numbers, number)
		byNumber[number] = episodes
	}
	sort.Ints(numbers)
	for _, seasonNumber := range numbers {
		episodes := byNumber[seasonNumber]
		episodeNumbers := make([]int, 0, len(episodes))
		byEpisode := make(map[int]map[string]fileEntry, len(episodes))
		for key, voices := range episodes {
			number, err := strconv.Atoi(strings.TrimSpace(key))
			if err != nil {
				continue
			}
			episodeNumbers = append(episodeNumbers, number)
			byEpisode[number] = voices
		}
		sort.Ints(episodeNumbers)
		if len(episodeNumbers) == 0 {
			continue
		}
		catalog.Seasons = append(catalog.Seasons, Season{Number: seasonNumber, Episodes: episodeNumbers})
		for _, episodeNumber := range episodeNumbers {
			key := EpisodeKey(seasonNumber, episodeNumber)
			catalog.Episodes = append(catalog.Episodes, key)
			for voiceKey := range byEpisode[episodeNumber] {
				voiceID := parseVoiceKey(voiceKey)
				if voiceID <= 0 {
					continue
				}
				catalog.Available[voiceID] = append(catalog.Available[voiceID], key)
			}
		}
	}
	if len(catalog.Episodes) == 0 {
		return Catalog{}, ErrNoEpisodes
	}
	sortVoices(catalog.Available, catalog.Episodes)
	return catalog, nil
}

// parseMovieList reads {"theatrical":{"t62":{"BDRip":{...}}}}. Films carry one
// file for every translation, so the film itself is a single episode and the
// voices differ only by the audio track the theatre selects.
func parseMovieList(raw json.RawMessage) (Catalog, error) {
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sections); err != nil {
		return Catalog{}, fmt.Errorf("invalid file list: %w", err)
	}
	catalog := Catalog{Movie: true, Available: make(map[int][]string)}
	for _, section := range []string{"theatrical", "directors"} {
		payload, ok := sections[section]
		if !ok || len(payload) == 0 || string(payload) == "null" || string(payload) == "[]" {
			continue
		}
		var voices map[string]map[string]fileEntry
		if err := json.Unmarshal(payload, &voices); err != nil {
			continue
		}
		for voiceKey := range voices {
			voiceID := parseVoiceKey(voiceKey)
			if voiceID <= 0 {
				continue
			}
			catalog.Available[voiceID] = append(catalog.Available[voiceID], "1")
		}
		break
	}
	if len(catalog.Available) == 0 {
		return Catalog{}, ErrNoMovieFile
	}
	catalog.Seasons = []Season{{Number: 0, Episodes: []int{1}}}
	catalog.Episodes = []string{"1"}
	return catalog, nil
}

// parseVoiceKey converts a "t82" file key into the translation id.
func parseVoiceKey(key string) int {
	key = strings.TrimSpace(key)
	if len(key) < 2 || (key[0] != 't' && key[0] != 'T') {
		return 0
	}
	id, err := strconv.Atoi(key[1:])
	if err != nil {
		return 0
	}
	return id
}

// sortVoices orders every voice by the catalogue episode order and drops keys
// the catalogue does not know.
func sortVoices(voices map[int][]string, order []string) {
	position := make(map[string]int, len(order))
	for index, key := range order {
		position[key] = index
	}
	for id, keys := range voices {
		sorted := make([]string, 0, len(keys))
		seen := make(map[string]bool, len(keys))
		for _, key := range keys {
			if seen[key] {
				continue
			}
			if _, ok := position[key]; !ok {
				continue
			}
			seen[key] = true
			sorted = append(sorted, key)
		}
		sort.SliceStable(sorted, func(i, j int) bool { return position[sorted[i]] < position[sorted[j]] })
		voices[id] = sorted
	}
}

// asInt reads a JSON value that may be a number, a numeric string or null.
func asInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case json.Number:
		number, err := typed.Int64()
		if err == nil {
			return int(number)
		}
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return number
		}
	}
	return 0
}

var logger = log.For(log.SourceFilmlib)
