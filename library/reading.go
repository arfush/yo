package library

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/arfush/yo/animelib"
	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/store"
)

// loadReading refreshes a LibSocial title. Chapters are never persisted: only
// metadata, the chosen translation and the chosen chapter are stored.
func (l *Library) loadReading(ctx context.Context, link animelib.ReadingLink, persist bool) (Title, error) {
	logger.Info("loading reading title", "site", link.SiteID, "slug", link.Slug, "persist", persist)
	token, err := l.readToken()
	if err != nil {
		logger.Error("cannot read AnimeLIB token", "error", err)
		return Title{}, err
	}
	client := animelib.New(token)
	data, err := client.FetchReading(ctx, link.Slug, link.SiteID)
	if err != nil {
		if ctx.Err() != nil {
			logger.Info("reading title load cancelled", "slug", link.Slug)
			return Title{}, ctx.Err()
		}
		logger.Error("reading title load failed", "slug", link.Slug, "site", link.SiteID, "error", err)
		return Title{}, err
	}

	title := titleFromReadingData(data)
	if persist {
		stored, err := l.upsertLoadedTitle(storedTitleFromReading(data, title))
		if err != nil {
			return Title{}, err
		}
		title.ID = stored.ID
		title.UserStatus = NormalizeUserStatus(UserStatus(stored.UserStatus))
		title.UpdatedAt = stored.UpdatedAt
	} else if existing, err := l.store.GetTitle(data.SlugURL); err != nil {
		return Title{}, err
	} else if existing != nil {
		title.ID = existing.ID
		title.UserStatus = NormalizeUserStatus(UserStatus(existing.UserStatus))
		title.UpdatedAt = existing.UpdatedAt
	}

	var entry *store.HistoryEntry
	if title.ID != 0 {
		entry, err = l.store.GetHistory(title.ID)
		if err != nil {
			return Title{}, err
		}
	}
	if entry != nil {
		title.ResumeAt = entry.ResumeAt
		title.Duration = entry.Duration
	}
	voice, chapter := chooseSelection(title, readingHistoryVoice(entry), readingHistoryChapter(entry))
	title.SelectedVoice = voice
	title.SelectedEpisode = chapter
	l.ensureCover(ctx, &title)
	logger.Info("reading title loaded", "title", title.Russian, "slug", title.Slug, "chapters", len(title.Episodes), "translations", len(title.Voiceovers))
	return title, nil
}

func readingHistoryVoice(entry *store.HistoryEntry) string {
	if entry == nil {
		return ""
	}
	return entry.Voiceover
}

func readingHistoryChapter(entry *store.HistoryEntry) string {
	if entry == nil {
		return ""
	}
	return entry.Episode
}

func readingSource(siteID int) string {
	switch siteID {
	case animelib.SiteMangaLib:
		return catalog.SourceMangaLib
	case animelib.SiteYaoiLib:
		return catalog.SourceYaoiLib
	case animelib.SiteRanobeLib:
		return catalog.SourceRanobeLib
	case animelib.SiteHentaiLib:
		return catalog.SourceHentaiLib
	default:
		return ""
	}
}

func storedTitleFromReading(data *animelib.ReadingTitleData, title Title) store.Title {
	source := readingSource(data.SiteID)
	english := strings.TrimSpace(data.EngName)
	if english == "" {
		english = strings.TrimSpace(data.Name)
	}
	russian := strings.TrimSpace(data.RusName)
	if russian == "" {
		russian = english
	}
	return store.Title{
		Slug:          data.SlugURL,
		Source:        source,
		ExternalID:    data.SlugURL,
		EngName:       english,
		RusName:       russian,
		Status:        int(title.Status),
		EpisodesTotal: len(title.Episodes),
		IsMovie:       false,
	}
}

