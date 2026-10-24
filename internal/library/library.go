// Package library connects the AnimeLIB API with the local SQLite cache and
// exposes display-ready titles to the UI.
package library

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arfush/yonime/internal/animelib"
	"github.com/arfush/yonime/internal/store"
	"github.com/arfush/yonime/internal/token"
)

// Status is the display status of a title.
type Status int

const (
	StatusCompleted Status = iota
	StatusOngoing
	StatusAnnounced
)

// Voiceover is a translation team and the episodes it covers. HasKodik marks
// voices with episodes that only the Kodik player carries.
type Voiceover struct {
	Name     string
	Episodes []string
	Missing  []string
	HasKodik bool
}

// Title is everything the title window needs, plus the saved selection.
type Title struct {
	ID              int64
	AnimeID         int64
	Slug            string
	Russian         string
	Original        string
	Status          Status
	CoverURL        string
	CoverPath       string
	CoverNeedsRetry bool
	Episodes        []string
	EpisodesTotal   int
	Voiceovers      []Voiceover
	SelectedVoice   string
	SelectedEpisode string
	IMDbID          string
	Season          int
	ResumeAt        float64
	Duration        float64
	UpdatedAt       int64
}

// Library owns the store and reads the token file on demand.
type Library struct {
	store     *store.Store
	tokenPath string
	covers    *coverManager
	// episodeFetcher is nil in production. Tests replace it with a local
	// function so the notification checker never touches AnimeLIB.
	episodeFetcher episodeFetcher
	// episodeCountFetcher is the lightweight counterpart used to backfill
	// episode totals for history saved before totals were cached.
	episodeCountFetcher episodeCountFetcher
}

// Open opens the local database and returns a library. The token is read lazily
// so a future token-entry screen can replace the file while the app is running.
func Open(databasePath, tokenPath string) (*Library, error) {
	db, err := store.Open(databasePath)
	if err != nil {
		return nil, err
	}
	return &Library{
		store:     db,
		tokenPath: tokenPath,
		covers:    newCoverManager(databasePath, db),
	}, nil
}

// Close closes the database.
func (l *Library) Close() error {
	if l == nil {
		return nil
	}
	return l.store.Close()
}

// History returns all saved titles in the visible order.
func (l *Library) History() ([]Title, error) {
	entries, err := l.store.ListHistory()
	if err != nil {
		return nil, err
	}
	out := make([]Title, 0, len(entries))
	for _, entry := range entries {
		title := titleFromStore(entry.Title)
		title.SelectedVoice = entry.Voiceover
		title.SelectedEpisode = entry.Episode
		title.ResumeAt = entry.ResumeAt
		title.Duration = entry.Duration
		l.applyCover(&title)
		out = append(out, title)
	}
	return out, nil
}

// Load always refreshes a title and its complete episode list from AnimeLIB.
// Nothing about the episodes, voiceovers or the cover URL is reused between
// loads: only the local metadata, saved selection and cover file are kept.
func (l *Library) Load(ctx context.Context, input string) (Title, error) {
	token, err := l.readToken()
	if err != nil {
		return Title{}, err
	}

	client := animelib.New(token)
	data, err := client.Fetch(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return Title{}, ctx.Err()
		}
		return Title{}, err
	}

	stored := storedTitleFromData(data)
	id, err := l.store.UpsertTitle(stored)
	if err != nil {
		return Title{}, fmt.Errorf("не удалось сохранить тайтл: %w", err)
	}
	stored.ID = id

	title := titleFromData(stored, data)
	if err := l.restoreSelection(&title); err != nil {
		return Title{}, err
	}
	l.ensureCover(ctx, &title)
	return title, nil
}

