-- name: history.list
SELECT t.id, t.source, t.external_id, t.slug, t.eng_name, t.rus_name, t.imdb_id, t.season, t.status, t.user_status, t.episodes_total, t.is_movie, t.updated_at,
       h.episode, h.resume_at, h.duration, h.voiceover, h.sort_order, h.status_order
FROM history h
JOIN titles t ON t.id = h.tid
ORDER BY h.sort_order ASC

-- name: history.get
SELECT tid, episode, resume_at, duration, voiceover, sort_order, status_order
FROM history WHERE tid = ?

-- name: history.insert
INSERT INTO history (tid, episode, resume_at, duration, voiceover, sort_order, status_order)
VALUES (?, ?, 0, 0, ?, ?, ?)

-- name: history.save_selection
UPDATE history SET episode = ?, resume_at = 0, voiceover = ? WHERE tid = ?

-- name: history.track_voiceover
UPDATE history SET voiceover = ? WHERE tid = ?

-- name: history.save_progress
UPDATE history SET
    episode   = CASE WHEN ? != '' THEN ? ELSE episode END,
    resume_at = ?,
    duration  = CASE WHEN ? > 0 THEN ? ELSE duration END
WHERE tid = ?

-- name: history.reset_user_status
UPDATE titles SET user_status = ? WHERE id = ?

-- name: history.delete_tracking
DELETE FROM episode_tracking WHERE tid = ?

-- name: history.delete
DELETE FROM history WHERE tid = ?

-- name: history.clear_user_status
UPDATE titles SET user_status = ?
WHERE id IN (SELECT tid FROM history)

-- name: history.clear_tracking
DELETE FROM episode_tracking

-- name: history.clear
DELETE FROM history

-- name: history.update_sort_order
UPDATE history SET sort_order = ? WHERE tid = ?

-- name: history.update_status_order
UPDATE history SET status_order = ?
WHERE tid = ? AND EXISTS (
    SELECT 1 FROM titles
    WHERE titles.id = history.tid AND titles.user_status = ?
)

-- name: history.min_sort_order
SELECT MIN(sort_order) FROM history

-- name: history.title_user_status
SELECT user_status FROM titles WHERE id = ?

-- name: history.min_status_order
SELECT MIN(h.status_order)
FROM history h
JOIN titles t ON t.id = h.tid
WHERE t.user_status = ?
