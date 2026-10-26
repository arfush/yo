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
	"github.com/arfush/yonime/internal/logging"
	"github.com/arfush/yonime/internal/metadata"
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

// UserStatus is the local status the user assigns to a title. It is separate
// from Status, which mirrors AnimeLIB's release state.
type UserStatus int

const (
	UserStatusWatching  UserStatus = 1
	UserStatusPlanned   UserStatus = 2
	UserStatusFavorite  UserStatus = 3
	UserStatusCompleted UserStatus = 4
	UserStatusDropped   UserStatus = 5
)

// NormalizeUserStatus maps an unset or unknown value to "В планах".
func NormalizeUserStatus(status UserStatus) UserStatus {
	switch status {
	case UserStatusWatching, UserStatusPlanned, UserStatusFavorite, UserStatusCompleted, UserStatusDropped:
		return status
	default:
		return UserStatusPlanned
	}
}

// Valid reports whether the value is one of the five statuses offered by the
// UI.
func (s UserStatus) Valid() bool {
	return s >= UserStatusWatching && s <= UserStatusDropped
}

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
	UserStatus      UserStatus
	IsMovie         bool
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
	// StatusOrder is the title's position inside its user status. The all-title
	// order is not stored in this struct because History already returns it.
	StatusOrder int64
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
		logging.Error("cannot load history", "error", err)
		return nil, err
	}
	logging.Debug("history loaded", "count", len(entries))
	out := make([]Title, 0, len(entries))
	for _, entry := range entries {
		title := titleFromStore(entry.Title)
		title.SelectedVoice = entry.Voiceover
		title.SelectedEpisode = entry.Episode
		title.ResumeAt = entry.ResumeAt
		title.Duration = entry.Duration
		title.StatusOrder = entry.StatusOrder
		l.applyCover(&title)
		out = append(out, title)
	}
	return out, nil
}

// Load always refreshes a title and its complete episode list from AnimeLIB.
// Nothing about the episodes, voiceovers or the cover URL is reused between
// loads: only the local metadata, saved selection and cover file are kept.
func (l *Library) Load(ctx context.Context, input string) (Title, error) {
	logging.Info("loading title", "input", input)
	token, err := l.readToken()
	if err != nil {
		logging.Error("cannot read AnimeLIB token", "error", err)
		return Title{}, err
	}

	client := animelib.New(token)
	data, err := client.Fetch(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			logging.Info("title load cancelled", "input", input)
			return Title{}, ctx.Err()
		}
		logging.Error("title load failed", "input", input, "error", err)
		return Title{}, err
	}

	stored, err := l.upsertLoadedTitle(storedTitleFromData(data))
	if err != nil {
		return Title{}, err
	}

	title := titleFromData(stored, data)
	if err := l.restoreSelection(&title); err != nil {
		return Title{}, err
	}
	l.ensureCover(ctx, &title)
	logging.Info("title loaded", "title", title.Russian, "slug", title.Slug, "episodes", len(title.Episodes), "voices", len(title.Voiceovers))
	return title, nil
}

// upsertLoadedTitle stores freshly loaded metadata and returns the row as it
// exists in the database. UpsertTitle deliberately preserves the current user
// status on conflict, so the in-memory copy still carries the schema default;
// reloading it is what makes the title page show the status chosen in history.
func (l *Library) upsertLoadedTitle(stored store.Title) (store.Title, error) {
	id, err := l.store.UpsertTitle(stored)
	if err != nil {
		return store.Title{}, fmt.Errorf("не удалось сохранить тайтл: %w", err)
	}
	persisted, err := l.store.GetTitleByID(id)
	if err != nil {
		return store.Title{}, err
	}
	if persisted != nil {
		return *persisted, nil
	}
	stored.ID = id
	return stored, nil
}

// Fetch loads a title and its episode list from AnimeLIB without adding it to
// the local history. A title typed into the URL field is only persisted when
// the user presses Save in the title window, so merely opening a link must not
// add it to history.
func (l *Library) Fetch(ctx context.Context, input string) (Title, error) {
	logging.Info("fetching title", "input", input)
	token, err := l.readToken()
	if err != nil {
		logging.Error("cannot read AnimeLIB token", "error", err)
		return Title{}, err
	}

	client := animelib.New(token)
	data, err := client.Fetch(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			logging.Info("title fetch cancelled", "input", input)
			return Title{}, ctx.Err()
		}
		logging.Error("title fetch failed", "input", input, "error", err)
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
	logging.Info("title fetched", "title", title.Russian, "slug", title.Slug, "episodes", len(title.Episodes), "voices", len(title.Voiceovers))
	return title, nil
}

