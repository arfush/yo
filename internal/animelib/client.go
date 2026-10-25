// Package animelib talks to the AnimeLIB (cdnlibs) API.
package animelib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	apiBaseURL = "https://api.cdnlibs.org"
	// Animelib's current frontend lives on the v5 subdomain. Use it for the
	// browser-like request headers instead of the old animelib.org origin.
	webOrigin = "https://v5.animelib.org"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/154.0.0.0 Safari/537.36"
)

const (
	// StatusOngoing, StatusCompleted and StatusAnnounced mirror the AnimeLIB
	// status ids. Some responses only carry a label, so both are checked.
	StatusOngoing   = 1
	StatusCompleted = 2
	StatusAnnounced = 3
	// TypeMovie is the AnimeLIB title-type id for films. The label is checked
	// as a fallback because other sites in the network may use different ids.
	TypeMovie = 17
)

var animeURLPattern = regexp.MustCompile(`(?i)/anime/([^/?#]+)`)

// backgroundRequestInterval keeps the new-episode checker well below
// AnimeLIB's 100-requests-per-minute limit while leaving budget for the UI.
const backgroundRequestInterval = time.Second

// Client is a small, concurrency-safe AnimeLIB API client.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
	sem     chan struct{}
	// pace serializes background requests so a large library cannot exhaust
	// the shared rate-limit window in one burst.
	pace        time.Duration
	paceMu      sync.Mutex
	lastRequest time.Time
}

// New returns a client. An empty token is allowed while the token-entry UI is
// not implemented yet; protected requests will then fail with a useful error.
func New(token string) *Client {
	return newClient(token, 0)
}

// NewBackground returns a client that spaces every request at least one second
// apart. It is used by the periodic new-episode checker, which must not starve
// foreground title loads.
func NewBackground(token string) *Client {
	return newClient(token, backgroundRequestInterval)
}

func newClient(token string, pace time.Duration) *Client {
	return &Client{
		token:   strings.TrimSpace(token),
		baseURL: strings.TrimRight(apiBaseURL, "/"),
		http:    &http.Client{Timeout: 25 * time.Second},
		sem:     make(chan struct{}, 10),
		pace:    pace,
	}
}

// Title is the metadata returned by /api/anime/{slug}.
type Title struct {
	ID          int64
	Name        string
	RusName     string
	EngName     string
	Slug        string
	SlugURL     string
	Shikimori   string
	StatusID    int
	StatusLabel string
	TypeID      int
	TypeLabel   string
	Cover       Cover
	Teams       []Team
}

// IsMovie reports whether AnimeLIB classifies the title as a film. Films have
// no episode list to choose from, so the title page only shows the quality.
func (t Title) IsMovie() bool {
	if t.TypeID == TypeMovie {
		return true
	}
	label := strings.ToLower(strings.TrimSpace(t.TypeLabel))
	return strings.Contains(label, "фильм") || strings.Contains(label, "movie")
}

// Team is one translation team attached to a title. Availability is queried
// per team, so the title page does not have to download every episode detail.
type Team struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Cover contains every size AnimeLIB returns for a title poster.
type Cover struct {
	Default   string `json:"default"`
	MD        string `json:"md"`
	Original  string `json:"original"`
	Thumbnail string `json:"thumbnail"`
}

// TitleData is everything the application needs to render a title page.
type TitleData struct {
	Title
	Episodes   []string
	Voiceovers []Voiceover
}

// Voiceover groups the episodes that are available for one translation team.
// HasKodik marks voices with at least one episode that only the Kodik player
// carries: AnimeLIB prefers its own player, so those episodes play via Kodik.
type Voiceover struct {
	Name     string   `json:"name"`
	Episodes []string `json:"episodes"`
	Missing  []string `json:"missing"`
	HasKodik bool     `json:"has_kodik"`
}

