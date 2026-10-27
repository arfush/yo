-- name: settings.get
SELECT value FROM settings WHERE key = ?

-- name: settings.upsert
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
