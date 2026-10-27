package storage

import (
	"database/sql"
	"errors"
)

// ListHistory returns titles in the user's order, newest first.
func (s *Storage) ListHistory() ([]HistoryEntry, error) {
	rows, err := s.db.Query(sqlQuery("history.list"))
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
func (s *Storage) GetHistory(tid int64) (*HistoryEntry, error) {
	var entry HistoryEntry
	err := s.db.QueryRow(sqlQuery("history.get"), tid).
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
func (s *Storage) EnsureHistory(tid int64, episode, voiceover string) (*HistoryEntry, error) {
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
func (s *Storage) insertHistoryRow(tid int64, episode, voiceover string) (*HistoryEntry, error) {
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
	_, err = s.db.Exec(sqlQuery("history.insert"), tid, episode, voiceover, sortOrder, statusOrder)
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
func (s *Storage) SaveSelection(tid int64, episode, voiceover string) error {
	if tid == 0 {
		return nil
	}
	result, err := s.db.Exec(sqlQuery("history.save_selection"), episode, voiceover, tid)
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
func (s *Storage) TrackViewing(tid int64, episode, voiceover string) error {
	if tid == 0 {
		return nil
	}
	if voiceover != "" {
		if _, err := s.db.Exec(sqlQuery("history.track_voiceover"), voiceover, tid); err != nil {
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
func (s *Storage) SaveProgress(tid int64, episode string, position, duration float64) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("history.save_progress"), episode, episode, position, duration, duration, tid)
	return err
}

// DeleteHistory removes one title from history without deleting its cached
// metadata. Its episode tracking goes with it: a deleted title must not keep
// producing notifications. The user status belongs to library membership, so
// it is reset too; otherwise the title would reappear with its old status.
func (s *Storage) DeleteHistory(tid int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(sqlQuery("history.reset_user_status"), defaultUserStatus, tid); err != nil {
		return err
	}
	if _, err := tx.Exec(sqlQuery("history.delete_tracking"), tid); err != nil {
		return err
	}
	if _, err := tx.Exec(sqlQuery("history.delete"), tid); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearHistory removes every history row but keeps cached title metadata. The
// statuses of the titles that were in the library are reset with the rows.
func (s *Storage) ClearHistory() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(sqlQuery("history.clear_user_status"), defaultUserStatus); err != nil {
		return err
	}
	if _, err := tx.Exec(sqlQuery("history.clear_tracking")); err != nil {
		return err
	}
	if _, err := tx.Exec(sqlQuery("history.clear")); err != nil {
		return err
	}
	return tx.Commit()
}

// ReorderHistory persists the visible order of title ids.
func (s *Storage) ReorderHistory(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, tid := range ids {
		if _, err := tx.Exec(sqlQuery("history.update_sort_order"), index, tid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReorderStatusHistory persists the order of titles inside one user status.
// Titles from other statuses keep their own order.
func (s *Storage) ReorderStatusHistory(status int, ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, tid := range ids {
		if _, err := tx.Exec(sqlQuery("history.update_status_order"), index, tid, status); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) nextSortOrder() (int64, error) {
	var minimum sql.NullInt64
	if err := s.db.QueryRow(sqlQuery("history.min_sort_order")).Scan(&minimum); err != nil {
		return 0, err
	}
	if !minimum.Valid {
		return 0, nil
	}
	return minimum.Int64 - 1000, nil
}

// historyTitleStatus returns the user status that decides which order list a
// new history row belongs to.
func (s *Storage) historyTitleStatus(tid int64) (int, error) {
	var status int
	err := s.db.QueryRow(sqlQuery("history.title_user_status"), tid).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultUserStatus, nil
	}
	return status, err
}

// nextStatusSortOrder places a newly added or promoted title above the current
// first title of its status.
func nextStatusSortOrder(q rowQuerier, status int) (int64, error) {
	var minimum sql.NullInt64
	err := q.QueryRow(sqlQuery("history.min_status_order"), status).Scan(&minimum)
	if err != nil {
		return 0, err
	}
	if !minimum.Valid {
		return 0, nil
	}
	return minimum.Int64 - 1000, nil
}
