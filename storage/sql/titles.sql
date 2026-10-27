-- name: title.upsert
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

-- name: title.id_by_slug
SELECT id FROM titles WHERE slug = ?

-- name: title.by_slug
SELECT id, source, external_id, slug, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at
FROM titles WHERE slug = ?

-- name: title.by_id
SELECT id, source, external_id, slug, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at
FROM titles WHERE id = ?

-- name: title.update_episodes_total
UPDATE titles SET episodes_total = ?, updated_at = ? WHERE id = ?

-- name: title.update_status
UPDATE titles SET status = ?, updated_at = ? WHERE id = ?

-- name: title.user_status
SELECT user_status FROM titles WHERE id = ?

-- name: title.update_user_status
UPDATE titles SET user_status = ? WHERE id = ?

-- name: title.update_history_status_order
UPDATE history SET status_order = ? WHERE tid = ?

-- name: title.update_names
UPDATE titles SET
    eng_name = CASE WHEN ? != '' THEN ? ELSE eng_name END,
    rus_name = CASE WHEN ? != '' THEN ? ELSE rus_name END
WHERE id = ?

-- name: title.update_metadata
UPDATE titles SET
    imdb_id = CASE WHEN ? != '' THEN ? ELSE imdb_id END,
    season  = CASE WHEN ? > 0 THEN ? ELSE season END
WHERE id = ?
