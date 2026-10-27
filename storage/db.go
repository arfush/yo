// Package storage persists titles, history and local settings in SQLite.
//
// The schema is created from scratch on first run. Until the first public
// release there are no versioned migrations: opening a database stamps the
// current version and rewrites the known legacy source values in place. A file
// whose tables miss the columns of the current schema is rejected as corrupted.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/arfush/yo/log"
)

// defaultUserStatus is the schema default ("Planned"). Removing a title from
// the library must clear its status so a later re-add starts from scratch.
const defaultUserStatus = 2

// Source names accepted by the titles.source column.
const (
	SourceLibSocial = "libsocial"
	SourceFilmlib   = "filmlib"
	SourceMangaLib  = "mangalib"
	SourceYaoiLib   = "yaoilib"
	SourceRanobeLib = "ranobelib"
	SourceHentaiLib = "hentailib"
)

// Schema version constants. Version 2 is the current schema. There is no
// migration chain before the first release: prepareDatabase stamps this value
// and normalizes old source names directly. The pragma is stored on the
// database header, so it survives renames of the file.
const schemaVersion = 2

// Errors that Open returns for an unusable database file.
//
// ErrCorruptDatabase lives in errors.go together with the other package
// sentinels.

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
	// Source names the catalogue the title came from ("libsocial" or
	// "filmlib" or one of the LibSocial reading sites). It decides which
	// resolver may open the title and which background checks apply to it.
	Source string
	// ExternalID is the id inside Source: the AnimeLIB slug or the Filmlib
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

// Storage is a SQLite-backed title and history storage.
type Storage struct {
	db *sql.DB
}

// Open creates the database (and its directory) if needed, stamps the current
// schema version and rejects files that no longer match the current schema.
func Open(path string) (*Storage, error) {
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
	if err := db.QueryRow(sqlQuery("system.table_count")).Scan(&existingTables); err != nil {
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
	if err := verifySchema(db); err != nil {
		db.Close()
		logger.Error("database schema does not match this build", "path", path, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrCorruptDatabase, err)
	}
	if err := prepareDatabase(db); err != nil {
		db.Close()
		logger.Error("cannot prepare database", "path", path, "error", err)
		return nil, err
	}
	logger.Info("database opened", "path", path)
	return &Storage{db: db}, nil
}

// prepareDatabase stamps the current schema version and rewrites the source
// names used before the second content source was renamed. The statements are
// idempotent and deliberately live outside any migration framework: until the
// first release the schema is simply replaced in place.
func prepareDatabase(db *sql.DB) error {
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("stamp schema version: %w", err)
	}
	if _, err := db.Exec(sqlQuery("system.normalize_filmlib_source")); err != nil {
		return fmt.Errorf("normalize title source: %w", err)
	}
	if _, err := db.Exec(sqlQuery("system.normalize_libsocial_source")); err != nil {
		return fmt.Errorf("normalize libsocial source: %w", err)
	}
	if _, err := db.Exec(sqlQuery("system.normalize_filmlib_slug")); err != nil {
		return fmt.Errorf("normalize title slug: %w", err)
	}
	return nil
}

// Close closes the underlying database.
func (s *Storage) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Validate checks an opened database with a cheap query.
func (s *Storage) Validate() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is not open")
	}
	var count int
	return s.db.QueryRow(sqlQuery("system.title_count")).Scan(&count)
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
		title.Source = SourceLibSocial
	}
	return &title, nil
}

// titleSource keeps rows written before sources existed readable.
func titleSource(title Title) string {
	if title.Source == "" {
		return SourceLibSocial
	}
	return title.Source
}

// rowQuerier is the smallest interface shared by *sql.DB and *sql.Tx for order
// queries.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

var logger = log.For(log.SourceStorage)
