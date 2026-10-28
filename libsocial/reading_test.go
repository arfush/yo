package libsocial

import (
	"encoding/json"
	"testing"
)

func TestParseReadingInputDetectsSitesAndSlugs(t *testing.T) {
	cases := []struct {
		input string
		site  int
		slug  string
		ok    bool
	}{
		{"https://mangalib.me/manga/206--one-piece", SiteMangaLib, "206--one-piece", true},
		{"https://mangalib.org/manga/206--one-piece", SiteMangaLib, "206--one-piece", true},
		{"https://www.mangalib.me/ru/manga/206--one-piece", SiteMangaLib, "206--one-piece", true},
		{"https://slashlib.me/manga/7580--i-alone-level-up", SiteYaoiLib, "7580--i-alone-level-up", true},
		{"https://yaoilib.net/manga/7580--i-alone-level-up", SiteYaoiLib, "7580--i-alone-level-up", true},
		{"https://v1.yaoilib.net/manga/7580--i-alone-level-up", SiteYaoiLib, "7580--i-alone-level-up", true},
		{"https://ranobelib.me/manga/38728--supreme-magus", SiteRanobeLib, "38728--supreme-magus", true},
		{"https://hentailib.me/manga/123--some-title", SiteHentaiLib, "123--some-title", true},
		{"https://hentailib.org/manga/123--some-title", SiteHentaiLib, "123--some-title", true},
		{"mangalib.me/manga/206--one-piece", SiteMangaLib, "206--one-piece", true},
		// Chapter links still resolve to the title slug.
		{"https://mangalib.me/manga/206--one-piece/v1/c54644", SiteMangaLib, "206--one-piece", true},
		// AnimeLIB and everything else are not reading links.
		{"https://v5.animelib.org/anime/123--title", 0, "", false},
		{"https://youtube.com/watch?v=abc", 0, "", false},
		{"206--one-piece", 0, "", false},
		{"", 0, "", false},
	}
	for _, tc := range cases {
		link, ok, err := ParseReadingInput(tc.input)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.input, err)
		}
		if ok != tc.ok {
			t.Fatalf("%s: ok = %v, want %v", tc.input, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if link.SiteID != tc.site || link.Slug != tc.slug {
			t.Fatalf("%s: got site=%d slug=%q, want site=%d slug=%q", tc.input, link.SiteID, link.Slug, tc.site, tc.slug)
		}
	}
}

func TestParseReadingInputRejectsBadSlugs(t *testing.T) {
	// A reading link with a broken slug is still a reading link: ok stays true
	// so the library router keeps it out of the anime resolver and surfaces the
	// parse error to the user.
	if _, ok, err := ParseReadingInput("https://mangalib.me/manga/"); !ok || err == nil {
		t.Fatalf("missing slug: ok=%v err=%v, want a reading-link error", ok, err)
	}
}

func TestNormalizeReadingImageURL(t *testing.T) {
	cases := []struct {
		raw  string
		site int
		want string
	}{
		{"//manga/one-piece/chapters/54644/01.jpg", SiteMangaLib, "https://img3.cdnlibs.org/manga/one-piece/chapters/54644/01.jpg"},
		{"/manga/one-piece/chapters/54644/02.jpg", SiteMangaLib, "https://img3.cdnlibs.org/manga/one-piece/chapters/54644/02.jpg"},
		{"manga/one-piece/chapters/54644/03.jpg", SiteMangaLib, "https://img3.cdnlibs.org/manga/one-piece/chapters/54644/03.jpg"},
		{"//manga/x/chapters/1/04.jpg", SiteYaoiLib, "https://img3.hentaicdn.org/manga/x/chapters/1/04.jpg"},
		{"//manga/x/chapters/1/05.jpg", SiteHentaiLib, "https://img3h.hentaicdn.org/manga/x/chapters/1/05.jpg"},
		{"https://cdn.example.com/img.jpg", SiteMangaLib, "https://cdn.example.com/img.jpg"},
		{"//cdn.example.com/img.jpg", SiteMangaLib, "https://cdn.example.com/img.jpg"},
		{"", SiteMangaLib, ""},
	}
	for _, tc := range cases {
		if got := NormalizeReadingImageURL(tc.raw, tc.site); got != tc.want {
			t.Fatalf("NormalizeReadingImageURL(%q, %d) = %q, want %q", tc.raw, tc.site, got, tc.want)
		}
	}
}

