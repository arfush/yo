package libsocial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// LibSocial site ids. The API is shared, but the Site-Id header and the image
// CDN still differ between the sites.
const (
	SiteMangaLib  = 1
	SiteYaoiLib   = 2
	SiteRanobeLib = 3
	SiteHentaiLib = 4
	SiteAnimeLib  = 5
)

var readingOrigins = map[int]string{
	SiteMangaLib:  "https://mangalib.me",
	SiteYaoiLib:   "https://slashlib.me",
	SiteRanobeLib: "https://ranobelib.me",
	SiteHentaiLib: "https://hentailib.me",
	SiteAnimeLib:  webOrigin,
}

var readingImageBases = map[int]string{
	SiteMangaLib:  "https://img3.cdnlibs.org",
	SiteYaoiLib:   "https://img3.hentaicdn.org",
	SiteRanobeLib: "https://img3.cdnlibs.org",
	SiteHentaiLib: "https://img3h.hentaicdn.org",
}

// ReadingOrigin returns the site origin used in browser-like API headers.
func ReadingOrigin(siteID int) string {
	if origin := readingOrigins[siteID]; origin != "" {
		return origin
	}
	return webOrigin
}

func siteOrigin(siteID int) string { return ReadingOrigin(siteID) }

// ReadingImageBase returns the CDN that serves chapter page images.
func ReadingImageBase(siteID int) string { return readingImageBases[siteID] }

// BrowserUserAgent is the browser-like user agent used by the shared API
// client. The image CDN checks it as well.
func BrowserUserAgent() string { return userAgent }

// ReadingLink is a parsed LibSocial title URL.
type ReadingLink struct {
	SiteID int
	Slug   string
	Host   string
}

// ParseReadingInput recognizes MangaLib, SlashLIB/YaoiLib, RanobeLib and
// HentaiLib links. AnimeLIB links deliberately return ok=false so the existing
// anime resolver keeps handling them.
func ParseReadingInput(input string) (ReadingLink, bool, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ReadingLink{}, false, nil
	}
	if !strings.Contains(raw, "://") {
		if !strings.Contains(strings.SplitN(raw, "/", 2)[0], ".") {
			return ReadingLink{}, false, nil
		}
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return ReadingLink{}, false, nil
	}
	siteID, ok := readingSiteForHost(parsed.Hostname())
	if !ok {
		return ReadingLink{}, false, nil
	}
	slug, err := readingSlugFromPath(parsed.Path)
	if err != nil {
		return ReadingLink{}, true, err
	}
	return ReadingLink{SiteID: siteID, Slug: slug, Host: parsed.Hostname()}, true, nil
}

func readingSiteForHost(host string) (int, bool) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	switch {
	case hostMatches(host, "mangalib.me"), hostMatches(host, "mangalib.org"):
		return SiteMangaLib, true
	case hostMatches(host, "slashlib.me"), hostMatches(host, "yaoilib.net"):
		return SiteYaoiLib, true
	case hostMatches(host, "ranobelib.me"), hostMatches(host, "ranobelib.org"):
		return SiteRanobeLib, true
	case hostMatches(host, "hentailib.me"), hostMatches(host, "hentailib.org"), hostMatches(host, "hentailib.com"):
		return SiteHentaiLib, true
	default:
		return 0, false
	}
}

func hostMatches(host, base string) bool {
	return host == base || strings.HasSuffix(host, "."+base)
}

func readingSlugFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for index, part := range parts {
		switch strings.ToLower(part) {
		case "manga", "ranobe", "comic", "book":
			if index+1 >= len(parts) {
				break
			}
			slug, err := url.PathUnescape(parts[index+1])
			if err != nil {
				return "", fmt.Errorf("%w: cannot decode the title slug", ErrInvalidInput)
			}
			slug = strings.TrimSpace(slug)
			if slug != "" {
				return slug, nil
			}
		}
	}
	if len(parts) == 1 {
		slug, err := url.PathUnescape(parts[0])
		if err != nil {
			return "", fmt.Errorf("%w: cannot decode the title slug", ErrInvalidInput)
		}
		slug = strings.TrimSpace(slug)
		// A bare section keyword (like /manga/) is not a title slug.
		switch strings.ToLower(slug) {
		case "manga", "ranobe", "comic", "book", "ru":
			return "", fmt.Errorf("%w: link has no title slug", ErrInvalidInput)
		}
		if slug != "" {
			return slug, nil
		}
	}
	return "", fmt.Errorf("%w: link has no title slug", ErrInvalidInput)
}

// SiteIDForSource maps the persisted source name back to a LibSocial site.
func SiteIDForSource(source string) (int, bool) {
	switch source {
	case "mangalib":
		return SiteMangaLib, true
	case "yaoilib":
		return SiteYaoiLib, true
	case "ranobelib":
		return SiteRanobeLib, true
	case "hentailib":
		return SiteHentaiLib, true
	default:
		return 0, false
	}
}

// ReadingTitleData is the fresh metadata and chapter list of a reading title.
// Chapters are not persisted.
type ReadingTitleData struct {
	SiteID      int
	ID          int64
	Name        string
	RusName     string
	EngName     string
	Slug        string
	SlugURL     string
	StatusID    int
	StatusLabel string
	TypeID      int
	TypeLabel   string
	Cover       Cover
	Chapters    []ReadingChapterData
}

// ReadingChapterData is one published branch of a chapter. A chapter number
// can appear in several translations, each with its own branch id.
type ReadingChapterData struct {
	ID              int64
	Volume          string
	Number          string
	NumberSecondary string
	Name            string
	Translation     string
	BranchID        int64
}

// ReadingChapterContent is a loaded chapter body. Manga responses carry page
// images; ranobe responses may carry HTML or a ProseMirror JSON document.
type ReadingChapterContent struct {
	ID         int64
	Volume     string
	Number     string
	Name       string
	Pages      []ReadingPage
	RawContent string
	Paragraphs []string
	Images     []ReadingImage
}

// ReadingPage is one scanned page returned by the chapter endpoint.
type ReadingPage struct {
	URL    string
	Width  int
	Height int
	Ratio  string
}

// ReadingImage is one ranobe illustration placed after the given number of
// text paragraphs (see catalog.ReadingImage).
type ReadingImage struct {
	URL   string
	After int
}

type readingMetaResponse struct {
	Data struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		RusName string `json:"rus_name"`
		EngName string `json:"eng_name"`
		Slug    string `json:"slug"`
		SlugURL string `json:"slug_url"`
		Status  struct {
			ID    int    `json:"id"`
			Label string `json:"label"`
		} `json:"status"`
		Type struct {
			ID    int    `json:"id"`
			Label string `json:"label"`
		} `json:"type"`
		Cover Cover `json:"cover"`
	} `json:"data"`
}

type readingChaptersResponse struct {
	Data []readingChapterItem `json:"data"`
}

type readingChapterItem struct {
	ID              int64           `json:"id"`
	Volume          flexString      `json:"volume"`
	Number          flexString      `json:"number"`
	NumberSecondary flexString      `json:"number_secondary"`
	Name            string          `json:"name"`
	Branches        []readingBranch `json:"branches"`
}

type readingBranch struct {
	ID    int64         `json:"id"`
	Teams []readingTeam `json:"teams"`
}

type readingTeam struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type readingChapterResponse struct {
	Data struct {
		ID      int64           `json:"id"`
		Volume  flexString      `json:"volume"`
		Number  flexString      `json:"number"`
		Name    string          `json:"name"`
		Content json.RawMessage `json:"content"`
		Pages   []struct {
			URL    string     `json:"url"`
			Image  string     `json:"image"`
			Width  int        `json:"width"`
			Height int        `json:"height"`
			Ratio  flexString `json:"ratio"`
		} `json:"pages"`
	} `json:"data"`
}

