package kinopoisk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// metaAPIKey is the public demo key the unofficial Kinopoisk API hands to
	// small clients. It is intentionally hardcoded: the application must work
	// without any setup, and metadata is cached in the local database after the
	// first successful load, so a key outage only affects new titles.
	metaAPIKey = "c20595a1-3d8c-4cbd-92eb-fd7b0fa75c67"

	metaEndpoint    = "https://kinopoiskapiunofficial.tech/api/v2.2/films/"
	metaEndpointOld = "https://kinopoiskapiunofficial.tech/api/v2.1/films/"

	// posterEndpoint is the public Kinopoisk image host. Unlike the API it needs
	// no key, so a title always has a poster even when metadata fails.
	posterEndpoint = "https://st.kp.yandex.net/images/film_big/%d.jpg"

	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36"
)

// Meta is the display metadata of a Kinopoisk title.
type Meta struct {
	ID     int
	NameRu string
	NameEn string
	Year   int
	Movie  bool
	// Completed reports whether the release is finished. It decides whether the
	// last available episode may move the title to "Completed".
	Completed   bool
	Rating      float64
	Description string
	PosterURL   string
	IMDbID      string
	// Partial reports that the unofficial API did not answer and only the
	// poster and id are known. The title still opens; its name falls back to
	// "#id".
	Partial bool
}

// Client talks to the mirror and the metadata API.
type Client struct {
	http   *http.Client
	config Config

	mu      sync.Mutex
	players string
}

// NewClient returns a client with the configured or default mirror hosts.
func NewClient() *Client {
	return newClient(LoadConfig())
}

func newClient(config Config) *Client {
	return &Client{
		http:    &http.Client{Timeout: 25 * time.Second},
		config:  config.withDefaults(),
		players: config.withDefaults().Players,
	}
}

// Config returns the effective mirror configuration.
func (c *Client) Config() Config {
	if c == nil {
		return Config{}.withDefaults()
	}
	return c.config
}

// Meta loads the title metadata. The unofficial API is the only source of names
// and years; when it fails the poster and the id still let the title open.
func (c *Client) Meta(ctx context.Context, link Link) (Meta, error) {
	if link.ID <= 0 {
		return Meta{}, fmt.Errorf("invalid Kinopoisk id")
	}
	meta := Meta{
		ID:        link.ID,
		Movie:     link.IsMovie,
		Completed: true,
		PosterURL: fmt.Sprintf(posterEndpoint, link.ID),
	}
	for _, endpoint := range []string{metaEndpoint, metaEndpointOld} {
		loaded, err := c.fetchMeta(ctx, endpoint, link.ID)
		if err == nil {
			loaded.PosterURL = fmt.Sprintf(posterEndpoint, link.ID)
			return loaded, nil
		}
		if ctx.Err() != nil {
			return Meta{}, ctx.Err()
		}
		logger.Warn("kinopoisk metadata request failed", "endpoint", endpoint, "title", link.ID, "error", err)
	}
	meta.Partial = true
	meta.NameRu = fmt.Sprintf("#%d", link.ID)
	return meta, nil
}

func (c *Client) fetchMeta(ctx context.Context, endpoint string, id int) (Meta, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+strconv.Itoa(id), nil)
	if err != nil {
		return Meta{}, err
	}
	request.Header.Set("X-API-KEY", metaAPIKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)

	response, err := c.http.Do(request)
	if err != nil {
		return Meta{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Meta{}, err
	}
	if response.StatusCode != http.StatusOK {
		return Meta{}, fmt.Errorf("metadata status %d", response.StatusCode)
	}

	var payload struct {
		KinopoiskID   int     `json:"kinopoiskId"`
		KinopoiskIDV1 int     `json:"filmId"`
		NameRu        string  `json:"nameRu"`
		NameEn        string  `json:"nameEn"`
		NameOriginal  string  `json:"nameOriginal"`
		Year          any     `json:"year"`
		Type          string  `json:"type"`
		Serial        any     `json:"serial"`
		Completed     *bool   `json:"completed"`
		Rating        float64 `json:"ratingKinopoisk"`
		Description   string  `json:"description"`
		PosterURL     string  `json:"posterUrl"`
		IMDbID        string  `json:"imdbId"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Meta{}, err
	}

	meta := Meta{
		ID:          id,
		NameRu:      strings.TrimSpace(payload.NameRu),
		NameEn:      strings.TrimSpace(payload.NameOriginal),
		Year:        parseYear(payload.Year),
		Movie:       !isSerial(payload.Type, payload.Serial),
		Completed:   payload.Completed == nil || *payload.Completed,
		Rating:      payload.Rating,
		Description: strings.TrimSpace(payload.Description),
		IMDbID:      strings.TrimSpace(payload.IMDbID),
	}
	if meta.NameEn == "" {
		meta.NameEn = strings.TrimSpace(payload.NameEn)
	}
	if meta.NameRu == "" {
		meta.NameRu = meta.NameEn
	}
	if meta.NameRu == "" {
		meta.NameRu = fmt.Sprintf("#%d", id)
	}
	if meta.NameEn == meta.NameRu {
		meta.NameEn = ""
	}
	return meta, nil
}

// isSerial reads the heterogeneous "type"/"serial" fields of both API versions.
func isSerial(mediaType string, serial any) bool {
	if strings.Contains(strings.ToUpper(strings.TrimSpace(mediaType)), "SERIES") {
		return true
	}
	switch value := serial.(type) {
	case bool:
		return value
	case nil:
		return false
	case string:
		return strings.EqualFold(value, "true")
	case float64:
		return value != 0
	default:
		return false
	}
}

// parseYear accepts a number or a string, because the two API versions differ.
func parseYear(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		year, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return year
		}
	case json.Number:
		year, err := typed.Int64()
		if err == nil {
			return int(year)
		}
	}
	return 0
}