// titleFromReadingData builds the display title, grouping chapters by
// translation team. The global episode list keeps every unique chapter key,
// while each voiceover lists the keys that team actually published.
func titleFromReadingData(data *animelib.ReadingTitleData) Title {
	english := strings.TrimSpace(data.EngName)
	if english == "" {
		english = strings.TrimSpace(data.Name)
	}
	russian := strings.TrimSpace(data.RusName)
	if russian == "" {
		russian = english
	}
	if strings.TrimSpace(russian) == strings.TrimSpace(english) {
		english = ""
	}
	title := Title{
		Source:     readingSource(data.SiteID),
		ExternalID: data.SlugURL,
		Slug:       data.SlugURL,
		Russian:    russian,
		Original:   english,
		Status:     statusFrom(data.StatusID, data.StatusLabel),
		UserStatus: UserStatusPlanned,
		IsMovie:    false,
		CoverURL:   coverURL(data.Cover),
		Episodes:   []string{},
		Voiceovers: []Voiceover{},
		Chapters:   []catalog.ReadingChapter{},
	}
	if title.Source == "" {
		title.Source = catalog.SourceMangaLib
	}

	unique := make(map[string]bool, len(data.Chapters))
	byTranslation := make(map[string]map[string]catalog.ReadingChapter)
	for _, raw := range data.Chapters {
		key := catalog.ReadingChapterKey(raw.Volume, raw.Number)
		unique[key] = true
		translation := strings.TrimSpace(raw.Translation)
		if translation == "" {
			translation = "Без команды"
		}
		if byTranslation[translation] == nil {
			byTranslation[translation] = make(map[string]catalog.ReadingChapter)
		}
		if _, exists := byTranslation[translation][key]; exists {
			continue
		}
		byTranslation[translation][key] = catalog.ReadingChapter{
			Key:         key,
			Volume:      strings.TrimSpace(raw.Volume),
			Number:      strings.TrimSpace(raw.Number),
			Name:        raw.Name,
			Translation: translation,
			BranchID:    raw.BranchID,
		}
	}

	episodes := make([]string, 0, len(unique))
	for key := range unique {
		episodes = append(episodes, key)
	}
	sortReadingChapterKeys(episodes)
	title.Episodes = episodes
	title.EpisodesTotal = len(episodes)

	for translation, chapters := range byTranslation {
		keys := make([]string, 0, len(chapters))
		for key := range chapters {
			keys = append(keys, key)
		}
		sortReadingChapterKeys(keys)
		missing := make([]string, 0, len(episodes))
		for _, key := range episodes {
			if _, ok := chapters[key]; !ok {
				missing = append(missing, key)
			}
		}
		title.Voiceovers = append(title.Voiceovers, Voiceover{
			Name:     translation,
			Episodes: keys,
			Missing:  missing,
		})
		for _, key := range keys {
			title.Chapters = append(title.Chapters, chapters[key])
		}
	}
	sort.SliceStable(title.Voiceovers, func(i, j int) bool {
		if len(title.Voiceovers[i].Episodes) != len(title.Voiceovers[j].Episodes) {
			return len(title.Voiceovers[i].Episodes) > len(title.Voiceovers[j].Episodes)
		}
		return strings.ToLower(title.Voiceovers[i].Name) < strings.ToLower(title.Voiceovers[j].Name)
	})
	return title
}

func sortReadingChapterKeys(keys []string) {
	sort.SliceStable(keys, func(i, j int) bool {
		leftVolume, leftNumber := catalog.SplitReadingChapterKey(keys[i])
		rightVolume, rightNumber := catalog.SplitReadingChapterKey(keys[j])
		if compared := compareReadingNumber(leftVolume, rightVolume); compared != 0 {
			return compared < 0
		}
		if compared := compareReadingNumber(leftNumber, rightNumber); compared != 0 {
			return compared < 0
		}
		return keys[i] < keys[j]
	})
}

func compareReadingNumber(left, right string) int {
	leftValue, leftErr := strconv.ParseFloat(strings.TrimSpace(left), 64)
	rightValue, rightErr := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftValue < rightValue:
			return -1
		case leftValue > rightValue:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
}

func findReadingChapter(title Title, translation, key string) (catalog.ReadingChapter, bool) {
	for _, chapter := range title.Chapters {
		if chapter.Key != key {
			continue
		}
		if translation == "" || chapter.Translation == translation {
			return chapter, true
		}
	}
	return catalog.ReadingChapter{}, false
}

