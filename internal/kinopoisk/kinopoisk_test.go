package kinopoisk

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseInput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID int
		movie  bool
		wantOK bool
	}{
		{name: "film link", input: "https://www.kinopoisk.ru/film/5494049/?socialAlias=abc", wantID: 5494049, movie: true, wantOK: true},
		{name: "series link", input: "https://www.kinopoisk.ru/series/843389/", wantID: 843389, wantOK: true},
		{name: "mirror zone", input: "kinopoisk.ru/film/77439/", wantID: 77439, movie: true, wantOK: true},
		{name: "uppercase", input: "https://WWW.KINOPOISK.RU/SERIES/1100777", wantID: 1100777, wantOK: true},
		{name: "explicit prefix", input: "kinopoisk:88198", wantID: 88198, wantOK: true},
		{name: "short prefix", input: "kp: 5942378", wantID: 5942378, wantOK: true},
		{name: "bare id", input: "88198", wantID: 88198, wantOK: true},
		{name: "hd is unsupported", input: "https://hd.kinopoisk.ru/film/1234", wantOK: false},
		{name: "anime slug", input: "cyberpunk-edgerunners", wantOK: false},
		{name: "animelib film path", input: "https://v5.animelib.org/film/12345", wantOK: false},
		{name: "empty", input: "   ", wantOK: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			link, ok := ParseInput(test.input)
			if ok != test.wantOK {
				t.Fatalf("ParseInput(%q) ok = %v, want %v", test.input, ok, test.wantOK)
			}
			if !ok {
				return
			}
			if link.ID != test.wantID {
				t.Fatalf("ParseInput(%q) id = %d, want %d", test.input, link.ID, test.wantID)
			}
			if link.IsMovie != test.movie {
				t.Fatalf("ParseInput(%q) movie = %v, want %v", test.input, link.IsMovie, test.movie)
			}
			if want := SlugPrefix + strconv.Itoa(test.wantID); link.Slug() != want {
				t.Fatalf("slug = %q, want %q", link.Slug(), want)
			}
		})
	}
}

func TestEpisodeKeys(t *testing.T) {
	if got := EpisodeKey(0, 1); got != "1" {
		t.Fatalf("film key = %q", got)
	}
	if got := EpisodeKey(2, 5); got != "S2E5" {
		t.Fatalf("series key = %q", got)
	}
	season, episode, ok := SplitEpisodeKey("S2E5")
	if !ok || season != 2 || episode != 5 {
		t.Fatalf("SplitEpisodeKey(S2E5) = %d, %d, %v", season, episode, ok)
	}
	season, episode, ok = SplitEpisodeKey("5")
	if !ok || season != 0 || episode != 5 {
		t.Fatalf("SplitEpisodeKey(5) = %d, %d, %v", season, episode, ok)
	}
	if _, _, ok := SplitEpisodeKey("special"); ok {
		t.Fatal("SplitEpisodeKey accepted a non-episode key")
	}
	if got := EpisodeLabel("S2E5"); got != "5" {
		t.Fatalf("EpisodeLabel(S2E5) = %q", got)
	}
	if got := EpisodeLabel("5"); got != "5" {
		t.Fatalf("EpisodeLabel(5) = %q", got)
	}
}

