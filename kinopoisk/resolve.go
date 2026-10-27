package kinopoisk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/arfush/yo/browser"
)

const (
	// mediaTimeout covers the theatre boot, the /bnsi request and the first
	// manifest request. A cold headless Chromium needs the larger part of it.
	mediaTimeout = 45 * time.Second
	// headerTimeout only waits for the manifest after the /bnsi response, which
	// the player already fetched by then.
	headerTimeout = 20 * time.Second
	// mediaTTL keeps resolved manifests for the session. The signed URLs live
	// longer than a playlist window; the Bearer is refreshed on every resolve.
	mediaTTL = 40 * time.Minute

	iframeID = "yoanime-kinopoisk"
)

// Request is one episode to resolve.
type Request struct {
	Iframe        string
	Season        int
	Episode       int
	TranslationID int
	Quality       int
}

// Subtitle is one caption track offered by the theatre.
type Subtitle struct {
	Label    string
	Language string
	URL      string
	Default  bool
}

// SkipRange is one intro/outro span reported by the theatre.
type SkipRange struct {
	Start float64
	End   float64
}

// Media is a resolved stream with everything mpv needs to open it.
type Media struct {
	URL     string
	Headers []string
	Quality int
	// Qualities lists every quality the chosen audio track offers, ascending.
	// The title page uses it to grey out the buttons the voice cannot deliver;
	// mpv only needs Quality.
	Qualities []int
	Subtitles []Subtitle
	Skip      []SkipRange
}

// Resolver owns the headless Chromium used for Alloha streams.
type Resolver struct {
	manager *browser.Manager
	config  Config

	// resolveMu serializes page work: one iframe at a time keeps Chromium and
	// the mirror quiet, and resolving the prefetch window is fast enough.
	resolveMu sync.Mutex

	mu       sync.Mutex
	closed   bool
	headless *browser.Headless
	page     playwright.Page
	capture  *capture

	authMu sync.Mutex
	auth   string

	cacheMu sync.Mutex
	cache   map[string]cacheEntry
}

type capture struct {
	media  chan []byte
	media2 chan string
	auth   chan string
}

type cacheEntry struct {
	media   Media
	expires time.Time
}

// NewResolver returns a resolver backed by the application's Playwright manager.
func NewResolver(manager *browser.Manager) *Resolver {
	return &Resolver{
		manager: manager,
		config:  LoadConfig(),
		cache:   make(map[string]cacheEntry),
	}
}

// ResolveFresh forgets the cached manifest for the request and resolves it
// again. The player uses it for a retry after mpv failed to open a URL: the
// signed CDN links can expire faster than the 40-minute cache TTL.
func (r *Resolver) ResolveFresh(ctx context.Context, request Request) (Media, error) {
	if r == nil {
		return Media{}, ErrBrowserUnavailable
	}
	r.cacheMu.Lock()
	delete(r.cache, cacheKey(request))
	r.cacheMu.Unlock()
	return r.Resolve(ctx, request)
}

// Resolve returns a playable manifest for one episode.
func (r *Resolver) Resolve(ctx context.Context, request Request) (Media, error) {
	request.Iframe = strings.TrimSpace(request.Iframe)
	if request.Iframe == "" {
		return Media{}, ErrNoPlayerLink
	}
	key := cacheKey(request)
	if media, ok := r.cached(key); ok {
		return media, nil
	}

	r.resolveMu.Lock()
	defer r.resolveMu.Unlock()
	if media, ok := r.cached(key); ok {
		return media, nil
	}

	target := IframeURL(request.Iframe, request.Season, request.Episode)
	logger.Info("resolving Kinopoisk stream", "title", target, "season", request.Season, "episode", request.Episode, "quality", request.Quality)
	media, err := r.resolve(ctx, target, request)
	if err != nil {
		// A dropped page or a crashed Chromium leaves the cached handle
		// unusable; retry once from a clean page before giving up.
		logger.Warn("Kinopoisk resolve failed, retrying with a fresh page", "error", err)
		r.dropPage()
		media, err = r.resolve(ctx, target, request)
		if err != nil {
			logger.Error("Kinopoisk resolve failed", "season", request.Season, "episode", request.Episode, "error", err)
			return Media{}, err
		}
	}
	r.store(key, media)
	logger.Info("Kinopoisk stream resolved", "quality", media.Quality, "subtitles", len(media.Subtitles))
	return media, nil
}

