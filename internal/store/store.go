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

	"github.com/arfush/yoanime/log"
)

// defaultUserStatus is the schema default ("В планах"). Removing a title from
// the library must clear its status so a later re-add starts from scratch.
const defaultUserStatus = 2

const schemaSQL = `
CREATE TABLE IF NOT EXISTS titles (
    id             INTEGER PRIMARY KEY,
    slug           TEXT    UNIQUE NOT NULL,
    source         TEXT    NOT NULL DEFAULT 'animelib',
    external_id    TEXT    NOT NULL DEFAULT '',
    eng_name       TEXT    NOT NULL DEFAULT '',
    rus_name       TEXT    NOT NULL DEFAULT '',
    imdb_id        TEXT    NOT NULL DEFAULT '',
    season         INTEGER NOT NULL DEFAULT 0,
    status         INTEGER NOT NULL DEFAULT 0,
    user_status    INTEGER NOT NULL DEFAULT 2,
    episodes_total INTEGER NOT NULL DEFAULT 0,
    is_movie       INTEGER NOT NULL DEFAULT 0,
    updated_at     INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS history (
    tid          INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    episode      TEXT    NOT NULL DEFAULT '',
    resume_at    REAL    NOT NULL DEFAULT 0,
    duration     REAL    NOT NULL DEFAULT 0,
    voiceover    TEXT    NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    status_order INTEGER NOT NULL DEFAULT 0
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
CREATE TABLE IF NOT EXISTS title_qualities (
    tid  INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    data TEXT    NOT NULL DEFAULT ''
);
`

// Title is the persisted AnimeLIB metadata plus local display fields. Episode
// lists and the remote cover URL are intentionally not stored: they are fetched
// again every time the title page is opened. EpisodesTotal is only the cached
// number of episodes so history rows can draw a progress bar without a network
// request. IsMovie keeps films from showing an episode counter in history.
type Title struct {
	ID int64
	// Source names the catalogue the title came from ("animelib" or
	// "kinopoisk"). It decides which resolver may open the title and which
	// background checks apply to it.
	Source string
	// ExternalID is the id inside Source: the AnimeLIB slug or the Kinopoisk
	// film id.
	ExternalID string
	Slug       string
	EngName    string
	RusName    string
	IMDbID     string
	Season     int64
	Status     int
	// UserStatus is the local library status chosen by the user. It is kept
	// separately from Status, which mirrors AnimeLIB's release state.
	UserStatus    int
	EpisodesTotal int
	IsMovie       bool
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
	// StatusOrder is the title's position inside its user status, independently
	// of the all-titles order in SortOrder.
	StatusOrder int64
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
		log.Error("cannot create database directory", "path", filepath.Dir(path), "error", err)
		return nil, err
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Error("cannot open database", "path", path, "error", err)
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		log.Error("database ping failed", "path", path, "error", err)
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		log.Error("cannot create database schema", "path", path, "error", err)
		return nil, err
	}
	// The first build stored no episode totals. Adding the column in place
	// keeps an existing database usable without a migration framework.
	if err := ensureTitleColumns(db); err != nil {
		db.Close()
		log.Error("cannot migrate database schema", "path", path, "error", err)
		return nil, err
	}
	if err := ensureHistoryColumns(db); err != nil {
		db.Close()
		log.Error("cannot migrate history schema", "path", path, "error", err)
		return nil, err
	}
	if err := normalizeOrphanUserStatuses(db); err != nil {
		db.Close()
		log.Error("cannot clear orphan user statuses", "path", path, "error", err)
		return nil, err
	}
	log.Info("database opened", "path", path)
	return &Store{db: db}, nil
}

// normalizeOrphanUserStatuses resets cached titles that are not in the
// library. Older builds kept a deleted title's user status because only the
// history row was removed, so a later re-add brought the old status back.
func normalizeOrphanUserStatuses(db *sql.DB) error {
	result, err := db.Exec(`
        UPDATE titles SET user_status = ?
        WHERE user_status != ? AND id NOT IN (SELECT tid FROM history)`,
		defaultUserStatus, defaultUserStatus)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err == nil && changed > 0 {
		log.Info("cleared orphan user statuses", "count", changed)
	}
	return nil
}

