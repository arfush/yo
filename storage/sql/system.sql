-- name: system.table_count
SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'

-- name: system.title_count
SELECT COUNT(*) FROM titles

-- name: system.normalize_filmlib_source
UPDATE titles SET source = 'filmlib' WHERE source = 'kinopoisk'

-- name: system.normalize_libsocial_source
UPDATE titles SET source = 'libsocial' WHERE source = 'animelib'

-- name: system.normalize_filmlib_slug
UPDATE titles
SET slug = 'filmlib:' || CASE
    WHEN external_id <> '' THEN external_id
    ELSE substr(slug, 11)
END
WHERE slug LIKE 'kinopoisk:%'