func (r *Resolver) resolve(ctx context.Context, target string, request Request) (Media, error) {
	page, err := r.ensurePage(ctx)
	if err != nil {
		return Media{}, err
	}
	cap := &capture{
		media:  make(chan []byte, 2),
		media2: make(chan string, 4),
		auth:   make(chan string, 4),
	}
	r.setCapture(cap)
	defer r.setCapture(nil)

	if err := r.showIframe(page, target); err != nil {
		return Media{}, fmt.Errorf("cannot open the player: %w", err)
	}

	var body []byte
	select {
	case body = <-cap.media:
	case <-time.After(mediaTimeout):
		return Media{}, ErrNoVideoSources
	case <-ctx.Done():
		return Media{}, ctx.Err()
	}

	manifest := parseManifest(body)
	if len(manifest.Sources) == 0 {
		return Media{}, ErrNoQualityLinks
	}

	// The player picks the audio track of the selected translation itself. The
	// first manifest request therefore names both the right translation and a
	// working quality, and matching it against hlsSource keeps our choice of
	// quality without guessing how translations map to audio tracks.
	var captured string
	var bearer string
	select {
	case captured = <-cap.media2:
	case <-time.After(headerTimeout):
	case <-ctx.Done():
		return Media{}, ctx.Err()
	}
	select {
	case bearer = <-cap.auth:
	default:
	}

	// Free the bandwidth the hidden player would otherwise keep consuming.
	r.hideIframe(page)

	sourceIndex := matchSource(manifest.Sources, captured)
	streamURL, quality := pickQuality(manifest.Sources[sourceIndex], request.Quality)
	if streamURL == "" {
		return Media{}, ErrNoStreamURL
	}

	if bearer == "" {
		bearer = r.cachedAuth()
	}
	if bearer == "" {
		return Media{}, ErrNoAccessKey
	}
	r.storeAuth(bearer)

	media := Media{
		URL:       streamURL,
		Quality:   quality,
		Qualities: sourceQualities(manifest.Sources[sourceIndex]),
		Headers:   streamHeaders(target, bearer),
		Subtitles: manifest.subtitles(),
		Skip:      manifest.skipRanges(),
	}
	if media.Quality == 0 {
		media.Quality = request.Quality
	}
	return media, nil
}

// streamHeaders builds the header set the CDN expects. The manifest is only
// served to a request that carries the theatre page as its referer and the
// session's authorizations header.
func streamHeaders(target, bearer string) []string {
	headers := []string{
		"Referer: " + target,
		"authorizations: " + bearer,
	}
	if parsed, err := url.Parse(target); err == nil && parsed.Host != "" {
		headers = append(headers, "Origin: "+parsed.Scheme+"://"+parsed.Host)
	}
	return headers
}

// manifest mirrors the /bnsi/movies response.
type manifest struct {
	SkipTime   string           `json:"skipTime"`
	RemoveTime string           `json:"removeTime"`
	Sources    []manifestSource `json:"hlsSource"`
	Tracks     []struct {
		Kind     string `json:"kind"`
		Label    string `json:"label"`
		Src      string `json:"src"`
		Language string `json:"language"`
		Default  bool   `json:"default"`
	} `json:"tracks"`
	Skip []SkipRange `json:"-"`
}

type manifestSource struct {
	AudioID any               `json:"audioId"`
	Label   string            `json:"label"`
	Quality map[string]string `json:"quality"`
}

func parseManifest(body []byte) manifest {
	var payload manifest
	_ = json.Unmarshal(body, &payload)
	return payload
}

// subtitles converts the caption tracks into mpv candidates. The theatre lists
// one track per language variant, so every variant is passed to mpv and the
// default one is preselected.
func (m manifest) subtitles() []Subtitle {
	out := make([]Subtitle, 0, len(m.Tracks))
	for _, track := range m.Tracks {
		src := strings.TrimSpace(track.Src)
		if src == "" || !strings.EqualFold(strings.TrimSpace(track.Kind), "captions") {
			continue
		}
		out = append(out, Subtitle{
			Label:    cleanName(track.Label),
			Language: strings.TrimSpace(track.Language),
			URL:      src,
			Default:  track.Default,
		})
	}
	return out
}

func (m manifest) skipRanges() []SkipRange {
	return parseSkipRanges(m.SkipTime)
}

