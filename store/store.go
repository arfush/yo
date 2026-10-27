// Package store persists titles, history and local settings in SQLite.
//
// The schema is created from scratch on first run. Older database files are
// not supported: the versioned migration list in migrations.go is the only way
// a future build may extend the schema, and a file whose tables miss the
// columns of the current schema is rejected as corrupted.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/arfush/yo/log"
)

// defaultUserStatus is the schema default ("В планах"). Removing a title from
// the library must clear its status so a later re-add starts from scratch.
const defaultUserStatus = 2

// Source names accepted by the titles.source column.
const (
	SourceAnimeLIB  = "animelib"
	SourceKinopoisk = "kinopoisk"
	SourceMangaLib  = "mangalib"
	SourceYaoiLib   = "yaoilib"
	SourceRanobeLib = "ranobelib"
	SourceHentaiLib = "hentailib"
)

// Schema version constants. Version 1 is the initial schema; every future
// change must add one entry to migrations in migrations.go. The pragma is
// stored on the database header, so it survives renames of the file.
const schemaVersion = 1

// Errors that Open returns for an unusable database file.
//
// ErrCorruptDatabase lives in errors.go together with the other package
// sentinels.

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
CREATE INDEX IF NOT EXISTS idx_history_status_order ON history(status_order);
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

// expectedColumns is the completeness contract of the current schema. verify
// uses it to distinguish a fresh/current database from an old or damaged file.
var expectedColumns = map[string][]string{
	"titles": {
		"id", "slug", "source", "external_id", "eng_name", "rus_name",
		"imdb_id", "season", "status", "user_status", "episodes_total",
		"is_movie", "updated_at",
	},
	"history": {
		"tid", "episode", "resume_at", "duration", "voiceover",
		"sort_order", "status_order",
	},
	"cover_cache":      {"tid", "filename", "expires_at", "failed_at"},
	"introdb_cache":    {"tid", "episode", "chapters", "no_data"},
	"settings":         {"key", "value"},
	"episode_tracking": {"tid", "last_episode", "any_voice_notified", "preferred_voice_notified"},
	"title_qualities":  {"tid", "data"},
}

// Title is the persisted anime metadata plus local display fields. Episode
// lists and the remote cover URL are intentionally not stored: they are fetched
// again every time the title page is opened. EpisodesTotal is only the cached
// number of episodes so history rows can draw a progress bar without a network
// request. IsMovie keeps films from showing an episode counter in history.
type Title struct {
	ID int64
	// Source names the catalogue the title came from ("animelib" or
	// "kinopoisk" or one of the LibSocial reading sites). It decides which
	// resolver may open the title and which background checks apply to it.
	Source string
	// ExternalID is the id inside Source: the AnimeLIB slug or the Kinopoisk
	// film id. Reading titles use the LibSocial slug_url here.
	ExternalID string
	Slug       string
	EngName    string
	RusName    string
	IMDbID     string
	Season     int64
	Status     int
	// UserStatus is the local library status chosen by the user. It is kept
	// separately from Status, which mirrors the source's release state.
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
	EpisodeTracking
	Tracked bool
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

// Open creates the database (and its directory) if needed, applies pending
// migrations and rejects files that no longer match the current schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Error("cannot create database directory", "path", filepath.Dir(path), "error", err)
		return nil, err
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		logger.Error("cannot open database", "path", path, "error", err)
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		logger.Error("database ping failed", "path", path, "error", err)
		return nil, err
	}
	existingTables := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&existingTables); err != nil {
		db.Close()
		logger.Error("cannot inspect database", "path", path, "error", err)
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		if existingTables > 0 {
			// Schema statements are idempotent for a matching database, so a
			// failure here means the file was written by an unsupported build
			// (or is damaged) and CREATE IF NOT EXISTS hit a missing column.
			logger.Error("database schema is incompatible with this build", "path", path, "error", err)
			return nil, fmt.Errorf("%w: %v", ErrCorruptDatabase, err)
		}
		logger.Error("cannot create database schema", "path", path, "error", err)
		return nil, err
	}
	if err := applyMigrations(db); err != nil {
		db.Close()
		logger.Error("cannot apply database migrations", "path", path, "error", err)
		return nil, err
	}
	if err := verifySchema(db); err != nil {
		db.Close()
		logger.Error("database schema does not match this build", "path", path, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrCorruptDatabase, err)
	}
	logger.Info("database opened", "path", path)
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Validate checks an opened database with a cheap query.
func (s *Store) Validate() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is not open")
	}
	var count int
	return s.db.QueryRow("SELECT COUNT(*) FROM titles").Scan(&count)
}

// verifySchema compares the live column list of every table with the contract
// of this build. The schema is created with IF NOT EXISTS so a reopened file
// never errors on CREATE; a file from an unsupported build simply fails the
// check below and is reported as corrupted instead of half-migrated.
func verifySchema(db *sql.DB) error {
	for table, expected := range expectedColumns {
		rows, err := db.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			return fmt.Errorf("cannot inspect table %s: %w", table, err)
		}
		columnSet := make(map[string]bool)
		for rows.Next() {
			var (
				cid        int
				name       string
				kind       string
				notNull    int
				defaultVal sql.NullString
				primaryKey int
			)
			if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultVal, &primaryKey); err != nil {
				rows.Close()
				return err
			}
			columnSet[name] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, column := range expected {
			if !columnSet[column] {
				return fmt.Errorf("table %s misses column %s", table, column)
			}
		}
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// scanTitle maps one titles row into a Title, defaulting the source for rows
// written before sources existed.
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

// titleSource keeps rows written before sources existed readable.
func titleSource(title Title) string {
	if title.Source == "" {
		return SourceAnimeLIB
	}
	return title.Source
}

// rowQuerier is the smallest interface shared by *sql.DB and *sql.Tx for order
// queries.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

var logger = log.For(log.SourceStore)
