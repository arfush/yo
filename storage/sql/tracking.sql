-- name: tracking.list_watches
SELECT t.id, t.slug, t.eng_name, t.rus_name, t.status, h.voiceover,
       e.tid IS NOT NULL,
       COALESCE(e.last_episode, ''),
       COALESCE(e.any_voice_notified, 0),
       COALESCE(e.preferred_voice_notified, 0)
FROM history h
JOIN titles t ON t.id = h.tid
LEFT JOIN episode_tracking e ON e.tid = t.id
WHERE t.source = ?
ORDER BY h.sort_order ASC

-- name: tracking.seed
INSERT INTO episode_tracking (tid, last_episode)
VALUES (?, ?)
ON CONFLICT(tid) DO NOTHING

-- name: tracking.upsert
INSERT INTO episode_tracking (tid, last_episode, any_voice_notified, preferred_voice_notified)
VALUES (?, ?, ?, ?)
ON CONFLICT(tid) DO UPDATE SET
    last_episode             = excluded.last_episode,
    any_voice_notified       = excluded.any_voice_notified,
    preferred_voice_notified = excluded.preferred_voice_notified

-- name: tracking.delete
DELETE FROM episode_tracking WHERE tid = ?

-- name: introdb.get
SELECT chapters, no_data FROM introdb_cache WHERE tid = ? AND episode = ?

-- name: introdb.upsert
INSERT INTO introdb_cache (tid, episode, chapters, no_data)
VALUES (?, ?, ?, ?)
ON CONFLICT(tid, episode) DO UPDATE SET
    chapters = excluded.chapters,
    no_data  = excluded.no_data

-- name: qualities.get
SELECT data FROM title_qualities WHERE tid = ?

-- name: qualities.upsert
INSERT INTO title_qualities (tid, data) VALUES (?, ?)
ON CONFLICT(tid) DO UPDATE SET data = excluded.data
