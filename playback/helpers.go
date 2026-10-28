package playback

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"github.com/arfush/yo/libsocial"
)

func pickLibSocialQuality(links []libsocial.QualityLink, preferred int) libsocial.QualityLink {
	if len(links) == 0 {
		return libsocial.QualityLink{}
	}
	if preferred <= 0 {
		preferred = 1080
	}
	sorted := append([]libsocial.QualityLink(nil), links...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Quality < sorted[j].Quality })
	for _, link := range sorted {
		if link.Quality == preferred {
			return link
		}
	}
	lower := libsocial.QualityLink{}
	for _, link := range sorted {
		if link.Quality < preferred {
			lower = link
		}
	}
	if lower.Quality > 0 {
		return lower
	}
	return sorted[0]
}

func selectVoicePlayers(players []libsocial.EpisodePlayer, voice string) (*libsocial.EpisodePlayer, *libsocial.EpisodePlayer) {
	var libsocialPlayer *libsocial.EpisodePlayer
	var kodikPlayer *libsocial.EpisodePlayer
	for playerIndex := range players {
		player := &players[playerIndex]
		if !sameTeam(player.TeamName, voice) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(player.Player)) {
		case "animelib":
			if libsocialPlayer == nil && len(player.Qualities) > 0 {
				libsocialPlayer = player
			}
		case "kodik":
			if kodikPlayer == nil && player.Src != "" {
				kodikPlayer = player
			}
		}
	}
	return libsocialPlayer, kodikPlayer
}

// pickSubtitleTrack enforces the playback preference: ASS first, VTT only when
// the selected player has no ASS track.
func pickSubtitleTrack(tracks []libsocial.SubtitleTrack) (libsocial.SubtitleTrack, bool) {
	var fallback libsocial.SubtitleTrack
	hasFallback := false
	for _, track := range tracks {
		if strings.TrimSpace(track.Src) == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(track.Format)) {
		case "ass":
			return track, true
		case "vtt":
			if !hasFallback {
				fallback = track
				hasFallback = true
			}
		}
	}
	return fallback, hasFallback
}

func (s *Session) subtitleForPlayer(ctx context.Context, player *libsocial.EpisodePlayer) (string, string) {
	if player == nil {
		return "", ""
	}
	track, ok := pickSubtitleTrack(player.Subtitles)
	if !ok {
		return "", ""
	}
	label := strings.ToUpper(strings.TrimSpace(track.Format))
	src := strings.TrimSpace(track.Src)
	if isAbsoluteURL(src) {
		return src, label
	}
	bases, err := s.libsocialBases(ctx)
	if err != nil {
		return "", ""
	}
	return resolveRelativeSubtitle(src, player.VideoDomain, bases), label
}

// videoBaseHost maps the API's video_domain to the CDN host that actually
// stores the file. The constants endpoint lists several servers and a relative
// href belongs to exactly one of them: joining it to the first server yields a
// 404 for titles hosted on a secondary one.
func videoBaseHost(domain string) string {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "secondary_1":
		return "video2.cdnlibs.org"
	case "secondary_2":
		return "video1.imglib.info"
	default:
		return "video1.cdnlibs.org"
	}
}

// pickVideoBase returns the server matching the player's domain and falls back
// to the first known server when the list does not contain it.
func pickVideoBase(bases []string, domain string) string {
	wantedHost := videoBaseHost(domain)
	for _, base := range bases {
		parsed, err := url.Parse(strings.TrimSpace(base))
		if err == nil && strings.EqualFold(parsed.Hostname(), wantedHost) {
			return base
		}
	}
	if len(bases) > 0 {
		return bases[0]
	}
	return ""
}

// resolveRelativeSubtitle joins a relative subtitle path to the video server
// named by the player's video_domain. The API currently returns absolute
// subtitle URLs, but relative paths exist in older clients' data.
func resolveRelativeSubtitle(src, domain string, bases []string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	if isAbsoluteURL(src) {
		return src
	}
	return libsocial.JoinVideoBase(pickVideoBase(bases, domain), src)
}

func sameTeam(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	return strings.EqualFold(left, right)
}

func isAbsoluteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.IsAbs()
}