// ReadChapter resolves one chapter image list or ranobe text. The chapter list
// itself is never cached; the caller must pass the title loaded for this
// reader session.
func (l *Library) ReadChapter(ctx context.Context, title Title, translation, key string) (catalog.ReadingContent, error) {
	siteID, ok := animelib.SiteIDForSource(title.Source)
	if !ok {
		return catalog.ReadingContent{}, ErrUnsupportedSource
	}
	chapter, ok := findReadingChapter(title, translation, key)
	if !ok {
		return catalog.ReadingContent{}, ErrChapterNotFound
	}
	token, err := l.readToken()
	if err != nil {
		return catalog.ReadingContent{}, err
	}
	client := animelib.New(token)
	data, err := client.FetchReadingChapter(ctx, title.Slug, siteID, chapter.Volume, chapter.Number, chapter.BranchID)
	if err != nil {
		logger.Error("cannot load reading chapter", "slug", title.Slug, "chapter", key, "translation", translation, "error", err)
		return catalog.ReadingContent{}, err
	}
	content := catalog.ReadingContent{
		ID:     data.ID,
		Volume: data.Volume,
		Number: data.Number,
		Name:   data.Name,
	}
	if name := strings.TrimSpace(chapter.Name); content.Name == "" {
		content.Name = name
	}
	content.Pages = make([]catalog.ReadingPage, 0, len(data.Pages))
	for _, page := range data.Pages {
		content.Pages = append(content.Pages, catalog.ReadingPage{
			URL:    page.URL,
			Width:  page.Width,
			Height: page.Height,
			Ratio:  page.Ratio,
		})
	}
	if strings.TrimSpace(data.RawContent) != "" {
		content.Paragraphs = parseRanobeParagraphs(data.RawContent)
	}
	logger.Info("reading chapter loaded", "slug", title.Slug, "chapter", key, "translation", translation, "pages", len(content.Pages), "paragraphs", len(content.Paragraphs))
	return content, nil
}

var readingImageClient = &http.Client{Timeout: 45 * time.Second}

// ReadingImage downloads and decodes one manga page. The reader owns the
// in-memory cache; this method only performs the network and WebP work.
func (l *Library) ReadingImage(ctx context.Context, title Title, rawURL string) (image.Image, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, ErrBadPageURL
	}
	siteID, ok := animelib.SiteIDForSource(title.Source)
	if !ok {
		siteID = animelib.SiteMangaLib
	}
	origin := animelib.ReadingOrigin(siteID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	request.Header.Set("Referer", origin+"/")
	request.Header.Set("Origin", origin)
	request.Header.Set("Site-Id", strconv.Itoa(siteID))
	request.Header.Set("User-Agent", animelib.BrowserUserAgent())
	logger.Debug("reading page request", "url", parsed.String(), "site", siteID)
	response, err := readingImageClient.Do(request)
	if err != nil {
		logger.Warn("reading page request failed", "url", parsed.String(), "error", err)
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		logger.Warn("reading page returned an error", "url", parsed.String(), "status", response.StatusCode)
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	page, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		logger.Warn("cannot decode reading page", "url", parsed.String(), "error", err)
		return nil, fmt.Errorf("не удалось открыть изображение: %w", err)
	}
	logger.Debug("reading page loaded", "url", parsed.String(), "bytes", len(data))
	return page, nil
}

// parseRanobeParagraphs extracts readable text from the HTML returned by the
// ranobe chapter endpoint. Paragraph boundaries are preserved; inline markup is
// dropped because the reader renders plain prose, not site HTML.
func parseRanobeParagraphs(raw string) []string {
	matches := ranobeParagraphPattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		text := strings.ReplaceAll(raw, "</div>", "\n")
		text = strings.ReplaceAll(text, "</p>", "\n")
		text = ranobeBreakPattern.ReplaceAllString(text, "\n")
		text = ranobeTagPattern.ReplaceAllString(text, "")
		text = html.UnescapeString(text)
		matches = [][]string{{"", text}}
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		text := match[1]
		text = ranobeBreakPattern.ReplaceAllString(text, "\n")
		text = ranobeTagPattern.ReplaceAllString(text, "")
		text = html.UnescapeString(text)
		text = strings.ReplaceAll(text, "\r\n", "\n")
		lines := strings.Split(text, "\n")
		cleaned := make([]string, 0, len(lines))
		for _, line := range lines {
			cleaned = append(cleaned, strings.TrimSpace(line))
		}
		text = strings.TrimSpace(strings.Join(cleaned, "\n"))
		if text == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, text)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

var (
	ranobeParagraphPattern = regexpMustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)
	ranobeBreakPattern     = regexpMustCompile(`(?is)<br\s*/?>`)
	ranobeTagPattern       = regexpMustCompile(`(?s)<[^>]*>`)
)

// regexpMustCompile is a tiny indirection so the patterns are declared together
// with the parser instead of at the top of the file.
func regexpMustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