// Fetch loads a title and its episode list from AnimeLIB without adding it to
// the local history. A title typed into the URL field is only persisted when
// the user presses Save in the title window, so merely opening a link must not
// add it to history.
func (l *Library) Fetch(ctx context.Context, input string) (Title, error) {
	token, err := l.readToken()
	if err != nil {
		return Title{}, err
	}

	client := animelib.New(token)
	data, err := client.Fetch(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return Title{}, ctx.Err()
		}
		return Title{}, err
	}

	// A title that was saved earlier keeps its local id and selection, so the
	// Save button updates that history entry instead of creating a duplicate.
	existing, err := l.store.GetTitle(data.SlugURL)
	if err != nil {
		return Title{}, err
	}
	var stored store.Title
	if existing != nil {
		stored = *existing
	}
	title := titleFromData(stored, data)
	var voice, episode string
	if existing != nil {
		title.ID = existing.ID
		entry, err := l.store.GetHistory(existing.ID)
		if err != nil {
			return Title{}, err
		}
		if entry != nil {
			voice, episode = chooseSelection(title, entry.Voiceover, entry.Episode)
		}
	}
	if voice == "" && episode == "" {
		voice, episode = chooseSelection(title, "", "")
	}
	title.SelectedVoice = voice
	title.SelectedEpisode = episode
	l.ensureCover(ctx, &title)
	return title, nil
}

// Save writes a URL-opened title to the local history and stores the voiceover
// and episode chosen in the title window.
func (l *Library) Save(title Title, voice, episode string) (Title, error) {
	stored := storedTitleFromLibrary(title)
	id, err := l.store.UpsertTitle(stored)
	if err != nil {
		return Title{}, fmt.Errorf("не удалось сохранить тайтл: %w", err)
	}
	stored.ID = id

	// The episode list and voiceovers are not persisted. Keep the fresh data in
	// the returned title so the open window can continue using it.
	saved := titleFromData(stored, nil)
	saved.AnimeID = title.AnimeID
	saved.CoverURL = title.CoverURL
	saved.CoverPath = title.CoverPath
	saved.Episodes = append([]string(nil), title.Episodes...)
	saved.Voiceovers = append([]Voiceover(nil), title.Voiceovers...)
	saved.SelectedVoice, saved.SelectedEpisode = chooseSelection(saved, voice, episode)
	if _, err := l.store.EnsureHistory(id, saved.SelectedEpisode, saved.SelectedVoice); err != nil {
		return Title{}, err
	}
	if err := l.store.SaveSelection(id, saved.SelectedEpisode, saved.SelectedVoice); err != nil {
		return Title{}, err
	}
	if err := l.seedEpisodeTracking(saved); err != nil {
		return Title{}, err
	}
	l.applyCover(&saved)
	if saved.CoverPath == "" {
		if path, ok := l.adoptPreview(saved.ID, saved.CoverURL, title.CoverPath); ok {
			saved.CoverPath = path
			saved.CoverNeedsRetry = false
		}
	}
	return saved, nil
}

// Token returns the AnimeLIB access token for background playback requests.
func (l *Library) Token() (string, error) {
	return l.readToken()
}

// SaveSelection persists the voiceover and episode selected in the title
// window.
func (l *Library) SaveSelection(tid int64, voice, episode string) error {
	return l.store.SaveSelection(tid, episode, voice)
}

// SaveProgress stores the current episode and playback position.
func (l *Library) SaveProgress(tid int64, episode string, position, duration float64) error {
	return l.store.SaveProgress(tid, episode, position, duration)
}

// Progress returns the latest saved episode and position for a title.
func (l *Library) Progress(tid int64) (episode string, resumeAt, duration float64) {
	entry, err := l.store.GetHistory(tid)
	if err != nil || entry == nil {
		return "", 0, 0
	}
	return entry.Episode, entry.ResumeAt, entry.Duration
}

// UpdateNames refreshes the stored English and Russian titles discovered during
// playback. Existing values are kept when the new one is empty.
func (l *Library) UpdateNames(tid int64, english, russian string) error {
	return l.store.UpdateNames(tid, english, russian)
}

// UpdateMetadata stores an IMDb id and franchise season discovered for a title.
func (l *Library) UpdateMetadata(tid int64, imdbID string, season int) error {
	return l.store.UpdateMetadata(tid, imdbID, season)
}

// PreferredQuality returns the globally remembered playback quality.
func (l *Library) PreferredQuality() int {
	return l.store.PreferredQuality()
}

// SetPreferredQuality remembers playback quality for all titles.
func (l *Library) SetPreferredQuality(quality int) error {
	return l.store.SetPreferredQuality(quality)
}

