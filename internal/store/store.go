// Package store persists AnimeLIB titles and local watch history in SQLite.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS titles (
    id             INTEGER PRIMARY KEY,
    slug           TEXT    UNIQUE NOT NULL,
    eng_name       TEXT    NOT NULL DEFAULT '',
    rus_name       TEXT    NOT NULL DEFAULT '',
    imdb_id        TEXT    NOT NULL DEFAULT '',
    season         INTEGER NOT NULL DEFAULT 0,
    status         INTEGER NOT NULL DEFAULT 0,
    episodes_total INTEGER NOT NULL DEFAULT 0,
    updated_at     INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS history (
    tid        INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    episode    TEXT    NOT NULL DEFAULT '',
    resume_at  REAL    NOT NULL DEFAULT 0,
    duration   REAL    NOT NULL DEFAULT 0,
    voiceover  TEXT    NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_history_order ON history(sort_order);
CREATE TABLE IF NOT EXISTS cover_cache (
    tid        INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    filename   TEXT    NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL DEFAULT 0,
    failed_at  INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS introdb_cache (
    tid      INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode  TEXT    NOT NULL,
    chapters TEXT    NOT NULL DEFAULT '',
    no_data  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (tid, episode)
);
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS episode_tracking (
    tid                      INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    last_episode             TEXT    NOT NULL DEFAULT '',
    any_voice_notified       INTEGER NOT NULL DEFAULT 0,
    preferred_voice_notified INTEGER NOT NULL DEFAULT 0
);
`

// Title is the persisted AnimeLIB metadata plus local display fields. Episode
// lists and the remote cover URL are intentionally not stored: they are fetched
// again every time the title page is opened. EpisodesTotal is only the cached
// number of episodes so history rows can draw a progress bar without a network
// request.
type Title struct {
	ID            int64
	Slug          string
	EngName       string
	RusName       string
	IMDbID        string
	Season        int64
	Status        int
	EpisodesTotal int
	UpdatedAt     int64
}

// CoverCache records the local cover file and the last download attempt.
type CoverCache struct {
	Filename string
	FailedAt int64
}

// HistoryEntry is a title in the local history, with the last selection.
type HistoryEntry struct {
	Title
	Episode   string
	ResumeAt  float64
	Duration  float64
	Voiceover string
	SortOrder int64
}

// EpisodeWatch is a title from history together with the episode-tracking row
// used by the background new-episode checker. Tracked is false when the title
// has been in history since before notification tracking existed; the checker
// seeds such a row before it starts sending notifications.
type EpisodeWatch struct {
	TitleID   int64
	Slug      string
	EngName   string
	RusName   string
	Status    int
	Voiceover string

	Tracked                bool
	LastEpisode            string
	AnyVoiceNotified       bool
	PreferredVoiceNotified bool
}

// EpisodeTracking is the persisted state of the last episode seen for one
// title. The two notification flags belong to LastEpisode: they reset when a
// newer episode appears.
type EpisodeTracking struct {
	LastEpisode            string
	AnyVoiceNotified       bool
	PreferredVoiceNotified bool
}

// Store is a SQLite-backed title and history store.
type Store struct {
	db *sql.DB
}

// Open creates the database (and its directory) if needed.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	// The first build stored no episode totals. Adding the column in place
	// keeps an existing database usable without a migration framework.
	if err := ensureTitleColumns(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// ensureTitleColumns adds columns introduced after the first public schema.
func ensureTitleColumns(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(titles)")
	if err != nil {
		return err
	}
	defer rows.Close()

	hasEpisodesTotal := false
	for rows.Next() {
		var (
			cid        int
			name       string
			kind       string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultV, &primaryKey); err != nil {
			return err
		}
		if name == "episodes_total" {
			hasEpisodesTotal = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasEpisodesTotal {
		return nil
	}
	_, err = db.Exec("ALTER TABLE titles ADD COLUMN episodes_total INTEGER NOT NULL DEFAULT 0")
	return err
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// UpsertTitle stores fresh metadata and returns the local title id.
func (s *Store) UpsertTitle(title Title) (int64, error) {
	if title.UpdatedAt == 0 {
		title.UpdatedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`
        INSERT INTO titles (slug, eng_name, rus_name, imdb_id, season, status, episodes_total, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(slug) DO UPDATE SET
            eng_name       = CASE WHEN excluded.eng_name != '' THEN excluded.eng_name ELSE titles.eng_name END,
            rus_name       = CASE WHEN excluded.rus_name != '' THEN excluded.rus_name ELSE titles.rus_name END,
            imdb_id        = CASE WHEN excluded.imdb_id != '' THEN excluded.imdb_id ELSE titles.imdb_id END,
            season         = CASE WHEN excluded.season != 0 THEN excluded.season ELSE titles.season END,
            status         = excluded.status,
            episodes_total = CASE WHEN excluded.episodes_total > 0 THEN excluded.episodes_total ELSE titles.episodes_total END,
            updated_at     = excluded.updated_at
    `, title.Slug, title.EngName, title.RusName, title.IMDbID, title.Season, title.Status, title.EpisodesTotal, title.UpdatedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := s.db.QueryRow("SELECT id FROM titles WHERE slug = ?", title.Slug).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// GetTitle returns a cached title by slug.
func (s *Store) GetTitle(slug string) (*Title, error) {
	row := s.db.QueryRow(`
        SELECT id, slug, eng_name, rus_name, imdb_id, season, status, episodes_total, updated_at
        FROM titles WHERE slug = ?`, slug)
	return scanTitle(row)
}

// GetCoverCache returns the cached cover metadata for a title.
func (s *Store) GetCoverCache(tid int64) (*CoverCache, error) {
	var cache CoverCache
	err := s.db.QueryRow(`
        SELECT filename, failed_at
        FROM cover_cache WHERE tid = ?`, tid).
		Scan(&cache.Filename, &cache.FailedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// SaveCoverSuccess records a successfully cached cover file.
func (s *Store) SaveCoverSuccess(tid int64, filename string) error {
	_, err := s.db.Exec(`
        INSERT INTO cover_cache (tid, filename, expires_at, failed_at)
        VALUES (?, ?, 0, 0)
        ON CONFLICT(tid) DO UPDATE SET
            filename   = excluded.filename,
            expires_at = 0,
            failed_at  = 0`,
		tid, filename)
	return err
}

// SaveCoverFailure remembers a failed cover download so a later title open can
// retry it. An already cached filename is kept: the previous cover stays
// visible until a replacement has been downloaded successfully.
func (s *Store) SaveCoverFailure(tid int64) error {
	_, err := s.db.Exec(`
        INSERT INTO cover_cache (tid, filename, expires_at, failed_at)
        VALUES (?, '', 0, ?)
        ON CONFLICT(tid) DO UPDATE SET
            expires_at = 0,
            failed_at  = excluded.failed_at`,
		tid, time.Now().Unix())
	return err
}

// ListHistory returns titles in the user's order, newest first.
func (s *Store) ListHistory() ([]HistoryEntry, error) {
	rows, err := s.db.Query(`
        SELECT t.id, t.slug, t.eng_name, t.rus_name, t.imdb_id, t.season, t.status, t.episodes_total, t.updated_at,
               h.episode, h.resume_at, h.duration, h.voiceover, h.sort_order
        FROM history h
        JOIN titles t ON t.id = h.tid
        ORDER BY h.sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		err := rows.Scan(
			&entry.ID, &entry.Slug, &entry.EngName, &entry.RusName, &entry.IMDbID, &entry.Season,
			&entry.Status, &entry.EpisodesTotal, &entry.UpdatedAt,
			&entry.Episode, &entry.ResumeAt, &entry.Duration, &entry.Voiceover, &entry.SortOrder,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// GetHistory returns the saved selection for a title.
func (s *Store) GetHistory(tid int64) (*HistoryEntry, error) {
	var entry HistoryEntry
	err := s.db.QueryRow(`
        SELECT tid, episode, resume_at, duration, voiceover, sort_order
        FROM history WHERE tid = ?`, tid).
		Scan(&entry.ID, &entry.Episode, &entry.ResumeAt, &entry.Duration, &entry.Voiceover, &entry.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// EnsureHistory creates a history row on first open. If the title is already
// in history, its saved selection is kept.
func (s *Store) EnsureHistory(tid int64, episode, voiceover string) (*HistoryEntry, error) {
	existing, err := s.GetHistory(tid)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	sortOrder, err := s.nextSortOrder()
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`
        INSERT INTO history (tid, episode, resume_at, duration, voiceover, sort_order)
        VALUES (?, ?, 0, 0, ?, ?)`, tid, episode, voiceover, sortOrder)
	if err != nil {
		return nil, err
	}
	return &HistoryEntry{Title: Title{ID: tid}, Episode: episode, Voiceover: voiceover, SortOrder: sortOrder}, nil
}

// SaveSelection updates the selected episode and voiceover for a title.
func (s *Store) SaveSelection(tid int64, episode, voiceover string) error {
	if tid == 0 {
		return nil
	}
	result, err := s.db.Exec(`
        UPDATE history SET episode = ?, resume_at = 0, voiceover = ? WHERE tid = ?`,
		episode, voiceover, tid)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	sortOrder, err := s.nextSortOrder()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
        INSERT INTO history (tid, episode, resume_at, duration, voiceover, sort_order)
        VALUES (?, ?, 0, 0, ?, ?)`, tid, episode, voiceover, sortOrder)
	return err
}

// SaveProgress updates the current episode and playback position. A zero
// duration keeps the previous duration, which is useful while mpv has not
// reported it yet.
func (s *Store) SaveProgress(tid int64, episode string, position, duration float64) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        UPDATE history SET
            episode   = CASE WHEN ? != '' THEN ? ELSE episode END,
            resume_at = ?,
            duration  = CASE WHEN ? > 0 THEN ? ELSE duration END
        WHERE tid = ?`,
		episode, episode, position, duration, duration, tid)
	return err
}

// ListEpisodeWatches returns every title in history with its tracking row.
// The caller decides which statuses should be polled.
func (s *Store) ListEpisodeWatches() ([]EpisodeWatch, error) {
	rows, err := s.db.Query(`
        SELECT t.id, t.slug, t.eng_name, t.rus_name, t.status, h.voiceover,
               e.tid IS NOT NULL,
               COALESCE(e.last_episode, ''),
               COALESCE(e.any_voice_notified, 0),
               COALESCE(e.preferred_voice_notified, 0)
        FROM history h
        JOIN titles t ON t.id = h.tid
        LEFT JOIN episode_tracking e ON e.tid = t.id
        ORDER BY h.sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EpisodeWatch
	for rows.Next() {
		var watch EpisodeWatch
		var tracked, anyNotified, preferredNotified int
		if err := rows.Scan(
			&watch.TitleID, &watch.Slug, &watch.EngName, &watch.RusName, &watch.Status, &watch.Voiceover,
			&tracked, &watch.LastEpisode, &anyNotified, &preferredNotified,
		); err != nil {
			return nil, err
		}
		watch.Tracked = tracked != 0
		watch.AnyVoiceNotified = anyNotified != 0
		watch.PreferredVoiceNotified = preferredNotified != 0
		out = append(out, watch)
	}
	return out, rows.Err()
}

// SeedEpisodeTracking starts tracking a title without notifying about episodes
// that were already available when it was saved. An existing row is kept, so
// pressing Save again never rewinds the notification state.
func (s *Store) SeedEpisodeTracking(tid int64, lastEpisode string) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO episode_tracking (tid, last_episode)
        VALUES (?, ?)
        ON CONFLICT(tid) DO NOTHING`, tid, lastEpisode)
	return err
}

// SaveEpisodeTracking updates the latest seen episode and its notification
// flags.
func (s *Store) SaveEpisodeTracking(tid int64, tracking EpisodeTracking) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO episode_tracking (tid, last_episode, any_voice_notified, preferred_voice_notified)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(tid) DO UPDATE SET
            last_episode             = excluded.last_episode,
            any_voice_notified       = excluded.any_voice_notified,
            preferred_voice_notified = excluded.preferred_voice_notified`,
		tid, tracking.LastEpisode, boolInt(tracking.AnyVoiceNotified), boolInt(tracking.PreferredVoiceNotified))
	return err
}

// DeleteEpisodeTracking stops tracking a title, for example when it completes
// or leaves history.
func (s *Store) DeleteEpisodeTracking(tid int64) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM episode_tracking WHERE tid = ?", tid)
	return err
}

// UpdateEpisodesTotal refreshes the cached number of episodes after a
// background check found a newly released episode.
func (s *Store) UpdateEpisodesTotal(tid int64, total int) error {
	if tid == 0 || total <= 0 {
		return nil
	}
	_, err := s.db.Exec("UPDATE titles SET episodes_total = ?, updated_at = ? WHERE id = ?", total, time.Now().Unix(), tid)
	return err
}

// UpdateTitleStatus refreshes the cached status after a background check.
func (s *Store) UpdateTitleStatus(tid int64, status int) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec("UPDATE titles SET status = ?, updated_at = ? WHERE id = ?", status, time.Now().Unix(), tid)
	return err
}

// UpdateNames refreshes the localized/English names discovered during
// playback. Empty values do not overwrite existing names.
func (s *Store) UpdateNames(tid int64, english, russian string) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        UPDATE titles SET
            eng_name = CASE WHEN ? != '' THEN ? ELSE eng_name END,
            rus_name = CASE WHEN ? != '' THEN ? ELSE rus_name END
        WHERE id = ?`,
		english, english, russian, russian, tid)
	return err
}

// UpdateMetadata stores the IMDb id and franchise season discovered during
// playback. Empty values do not overwrite existing metadata.
func (s *Store) UpdateMetadata(tid int64, imdbID string, season int) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        UPDATE titles SET
            imdb_id = CASE WHEN ? != '' THEN ? ELSE imdb_id END,
            season  = CASE WHEN ? > 0 THEN ? ELSE season END
        WHERE id = ?`,
		imdbID, imdbID, season, season, tid)
	return err
}

// EpisodeNotificationsEnabled reports whether the user wants new-episode
// notifications. The setting is enabled by default for a fresh database.
func (s *Store) EpisodeNotificationsEnabled() bool {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'episode_notifications'").Scan(&value); err != nil {
		return true
	}
	return value != "0" && value != "false"
}

// SetEpisodeNotificationsEnabled stores the new-episode notification choice.
func (s *Store) SetEpisodeNotificationsEnabled(enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES ('episode_notifications', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		value)
	return err
}

// PreferredQuality returns the globally remembered quality, defaulting to 1080.
func (s *Store) PreferredQuality() int {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'quality'").Scan(&value); err == nil {
		if quality, err := strconv.Atoi(value); err == nil && quality > 0 {
			return quality
		}
	}
	return 1080
}

// SetPreferredQuality remembers the quality globally.
func (s *Store) SetPreferredQuality(quality int) error {
	if quality <= 0 {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES ('quality', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		strconv.Itoa(quality))
	return err
}

// GetIntrodbCache returns cached chapters for one episode. The boolean is true
// when introdb previously reported that it has no entry.
func (s *Store) GetIntrodbCache(tid int64, episode string) ([]byte, bool, bool) {
	var chapters string
	var noData int
	err := s.db.QueryRow(`
        SELECT chapters, no_data FROM introdb_cache WHERE tid = ? AND episode = ?`,
		tid, episode).Scan(&chapters, &noData)
	if err != nil {
		return nil, false, false
	}
	return []byte(chapters), noData != 0, true
}

// SaveIntrodbCache stores chapters (JSON) or a no-data marker.
func (s *Store) SaveIntrodbCache(tid int64, episode string, chapters []byte, noData bool) error {
	if tid == 0 || episode == "" {
		return nil
	}
	value := 0
	if noData {
		value = 1
	}
	_, err := s.db.Exec(`
        INSERT INTO introdb_cache (tid, episode, chapters, no_data)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(tid, episode) DO UPDATE SET
            chapters = excluded.chapters,
            no_data  = excluded.no_data`,
		tid, episode, string(chapters), value)
	return err
}

// DeleteHistory removes one title from history without deleting its cached
// metadata. Its episode tracking goes with it: a deleted title must not keep
// producing notifications.
func (s *Store) DeleteHistory(tid int64) error {
	if _, err := s.db.Exec("DELETE FROM episode_tracking WHERE tid = ?", tid); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM history WHERE tid = ?", tid)
	return err
}

// ClearHistory removes every history row but keeps cached title metadata.
func (s *Store) ClearHistory() error {
	if _, err := s.db.Exec("DELETE FROM episode_tracking"); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM history")
	return err
}

// ReorderHistory persists the visible order of title ids.
func (s *Store) ReorderHistory(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, tid := range ids {
		if _, err := tx.Exec("UPDATE history SET sort_order = ? WHERE tid = ?", index, tid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) nextSortOrder() (int64, error) {
	var minimum sql.NullInt64
	if err := s.db.QueryRow("SELECT MIN(sort_order) FROM history").Scan(&minimum); err != nil {
		return 0, err
	}
	if !minimum.Valid {
		return 0, nil
	}
	return minimum.Int64 - 1000, nil
}

func scanTitle(row *sql.Row) (*Title, error) {
	var title Title
	err := row.Scan(
		&title.ID, &title.Slug, &title.EngName, &title.RusName, &title.IMDbID, &title.Season,
		&title.Status, &title.EpisodesTotal, &title.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &title, nil
}

// Validate checks an opened database with a cheap query.
func (s *Store) Validate() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is not open")
	}
	var count int
	return s.db.QueryRow("SELECT COUNT(*) FROM titles").Scan(&count)
}