func TestIframeURL(t *testing.T) {
	base := "https://theatre.stravers.live/?token_movie=abc&translation=82&token=xyz"
	parsed, err := url.Parse(IframeURL(base, 1, 5))
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("translation") != "82" || query.Get("token_movie") != "abc" {
		t.Fatalf("original parameters lost: %v", query)
	}
	if query.Get("season") != "1" || query.Get("episode") != "5" || query.Get("autoplay") != "1" {
		t.Fatalf("episode parameters missing: %v", query)
	}
	film, err := url.Parse(IframeURL(base, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if film.Query().Get("season") != "" || film.Query().Get("episode") != "" {
		t.Fatalf("film gained season parameters: %v", film.Query())
	}
}

func TestExtractFileList(t *testing.T) {
	html := []byte(`<html><script>const fileList = JSON.parse('{"type":"movie","all":{"theatrical":{"t62":{"BDRip":{"id":1,"translation":"\u0413\u0430\u0432\u0440\u0438\u043b\u043e\u0432"}}}}}');</script></html>`)
	raw, err := extractFileList(html)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Type string `json:"type"`
		All  struct {
			Theatrical map[string]map[string]struct {
				Translation string `json:"translation"`
			} `json:"theatrical"`
		} `json:"all"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("extracted payload is not JSON: %v (%s)", err, raw)
	}
	if got := payload.All.Theatrical["t62"]["BDRip"].Translation; got != "Гаврилов" {
		t.Fatalf("translation = %q", got)
	}
	if _, err := extractFileList([]byte("<html></html>")); err == nil {
		t.Fatal("expected an error for a page without a file list")
	}
}

func TestUnescapeJSString(t *testing.T) {
	cases := map[string]string{
		`{"a":"\u0413"}`: "{\"a\":\"Г\"}",
		`it\'s "ok"`:     `it's "ok"`,
		`line\nbreak`:    "line\nbreak",
		`\x41`:           "A",
	}
	for input, want := range cases {
		if got := unescapeJSString(input); got != want {
			t.Fatalf("unescapeJSString(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseSerialList(t *testing.T) {
	raw := []byte(`{"type":"serial","all":{
		"1":{"1":{"t82":{"id":1,"seasons":1,"episode":1,"id_translation":82},"t79":{"id":2,"seasons":1,"episode":1,"id_translation":79}},
		     "2":{"t82":{"id":3,"seasons":1,"episode":2,"id_translation":82}}},
		"2":{"1":{"t82":{"id":4,"seasons":2,"episode":1,"id_translation":82}}}
	}}`)
	catalog, err := parseFileList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Movie {
		t.Fatal("series parsed as a film")
	}
	want := []string{"S1E1", "S1E2", "S2E1"}
	if len(catalog.Episodes) != len(want) {
		t.Fatalf("episodes = %v", catalog.Episodes)
	}
	for index, key := range want {
		if catalog.Episodes[index] != key {
			t.Fatalf("episode %d = %q, want %q", index, catalog.Episodes[index], key)
		}
	}
	if len(catalog.Seasons) != 2 || catalog.Seasons[0].Number != 1 || len(catalog.Seasons[0].Episodes) != 2 {
		t.Fatalf("seasons = %+v", catalog.Seasons)
	}
	if got := catalog.Available[82]; len(got) != 3 {
		t.Fatalf("voice 82 = %v", got)
	}
	if got := catalog.Available[79]; len(got) != 1 || got[0] != "S1E1" {
		t.Fatalf("voice 79 = %v", got)
	}
}

func TestParseMovieList(t *testing.T) {
	raw := []byte(`{"type":"movie","all":{"directors":[],"theatrical":{
		"t66":{"BDRip":{"id":1,"translation":"Дублированный"}},
		"t62":{"BDRip":{"id":2,"translation":"Гаврилов"}}
	}}}`)
	catalog, err := parseFileList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Movie {
		t.Fatal("film parsed as a series")
	}
	if len(catalog.Episodes) != 1 || catalog.Episodes[0] != "1" {
		t.Fatalf("episodes = %v", catalog.Episodes)
	}
	if len(catalog.Seasons) != 1 || catalog.Seasons[0].Number != 0 {
		t.Fatalf("seasons = %+v", catalog.Seasons)
	}
	for _, voice := range []int{62, 66} {
		if got := catalog.Available[voice]; len(got) != 1 || got[0] != "1" {
			t.Fatalf("voice %d = %v", voice, got)
		}
	}
}

func TestParseSkipRanges(t *testing.T) {
	ranges := parseSkipRanges("0-34")
	if len(ranges) != 1 || ranges[0].Start != 0 || ranges[0].End != 34 {
		t.Fatalf("ranges = %+v", ranges)
	}
	ranges = parseSkipRanges("0-34, 1200-1290")
	if len(ranges) != 2 || ranges[1].Start != 1200 {
		t.Fatalf("ranges = %+v", ranges)
	}
	for _, invalid := range []string{"", "34", "10-5", "a-b"} {
		if got := parseSkipRanges(invalid); got != nil {
			t.Fatalf("parseSkipRanges(%q) = %+v", invalid, got)
		}
	}
}

func TestMatchSourceAndQuality(t *testing.T) {
	sources := []manifestSource{
		{AudioID: 1, Label: "Дубляж", Quality: map[string]string{"1080": "https://cdn/1-1080.m3u8", "720": "https://cdn/1-720.m3u8"}},
		{AudioID: 2, Label: "Гаврилов", Quality: map[string]string{"1080": "https://cdn/2-1080.m3u8", "720": "https://cdn/2-720.m3u8"}},
	}
	if index := matchSource(sources, "https://cdn/2-720.m3u8?token=1"); index != 1 {
		t.Fatalf("matched source %d, want 1", index)
	}
	if index := matchSource(sources, ""); index != 0 {
		t.Fatalf("empty capture matched source %d, want 0", index)
	}
	url, quality := pickQuality(sources[1], 720)
	if url != "https://cdn/2-720.m3u8" || quality != 720 {
		t.Fatalf("pickQuality = %q, %d", url, quality)
	}
	url, quality = pickQuality(sources[1], 900)
	if url != "https://cdn/2-720.m3u8" || quality != 720 {
		t.Fatalf("nearest lower = %q, %d", url, quality)
	}
	url, quality = pickQuality(sources[1], 360)
	if url != "https://cdn/2-720.m3u8" || quality != 720 {
		t.Fatalf("lowest available = %q, %d", url, quality)
	}
	// The quality block is built from the same set: ascending, with broken
	// entries dropped instead of rendered as dead buttons.
	sources[1].Quality["480"] = "not a url"
	sources[1].Quality["2160"] = "https://cdn/2-2160.m3u8"
	if got := sourceQualities(sources[1]); len(got) != 3 || got[0] != 720 || got[1] != 1080 || got[2] != 2160 {
		t.Fatalf("sourceQualities = %v", got)
	}
	if got := sourceQualities(manifestSource{}); got != nil {
		t.Fatalf("empty source qualities = %v", got)
	}
}

// A film mirrors every quality on two CDN hosts and joins them with " or ".
// The raw value is not a URL, so the resolver must pick one host before handing
// it to mpv and still recognise the source when the player used the other one.
func TestMirrorURLs(t *testing.T) {
	const value = "https://a.vk/0/x/master.m3u8 or https://b.vk/0/x/master.m3u8"
	mirrors := splitMirrors(value)
	if len(mirrors) != 2 || mirrors[0] != "https://a.vk/0/x/master.m3u8" || mirrors[1] != "https://b.vk/0/x/master.m3u8" {
		t.Fatalf("splitMirrors = %v", mirrors)
	}
	if got := normalizeStreamURL(value); got != "https://a.vk/0/x/master.m3u8" {
		t.Fatalf("normalizeStreamURL = %q", got)
	}
	if got := normalizeStreamURL("not a url"); got != "" {
		t.Fatalf("normalizeStreamURL accepted %q", got)
	}
	if strings.ContainsAny(normalizeStreamURL(value), " ") {
		t.Fatalf("normalized URL still has a space")
	}

	sources := []manifestSource{
		{AudioID: 1, Label: "Дубляж", Quality: map[string]string{"1080": "https://a.vk/1/1080.m3u8 or https://b.vk/1/1080.m3u8"}},
		{AudioID: 2, Label: "Оригинал", Quality: map[string]string{"1080": "https://a.vk/2/1080.m3u8 or https://b.vk/2/1080.m3u8"}},
	}
	if index := matchSource(sources, "https://b.vk/2/1080.m3u8?token=1"); index != 1 {
		t.Fatalf("mirror capture matched source %d, want 1", index)
	}
	url, quality := pickQuality(sources[0], 1080)
	if url != "https://a.vk/1/1080.m3u8" || quality != 1080 {
		t.Fatalf("pickQuality = %q, %d", url, quality)
	}
}

func TestResolveFreshClearsTheCache(t *testing.T) {
	resolver := NewResolver(nil)
	request := Request{Iframe: "https://theatre.example/room", Season: 1, Episode: 3, TranslationID: 7, Quality: 1080}
	resolver.cache[cacheKey(request)] = cacheEntry{expires: time.Now().Add(time.Hour)}
	// The resolver has no browser manager, so the fresh call fails, but the
	// stale manifest must be gone before the attempt.
	_, _ = resolver.ResolveFresh(context.Background(), request)
	if _, ok := resolver.cached(cacheKey(request)); ok {
		t.Fatal("ResolveFresh left the stale manifest in the cache")
	}
}

func TestConfigDefaults(t *testing.T) {
	config := Config{}.withDefaults()
	if config.Wrapper != defaultWrapper {
		t.Fatalf("wrapper = %q", config.Wrapper)
	}
	if config.Players != defaultPlayers {
		t.Fatalf("players = %q", config.Players)
	}
	custom := Config{Wrapper: "linkpp.cx"}.withDefaults()
	if custom.Wrapper != "https://linkpp.cx/" {
		t.Fatalf("custom wrapper = %q", custom.Wrapper)
	}
	if custom.Players != "https://p.linkpp.cx/api/players" {
		t.Fatalf("derived players = %q", custom.Players)
	}
	override := Config{Players: "https://example.com/api/players"}.withDefaults()
	if override.Players != "https://example.com/api/players" {
		t.Fatalf("overridden players = %q", override.Players)
	}
}