// IntrodbCache returns cached chapters or a no-data marker. The third result
// reports whether an entry exists at all.
func (l *Library) IntrodbCache(tid int64, episode string) ([]byte, bool, bool) {
	return l.store.GetIntrodbCache(tid, episode)
}

// SaveIntrodbCache stores chapters or a no-data marker for an episode.
func (l *Library) SaveIntrodbCache(tid int64, episode string, chapters []byte, noData bool) error {
	return l.store.SaveIntrodbCache(tid, episode, chapters, noData)
}

// DeleteHistory removes one title from the visible history and drops its cached
// cover. Title metadata stays in the database, but the cover file would
// otherwise stay on disk forever after the history row is gone.
func (l *Library) DeleteHistory(tid int64) error {
	if err := l.store.DeleteHistory(tid); err != nil {
		return err
	}
	l.removeCover(tid)
	return nil
}

// ClearHistory removes every title from the visible history.
func (l *Library) ClearHistory() error {
	return l.store.ClearHistory()
}

// ReorderHistory persists an in-memory reorder.
func (l *Library) ReorderHistory(titles []Title) error {
	ids := make([]int64, 0, len(titles))
	for _, title := range titles {
		ids = append(ids, title.ID)
	}
	return l.store.ReorderHistory(ids)
}

func (l *Library) readToken() (string, error) {
	value, err := token.Load(l.tokenPath)
	if err != nil {
		if errors.Is(err, token.ErrNotFound) {
			return "", errors.New("токен AnimeLIB не найден — перезапусти Yonime, чтобы авторизоваться")
		}
		return "", fmt.Errorf("не удалось прочитать токен AnimeLIB: %w", err)
	}
	if value.Expired(time.Now()) {
		return "", errors.New("токен AnimeLIB истёк — перезапусти Yonime, чтобы авторизоваться")
	}
	return value.AccessToken, nil
}

// restoreSelection loads the saved voiceover and episode, falling back to the
// first complete selection. New titles are inserted into history here.
func (l *Library) restoreSelection(title *Title) error {
	entry, err := l.store.GetHistory(title.ID)
	if err != nil {
		return err
	}
	voice, episode := chooseSelection(*title, "", "")
	if entry != nil {
		voice, episode = chooseSelection(*title, entry.Voiceover, entry.Episode)
		title.ResumeAt = entry.ResumeAt
		title.Duration = entry.Duration
	}
	if entry == nil {
		if _, err := l.store.EnsureHistory(title.ID, episode, voice); err != nil {
			return err
		}
	} else if entry.Voiceover != voice || entry.Episode != episode {
		if err := l.store.SaveSelection(title.ID, episode, voice); err != nil {
			return err
		}
	}
	title.SelectedVoice = voice
	title.SelectedEpisode = episode
	return nil
}

func storedTitleFromData(data *animelib.TitleData) store.Title {
	// eng_name is null for some titles (for example Cyberpunk: Edgerunners)
	// while name already contains the English title. Never fall back to the
	// Russian name: the title window would then show the same line twice.
	english := strings.TrimSpace(data.EngName)
	if english == "" {
		english = strings.TrimSpace(data.Name)
	}
	russian := strings.TrimSpace(data.RusName)
	if russian == "" {
		russian = english
	}
	return store.Title{
		Slug:          data.SlugURL,
		EngName:       english,
		RusName:       russian,
		Status:        int(statusFrom(data.StatusID, data.StatusLabel)),
		EpisodesTotal: len(data.Episodes),
	}
}

func storedTitleFromLibrary(title Title) store.Title {
	return store.Title{
		Slug:          title.Slug,
		EngName:       title.Original,
		RusName:       title.Russian,
		IMDbID:        title.IMDbID,
		Season:        int64(title.Season),
		Status:        int(title.Status),
		EpisodesTotal: len(title.Episodes),
	}
}

