package metadata

import "errors"

// Sentinels returned by the metadata enrichment helpers.
var (
	// ErrNoRelations means AniList answered without any anime relations to
	// derive the season from.
	ErrNoRelations = errors.New("AniList returned no relations")
	// ErrNoMatches means neither MAL nor AniList knew the anime.
	ErrNoMatches = errors.New("anime not found in MAL or AniList")
)