type titleResponse struct {
	Data struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		RusName   string `json:"rus_name"`
		EngName   string `json:"eng_name"`
		Slug      string `json:"slug"`
		SlugURL   string `json:"slug_url"`
		Shikimori string `json:"shikimori_href"`
		Status    struct {
			ID    int    `json:"id"`
			Label string `json:"label"`
		} `json:"status"`
		Type struct {
			ID    int    `json:"id"`
			Label string `json:"label"`
		} `json:"type"`
		Cover Cover  `json:"cover"`
		Teams []Team `json:"teams"`
	} `json:"data"`
}

// EpisodeListItem is one row of the title's flat episode list. Season is the
// value AnimeLIB stores for the episode; it is a string in the API response and
// is not always the franchise season number, so callers that need a reliable
// season must cross-check it with AniList.
type EpisodeListItem struct {
	ID     int    `json:"id"`
	Number string `json:"number"`
	Season string `json:"season"`
}

type episodeListResponse struct {
	Data []EpisodeListItem `json:"data"`
}

// ParseInput accepts a raw AnimeLIB slug or a link to a title. The API only
// needs the title slug, so links from any mirror or site work as long as they
// point at a title: /anime/<slug> is preferred, otherwise the last path
// segment is used. Links without a scheme are accepted too.
func ParseInput(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("введите ссылку на тайтл AnimeLIB")
	}
	if raw, ok := asURL(input); ok {
		if slug := slugFromURL(raw); slug != "" {
			return slug, nil
		}
		return "", errors.New("в ссылке нет названия тайтла")
	}
	if strings.ContainsAny(input, " /\\?#") {
		return "", errors.New("введите ссылку AnimeLIB или slug тайтла")
	}
	return input, nil
}

// asURL reports whether input looks like a link and returns it with a scheme.
func asURL(input string) (string, bool) {
	if strings.Contains(input, "://") {
		return input, true
	}
	if strings.ContainsAny(input, " \t\r\n") {
		return "", false
	}
	host := input
	if index := strings.IndexAny(host, "/?#"); index >= 0 {
		host = host[:index]
	}
	// A bare slug never contains a dot, so a dotted first segment marks a host.
	if host == "" || !strings.Contains(host, ".") {
		return "", false
	}
	return "https://" + input, true
}

func slugFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if match := animeURLPattern.FindStringSubmatch(parsed.Path); match != nil {
		return unescapePathSegment(match[1])
	}
	segments := strings.Split(parsed.Path, "/")
	for index := len(segments) - 1; index >= 0; index-- {
		if segment := strings.TrimSpace(segments[index]); segment != "" {
			return unescapePathSegment(segment)
		}
	}
	return ""
}

func unescapePathSegment(segment string) string {
	if unescaped, err := url.PathUnescape(segment); err == nil {
		return unescaped
	}
	return segment
}

// Fetch resolves a title and loads every episode and voiceover for it. The
// complete episode list and the availability of every voice team are fetched
// fresh on every call; nothing is persisted between title-page loads. Announced
// titles do not have episodes, so their episode list is not requested.
func (c *Client) Fetch(ctx context.Context, input string) (*TitleData, error) {
	slug, err := ParseInput(input)
	if err != nil {
		return nil, err
	}
	title, err := c.resolve(ctx, slug)
	if err != nil {
		return nil, err
	}
	if announced(title) {
		return &TitleData{
			Title:      *title,
			Episodes:   []string{},
			Voiceovers: []Voiceover{},
		}, nil
	}

	list, err := c.fetchEpisodeList(ctx, title.SlugURL)
	if err != nil {
		return nil, err
	}

	episodeByID := make(map[int]string, len(list))
	seen := make(map[int]bool, len(list))
	for _, item := range list {
		if item.ID == 0 || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		if number := NormalizeEpisodeNumber(item.Number); number != "" {
			episodeByID[item.ID] = number
		}
	}
	episodes := uniqueEpisodeNumbers(list)

	groups, complete, kodik := c.fetchVoiceAvailability(ctx, title, episodeByID)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(list) > 0 && len(groups) == 0 {
		return nil, errors.New("AnimeLIB не вернул данные ни по одной озвучке")
	}

	return &TitleData{
		Title:      *title,
		Episodes:   episodes,
		Voiceovers: buildVoiceovers(episodes, groups, complete, kodik),
	}, nil
}