// readingRichNode is the subset of ProseMirror nodes used by LibSocial
// ranobe chapters. The API returns the document as an object for some titles
// and as HTML for others, so both shapes are accepted.
type readingRichNode struct {
	Type    string            `json:"type"`
	Text    string            `json:"text"`
	Attrs   readingNodeAttrs  `json:"attrs"`
	Content []readingRichNode `json:"content"`
}

// readingNodeAttrs carries the attributes the reader cares about: the image
// source of illustration nodes.
type readingNodeAttrs struct {
	Src string `json:"src"`
}

// parseReadingContent returns HTML for the legacy string shape and plain
// paragraphs for the ProseMirror object shape, together with the illustrations
// found in either shape.
func parseReadingContent(raw json.RawMessage, siteID int) (string, []string, []ReadingImage) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil, nil
	}
	if raw[0] == '"' {
		var html string
		if err := json.Unmarshal(raw, &html); err == nil {
			return html, nil, extractHTMLImages(html, siteID)
		}
		return "", nil, nil
	}
	if raw[0] != '{' && raw[0] != '[' {
		return "", nil, nil
	}
	var node readingRichNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return "", nil, nil
	}
	paragraphs := make([]string, 0, 32)
	images := make([]ReadingImage, 0, 8)
	collectReadingBlocks(node, &paragraphs, &images, siteID)
	return "", paragraphs, images
}

func collectReadingBlocks(node readingRichNode, out *[]string, images *[]ReadingImage, siteID int) {
	switch node.Type {
	case "paragraph", "heading", "codeBlock":
		if text := strings.TrimSpace(readingRichText(node)); text != "" {
			*out = append(*out, text)
		}
	case "bulletList", "orderedList", "blockquote", "listItem":
		for _, child := range node.Content {
			collectReadingBlocks(child, out, images, siteID)
		}
	case "image":
		// Illustrations keep their place in the document: the picture belongs
		// after the paragraphs collected so far.
		if src := strings.TrimSpace(node.Attrs.Src); src != "" {
			*images = append(*images, ReadingImage{
				URL:   NormalizeReadingImageURL(src, siteID),
				After: len(*out),
			})
		}
	case "horizontalRule":
		// The horizontal rule is the "***" scene separator of the printed
		// edition; the reader paints it as a faint centred line.
		*out = append(*out, "***")
	default:
		for _, child := range node.Content {
			collectReadingBlocks(child, out, images, siteID)
		}
	}
}

func readingRichText(node readingRichNode) string {
	var builder strings.Builder
	var walk func(readingRichNode)
	walk = func(current readingRichNode) {
		switch current.Type {
		case "text":
			builder.WriteString(current.Text)
		case "hardBreak":
			builder.WriteByte('\n')
		default:
			for _, child := range current.Content {
				walk(child)
			}
		}
	}
	walk(node)
	return builder.String()
}

// readingImageTagPattern matches an <img> element. The src attribute alone is
// not enough: some editors emit lazy-loaded pictures where the real address
// lives in data-src.
var (
	readingImageTagPattern  = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc\s*=\s*("([^"]*)"|'([^']*)')`)
	readingImageLazyPattern = regexp.MustCompile(`(?is)<img\b[^>]*\bdata-src\s*=\s*("([^"]*)"|'([^']*)')`)
	// readingParagraphOpenPattern and readingParagraphBreakPattern mirror the
	// boundaries of library.parseRanobeParagraphs, so the picture positions
	// line up with the paragraph list the reader displays.
	readingParagraphOpenPattern  = regexp.MustCompile(`(?is)<p\b[^>]*>`)
	readingParagraphBreakPattern = regexp.MustCompile(`(?is)</(?:p|div)\b[^>]*>|<br\s*/?>`)
)