// ensureTitleColumns adds columns introduced after the first public schema.
func ensureTitleColumns(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(titles)")
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := make(map[string]bool)
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
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !columns["episodes_total"] {
		log.Info("migrating database: adding episodes_total")
		if _, err := db.Exec("ALTER TABLE titles ADD COLUMN episodes_total INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	if !columns["is_movie"] {
		log.Info("migrating database: adding is_movie")
		if _, err := db.Exec("ALTER TABLE titles ADD COLUMN is_movie INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	if !columns["user_status"] {
		// Existing libraries start in "В планах"; the user can change it from
		// the history row or the title page.
		log.Info("migrating database: adding user_status")
		if _, err := db.Exec("ALTER TABLE titles ADD COLUMN user_status INTEGER NOT NULL DEFAULT 2"); err != nil {
			return err
		}
	}
	if !columns["source"] {
		// Every title stored before sources existed came from AnimeLIB.
		log.Info("migrating database: adding source")
		if _, err := db.Exec("ALTER TABLE titles ADD COLUMN source TEXT NOT NULL DEFAULT 'animelib'"); err != nil {
			return err
		}
	}
	if !columns["external_id"] {
		log.Info("migrating database: adding external_id")
		if _, err := db.Exec("ALTER TABLE titles ADD COLUMN external_id TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	return nil
}

// ensureHistoryColumns adds the per-status order introduced after the first
// public schema. Existing rows inherit the global order, so the status lists
// keep the order the user already saw until they drag a row.
func ensureHistoryColumns(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(history)")
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := make(map[string]bool)
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
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !columns["status_order"] {
		log.Info("migrating database: adding status_order")
		if _, err := db.Exec("ALTER TABLE history ADD COLUMN status_order INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE history SET status_order = sort_order"); err != nil {
			return err
		}
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_history_status_order ON history(status_order)"); err != nil {
		return err
	}
	return nil
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
	userStatus := title.UserStatus
	if userStatus <= 0 {
		userStatus = 2
	}
	_, err := s.db.Exec(`
        INSERT INTO titles (slug, source, external_id, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(slug) DO UPDATE SET
            source         = excluded.source,
            external_id    = excluded.external_id,
            eng_name       = CASE WHEN excluded.eng_name != '' THEN excluded.eng_name ELSE titles.eng_name END,
            rus_name       = CASE WHEN excluded.rus_name != '' THEN excluded.rus_name ELSE titles.rus_name END,
            imdb_id        = CASE WHEN excluded.imdb_id != '' THEN excluded.imdb_id ELSE titles.imdb_id END,
            season         = CASE WHEN excluded.season != 0 THEN excluded.season ELSE titles.season END,
            status         = excluded.status,
            user_status    = titles.user_status,
            episodes_total = CASE WHEN excluded.episodes_total > 0 THEN excluded.episodes_total ELSE titles.episodes_total END,
            is_movie       = excluded.is_movie,
            updated_at     = excluded.updated_at
    `, title.Slug, titleSource(title), title.ExternalID, title.EngName, title.RusName, title.IMDbID, title.Season, title.Status, userStatus, title.EpisodesTotal, boolInt(title.IsMovie), title.UpdatedAt)
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
        SELECT id, source, external_id, slug, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at
        FROM titles WHERE slug = ?`, slug)
	return scanTitle(row)
}

// GetTitleByID returns a cached title by local id.
func (s *Store) GetTitleByID(tid int64) (*Title, error) {
	row := s.db.QueryRow(`
        SELECT id, source, external_id, slug, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at
        FROM titles WHERE id = ?`, tid)
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

// DeleteCoverCache forgets the cached cover of a title. The caller removes the
// file itself, because only the library knows the cover directory.
func (s *Store) DeleteCoverCache(tid int64) error {
	_, err := s.db.Exec("DELETE FROM cover_cache WHERE tid = ?", tid)
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
        SELECT t.id, t.source, t.external_id, t.slug, t.eng_name, t.rus_name, t.imdb_id, t.season, t.status, t.user_status, t.episodes_total, t.is_movie, t.updated_at,
               h.episode, h.resume_at, h.duration, h.voiceover, h.sort_order, h.status_order
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
			&entry.ID, &entry.Source, &entry.ExternalID, &entry.Slug, &entry.EngName, &entry.RusName, &entry.IMDbID, &entry.Season,
			&entry.Status, &entry.UserStatus, &entry.EpisodesTotal, &entry.IsMovie, &entry.UpdatedAt,
			&entry.Episode, &entry.ResumeAt, &entry.Duration, &entry.Voiceover, &entry.SortOrder, &entry.StatusOrder,
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
        SELECT tid, episode, resume_at, duration, voiceover, sort_order, status_order
        FROM history WHERE tid = ?`, tid).
		Scan(&entry.ID, &entry.Episode, &entry.ResumeAt, &entry.Duration, &entry.Voiceover, &entry.SortOrder, &entry.StatusOrder)
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
	return s.insertHistoryRow(tid, episode, voiceover)
}

// insertHistoryRow adds a title to the history. New rows go to the top of both
// the complete list and their user-status list.
func (s *Store) insertHistoryRow(tid int64, episode, voiceover string) (*HistoryEntry, error) {
	status, err := s.historyTitleStatus(tid)
	if err != nil {
		return nil, err
	}
	sortOrder, err := s.nextSortOrder()
	if err != nil {
		return nil, err
	}
	statusOrder, err := nextStatusSortOrder(s.db, status)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`
        INSERT INTO history (tid, episode, resume_at, duration, voiceover, sort_order, status_order)
        VALUES (?, ?, 0, 0, ?, ?, ?)`, tid, episode, voiceover, sortOrder, statusOrder)
	if err != nil {
		return nil, err
	}
	return &HistoryEntry{
		Title:       Title{ID: tid},
		Episode:     episode,
		Voiceover:   voiceover,
		SortOrder:   sortOrder,
		StatusOrder: statusOrder,
	}, nil
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
	_, err = s.insertHistoryRow(tid, episode, voiceover)
	return err
}

// TrackViewing makes sure a title that is about to play has a history row. A
// title opened from a link has no row until it is saved, but watching it must
// still keep progress. An existing row keeps its episode and saved position;
// only the voiceover follows what is being watched now.
func (s *Store) TrackViewing(tid int64, episode, voiceover string) error {
	if tid == 0 {
		return nil
	}
	if voiceover != "" {
		if _, err := s.db.Exec(`UPDATE history SET voiceover = ? WHERE tid = ?`, voiceover, tid); err != nil {
			return err
		}
	}
	existing, err := s.GetHistory(tid)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	_, err = s.EnsureHistory(tid, episode, voiceover)
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
        WHERE t.source = ?
        ORDER BY h.sort_order ASC`, SourceAnimeLIB)
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

// SetUserStatus stores the local library status for a title. A changed status
// also moves the title to the top of its new status list, while the all-titles
// order stays as the user arranged it.
func (s *Store) SetUserStatus(tid int64, status int) error {
	changed, err := s.setUserStatus(tid, status, 0, false)
	if err != nil {
		log.Error("cannot save user status", "title", tid, "status", status, "error", err)
		return err
	}
	if changed {
		log.Info("user status saved", "title", tid, "status", status)
	}
	return nil
}

// SetUserStatusFrom changes a status only when it still has the expected old
// value. It is used to promote a planned title to "Смотрю" when playback starts
// without overwriting a choice the user made in the meantime.
func (s *Store) SetUserStatusFrom(tid int64, from, to int) error {
	_, err := s.setUserStatus(tid, to, from, true)
	return err
}

// setUserStatus applies a status change in one transaction. requireFrom makes
// the write conditional on the status the caller last saw.
func (s *Store) setUserStatus(tid int64, status, from int, requireFrom bool) (bool, error) {
	if tid == 0 {
		return false, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var current int
	err = tx.QueryRow("SELECT user_status FROM titles WHERE id = ?", tid).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	if requireFrom && current != from {
		return false, tx.Commit()
	}
	if current == status {
		return false, tx.Commit()
	}
	statusOrder, err := nextStatusSortOrder(tx, status)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec("UPDATE titles SET user_status = ? WHERE id = ?", status, tid); err != nil {
		return false, err
	}
	if _, err := tx.Exec("UPDATE history SET status_order = ? WHERE tid = ?", statusOrder, tid); err != nil {
		return false, err
	}
	return true, tx.Commit()
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

// Settings returns the raw value of a settings row. The boolean is false when
// the key was never stored.
func (s *Store) Settings(key string) (string, bool) {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value); err != nil {
		return "", false
	}
	return value, true
}

// SetSettings stores a raw settings row.
func (s *Store) SetSettings(key, value string) error {
	if key == "" {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES (?, ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	return err
}

// boolSetting reads a boolean setting and falls back to defaultValue when the
// row is missing or unreadable.
func (s *Store) boolSetting(key string, defaultValue bool) bool {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value); err != nil {
		return defaultValue
	}
	return value != "0" && value != "false"
}

// setBoolSetting stores a boolean setting.
func (s *Store) setBoolSetting(key string, enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES (?, ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	return err
}

// EpisodeNotificationsEnabled reports whether the user wants new-episode
// notifications. The setting is enabled by default for a fresh database.
func (s *Store) EpisodeNotificationsEnabled() bool {
	return s.boolSetting("episode_notifications", true)
}

// SetEpisodeNotificationsEnabled stores the new-episode notification choice.
func (s *Store) SetEpisodeNotificationsEnabled(enabled bool) error {
	return s.setBoolSetting("episode_notifications", enabled)
}

// AutoUpdateEnabled reports whether yoanime may install updates without asking.
// It is on by default so a fresh install picks up the latest release; an
// explicit "0" row still turns it off for good.
func (s *Store) AutoUpdateEnabled() bool {
	return s.boolSetting("auto_update", true)
}

// SetAutoUpdateEnabled stores the automatic-update choice.
func (s *Store) SetAutoUpdateEnabled(enabled bool) error {
	return s.setBoolSetting("auto_update", enabled)
}

// BetaUpdatesEnabled reports whether patch releases (major.minor.patch) may be
// installed. It is off by default because patch builds may be less polished.
func (s *Store) BetaUpdatesEnabled() bool {
	return s.boolSetting("beta_updates", false)
}

// SetBetaUpdatesEnabled stores the beta-testing choice.
func (s *Store) SetBetaUpdatesEnabled(enabled bool) error {
	return s.setBoolSetting("beta_updates", enabled)
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

// SharedControlEnabled reports whether watch-party guests may pause, resume and
// seek the host's player. It is enabled by default so a room behaves the way
// people expect without opening the settings first.
func (s *Store) SharedControlEnabled() bool {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'party_control'").Scan(&value); err != nil {
		return true
	}
	return value != "0" && value != "false"
}

// SetSharedControlEnabled stores the shared-control choice.
func (s *Store) SetSharedControlEnabled(enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES ('party_control', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		value)
	return err
}

// AutoSkipSegment reports whether chapters of this introdb segment type are
// skipped automatically. Auto-skip is opt-in, so a missing row means off.
func (s *Store) AutoSkipSegment(segment string) bool {
	return s.boolSetting("autoskip_"+segment, false)
}

// SetAutoSkipSegment stores the per-type auto-skip choice.
func (s *Store) SetAutoSkipSegment(segment string, enabled bool) error {
	return s.setBoolSetting("autoskip_"+segment, enabled)
}

// PartyProvider returns the relay the user picked for new rooms. An empty
// string means the user never picked one and the first configured relay wins.
func (s *Store) PartyProvider() string {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'party_provider'").Scan(&value); err != nil {
		return ""
	}
	return value
}

// SetPartyProvider remembers the relay chosen in the settings.
func (s *Store) SetPartyProvider(provider string) error {
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES ('party_provider', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		provider)
	return err
}

// PartyName returns the nickname shown in watch-party rooms. An empty
// string means the user never picked one and the UI generates a default.
func (s *Store) PartyName() string {
	var value string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'party_name'").Scan(&value); err != nil {
		return ""
	}
	return value
}

// SetPartyName remembers the nickname used in watch-party rooms.
func (s *Store) SetPartyName(name string) error {
	_, err := s.db.Exec(`
        INSERT INTO settings (key, value) VALUES ('party_name', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		name)
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

// GetTitleQualities returns the quality ladder cached for a saved Kinopoisk
// title. The ladder is JSON keyed by voice name; an empty string means the
// title page has to probe it again.
func (s *Store) GetTitleQualities(tid int64) (string, error) {
	if tid == 0 {
		return "", nil
	}
	var data string
	if err := s.db.QueryRow("SELECT data FROM title_qualities WHERE tid = ?", tid).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return data, nil
}

// SaveTitleQualities caches the quality ladders of a saved Kinopoisk title. Only
// a non-empty payload is written: a failed probe must not erase the ladder the
// previous save managed to learn.
func (s *Store) SaveTitleQualities(tid int64, data string) error {
	if tid == 0 || data == "" {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO title_qualities (tid, data) VALUES (?, ?)
        ON CONFLICT(tid) DO UPDATE SET data = excluded.data`,
		tid, data)
	return err
}

// DeleteHistory removes one title from history without deleting its cached
// metadata. Its episode tracking goes with it: a deleted title must not keep
// producing notifications. The user status belongs to library membership, so
// it is reset too; otherwise the title would reappear with its old status.
func (s *Store) DeleteHistory(tid int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE titles SET user_status = ? WHERE id = ?", defaultUserStatus, tid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM episode_tracking WHERE tid = ?", tid); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM history WHERE tid = ?", tid); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearHistory removes every history row but keeps cached title metadata. The
// statuses of the titles that were in the library are reset with the rows.
func (s *Store) ClearHistory() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
        UPDATE titles SET user_status = ?
        WHERE id IN (SELECT tid FROM history)`, defaultUserStatus); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM episode_tracking"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM history"); err != nil {
		return err
	}
	return tx.Commit()
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

// ReorderStatusHistory persists the order of titles inside one user status.
// Titles from other statuses keep their own order.
func (s *Store) ReorderStatusHistory(status int, ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, tid := range ids {
		if _, err := tx.Exec(`
            UPDATE history SET status_order = ?
            WHERE tid = ? AND EXISTS (
                SELECT 1 FROM titles
                WHERE titles.id = history.tid AND titles.user_status = ?
            )`, index, tid, status); err != nil {
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

// historyTitleStatus returns the user status that decides which order list a
// new history row belongs to.
func (s *Store) historyTitleStatus(tid int64) (int, error) {
	var status int
	err := s.db.QueryRow("SELECT user_status FROM titles WHERE id = ?", tid).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultUserStatus, nil
	}
	return status, err
}

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// nextStatusSortOrder places a newly added or promoted title above the current
// first title of its status.
func nextStatusSortOrder(q rowQuerier, status int) (int64, error) {
	var minimum sql.NullInt64
	err := q.QueryRow(`
        SELECT MIN(h.status_order)
        FROM history h
        JOIN titles t ON t.id = h.tid
        WHERE t.user_status = ?`, status).Scan(&minimum)
	if err != nil {
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
		&title.ID, &title.Source, &title.ExternalID, &title.Slug, &title.EngName, &title.RusName, &title.IMDbID, &title.Season,
		&title.Status, &title.UserStatus, &title.EpisodesTotal, &title.IsMovie, &title.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if title.Source == "" {
		title.Source = SourceAnimeLIB
	}
	return &title, nil
}

// Source names accepted by the titles.source column.
const (
	SourceAnimeLIB  = "animelib"
	SourceKinopoisk = "kinopoisk"
)

// titleSource keeps rows written before sources existed readable.
func titleSource(title Title) string {
	if title.Source == "" {
		return SourceAnimeLIB
	}
	return title.Source
}

// Validate checks an opened database with a cheap query.
func (s *Store) Validate() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is not open")
	}
	var count int
	return s.db.QueryRow("SELECT COUNT(*) FROM titles").Scan(&count)
}
