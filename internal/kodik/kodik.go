// Package kodik resolves Kodik player embeds into playable HLS manifests.
//
// Kodik's /ftor endpoint is protected by a browser-side session: a plain HTTP
// request is rejected even with copied cookies and an identical form body. A
// headless Chromium page, opened with the AnimeLIB referer, produces a valid
// response and the returned signed URLs are publicly playable. The resolver
// keeps one headless browser alive for the duration of a playback session and
// caches resolved links.
package kodik

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/arfush/yonime/internal/browser"
)

const (
	embedReferer = "https://v5.animelib.org/"
	cacheTTL     = 45 * time.Minute
)

// Resolver owns the headless Chromium used for Kodik.
type Resolver struct {
	manager *browser.Manager

	// resolveMu serializes page visits. mu only protects the headless handle
	// so Close can interrupt an in-flight resolve without waiting for it.
	resolveMu sync.Mutex
	mu        sync.Mutex
	headless  *browser.Headless
	closed    bool

	cacheMu sync.Mutex
	cache   map[string]cacheEntry
}

type cacheEntry struct {
	links   map[int]string
	expires time.Time
}

// New returns a resolver backed by the application's Playwright manager.
func New(manager *browser.Manager) *Resolver {
	return &Resolver{
		manager: manager,
		cache:   make(map[string]cacheEntry),
	}
}

// Resolve returns playable URLs keyed by quality. The embed URL is the value
// stored by AnimeLIB in the episode's Kodik player (usually //kodikplayer.com/
// seria/.../720p).
func (r *Resolver) Resolve(ctx context.Context, embedURL string) (map[int]string, error) {
	embedURL = normalizeEmbed(embedURL)
	if embedURL == "" {
		return nil, errors.New("пустая ссылка Kodik")
	}
	if links, ok := r.cached(embedURL); ok {
		return links, nil
	}

	// One page at a time. Kodik is sensitive to parallel sessions from the same
	// address and sequential resolution is fast enough for a 5/5 window.
	r.resolveMu.Lock()
	defer r.resolveMu.Unlock()

	if links, ok := r.cached(embedURL); ok {
		return links, nil
	}
	links, err := r.resolve(ctx, embedURL)
	if err != nil {
		// A Chromium crash leaves the cached handle unusable. Drop it and retry
		// once with a fresh browser.
		r.dropHeadless()
		links, err = r.resolve(ctx, embedURL)
		if err != nil {
			return nil, err
		}
	}
	r.store(embedURL, links)
	return links, nil
}

// PickQuality selects the preferred quality if present, otherwise the nearest
// lower quality, otherwise the nearest higher one.
func PickQuality(links map[int]string, preferred int) (string, int) {
	if len(links) == 0 {
		return "", 0
	}
	if preferred <= 0 {
		preferred = 1080
	}
	if url, ok := links[preferred]; ok && url != "" {
		return url, preferred
	}
	qualities := make([]int, 0, len(links))
	for quality, url := range links {
		if url != "" {
			qualities = append(qualities, quality)
		}
	}
	if len(qualities) == 0 {
		return "", 0
	}
	sort.Ints(qualities)
	lower := 0
	for _, quality := range qualities {
		if quality < preferred {
			lower = quality
			continue
		}
		break
	}
	if lower > 0 {
		return links[lower], lower
	}
	return links[qualities[0]], qualities[0]
}

func (r *Resolver) resolve(ctx context.Context, embedURL string) (map[int]string, error) {
	headless, err := r.ensureHeadless(ctx)
	if err != nil {
		return nil, err
	}
	page, err := headless.NewPage()
	if err != nil {
		return nil, fmt.Errorf("открыть страницу Kodik: %w", err)
	}
	defer page.Close()

	responses := make(chan string, 1)
	page.On("response", func(response playwright.Response) {
		if !strings.Contains(response.URL(), "kodikplayer.com/ftor") {
			return
		}
		if response.Status() != 200 {
			return
		}
		body, err := response.Text()
		if err != nil || strings.TrimSpace(body) == "" {
			return
		}
		select {
		case responses <- body:
		default:
		}
	})

	if _, err := page.Goto(embedURL, playwright.PageGotoOptions{
		Referer:   playwright.String(embedReferer),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60000),
	}); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("открыть Kodik: %w", err)
	}

	// Kodik keeps the play button hidden (it is an overlay, not a visible
	// control), but clicking it is what makes the player call /ftor. Waiting
	// for the element to be attached is enough; waiting for visibility would
	// always time out and never start playback.
	_, _ = page.WaitForSelector(".play_button", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(15000),
	})
	_, _ = page.Evaluate(`(() => {
		const button = document.querySelector('.play_button');
		if (button) { button.click(); return true; }
		const video = document.querySelector('video');
		if (video) { video.muted = true; video.play().catch(() => {}); return true; }
		return false;
	})()`)

	var body string
	select {
	case body = <-responses:
	case <-time.After(30 * time.Second):
		return nil, errors.New("Kodik не вернул ссылки на видео")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return parseLinks(body)
}