// fetchVoiceAvailability asks the API which episode ids exist for every team.
// This is one lightweight request per team and player instead of downloading
// all ~1180 episode detail payloads for a long-running title.
func (c *Client) fetchVoiceAvailability(ctx context.Context, title *Title, episodeByID map[int]string) (map[string]map[string]bool, map[string]bool, map[string]bool) {
	groups := make(map[string]map[string]bool)
	complete := make(map[string]bool)
	kodik := make(map[string]bool)
	if len(title.Teams) == 0 || len(episodeByID) == 0 {
		return groups, complete, kodik
	}

	type result struct {
		name      string
		episodes  map[int]bool
		kodikOnly map[int]bool
		complete  bool
	}
	results := make([]result, len(title.Teams))
	var wg sync.WaitGroup
	for index, team := range title.Teams {
		wg.Add(1)
		go func(index int, team Team) {
			defer wg.Done()
			ids, kodikOnly, ok := c.fetchTeamAvailability(ctx, title.ID, team.ID)
			results[index] = result{
				name:      strings.TrimSpace(team.Name),
				episodes:  ids,
				kodikOnly: kodikOnly,
				complete:  ok,
			}
		}(index, team)
	}
	wg.Wait()

	for _, item := range results {
		if item.name == "" || len(item.episodes) == 0 {
			continue
		}
		available := groups[item.name]
		if available == nil {
			available = make(map[string]bool)
			groups[item.name] = available
		}
		for id := range item.episodes {
			if number, ok := episodeByID[id]; ok {
				available[number] = true
			}
		}
		for id := range item.kodikOnly {
			if _, ok := episodeByID[id]; ok {
				kodik[item.name] = true
				break
			}
		}
		if previous, ok := complete[item.name]; ok {
			complete[item.name] = previous && item.complete
		} else {
			complete[item.name] = item.complete
		}
	}
	return groups, complete, kodik
}

// fetchTeamAvailability merges both players. kodikOnly holds episodes that
// Kodik carries when AnimeLIB's own player does not, because those are the
// episodes that will actually be played through Kodik.
func (c *Client) fetchTeamAvailability(ctx context.Context, animeID, teamID int64) (map[int]bool, map[int]bool, bool) {
	episodes := make(map[int]bool)
	kodikOnly := make(map[int]bool)
	complete := true
	// AnimeLIB's own player and Kodik are both used by the site. Kodik often
	// carries dubs that are absent from the AnimeLIB player.
	animelib, animelibErr := c.fetchPlayerExists(ctx, animeID, teamID, "Animelib")
	animelibOK := animelibErr == nil
	if !animelibOK {
		complete = false
	}
	kodik, kodikErr := c.fetchPlayerExists(ctx, animeID, teamID, "Kodik")
	if kodikErr != nil {
		complete = false
	}
	for id := range animelib {
		episodes[id] = true
	}
	for id := range kodik {
		episodes[id] = true
		// Without a trustworthy AnimeLIB answer every Kodik episode would look
		// Kodik-only, so the badge is only reported for confirmed fallbacks.
		if animelibOK && !animelib[id] {
			kodikOnly[id] = true
		}
	}
	return episodes, kodikOnly, complete
}

type playerExistsResponse struct {
	Data json.RawMessage `json:"data"`
}

func (c *Client) fetchPlayerExists(ctx context.Context, animeID, teamID int64, player string) (map[int]bool, error) {
	query := url.Values{}
	query.Set("anime_id", strconv.FormatInt(animeID, 10))
	query.Set("team_id", strconv.FormatInt(teamID, 10))
	query.Set("player", player)
	query.Set("translation_type", "2")
	endpoint := c.baseURL + "/api/players/exists?" + query.Encode()

	body, err := c.getWithRetry(ctx, endpoint, 1, 3)
	if err != nil {
		return nil, err
	}
	var response playerExistsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(response.Data))
	if raw == "" || raw == "null" || raw == "[]" {
		return map[int]bool{}, nil
	}
	var values map[string]int
	if err := json.Unmarshal(response.Data, &values); err == nil {
		out := make(map[int]bool, len(values))
		for key, value := range values {
			if value == 0 {
				continue
			}
			id, err := strconv.Atoi(key)
			if err != nil {
				continue
			}
			out[id] = true
		}
		return out, nil
	}

	var flags map[string]bool
	if err := json.Unmarshal(response.Data, &flags); err != nil {
		return nil, fmt.Errorf("некорректный ответ доступности озвучек: %w", err)
	}
	out := make(map[int]bool, len(flags))
	for key, value := range flags {
		if !value {
			continue
		}
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		out[id] = true
	}
	return out, nil
}