// extractHTMLImages pulls the illustrations out of a legacy HTML chapter in
// document order. After counts the text paragraphs that precede each picture,
// using the same boundaries parseRanobeParagraphs relies on: <p> elements when
// the chapter has them, </div>/</p>/<br> otherwise.
func extractHTMLImages(html string, siteID int) []ReadingImage {
	starts := readingParagraphOpenPattern.FindAllStringSubmatchIndex(html, -1)
	boundaries := starts
	if len(starts) == 0 {
		boundaries = readingParagraphBreakPattern.FindAllStringSubmatchIndex(html, -1)
	}
	var images []ReadingImage
	seen := make(map[string]bool)
	emit := func(src string, pos int) {
		src = strings.TrimSpace(src)
		if src == "" || strings.HasPrefix(src, "data:") || seen[src] {
			return
		}
		after := 0
		for _, start := range boundaries {
			if start[0] >= pos {
				break
			}
			after++
		}
		seen[src] = true
		images = append(images, ReadingImage{
			URL:   NormalizeReadingImageURL(src, siteID),
			After: after,
		})
	}
	for _, match := range readingImageTagPattern.FindAllStringSubmatchIndex(html, -1) {
		emit(imgAttrValue(html, match), match[0])
	}
	for _, match := range readingImageLazyPattern.FindAllStringSubmatchIndex(html, -1) {
		emit(imgAttrValue(html, match), match[0])
	}
	return images
}

// imgAttrValue returns the captured attribute from an img regex match; the
// pattern captures both quote styles, so exactly one group is non-empty.
// Groups are: 1 = the quoted span, 2 = double-quoted content, 3 = single-quoted
// content.
func imgAttrValue(html string, match []int) string {
	if match[4] >= 0 {
		return html[match[4]:match[5]]
	}
	if match[6] >= 0 {
		return html[match[6]:match[7]]
	}
	return ""
}

// flexString accepts strings and numbers. LibSocial occasionally changes
// volume and chapter numbers between the two representations.
type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = flexString(value)
		return nil
	}
	*s = flexString(strings.Trim(string(data), " \t\r\n"))
	return nil
}

// FetchReading loads metadata and the complete chapter list of a reading
// title. The chapter list is returned fresh on every call.
func (c *Client) FetchReading(ctx context.Context, slug string, siteID int) (*ReadingTitleData, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}
	if _, ok := readingOrigins[siteID]; !ok || siteID == SiteAnimeLib {
		return nil, ErrUnknownSite
	}
	logger.Debug("loading reading metadata", "slug", slug, "site", siteID)
	endpoint := c.baseURL + "/api/manga/" + url.PathEscape(slug)
	body, err := c.getWithRetry(ctx, endpoint, siteID, 2)
	if err != nil {
		return nil, fmt.Errorf("cannot load title: %w", err)
	}
	var response readingMetaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid title response: %w", err)
	}
	data := response.Data
	if strings.TrimSpace(data.SlugURL) == "" {
		return nil, ErrTitleNotFound
	}
	chapters, err := c.FetchReadingChapters(ctx, data.SlugURL, siteID)
	if err != nil {
		return nil, err
	}
	logger.Debug("reading metadata loaded", "slug", data.SlugURL, "site", siteID, "chapters", len(chapters))
	return &ReadingTitleData{
		SiteID:      siteID,
		ID:          data.ID,
		Name:        data.Name,
		RusName:     data.RusName,
		EngName:     data.EngName,
		Slug:        data.Slug,
		SlugURL:     data.SlugURL,
		StatusID:    data.Status.ID,
		StatusLabel: data.Status.Label,
		TypeID:      data.Type.ID,
		TypeLabel:   data.Type.Label,
		Cover:       data.Cover,
		Chapters:    chapters,
	}, nil
}

