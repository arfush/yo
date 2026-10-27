// Package kinopoisk reads Kinopoisk titles that the Alloha player mirrors.
//
// A Kinopoisk link is resolved without the Kinopoisk API for playback: the
// public Alloha catalogue ("players" endpoint) maps a Kinopoisk id to the
// translations it carries, and the theatre page for a translation embeds the
// complete season/episode file list. Streams are signed per page and are only
// produced by the theatre's own JavaScript, so playback goes through a headless
// Chromium page that mounts the theatre as an iframe, exactly like the mirror
// site does.
package kinopoisk

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/arfush/yo/catalog"
)

// Source is the value stored in titles.source for Kinopoisk rows.
const Source = catalog.SourceKinopoisk

// SlugPrefix makes a Kinopoisk slug distinguishable from an AnimeLIB one. The
// local schema still uses a single unique slug column, so the prefix is what
// keeps 88198 (a Kinopoisk id) apart from a title whose AnimeLIB slug would
// happen to be numeric.
const SlugPrefix = "kinopoisk:"

// Link is a parsed Kinopoisk title reference.
type Link struct {
	ID int
	// IsMovie is only a hint taken from the URL; the catalogue is the source
	// of truth because a link can point at a film while the mirror carries it
	// as a series.
	IsMovie bool
}

// Slug returns the local slug used for the parsed link.
func (l Link) Slug() string { return SlugPrefix + strconv.Itoa(l.ID) }

var (
	kinopoiskPath = regexp.MustCompile(`(?i)/(film|series)/(\d+)`)
	numericInput  = regexp.MustCompile(`^\d{2,9}$`)
)

// ParseInput reports whether input addresses a Kinopoisk title. Accepted forms
// are a Kinopoisk link in any locale or mirror, an explicit "kinopoisk:ID"
// prefix and a bare numeric id. A plain AnimeLIB slug never matches.
func ParseInput(input string) (Link, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Link{}, false
	}
	lower := strings.ToLower(raw)

	// hd.kinopoisk.ru uses a different identifier space, so its links cannot be
	// mapped onto a Kinopoisk film id.
	if strings.Contains(lower, "hd.kinopoisk.") {
		return Link{}, false
	}
	for _, prefix := range []string{"kinopoisk:", "kinopoisk.ru:", "kp:", "кп:"} {
		if strings.HasPrefix(lower, prefix) {
			id, err := strconv.Atoi(strings.TrimSpace(raw[len(prefix):]))
			if err != nil || id <= 0 {
				return Link{}, false
			}
			return Link{ID: id}, true
		}
	}
	if match := kinopoiskPath.FindStringSubmatch(raw); len(match) == 3 {
		if !isKinopoiskHost(raw) {
			return Link{}, false
		}
		id, err := strconv.Atoi(match[2])
		if err != nil || id <= 0 {
			return Link{}, false
		}
		return Link{ID: id, IsMovie: strings.EqualFold(match[1], "film")}, true
	}
	if numericInput.MatchString(raw) {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return Link{}, false
		}
		return Link{ID: id}, true
	}
	return Link{}, false
}

// isKinopoiskHost guards the /film/ path against AnimeLIB links that may use
// the same shape.
func isKinopoiskHost(raw string) bool {
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, "://") {
		// A bare "kinopoisk.ru/film/1" is still a link, not a path.
		lower = "https://" + lower
	}
	parsed, err := url.Parse(lower)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return strings.HasSuffix(host, "kinopoisk.ru") ||
		strings.HasSuffix(host, "kinopoisk.kz") ||
		strings.HasSuffix(host, "kinopoisk.by") ||
		strings.HasSuffix(host, "kinopoisk.ua") ||
		strings.HasSuffix(host, "kinopoisk.com")
}

// AcceptsInput reports whether input looks like a Kinopoisk link. The UI uses it
// to decide whether a pasted link needs an AnimeLIB token at all.
func AcceptsInput(input string) bool {
	_, ok := ParseInput(input)
	return ok
}

// EpisodeKey returns the storage key for one episode. A film (season 0) keeps a
// plain number so it looks like an anime film; series episodes are prefixed with
// their season because episode numbers repeat across seasons.
func EpisodeKey(season, episode int) string {
	if season <= 0 {
		return strconv.Itoa(episode)
	}
	return fmt.Sprintf("S%dE%d", season, episode)
}

// SplitEpisodeKey parses a key produced by EpisodeKey. A plain number is a film
// or a season-less episode: season is 0 in that case.
func SplitEpisodeKey(key string) (season, episode int, ok bool) {
	return catalog.SplitEpisodeKey(key)
}

// EpisodeLabel strips the season prefix for display. The episode grid shows the
// episode number only; the season is chosen by the season strip above it.
func EpisodeLabel(key string) string { return catalog.EpisodeLabel(key) }

// IsSeasonKey reports whether the key carries an explicit season prefix.
func IsSeasonKey(key string) bool { return catalog.IsSeasonKey(key) }

// IframeURL appends the episode selection to the theatre page URL returned by
// the players endpoint. The page expects the season/episode in the query, and
// autoplay makes it start playback so the signed manifest and its headers can be
// observed. A film needs neither parameter.
func IframeURL(base string, season, episode int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	if season > 0 && episode > 0 {
		query.Set("season", strconv.Itoa(season))
		query.Set("episode", strconv.Itoa(episode))
	}
	query.Set("autoplay", "1")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// cleanName trims a translation name and drops the empty "Не требуется" label
// the mirror uses for the original audio, which would otherwise show up as a
// voiceover literally called "Не требуется".
func cleanName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\u00a0", " ")
	name = strings.Join(strings.Fields(name), " ")
	return name
}

// baseName returns the last path element of a URL, used to sanity-check a
// configured host.
func baseName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return path.Base(parsed.Path)
}