func (c *Client) getWithRetry(ctx context.Context, endpoint string, siteID, attempts int) ([]byte, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, err := c.get(ctx, endpoint, siteID)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryableAPIError(err) {
			break
		}
		select {
		case <-time.After(time.Duration(1<<attempt) * 500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func retryableAPIError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"http 403", "http 429", "http 500", "http 502", "http 503", "http 504",
		"timeout", "deadline exceeded", "connection reset", "connection refused", "eof",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func announced(title *Title) bool {
	if title.StatusID == StatusAnnounced {
		return true
	}
	label := strings.ToLower(strings.TrimSpace(title.StatusLabel))
	return strings.Contains(label, "анонс") ||
		strings.Contains(label, "запланир") ||
		strings.Contains(label, "ожида")
}

func buildVoiceovers(episodes []string, groups map[string]map[string]bool, complete, kodik map[string]bool) []Voiceover {
	voices := make([]Voiceover, 0, len(groups))
	for name, available := range groups {
		missing := make([]string, 0, len(episodes))
		if complete == nil || complete[name] {
			for _, episode := range episodes {
				if !available[episode] {
					missing = append(missing, episode)
				}
			}
		}
		voices = append(voices, Voiceover{
			Name:     name,
			Episodes: sortedEpisodes(available),
			Missing:  missing,
			HasKodik: kodik[name],
		})
	}
	sort.SliceStable(voices, func(i, j int) bool {
		if len(voices[i].Episodes) != len(voices[j].Episodes) {
			return len(voices[i].Episodes) > len(voices[j].Episodes)
		}
		return strings.ToLower(voices[i].Name) < strings.ToLower(voices[j].Name)
	})
	return voices
}

func (c *Client) resolve(ctx context.Context, rawSlug string) (*Title, error) {
	variants := []string{rawSlug}
	if strings.HasSuffix(rawSlug, "-anime") {
		variants = append(variants, strings.TrimSuffix(rawSlug, "-anime"))
	} else {
		variants = append(variants, rawSlug+"-anime")
	}

	var failures []string
	for _, variant := range variants {
		for _, siteID := range []int{1, 4, 5} {
			endpoint := c.baseURL + "/api/anime/" + url.PathEscape(variant) + "?fields%5B0%5D=teams"
			body, err := c.getWithRetry(ctx, endpoint, siteID, 2)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s (site %d): %v", variant, siteID, err))
				continue
			}
			var response titleResponse
			if err := json.Unmarshal(body, &response); err != nil {
				failures = append(failures, fmt.Sprintf("%s (site %d): %v", variant, siteID, err))
				continue
			}
			if response.Data.SlugURL == "" {
				failures = append(failures, fmt.Sprintf("%s (site %d): тайтл не найден", variant, siteID))
				continue
			}
			data := response.Data
			return &Title{
				ID:          data.ID,
				Name:        data.Name,
				RusName:     data.RusName,
				EngName:     data.EngName,
				Slug:        data.Slug,
				SlugURL:     data.SlugURL,
				Shikimori:   data.Shikimori,
				StatusID:    data.Status.ID,
				StatusLabel: data.Status.Label,
				TypeID:      data.Type.ID,
				TypeLabel:   data.Type.Label,
				Cover:       data.Cover,
				Teams:       data.Teams,
			}, nil
		}
	}
	if len(failures) > 0 {
		return nil, fmt.Errorf("не удалось найти тайтл: %s", failures[len(failures)-1])
	}
	return nil, errors.New("не удалось найти тайтл")
}

func (c *Client) fetchEpisodeList(ctx context.Context, slugURL string) ([]EpisodeListItem, error) {
	endpoint := c.baseURL + "/api/episodes?anime_id=" + url.QueryEscape(slugURL)
	body, err := c.get(ctx, endpoint, 1)
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить список серий: %w", err)
	}
	var response episodeListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("некорректный ответ списка серий: %w", err)
	}
	return response.Data, nil
}