func (r *Resolver) ensureHeadless(ctx context.Context) (*browser.Headless, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, errors.New("браузер Kodik закрыт")
	}
	if r.headless != nil {
		headless := r.headless
		r.mu.Unlock()
		return headless, nil
	}
	r.mu.Unlock()

	if r.manager == nil {
		return nil, errors.New("браузер Kodik недоступен")
	}
	headless, err := r.manager.NewHeadless(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		_ = headless.Close()
		return nil, errors.New("браузер Kodik закрыт")
	}
	if r.headless != nil {
		existing := r.headless
		r.mu.Unlock()
		_ = headless.Close()
		return existing, nil
	}
	r.headless = headless
	r.mu.Unlock()
	return headless, nil
}

func (r *Resolver) dropHeadless() {
	r.mu.Lock()
	headless := r.headless
	r.headless = nil
	r.mu.Unlock()
	if headless != nil {
		_ = headless.Close()
	}
}

// Close stops the headless browser without waiting for an in-flight resolve.
// The page calls made by that resolve fail once the browser is gone.
func (r *Resolver) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.closed = true
	headless := r.headless
	r.headless = nil
	r.mu.Unlock()
	if headless != nil {
		_ = headless.Close()
	}
	return nil
}

func (r *Resolver) cached(embedURL string) (map[int]string, bool) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	entry, ok := r.cache[embedURL]
	if !ok || time.Now().After(entry.expires) {
		if ok {
			delete(r.cache, embedURL)
		}
		return nil, false
	}
	return cloneLinks(entry.links), true
}

func (r *Resolver) store(embedURL string, links map[int]string) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cache[embedURL] = cacheEntry{
		links:   cloneLinks(links),
		expires: time.Now().Add(cacheTTL),
	}
}

func cloneLinks(links map[int]string) map[int]string {
	out := make(map[int]string, len(links))
	for quality, url := range links {
		out[quality] = url
	}
	return out
}

func normalizeEmbed(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	if !strings.Contains(raw, "?") {
		raw += "?translations=false"
	}
	return raw
}

type ftorResponse struct {
	Links map[string]json.RawMessage `json:"links"`
}

func parseLinks(body string) (map[int]string, error) {
	var response ftorResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("некорректный ответ Kodik: %w", err)
	}
	if len(response.Links) == 0 {
		return nil, errors.New("Kodik вернул пустой список ссылок")
	}
	out := make(map[int]string)
	for quality, raw := range response.Links {
		number, err := parseInt(quality)
		if err != nil || number <= 0 {
			continue
		}
		src := extractSrc(raw)
		if src == "" {
			continue
		}
		decoded, err := decodeKodik(src)
		if err != nil {
			continue
		}
		out[number] = decoded
	}
	if len(out) == 0 {
		return nil, errors.New("Kodik вернул ссылки в неизвестном формате")
	}
	return out, nil
}

func parseInt(value string) (int, error) {
	var number int
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &number)
	return number, err
}

func extractSrc(raw json.RawMessage) string {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var list []struct {
		Src string `json:"src"`
	}
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return strings.TrimSpace(list[0].Src)
	}
	var single struct {
		Src string `json:"src"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single.Src) != "" {
		return strings.TrimSpace(single.Src)
	}
	var direct string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return strings.TrimSpace(direct)
	}
	return ""
}

// decodeKodik reverses Kodik's small Caesar shift and base64 encoding. The
// shift value is not fixed in the response, so all 26 possibilities are tried
// and the first result that looks like a media manifest wins.
func decodeKodik(encoded string) (string, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return "", errors.New("пустая ссылка")
	}
	for shift := 0; shift < 26; shift++ {
		rotated := rotateLetters(encoded, shift)
		for _, decoded := range decodeBase64Variants(rotated) {
			text := string(decoded)
			if strings.Contains(text, ".m3u8") || strings.Contains(text, ".mp4") {
				return normalizePlaybackURL(text), nil
			}
		}
	}
	return "", errors.New("не удалось декодировать ссылку Kodik")
}

func rotateLetters(value string, shift int) string {
	if shift == 0 {
		return value
	}
	out := []byte(value)
	for index, char := range out {
		switch {
		case char >= 'A' && char <= 'Z':
			out[index] = 'A' + byte((int(char-'A')+shift)%26)
		case char >= 'a' && char <= 'z':
			out[index] = 'a' + byte((int(char-'a')+shift)%26)
		}
	}
	return string(out)
}

func decodeBase64Variants(value string) [][]byte {
	padded := value
	if remainder := len(padded) % 4; remainder != 0 {
		padded += strings.Repeat("=", 4-remainder)
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	out := make([][]byte, 0, len(encodings))
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(padded)
		if err == nil && len(decoded) > 0 {
			out = append(out, decoded)
		}
	}
	return out
}

func normalizePlaybackURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}
