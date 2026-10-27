// Package metadata fills in the small amount of external metadata yoanime needs
// for playback: IMDb ids for introdb, franchise season numbers and the poster
// shown in the title header from AniList.
package metadata

import (
	"bytes"
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
	"time"

	"github.com/arfush/yo/catalog"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/154.0.0.0 Safari/537.36"

var (
	httpClient = &http.Client{Timeout: 20 * time.Second}
	malRe      = regexp.MustCompile(`(?i)/(?:animes?|anime)/([0-9]+)`)
)

// Chapter is one mpv chapter. Time is in seconds.
type Chapter struct {
	Title string  `json:"title"`
	Time  float64 `json:"time"`
}

// Segment identifies one introdb segment type. The keys are stable because
// they are stored in the settings table. The definition lives in catalog so
// the settings screen does not import the metadata client.
type Segment = catalog.Segment

const (
	SegmentRecap       = catalog.SegmentRecap
	SegmentIntro       = catalog.SegmentIntro
	SegmentOutro       = catalog.SegmentOutro
	SegmentPreview     = catalog.SegmentPreview
	SegmentCredits     = catalog.SegmentCredits
	SegmentPostCredits = catalog.SegmentPostCredits
)

// Segments returns every introdb segment type in display order.
func Segments() []Segment { return catalog.Segments() }

// segmentTitles is the single source of truth for the English chapter labels.
// buildChapters, SegmentFromChapterTitle and out-of-introdb sources such as
// Alloha skip ranges all go through it, so auto-skip keeps recognising chapters.
var segmentTitles = map[Segment]string{
	SegmentRecap:       "Recap",
	SegmentIntro:       "Opening",
	SegmentOutro:       "Ending",
	SegmentPreview:     "Preview",
	SegmentCredits:     "Credits",
	SegmentPostCredits: "Post-credits",
}

// ChapterTitle returns the mpv chapter label of a segment.
func ChapterTitle(segment Segment) string {
	return segmentTitles[segment]
}

// SegmentFromChapterTitle maps an mpv chapter title back to its introdb
// segment. The titles come from buildChapters and stay English because mpv
// shows them above its seek bar.
func SegmentFromChapterTitle(title string) (Segment, bool) {
	for segment, label := range segmentTitles {
		if label == title {
			return segment, true
		}
	}
	return "", false
}

// MALID extracts a MyAnimeList id from a Shikimori link.
func MALID(shikimoriURL string) int {
	match := malRe.FindStringSubmatch(strings.TrimSpace(shikimoriURL))
	if len(match) != 2 {
		return 0
	}
	id, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return id
}

// IMDbID finds the most likely IMDb title id for an English title. It first
// searches the exact title and then a base title, which handles sequels such as
// "Attack on Titan Season 2".
func IMDbID(ctx context.Context, title string) string {
	for _, query := range imdbQueries(title) {
		if id := imdbSearch(ctx, query); id != "" {
			return id
		}
	}
	return ""
}

func imdbQueries(title string) []string {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	add(title)
	add(stripSeasonSuffix(title))
	if index := strings.Index(title, ":"); index > 0 {
		add(title[:index])
	}
	add(stripParenthesized(title))
	return out
}

var seasonSuffixRE = regexp.MustCompile(`(?i)\s*(?:[:\-]\s*)?(?:the\s+)?(?:final\s+)?season\s*[0-9IVX]+(?:\s+part\s*[0-9IVX]+)?(?:\s*:.*)?$`)
var partSuffixRE = regexp.MustCompile(`(?i)\s*(?:[:\-]\s*)?part\s*[0-9IVX]+(?:\s*:.*)?$`)
var ordinalSeasonRE = regexp.MustCompile(`(?i)\s*[:\-]?\s*[0-9]+(?:st|nd|rd|th)\s+season(?:\s*:.*)?$`)

func stripSeasonSuffix(title string) string {
	value := seasonSuffixRE.ReplaceAllString(title, "")
	value = ordinalSeasonRE.ReplaceAllString(value, "")
	value = partSuffixRE.ReplaceAllString(value, "")
	return strings.TrimSpace(strings.Trim(value, ":- "))
}

var parenthesizedRE = regexp.MustCompile(`\s*\([^)]*\)\s*$`)

func stripParenthesized(title string) string {
	return strings.TrimSpace(parenthesizedRE.ReplaceAllString(title, ""))
}

type imdbSuggestion struct {
	D []struct {
		ID string `json:"id"`
		L  string `json:"l"`
		Q  string `json:"q"`
	} `json:"d"`
}

func imdbSearch(ctx context.Context, title string) string {
	if ctx.Err() != nil {
		return ""
	}
	query := strings.ToLower(strings.TrimSpace(title))
	if query == "" {
		return ""
	}
	endpoint := "https://v3.sg.media-imdb.com/suggestion/titles/x/" + url.PathEscape(query) + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	response, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return ""
	}
	var parsed imdbSuggestion
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	for _, item := range parsed.D {
		if strings.HasPrefix(item.ID, "tt") {
			return item.ID
		}
	}
	return ""
}

// AniListTitle returns the franchise season number of a MAL id.
func AniListTitle(ctx context.Context, malID int) int {
	if malID <= 0 {
		return -1
	}
	edges, err := anilistPrequelEdges(ctx, malID)
	if err != nil {
		return -1
	}
	season := 1
	node := &anilistNode{Relations: &anilistRelations{Edges: edges}}
	for depth := 0; depth < 20; depth++ {
		prequel := findPrequelTV(node.Relations.Edges)
		if prequel == nil {
			break
		}
		season++
		node = prequel
	}
	if season < 1 {
		return -1
	}
	return season
}