// parseSkipRanges reads "0-34" or "0-34,1500-1600" into intro/outro spans.
func parseSkipRanges(value string) []SkipRange {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var out []SkipRange
	for _, part := range strings.Split(value, ",") {
		bounds := strings.SplitN(strings.TrimSpace(part), "-", 2)
		if len(bounds) != 2 {
			continue
		}
		start, err := strconv.ParseFloat(strings.TrimSpace(bounds[0]), 64)
		if err != nil {
			continue
		}
		end, err := strconv.ParseFloat(strings.TrimSpace(bounds[1]), 64)
		if err != nil || end <= start {
			continue
		}
		out = append(out, SkipRange{Start: start, End: end})
	}
	return out
}

// matchSource finds the hlsSource entry whose quality URL is the one the player
// requested. It returns 0 when nothing matches, which is the player's default
// audio.
func matchSource(sources []manifestSource, captured string) int {
	captured = normalizeStreamURL(captured)
	if captured == "" {
		return 0
	}
	withoutQuery := stripQuery(captured)
	for index, source := range sources {
		for _, raw := range source.Quality {
			for _, candidate := range splitMirrors(raw) {
				if candidate == captured {
					return index
				}
				if stripQuery(candidate) == withoutQuery {
					return index
				}
			}
		}
	}
	return 0
}

// splitMirrors returns the individual URLs of one quality entry. The film
// sources carry interchangeable CDN mirrors joined with " or ", which is not a
// URL mpv can open; a series entry is usually a single URL.
func splitMirrors(value string) []string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.EqualFold(field, "or") {
			continue
		}
		parsed, err := url.Parse(field)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			continue
		}
		out = append(out, parsed.String())
	}
	return out
}

// normalizeStreamURL percent-encodes a raw player URL and drops anything that is
// not a single absolute http(s) address.
func normalizeStreamURL(value string) string {
	mirrors := splitMirrors(value)
	if len(mirrors) == 0 {
		return ""
	}
	return mirrors[0]
}

// stripQuery drops the query string so a signed manifest still matches its
// source entry.
func stripQuery(value string) string {
	if index := strings.IndexByte(value, '?'); index >= 0 {
		return value[:index]
	}
	return value
}

// pickQuality chooses the requested quality from one source, preferring the
// nearest lower one and falling back to the lowest available quality.
func pickQuality(source manifestSource, preferred int) (string, int) {
	if len(source.Quality) == 0 {
		return "", 0
	}
	if preferred <= 0 {
		preferred = 1080
	}
	if streamURL := normalizeStreamURL(source.Quality[strconv.Itoa(preferred)]); streamURL != "" {
		return streamURL, preferred
	}
	qualities := sourceQualities(source)
	if len(qualities) == 0 {
		return "", 0
	}
	lower := 0
	for _, quality := range qualities {
		if quality < preferred {
			lower = quality
			continue
		}
		break
	}
	if lower == 0 {
		lower = qualities[0]
	}
	return normalizeStreamURL(source.Quality[strconv.Itoa(lower)]), lower
}

// sourceQualities returns the playable quality steps of one audio track in
// ascending order. Entries whose URLs cannot be opened by mpv are skipped, so
// the UI never offers a button the resolver would have to ignore.
func sourceQualities(source manifestSource) []int {
	if len(source.Quality) == 0 {
		return nil
	}
	qualities := make([]int, 0, len(source.Quality))
	for key, raw := range source.Quality {
		quality, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil || quality <= 0 || normalizeStreamURL(raw) == "" {
			continue
		}
		qualities = append(qualities, quality)
	}
	sort.Ints(qualities)
	return qualities
}

