package store

import (
	"database/sql"
	"errors"
	"time"
)

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