// SeasonNumber counts TV prequels in AniList's relation chain. It returns -1
// when AniList cannot answer. The query depth matches main.go's reference
// implementation; for most franchises it is enough to distinguish seasons.
func SeasonNumber(ctx context.Context, malID int) int {
	return AniListTitle(ctx, malID)
}

type anilistNode struct {
	IDMal     *int              `json:"idMal"`
	Format    string            `json:"format"`
	Relations *anilistRelations `json:"relations"`
}

type anilistRelations struct {
	Edges []anilistEdge `json:"edges"`
}

type anilistEdge struct {
	RelationType string       `json:"relationType"`
	Node         *anilistNode `json:"node"`
}

func findPrequelTV(edges []anilistEdge) *anilistNode {
	for _, edge := range edges {
		if edge.RelationType == "PREQUEL" && edge.Node != nil && edge.Node.Format == "TV" {
			return edge.Node
		}
	}
	return nil
}

func anilistPrequelEdges(ctx context.Context, malID int) ([]anilistEdge, error) {
	const query = `query ($idMal: Int) { Media(idMal: $idMal, type: ANIME) { relations { edges { relationType node { idMal format relations { edges { relationType node { idMal format relations { edges { relationType node { idMal format } } } } } } } } } } }`
	payload, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]int{"idMal": malID},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graphql.anilist.co", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AniList HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Data struct {
			Media *struct {
				Relations *anilistRelations `json:"relations"`
			} `json:"Media"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Data.Media == nil || parsed.Data.Media.Relations == nil {
		return nil, errors.New("AniList returned no relations")
	}
	return parsed.Data.Media.Relations.Edges, nil
}

// Chapters returns introdb chapters for one episode. The second return value
// reports that introdb has no entry for the episode; callers may cache that.
func Chapters(ctx context.Context, imdbID string, season int, episode string) ([]Chapter, bool, error) {
	episode = strings.TrimSpace(episode)
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	if imdbID == "" || season <= 0 || episode == "" {
		return nil, true, nil
	}
	number, err := strconv.ParseFloat(episode, 64)
	if err != nil || number <= 0 {
		return nil, true, nil
	}
	query := url.Values{}
	query.Set("imdb_id", imdbID)
	query.Set("season", strconv.Itoa(season))
	query.Set("episode", strconv.Itoa(int(number)))
	endpoint := "https://api.introdb.app/segments?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusNoContent {
		return nil, true, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("introdb HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, false, err
	}
	chapters, err := buildChapters(body)
	if err != nil {
		return nil, false, err
	}
	if len(chapters) == 0 {
		return nil, true, nil
	}
	return chapters, false, nil
}

type segment struct {
	StartSec *float64 `json:"start_sec"`
	StartMS  *float64 `json:"start_ms"`
	EndSec   *float64 `json:"end_sec"`
	EndMS    *float64 `json:"end_ms"`
}

func (s segment) start() (float64, bool) {
	if s.StartSec != nil {
		return *s.StartSec, true
	}
	if s.StartMS != nil {
		return *s.StartMS / 1000, true
	}
	return 0, false
}

func (s segment) end() (float64, bool) {
	if s.EndSec != nil {
		return *s.EndSec, true
	}
	if s.EndMS != nil {
		return *s.EndMS / 1000, true
	}
	return 0, false
}

func buildChapters(body []byte) ([]Chapter, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(body, &outer); err != nil {
		return nil, err
	}
	segments := outer
	if raw, ok := outer["segments"]; ok && len(bytes.TrimSpace(raw)) > 0 && string(bytes.TrimSpace(raw)) != "null" {
		if err := json.Unmarshal(raw, &segments); err != nil {
			return nil, err
		}
	}
	type event struct {
		time  float64
		title string
		order int
	}
	events := make([]event, 0, 12)
	add := func(key, title string, order int) {
		raw, ok := segments[key]
		if !ok {
			return
		}
		var value segment
		if err := json.Unmarshal(raw, &value); err != nil || value == (segment{}) {
			return
		}
		start, okStart := value.start()
		end, okEnd := value.end()
		if !okStart || !okEnd || end <= start {
			return
		}
		events = append(events, event{time: start, title: title, order: order})
		events = append(events, event{time: end, title: "Episode", order: order + 1})
	}
	add("recap", ChapterTitle(SegmentRecap), 10)
	add("intro", ChapterTitle(SegmentIntro), 20)
	add("outro", ChapterTitle(SegmentOutro), 30)
	add("preview", ChapterTitle(SegmentPreview), 40)
	add("credits", ChapterTitle(SegmentCredits), 50)
	add("post_credits", ChapterTitle(SegmentPostCredits), 60)
	if len(events) == 0 {
		return nil, nil
	}
	sort.SliceStable(events, func(i, j int) bool {
		if math.Abs(events[i].time-events[j].time) < 0.001 {
			return events[i].order < events[j].order
		}
		return events[i].time < events[j].time
	})
	out := []Chapter{{Title: "Episode", Time: 0}}
	last := 0.0
	for _, event := range events {
		if math.Abs(last-event.time) < 0.01 {
			out[len(out)-1].Title = event.title
			continue
		}
		out = append(out, Chapter{Title: event.title, Time: event.time})
		last = event.time
	}
	if len(out) <= 1 {
		return nil, nil
	}
	return out, nil
}