func (r *Resolver) ensurePage(ctx context.Context) (playwright.Page, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrBrowserClosed
	}
	if r.page != nil {
		page := r.page
		r.mu.Unlock()
		return page, nil
	}
	r.mu.Unlock()

	headless, err := r.ensureHeadless(ctx)
	if err != nil {
		return nil, err
	}
	page, err := headless.NewPage()
	if err != nil {
		return nil, fmt.Errorf("cannot open the player page: %w", err)
	}
	if err := page.AddInitScript(playwright.Script{
		Content: playwright.String(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`),
	}); err != nil {
		_ = page.Close()
		return nil, err
	}
	// The theatre refuses to run outside a frame, so the wrapper page owns the
	// iframe and provides the referer for every player request.
	page.OnResponse(func(response playwright.Response) {
		if !strings.Contains(response.URL(), "/bnsi/") {
			return
		}
		if response.Status() != 200 {
			return
		}
		body, err := response.Body()
		if err != nil || len(body) == 0 {
			return
		}
		if cap := r.currentCapture(); cap != nil {
			select {
			case cap.media <- body:
			default:
			}
		}
	})
	page.OnRequest(func(request playwright.Request) {
		manifestURL := request.URL()
		if !strings.Contains(manifestURL, ".m3u8") {
			return
		}
		cap := r.currentCapture()
		if cap == nil {
			return
		}
		if headers, err := request.AllHeaders(); err == nil {
			if bearer := strings.TrimSpace(headers["authorizations"]); bearer != "" {
				select {
				case cap.auth <- bearer:
				default:
				}
			}
		}
		select {
		case cap.media2 <- manifestURL:
		default:
		}
	})

	if _, err := page.Goto(r.config.referer(), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(90000),
	}); err != nil {
		_ = page.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("cannot open the mirror: %w", err)
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		_ = page.Close()
		return nil, ErrBrowserClosed
	}
	if r.page != nil {
		existing := r.page
		r.mu.Unlock()
		_ = page.Close()
		return existing, nil
	}
	r.page = page
	r.mu.Unlock()
	return page, nil
}

// showIframe points the player iframe at the episode. The iframe is created once
// and reused; changing src reloads the theatre without rebuilding the wrapper.
func (r *Resolver) showIframe(page playwright.Page, target string) error {
	_, err := page.Evaluate(`(args) => {
		let frame = document.getElementById(args.id);
		if (!frame) {
			frame = document.createElement('iframe');
			frame.id = args.id;
			frame.style.cssText = 'position:fixed;top:0;left:0;width:1280px;height:720px;border:0;z-index:-1';
			frame.allow = 'autoplay; encrypted-media';
			document.body.appendChild(frame);
		}
		frame.src = args.url;
		return true;
	}`, map[string]any{"id": iframeID, "url": target})
	return err
}

func (r *Resolver) hideIframe(page playwright.Page) {
	_, _ = page.Evaluate(`(id) => {
		const frame = document.getElementById(id);
		if (frame) { frame.src = 'about:blank'; }
		return true;
	}`, iframeID)
}

func (r *Resolver) ensureHeadless(ctx context.Context) (*browser.Headless, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrBrowserClosed
	}
	if r.headless != nil {
		headless := r.headless
		r.mu.Unlock()
		return headless, nil
	}
	r.mu.Unlock()

	if r.manager == nil {
		return nil, ErrBrowserUnavailable
	}
	headless, err := r.manager.NewHeadless(ctx)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.closed || r.headless != nil {
		existing := r.headless
		r.mu.Unlock()
		if existing != nil {
			_ = headless.Close()
			return existing, nil
		}
		_ = headless.Close()
		return nil, ErrBrowserClosed
	}
	r.headless = headless
	r.mu.Unlock()
	return headless, nil
}

func (r *Resolver) dropPage() {
	r.mu.Lock()
	page := r.page
	r.page = nil
	headless := r.headless
	r.headless = nil
	r.mu.Unlock()
	if page != nil {
		_ = page.Close()
	}
	if headless != nil {
		_ = headless.Close()
	}
}

func (r *Resolver) setCapture(cap *capture) {
	r.mu.Lock()
	r.capture = cap
	r.mu.Unlock()
}

func (r *Resolver) currentCapture() *capture {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.capture
}

func (r *Resolver) cachedAuth() string {
	r.authMu.Lock()
	defer r.authMu.Unlock()
	return r.auth
}

func (r *Resolver) storeAuth(value string) {
	r.authMu.Lock()
	r.auth = value
	r.authMu.Unlock()
}

func (r *Resolver) cached(key string) (Media, bool) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	entry, ok := r.cache[key]
	if !ok || time.Now().After(entry.expires) {
		if ok {
			delete(r.cache, key)
		}
		return Media{}, false
	}
	return entry.media, true
}

func (r *Resolver) store(key string, media Media) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cache[key] = cacheEntry{media: media, expires: time.Now().Add(mediaTTL)}
}

func cacheKey(request Request) string {
	return fmt.Sprintf("%s|%d|%d|%d|%d", request.Iframe, request.Season, request.Episode, request.TranslationID, request.Quality)
}

// Close stops the headless browser without waiting for an in-flight resolve.
func (r *Resolver) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.closed = true
	page := r.page
	r.page = nil
	headless := r.headless
	r.headless = nil
	r.mu.Unlock()
	if page != nil {
		_ = page.Close()
	}
	if headless != nil {
		_ = headless.Close()
	}
	return nil
}