// FetchReadingChapters downloads the flat chapter list and expands every
// translation branch into its own chapter entry.
func (c *Client) FetchReadingChapters(ctx context.Context, slug string, siteID int) ([]ReadingChapterData, error) {
	endpoint := c.baseURL + "/api/manga/" + url.PathEscape(slug) + "/chapters"
	body, err := c.getWithRetry(ctx, endpoint, siteID, 2)
	if err != nil {
		return nil, fmt.Errorf("cannot load chapter list: %w", err)
	}
	var response readingChaptersResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid chapter list response: %w", err)
	}
	out := make([]ReadingChapterData, 0, len(response.Data))
	for _, item := range response.Data {
		base := ReadingChapterData{
			ID:              item.ID,
			Volume:          string(item.Volume),
			Number:          string(item.Number),
			NumberSecondary: string(item.NumberSecondary),
			Name:            item.Name,
		}
		if len(item.Branches) == 0 {
			out = append(out, base)
			continue
		}
		for _, branch := range item.Branches {
			teams := branch.Teams
			if len(teams) == 0 {
				teams = []readingTeam{{}}
			}
			for _, team := range teams {
				chapter := base
				chapter.BranchID = branch.ID
				chapter.Translation = strings.TrimSpace(team.Name)
				out = append(out, chapter)
			}
		}
	}
	return out, nil
}

// FetchReadingChapter loads one chapter. branchID may be zero for titles that
// do not publish alternate branches.
func (c *Client) FetchReadingChapter(ctx context.Context, slug string, siteID int, volume, number string, branchID int64) (*ReadingChapterContent, error) {
	query := url.Values{}
	query.Set("number", strings.TrimSpace(number))
	if strings.TrimSpace(volume) == "" {
		volume = "0"
	}
	query.Set("volume", strings.TrimSpace(volume))
	if branchID > 0 {
		query.Set("branch_id", strconv.FormatInt(branchID, 10))
	}
	endpoint := c.baseURL + "/api/manga/" + url.PathEscape(slug) + "/chapter?" + query.Encode()
	body, err := c.getWithRetry(ctx, endpoint, siteID, 2)
	if err != nil {
		return nil, fmt.Errorf("cannot load chapter: %w", err)
	}
	var response readingChapterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid chapter response: %w", err)
	}
	data := response.Data
	rawContent, paragraphs, images := parseReadingContent(data.Content, siteID)
	content := &ReadingChapterContent{
		ID:         data.ID,
		Volume:     string(data.Volume),
		Number:     string(data.Number),
		Name:       data.Name,
		RawContent: rawContent,
		Paragraphs: paragraphs,
		Images:     images,
	}
	content.Pages = make([]ReadingPage, 0, len(data.Pages))
	for _, page := range data.Pages {
		raw := strings.TrimSpace(page.URL)
		if raw == "" {
			raw = strings.TrimSpace(page.Image)
		}
		if raw == "" {
			continue
		}
		content.Pages = append(content.Pages, ReadingPage{
			URL:    NormalizeReadingImageURL(raw, siteID),
			Width:  page.Width,
			Height: page.Height,
			Ratio:  string(page.Ratio),
		})
	}
	return content, nil
}

// NormalizeReadingImageURL turns the site-relative page path into a CDN URL.
// Page paths look like "//manga/one-piece/chapters/123/01.jpg" and must not be
// confused with protocol-relative absolute URLs.
func NormalizeReadingImageURL(raw string, siteID int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return raw
	}
	base := strings.TrimRight(ReadingImageBase(siteID), "/")
	if strings.HasPrefix(raw, "//") {
		rest := strings.TrimPrefix(raw, "//")
		first := strings.SplitN(rest, "/", 2)[0]
		if strings.Contains(first, ".") {
			return "https:" + raw
		}
		if base == "" {
			return raw
		}
		return base + "/" + rest
	}
	if base == "" {
		return raw
	}
	return base + "/" + strings.TrimLeft(raw, "/")
}