// Save writes a URL-opened title to the local history and stores the voiceover
// and episode chosen in the title window.
func (l *Library) Save(title Title, voice, episode string) (Title, error) {
	logging.Info("saving title", "title", title.Russian, "slug", title.Slug, "user_status", title.UserStatus)
	stored := storedTitleFromLibrary(title)
	id, err := l.store.UpsertTitle(stored)
	if err != nil {
		logging.Error("cannot save title", "slug", title.Slug, "error", err)
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
	// UpsertTitle keeps the existing status so background metadata refreshes do
	// not overwrite a manual choice. Save is explicit, so write the status from
	// the title window even when the title already existed without history.
	if err := l.store.SetUserStatus(id, int(stored.UserStatus)); err != nil {
		logging.Error("cannot save user status", "title", id, "error", err)
		return Title{}, err
	}
	saved.UserStatus = UserStatus(stored.UserStatus)
	l.applyCover(&saved)
	if saved.CoverPath == "" {
		if path, ok := l.adoptPreview(saved.ID, saved.CoverURL, title.CoverPath); ok {
			saved.CoverPath = path
			saved.CoverNeedsRetry = false
		}
	}
	logging.Info("title saved", "title", saved.ID, "slug", saved.Slug)
	return saved, nil
}

// Token returns the AnimeLIB access token for background playback requests.
func (l *Library) Token() (string, error) {
	return l.readToken()
}

// TrackViewing records that a title is being watched: the title row is created
// if needed and the history row is ensured, so progress survives even for a
// link-opened title that was never saved and for a title a watch-party guest
// only saw through the room. The saved position is kept.
func (l *Library) TrackViewing(title Title, voice, episode string) (int64, error) {
	stored := storedTitleFromLibrary(title)
	id, err := l.store.UpsertTitle(stored)
	if err != nil {
		return 0, fmt.Errorf("не удалось сохранить тайтл: %w", err)
	}
	if err := l.store.TrackViewing(id, episode, voice); err != nil {
		logging.Error("cannot track viewing", "title", id, "episode", episode, "error", err)
		return id, err
	}
	l.promoteWatching(id)
	logging.Info("viewing tracked", "title", id, "episode", episode, "voice", voice)
	return id, nil
}

// SaveSelection persists the voiceover and episode selected in the title
// window.
func (l *Library) SaveSelection(tid int64, voice, episode string) error {
	return l.store.SaveSelection(tid, episode, voice)
}

// SetUserStatus stores the local library status. Announced titles can only be
// planned: their episodes do not exist yet, so "Смотрю" would be misleading.
func (l *Library) SetUserStatus(tid int64, status UserStatus) error {
	if !status.Valid() {
		return errors.New("неизвестный статус тайтла")
	}
	if tid != 0 {
		title, err := l.store.GetTitleByID(tid)
		if err != nil {
			return err
		}
		if title != nil && Status(title.Status) == StatusAnnounced {
			status = UserStatusPlanned
		}
	}
	return l.store.SetUserStatus(tid, int(status))
}

// promoteWatching moves a planned title to "Смотрю" when playback starts. It
// deliberately leaves every other status alone.
func (l *Library) promoteWatching(tid int64) {
	if tid == 0 || l.store == nil {
		return
	}
	if err := l.store.SetUserStatusFrom(tid, int(UserStatusPlanned), int(UserStatusWatching)); err != nil {
		logging.Warn("cannot promote user status", "title", tid, "error", err)
		return
	}
}

// UserStatus returns the stored local status for a title. A missing row or a
// read failure falls back to the default "В планах"; the UI only needs this to
// refresh a pill after playback.
func (l *Library) UserStatus(tid int64) UserStatus {
	if tid == 0 || l.store == nil {
		return UserStatusPlanned
	}
	title, err := l.store.GetTitleByID(tid)
	if err != nil {
		logging.Warn("cannot read user status", "title", tid, "error", err)
		return UserStatusPlanned
	}
	if title == nil {
		return UserStatusPlanned
	}
	return NormalizeUserStatus(UserStatus(title.UserStatus))
}

// MarkCompleted moves a title to "Досмотрел" after the final episode finishes.
// Unlike SetUserStatus it is unconditional: reaching the end is a stronger
// signal than the status the user picked earlier.
func (l *Library) MarkCompleted(tid int64) error {
	if tid == 0 || l.store == nil {
		return nil
	}
	if err := l.store.SetUserStatus(tid, int(UserStatusCompleted)); err != nil {
		logging.Error("cannot mark title completed", "title", tid, "error", err)
		return err
	}
	logging.Info("title completed", "title", tid)
	return nil
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

// SharedControlEnabled reports whether watch-party guests may control playback.
func (l *Library) SharedControlEnabled() bool {
	if l == nil || l.store == nil {
		return true
	}
	return l.store.SharedControlEnabled()
}

// SetSharedControlEnabled stores the shared-control choice.
func (l *Library) SetSharedControlEnabled(enabled bool) error {
	return l.store.SetSharedControlEnabled(enabled)
}

// AutoSkipSegments returns the introdb segments that playback should skip.
// Every segment is off until the user enables it in the settings.
func (l *Library) AutoSkipSegments() map[metadata.Segment]bool {
	segments := metadata.Segments()
	out := make(map[metadata.Segment]bool, len(segments))
	if l == nil || l.store == nil {
		return out
	}
	for _, segment := range segments {
		out[segment] = l.store.AutoSkipSegment(string(segment))
	}
	return out
}

// SetAutoSkipSegment stores the per-type auto-skip choice.
func (l *Library) SetAutoSkipSegment(segment metadata.Segment, enabled bool) error {
	if l == nil || l.store == nil {
		return errors.New("база данных недоступна")
	}
	return l.store.SetAutoSkipSegment(string(segment), enabled)
}

// AutoUpdateEnabled reports whether updates may install without asking.
func (l *Library) AutoUpdateEnabled() bool {
	if l == nil || l.store == nil {
		return false
	}
	return l.store.AutoUpdateEnabled()
}

// SetAutoUpdateEnabled stores the automatic-update choice.
func (l *Library) SetAutoUpdateEnabled(enabled bool) error {
	return l.store.SetAutoUpdateEnabled(enabled)
}

// BetaUpdatesEnabled reports whether patch releases may be installed.
func (l *Library) BetaUpdatesEnabled() bool {
	if l == nil || l.store == nil {
		return false
	}
	return l.store.BetaUpdatesEnabled()
}

// SetBetaUpdatesEnabled stores the beta-testing choice.
func (l *Library) SetBetaUpdatesEnabled(enabled bool) error {
	return l.store.SetBetaUpdatesEnabled(enabled)
}

// DiscordPresenceEnabled reports whether the current episode may be shown in
// the Discord profile.
func (l *Library) DiscordPresenceEnabled() bool {
	if l == nil || l.store == nil {
		return false
	}
	return l.store.DiscordPresenceEnabled()
}

// SetDiscordPresenceEnabled stores the Discord activity choice.
func (l *Library) SetDiscordPresenceEnabled(enabled bool) error {
	return l.store.SetDiscordPresenceEnabled(enabled)
}

// WatchPartyProvider returns the relay picked for new rooms, or "" when the
// user never picked one.
func (l *Library) WatchPartyProvider() string {
	if l == nil || l.store == nil {
		return ""
	}
	return l.store.WatchPartyProvider()
}

// SetWatchPartyProvider remembers the relay picked for new rooms.
func (l *Library) SetWatchPartyProvider(provider string) error {
	return l.store.SetWatchPartyProvider(provider)
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
		logging.Error("cannot delete history", "title", tid, "error", err)
		return err
	}
	l.removeCover(tid)
	logging.Info("history deleted", "title", tid)
	return nil
}

// ClearHistory removes every title from the visible history.
func (l *Library) ClearHistory() error {
	if err := l.store.ClearHistory(); err != nil {
		logging.Error("cannot clear history", "error", err)
		return err
	}
	logging.Info("history cleared")
	return nil
}

// ReorderHistory persists an in-memory reorder.
func (l *Library) ReorderHistory(titles []Title) error {
	ids := make([]int64, 0, len(titles))
	for _, title := range titles {
		ids = append(ids, title.ID)
	}
	return l.store.ReorderHistory(ids)
}

// ReorderStatusHistory persists a reorder inside one user status.
func (l *Library) ReorderStatusHistory(status UserStatus, titles []Title) error {
	ids := make([]int64, 0, len(titles))
	for _, title := range titles {
		ids = append(ids, title.ID)
	}
	return l.store.ReorderStatusHistory(int(NormalizeUserStatus(status)), ids)
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
		IsMovie:       data.IsMovie(),
	}
}

func storedTitleFromLibrary(title Title) store.Title {
	userStatus := NormalizeUserStatus(title.UserStatus)
	if title.Status == StatusAnnounced {
		userStatus = UserStatusPlanned
	}
	return store.Title{
		Slug:          title.Slug,
		EngName:       title.Original,
		RusName:       title.Russian,
		IMDbID:        title.IMDbID,
		Season:        int64(title.Season),
		Status:        int(title.Status),
		UserStatus:    int(userStatus),
		EpisodesTotal: len(title.Episodes),
		IsMovie:       title.IsMovie,
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
	if title.Status == StatusAnnounced {
		title.UserStatus = UserStatusPlanned
	}
	title.IsMovie = data.IsMovie()
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
		UserStatus:    NormalizeUserStatus(UserStatus(stored.UserStatus)),
		Episodes:      []string{},
		EpisodesTotal: stored.EpisodesTotal,
		IsMovie:       stored.IsMovie,
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
	if title.Status == StatusAnnounced {
		title.UserStatus = UserStatusPlanned
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
