// Package catalog holds the display-ready types shared by the feature
// packages and the UI. It is a leaf: it must not import another project
// package, so every screen can depend on it without pulling in a network or
// database client.
package catalog

import (
	"fmt"
	"strconv"
	"strings"
)

// Source values name the catalogue a title came from. The LibSocial sources
// share one API and one reader, but keep their own value so history and covers
// can still tell the sites apart.
const (
	SourceAnimeLIB  = "animelib"
	SourceKinopoisk = "kinopoisk"
	SourceMangaLib  = "mangalib"
	SourceYaoiLib   = "yaoilib"
	SourceRanobeLib = "ranobelib"
	SourceHentaiLib = "hentailib"
)

// IsReadingSource reports whether a source belongs to the manga or ranobe
// reader rather than the anime player.
func IsReadingSource(source string) bool {
	switch source {
	case SourceMangaLib, SourceYaoiLib, SourceRanobeLib, SourceHentaiLib:
		return true
	default:
		return false
	}
}

// IsRanobeSource reports whether a source is a text novel site. The reader uses
// it to choose paragraph rendering over the page image canvas.
func IsRanobeSource(source string) bool {
	return source == SourceRanobeLib
}

// Status is the display status of a title.
type Status int

const (
	StatusCompleted Status = iota
	StatusOngoing
	StatusAnnounced
)

// UserStatus is the local status the user assigns to a title. It is separate
// from Status, which mirrors the release state of the source.
type UserStatus int

const (
	UserStatusWatching  UserStatus = 1
	UserStatusPlanned   UserStatus = 2
	UserStatusFavorite  UserStatus = 3
	UserStatusCompleted UserStatus = 4
	UserStatusDropped   UserStatus = 5
)

// NormalizeUserStatus maps an unset or unknown value to "В планах".
func NormalizeUserStatus(status UserStatus) UserStatus {
	switch status {
	case UserStatusWatching, UserStatusPlanned, UserStatusFavorite, UserStatusCompleted, UserStatusDropped:
		return status
	default:
		return UserStatusPlanned
	}
}

// Valid reports whether the value is one of the five statuses offered by the
// UI.
func (s UserStatus) Valid() bool {
	return s >= UserStatusWatching && s <= UserStatusDropped
}

// Voiceover is a translation team and the episodes it covers. HasKodik marks
// voices with episodes that only the Kodik player carries.
type Voiceover struct {
	Name     string
	Episodes []string
	Missing  []string
	HasKodik bool
	// Qualities is the stream ladder of a Kinopoisk voice. It is empty for
	// AnimeLIB, where qualities belong to the episode player and not to the
	// voice. The title page fills it from a background probe and Save caches it.
	Qualities []int
}

// Season is one season of a series. AnimeLIB titles have no explicit seasons
// (their episodes are a flat list), while a Kinopoisk series repeats episode
// numbers across seasons, so the title window groups them.
type Season struct {
	Number   int
	Episodes []string
}

// Title is everything the title window needs, plus the saved selection.
type Title struct {
	ID int64
	// Source is SourceAnimeLIB or SourceKinopoisk and decides which resolver
	// may open the title.
	Source string
	// ExternalID is the id inside Source (the Kinopoisk film id).
	ExternalID      string
	AnimeID         int64
	Slug            string
	Russian         string
	Original        string
	Status          Status
	UserStatus      UserStatus
	IsMovie         bool
	CoverURL        string
	CoverPath       string
	CoverNeedsRetry bool
	Episodes        []string
	EpisodesTotal   int
	Seasons         []Season
	Voiceovers      []Voiceover
	SelectedVoice   string
	SelectedEpisode string
	IMDbID          string
	Season          int
	ResumeAt        float64
	Duration        float64
	UpdatedAt       int64
	// StatusOrder is the title's position inside its user status. The all-title
	// order is not stored in this struct because History already returns it.
	StatusOrder int64
	// Chapters is the reader chapter list. It is fresh on every load and is
	// never written to SQLite, just like anime episode lists.
	Chapters []ReadingChapter
}

// IsKinopoisk reports whether the title came from Kinopoisk.
func (t Title) IsKinopoisk() bool { return t.Source == SourceKinopoisk }

// IsReading reports whether the title opens in the manga or ranobe reader.
func (t Title) IsReading() bool { return IsReadingSource(t.Source) }

