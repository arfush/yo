package store

import (
	"database/sql"
	"fmt"
)

// Migration is one versioned schema or data step. The version number is
// written into PRAGMA user_version after the step commits, so a database can
// never partially apply a migration: SQLite runs DDL inside transactions.
//
// Migrations are appended, never edited or reordered. Version 1 is the initial
// schema and is a no-op — the schema itself lives in schemaSQL (store.go) so
// fresh databases are created in one statement instead of replaying history.
type Migration struct {
	// Version must be schemaVersion + 1 for the next step and strictly
	// increasing afterwards.
	Version int
	// Name is the human-readable summary shown in the log.
	Name string
	// Apply runs inside a transaction. Steps must be idempotent enough to be
	// safe against a half-applied file only if the transaction failed, which
	// cannot happen because DDL is transactional in SQLite.
	Apply func(tx *sql.Tx) error
}

// migrations is the ordered migration history of the database. Future schema
// changes follow this template:
//
//	{Version: 2, Name: "add titles.description", Apply: func(tx *sql.Tx) error {
//		_, err := tx.Exec(`ALTER TABLE titles ADD COLUMN description TEXT NOT NULL DEFAULT ''`)
//		return err
//	}},
var migrations = []Migration{
	// Baseline: the initial schema. The tables are created by schemaSQL in
	// store.go; this entry only marks version 1 as applied.
	{Version: 1, Name: "initial schema", Apply: func(tx *sql.Tx) error { return nil }},
}

// applyMigrations brings the database up to the current schema version. A
// brand-new database has user_version 0 and receives the baseline entry; an
// older supported database runs every later step in order.
func applyMigrations(db *sql.DB) error {
	current, err := readSchemaVersion(db)
	if err != nil {
		return err
	}
	if current > schemaVersion {
		return fmt.Errorf("database version %d is newer than this build supports (%d)", current, schemaVersion)
	}
	for _, migration := range migrations {
		if migration.Version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := migration.Apply(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", migration.Version)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		logger.Info("database migration applied", "version", migration.Version, "name", migration.Name)
	}
	return nil
}

// readSchemaVersion reads PRAGMA user_version. A missing value (a database
// that was never touched) reads as zero.
func readSchemaVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}
