CREATE TABLE IF NOT EXISTS titles (
    id             INTEGER PRIMARY KEY,
    slug           TEXT    UNIQUE NOT NULL,
    source         TEXT    NOT NULL DEFAULT 'libsocial',
    external_id    TEXT    NOT NULL DEFAULT '',
    eng_name       TEXT    NOT NULL DEFAULT '',
    rus_name       TEXT    NOT NULL DEFAULT '',
    imdb_id        TEXT    NOT NULL DEFAULT '',
    season         INTEGER NOT NULL DEFAULT 0,
    status         INTEGER NOT NULL DEFAULT 0,
    user_status    INTEGER NOT NULL DEFAULT 2,
    episodes_total INTEGER NOT NULL DEFAULT 0,
    is_movie       INTEGER NOT NULL DEFAULT 0,
    updated_at     INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS history (
    tid          INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    episode      TEXT    NOT NULL DEFAULT '',
    resume_at    REAL    NOT NULL DEFAULT 0,
    duration     REAL    NOT NULL DEFAULT 0,
    voiceover    TEXT    NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    status_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_history_order ON history(sort_order);
CREATE INDEX IF NOT EXISTS idx_history_status_order ON history(status_order);
CREATE TABLE IF NOT EXISTS cover_cache (
    tid        INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    filename   TEXT    NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL DEFAULT 0,
    failed_at  INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS introdb_cache (
    tid      INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode  TEXT    NOT NULL,
    chapters TEXT    NOT NULL DEFAULT '',
    no_data  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (tid, episode)
);
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS episode_tracking (
    tid                      INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    last_episode             TEXT    NOT NULL DEFAULT '',
    any_voice_notified       INTEGER NOT NULL DEFAULT 0,
    preferred_voice_notified INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS title_qualities (
    tid  INTEGER PRIMARY KEY REFERENCES titles(id) ON DELETE CASCADE,
    data TEXT    NOT NULL DEFAULT ''
);