// titleFromData combines the persisted metadata with the fresh AnimeLIB
// response. Episodes and voiceovers always come from data and are never loaded
// from the database. A nil data keeps the lists empty; Save copies the
// already-loaded lists over the returned title.
func titleFromData(stored store.Title, data *animelib.TitleData) Title {
	title := titleFromStore(stored)
	if data == nil {
		return title
	}
	title.AnimeID = data.ID
	if slug := strings.TrimSpace(data.SlugURL); slug != "" {
		title.Slug = slug
	}
	title.Russian = strings.TrimSpace(data.RusName)
	title.Original = strings.TrimSpace(data.EngName)
	if title.Original == "" {
		title.Original = strings.TrimSpace(data.Name)
	}
	if title.Russian == "" {
		title.Russian = title.Original
	}
	title.Status = statusFrom(data.StatusID, data.StatusLabel)
	title.CoverURL = coverURL(data.Cover)
	title.Episodes = append([]string(nil), data.Episodes...)
	title.EpisodesTotal = len(title.Episodes)
	title.Voiceovers = make([]Voiceover, 0, len(data.Voiceovers))
	for _, voice := range data.Voiceovers {
		title.Voiceovers = append(title.Voiceovers, Voiceover{
			Name:     voice.Name,
			Episodes: append([]string(nil), voice.Episodes...),
			Missing:  append([]string(nil), voice.Missing...),
			HasKodik: voice.HasKodik,
		})
	}
	if title.Episodes == nil {
		title.Episodes = []string{}
	}
	if title.Voiceovers == nil {
		title.Voiceovers = []Voiceover{}
	}
	// Some responses repeat the Russian title in eng_name. Hide the duplicate
	// instead of drawing the same line twice under the poster.
	if strings.TrimSpace(title.Original) == strings.TrimSpace(title.Russian) {
		title.Original = ""
	}
	return title
}

func titleFromStore(stored store.Title) Title {
	title := Title{
		ID:            stored.ID,
		Slug:          stored.Slug,
		Russian:       stored.RusName,
		Original:      stored.EngName,
		IMDbID:        stored.IMDbID,
		Season:        int(stored.Season),
		Status:        Status(stored.Status),
		Episodes:      []string{},
		EpisodesTotal: stored.EpisodesTotal,
		Voiceovers:    []Voiceover{},
		UpdatedAt:     stored.UpdatedAt,
	}
	if title.Russian == "" {
		title.Russian = stored.EngName
	}
	// Hide an original title that duplicates the Russian one instead of
	// drawing it twice under the poster.
	if strings.TrimSpace(title.Original) == strings.TrimSpace(title.Russian) {
		title.Original = ""
	}
	return title
}

// ApplySelection makes sure the selected voice/episode points at real data.
func (t *Title) ApplySelection() {
	t.SelectedVoice, t.SelectedEpisode = chooseSelection(*t, t.SelectedVoice, t.SelectedEpisode)
}

func chooseSelection(title Title, savedVoice, savedEpisode string) (string, string) {
	if len(title.Voiceovers) == 0 {
		if len(title.Episodes) > 0 {
			return "", title.Episodes[0]
		}
		return "", ""
	}

	voiceIndex := -1
	for index, voice := range title.Voiceovers {
		if voice.Name == savedVoice {
			voiceIndex = index
			break
		}
	}
	if voiceIndex < 0 {
		voiceIndex = 0
	}
	voice := title.Voiceovers[voiceIndex]

	if savedEpisode != "" && savedEpisode != "-1" && contains(title.Episodes, savedEpisode) && !contains(voice.Missing, savedEpisode) {
		return voice.Name, savedEpisode
	}
	for _, episode := range title.Episodes {
		if !contains(voice.Missing, episode) {
			return voice.Name, episode
		}
	}
	if len(title.Episodes) > 0 {
		return voice.Name, title.Episodes[0]
	}
	return voice.Name, ""
}

func statusFrom(id int, label string) Status {
	switch id {
	case animelib.StatusOngoing:
		return StatusOngoing
	case animelib.StatusCompleted:
		return StatusCompleted
	case animelib.StatusAnnounced:
		return StatusAnnounced
	}
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(lower, "анонс"), strings.Contains(lower, "запланир"), strings.Contains(lower, "ожида"):
		return StatusAnnounced
	case strings.Contains(lower, "заверш"), strings.Contains(lower, "вышел"):
		return StatusCompleted
	case strings.Contains(lower, "онгоинг"), strings.Contains(lower, "выход"):
		return StatusOngoing
	default:
		return StatusCompleted
	}
}

func coverURL(cover animelib.Cover) string {
	for _, candidate := range []string{cover.Original, cover.MD, cover.Default, cover.Thumbnail} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