// IsRanobe reports whether the title is a text novel.
func (t Title) IsRanobe() bool { return IsRanobeSource(t.Source) }

// ReadingChapter is one chapter of a reading title. Translation is the team
// that published this chapter; a chapter number can exist in several teams and
// each team has its own branch id for the content request.
type ReadingChapter struct {
	Key         string
	Volume      string
	Number      string
	Name        string
	Translation string
	BranchID    int64
}

// ReadingPage is one scanned manga page returned by the chapter endpoint.
type ReadingPage struct {
	URL    string
	Width  int
	Height int
	Ratio  string
}

// ReadingContent is a loaded manga or ranobe chapter. Pages is used by manga,
// Paragraphs by ranobe; the two are mutually exclusive in normal responses.
type ReadingContent struct {
	ID         int64
	Volume     string
	Number     string
	Name       string
	Pages      []ReadingPage
	Paragraphs []string
}

// ReadingChapterKey builds the stable chapter key used by history. Volume is
// part of the key because several sites restart chapter numbers in every
// volume; an empty volume is stored as "0".
func ReadingChapterKey(volume, number string) string {
	volume = strings.TrimSpace(volume)
	number = strings.TrimSpace(number)
	if volume == "" {
		volume = "0"
	}
	return volume + ":" + number
}

// SplitReadingChapterKey splits a key produced by ReadingChapterKey.
func SplitReadingChapterKey(key string) (volume, number string) {
	key = strings.TrimSpace(key)
	volume, number, ok := strings.Cut(key, ":")
	if !ok {
		return "0", key
	}
	if volume == "" {
		volume = "0"
	}
	return volume, number
}

// ReadingChapterLabel formats a chapter key for the history row and reader.
func ReadingChapterLabel(key string) string {
	volume, number := SplitReadingChapterKey(key)
	if volume == "" || volume == "0" {
		return "Глава " + number
	}
	return "Том " + volume + " · Глава " + number
}

// EpisodeVoiceover is one translation team that published a new episode.
type EpisodeVoiceover struct {
	Name string
	// KodikOnly reports that AnimeLIB's own player does not carry this episode,
	// so it will be watched through Kodik.
	KodikOnly bool
}

// EpisodeNotification describes one notification the background checker wants
// to show. Preferred marks the second notification, sent when the user's own
// saved voiceover catches up with an episode that was already announced for
// another team.
type EpisodeNotification struct {
	Title         string
	Episode       string
	Voiceovers    []EpisodeVoiceover
	TitleReleased bool
	Preferred     bool
}

// SplitEpisodeKey parses a season-prefixed key. A plain number is a film or a
// season-less episode: season is 0 in that case.
func SplitEpisodeKey(key string) (season, episode int, ok bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0, 0, false
	}
	if episode, err := strconv.Atoi(key); err == nil {
		return 0, episode, episode > 0
	}
	if _, err := fmt.Sscanf(key, "S%dE%d", &season, &episode); err == nil && season > 0 && episode > 0 {
		return season, episode, true
	}
	return 0, 0, false
}

// IsSeasonKey reports whether the key carries an explicit season prefix.
func IsSeasonKey(key string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(key)), "S")
}

// EpisodeLabel strips the season prefix for display. The episode grid shows the
// episode number only; the season is chosen by the season strip above it.
func EpisodeLabel(key string) string {
	if _, episode, ok := SplitEpisodeKey(key); ok && episode > 0 && IsSeasonKey(key) {
		return strconv.Itoa(episode)
	}
	return key
}

// Segment is an introdb chapter type that the player can skip.
type Segment string

const (
	SegmentRecap       Segment = "recap"
	SegmentIntro       Segment = "intro"
	SegmentOutro       Segment = "outro"
	SegmentPreview     Segment = "preview"
	SegmentCredits     Segment = "credits"
	SegmentPostCredits Segment = "post_credits"
)

var segmentOrder = []Segment{
	SegmentRecap,
	SegmentIntro,
	SegmentOutro,
	SegmentPreview,
	SegmentCredits,
	SegmentPostCredits,
}

// Segments returns every introdb segment type in display order. The copy keeps
// callers from reordering the package table.
func Segments() []Segment { return append([]Segment(nil), segmentOrder...) }

// Bounds is a window rectangle in the coordinate space used by Chromium's
// command-line switches.
type Bounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Segment is an introdb chapter type that the player can skip.
