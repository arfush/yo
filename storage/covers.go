package storage

import (
	"database/sql"
	"errors"
	"time"
)

// GetCoverCache returns the cached cover metadata for a title.
func (s *Storage) GetCoverCache(tid int64) (*CoverCache, error) {
	var cache CoverCache
	err := s.db.QueryRow(sqlQuery("cover.get"), tid).
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
func (s *Storage) SaveCoverSuccess(tid int64, filename string) error {
	_, err := s.db.Exec(sqlQuery("cover.upsert_success"), tid, filename)
	return err
}

// DeleteCoverCache forgets the cached cover of a title. The caller removes the
// file itself, because only the library knows the cover directory.
func (s *Storage) DeleteCoverCache(tid int64) error {
	_, err := s.db.Exec(sqlQuery("cover.delete"), tid)
	return err
}

// SaveCoverFailure remembers a failed cover download so a later title open can
// retry it. An already cached filename is kept: the previous cover stays
// visible until a replacement has been downloaded successfully.
func (s *Storage) SaveCoverFailure(tid int64) error {
	_, err := s.db.Exec(sqlQuery("cover.upsert_failure"), tid, time.Now().Unix())
	return err
}