// waitPace pauses background checks so consecutive requests stay at least one
// pace interval apart. The mutex makes the delay hold even if a caller starts
// several lookups at once.
func (c *Client) waitPace(ctx context.Context) error {
	if c.pace <= 0 {
		return nil
	}
	c.paceMu.Lock()
	defer c.paceMu.Unlock()

	if !c.lastRequest.IsZero() {
		wait := c.pace - time.Since(c.lastRequest)
		if wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	c.lastRequest = time.Now()
	return nil
}

func (c *Client) get(ctx context.Context, endpoint string, siteID int) ([]byte, error) {
	if c.token == "" {
		return nil, errors.New("не найден токен AnimeLIB (animelib_token.txt в папке данных Yonime)")
	}
	for {
		// A previous 429 parks every request (including parallel voiceover
		// checks) until the server-side limit has had time to reset. The wait
		// happens before the semaphore slot is taken, so paused requests do not
		// block the queue.
		if err := waitForRateLimit(ctx); err != nil {
			return nil, err
		}
		if err := c.waitPace(ctx); err != nil {
			return nil, err
		}
		select {
		case c.sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		sentAt := time.Now()
		noteRequestSent(sentAt)
		body, status, retryAfter, err := c.do(ctx, endpoint, siteID)
		<-c.sem
		if err != nil {
			return nil, err
		}
		switch {
		case status == http.StatusTooManyRequests:
			// A request that hit the limit is never dropped: pause and send it
			// again. Every queued request is therefore retried with a delay
			// until AnimeLIB answers without a rate limit.
			noteRateLimit(sentAt, parseRetryAfter(retryAfter))
			continue
		case status >= http.StatusOK && status < http.StatusMultipleChoices:
			noteRateLimitSuccess(sentAt)
			return body, nil
		default:
			snippet := strings.TrimSpace(string(body))
			if len(snippet) > 240 {
				snippet = snippet[:240] + "…"
			}
			return nil, fmt.Errorf("HTTP %d: %s", status, snippet)
		}
	}
}

// do performs one HTTP round trip and returns the body together with the status
// and Retry-After header so get can decide whether to pause and retry.
func (c *Client) do(ctx context.Context, endpoint string, siteID int) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", webOrigin+"/")
	req.Header.Set("Origin", webOrigin)
	req.Header.Set("Site-Id", strconv.Itoa(siteID))
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer "+c.token)

	response, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, 0, "", err
	}
	return body, response.StatusCode, response.Header.Get("Retry-After"), nil
}

func uniqueEpisodeNumbers(list []EpisodeListItem) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, item := range list {
		number := NormalizeEpisodeNumber(item.Number)
		if number == "" || seen[number] {
			continue
		}
		seen[number] = true
		out = append(out, number)
	}
	return out
}

// NormalizeEpisodeNumber keeps API numbering stable: numeric strings lose
// leading zeroes, whole floats become integers and text numbers are preserved.
func NormalizeEpisodeNumber(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if number, err := strconv.Atoi(value); err == nil {
		return strconv.Itoa(number)
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		if number == math.Trunc(number) {
			return strconv.Itoa(int(number))
		}
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return value
}

func sortedEpisodes(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for episode := range set {
		out = append(out, episode)
	}
	SortEpisodes(out)
	return out
}

// SortEpisodes orders plain numeric numbers naturally and keeps named
// episodes after them in lexical order.
func SortEpisodes(episodes []string) {
	sort.SliceStable(episodes, func(i, j int) bool {
		left, leftErr := strconv.ParseFloat(episodes[i], 64)
		right, rightErr := strconv.ParseFloat(episodes[j], 64)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		if leftErr == nil {
			return true
		}
		if rightErr == nil {
			return false
		}
		return episodes[i] < episodes[j]
	})
}