func TestParseReadingContentProseMirror(t *testing.T) {
	raw := []byte(`{"data":{"content":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Первый абзац."}]},{"type":"image","attrs":{"src":"/uploads/1.jpg"}},{"type":"paragraph","content":[{"type":"text","text":"Второй"},{"type":"hardBreak"},{"type":"text","text":"абзац."}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Пункт списка."}]}]}]},{"type":"horizontalRule"}]}}}`)
	var response struct {
		Data struct {
			Content json.RawMessage `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	html, paragraphs, images := parseReadingContent(response.Data.Content, SiteRanobeLib)
	if html != "" {
		t.Fatalf("html = %q, want empty", html)
	}
	want := []string{"Первый абзац.", "Второй\nабзац.", "Пункт списка.", "***"}
	if len(paragraphs) != len(want) {
		t.Fatalf("paragraphs = %#v, want %#v", paragraphs, want)
	}
	for i := range want {
		if paragraphs[i] != want[i] {
			t.Fatalf("paragraph %d = %q, want %q", i, paragraphs[i], want[i])
		}
	}
	// The picture sits directly after the first paragraph, the horizontal
	// rule became the "***" separator.
	if len(images) != 1 {
		t.Fatalf("images = %#v, want one illustration", images)
	}
	if images[0].After != 1 || images[0].URL != "https://img3.cdnlibs.org/uploads/1.jpg" {
		t.Fatalf("image = %+v, want URL https://img3.cdnlibs.org/uploads/1.jpg after paragraph 1", images[0])
	}
}

func TestParseReadingContentHTMLString(t *testing.T) {
	html, paragraphs, images := parseReadingContent(json.RawMessage(`"<p>Текст</p>"`), SiteRanobeLib)
	if html != "<p>Текст</p>" || len(paragraphs) != 0 {
		t.Fatalf("html = %q, paragraphs = %#v", html, paragraphs)
	}
	if len(images) != 0 {
		t.Fatalf("images = %#v, want none", images)
	}
}

func TestParseReadingContentHTMLExtractsIllustrations(t *testing.T) {
	raw := json.RawMessage(`"<p>Абзац один</p><p>Абзац два</p><img src=\"/uploads/1.jpg\"><p>Абзац три</p><img data-src=\"//cdn.test/2.jpg\"><img src=\"data:image/gif;base64,AAA\">"`)
	html, _, images := parseReadingContent(raw, SiteRanobeLib)
	if html == "" {
		t.Fatal("html must be kept for the library paragraph parser")
	}
	want := []ReadingImage{
		{URL: "https://img3.cdnlibs.org/uploads/1.jpg", After: 2},
		{URL: "https://cdn.test/2.jpg", After: 3},
	}
	if len(images) != len(want) {
		t.Fatalf("images = %#v, want %#v", images, want)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("image %d = %+v, want %+v", i, images[i], want[i])
		}
	}
}

func TestParseReadingContentHTMLTracksParagraphCountWithoutPTags(t *testing.T) {
	raw := json.RawMessage(`"<div>Строка</div><div><img src=\"/a.jpg\"></div><div>Ещё</div>"`)
	_, _, images := parseReadingContent(raw, SiteRanobeLib)
	if len(images) != 1 || images[0].After != 1 {
		t.Fatalf("images = %#v, want one picture after the first div", images)
	}
}

func TestSiteIDForSource(t *testing.T) {
	cases := []struct {
		source string
		site   int
		ok     bool
	}{
		{"mangalib", SiteMangaLib, true},
		{"yaoilib", SiteYaoiLib, true},
		{"ranobelib", SiteRanobeLib, true},
		{"hentailib", SiteHentaiLib, true},
		{"animelib", 0, false},
		{"filmlib", 0, false},
	}
	for _, tc := range cases {
		site, ok := SiteIDForSource(tc.source)
		if site != tc.site || ok != tc.ok {
			t.Fatalf("SiteIDForSource(%q) = %d, %v; want %d, %v", tc.source, site, ok, tc.site, tc.ok)
		}
	}
}
