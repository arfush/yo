-- name: cover.get
SELECT filename, failed_at
FROM cover_cache WHERE tid = ?

-- name: cover.upsert_success
INSERT INTO cover_cache (tid, filename, expires_at, failed_at)
VALUES (?, ?, 0, 0)
ON CONFLICT(tid) DO UPDATE SET
    filename   = excluded.filename,
    expires_at = 0,
    failed_at  = 0

-- name: cover.delete
DELETE FROM cover_cache WHERE tid = ?

-- name: cover.upsert_failure
INSERT INTO cover_cache (tid, filename, expires_at, failed_at)
VALUES (?, '', 0, ?)
ON CONFLICT(tid) DO UPDATE SET
    expires_at = 0,
    failed_at  = excluded.failed_at
