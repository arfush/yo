package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/arfush/yonime/internal/token"
)

// DefaultDomain is the AnimeLIB domain shown in the token prompt. The browser
// opens this site and the user signs in through its own interface.
const DefaultDomain = "anilib.me"

// DefaultSiteURL is the page opened in Chromium before the user logs in.
const DefaultSiteURL = "https://" + DefaultDomain + "/"

// authPollInterval is a compromise between reacting to a completed login
// quickly and not keeping the Playwright connection busy.
const authPollInterval = 500 * time.Millisecond

// authNavigationTimeout bounds the initial site load. AnimeLIB normally
// commits in under a second; a domain that does not answer should fail fast
// enough for the button feedback to be useful.
const authNavigationTimeout = 12 * time.Second

var (
	// ErrInvalidLink is returned when the user-supplied project link cannot be
	// opened.
	ErrInvalidLink = errors.New("некорректная ссылка")
	// ErrSiteUnreachable is returned when the site rejects or drops the
	// navigation. The usual fix is trying another mirror.
	ErrSiteUnreachable = errors.New("сайт не открылся")
	// ErrWindowClosed is returned when the authorization browser is closed
	// before a token appears.
	ErrWindowClosed = errors.New("окно авторизации закрыто")
)

// authTokenScript reads the token persisted by AnimeLIB's Pinia store. The
// store key is "auth"; the token object holds access_token, refresh_token,
// expires_in and timestamp. Scanning the remaining keys is a fallback for a
// future site version that renames the store.
const authTokenScript = `(() => {
  const parse = (raw) => {
    if (!raw) return null;
    try {
      const value = JSON.parse(raw);
      if (!value || typeof value !== 'object') return null;
      const token = value.token && typeof value.token === 'object' ? value.token : value;
      if (typeof token.access_token === 'string' && token.access_token) return token;
    } catch (e) {}
    return null;
  };
  try {
    let token = parse(localStorage.getItem('auth'));
    if (token) return JSON.stringify(token);
    for (let i = 0; i < localStorage.length; i++) {
      let raw = null;
      try { raw = localStorage.getItem(localStorage.key(i)); } catch (e) { continue; }
      token = parse(raw);
      if (token) return JSON.stringify(token);
    }
  } catch (e) {}
  return '';
})()`

// WindowBounds is a window rectangle in device-independent pixels. The token
// prompt passes the main window's bounds so Chromium opens exactly over it.
type WindowBounds struct {
	X, Y, Width, Height int
}

// SiteURL normalizes a user-supplied project link. A bare domain opens that
// site's home page; a link with a path is used as-is. Yonime only opens the
// site: the user starts the login flow through its own interface.
func SiteURL(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		input = DefaultSiteURL
	}
	if !strings.Contains(input, "://") {
		input = "https://" + input
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidLink, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: поддерживаются только http и https", ErrInvalidLink)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%w: нет адреса сайта", ErrInvalidLink)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

// NormalizeDomain extracts the host from a user-supplied address. The domain
// editor accepts either a bare domain or a full site link and keeps only the
// host, so a pasted AnimeLIB page still yields a usable domain.
func NormalizeDomain(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("%w: пустой домен", ErrInvalidLink)
	}
	site, err := SiteURL(input)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(site)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidLink, err)
	}
	host := strings.ToLower(parsed.Host)
	if host == "" {
		return "", fmt.Errorf("%w: нет адреса сайта", ErrInvalidLink)
	}
	return host, nil
}

// Authorize opens siteURL in the installed Playwright Chromium and waits until
// AnimeLIB stores an access token, or until ctx is canceled. The user signs in
// on the site itself; Yonime only opens it and watches the stored token. A
// persistent profile keeps an existing site session between launches.
//
// When bounds is non-nil, Chromium is placed over the application window. The
// method is safe to call from a background goroutine. Only one authorization
// session may run at a time; Manager.Close cancels it.
func (m *Manager) Authorize(ctx context.Context, siteURL, profileDir string, bounds *WindowBounds) (token.Token, error) {
	if m == nil {
		return token.Token{}, errors.New("browser manager is nil")
	}
	if m.initErr != nil {
		return token.Token{}, fmt.Errorf("prepare Playwright: %w", m.initErr)
	}
	if !m.Installed() {
		return token.Token{}, errors.New("Chromium не установлен")
	}

	siteURL, err := SiteURL(siteURL)
	if err != nil {
		return token.Token{}, err
	}
	if strings.TrimSpace(profileDir) == "" {
		return token.Token{}, errors.New("не указан профиль Chromium")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return token.Token{}, errors.New("application is closing")
	}
	if m.authorizing {
		m.mu.Unlock()
		return token.Token{}, errors.New("authorization is already running")
	}
	m.authorizing = true
	m.authCancel = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.authorizing = false
		m.authCancel = nil
		m.authClose = nil
		m.mu.Unlock()
	}()

	pw, err := playwright.Run(m.opts)
	if err != nil {
		return token.Token{}, fmt.Errorf("запустить Playwright: %w", err)
	}
	defer pw.Stop()

	headless := false
	noViewport := true
	args := []string{"--disable-blink-features=AutomationControlled"}
	if bounds != nil && bounds.Width >= 320 && bounds.Height >= 240 {
		args = append(args,
			fmt.Sprintf("--window-size=%d,%d", bounds.Width, bounds.Height),
			fmt.Sprintf("--window-position=%d,%d", bounds.X, bounds.Y),
		)
	} else {
		args = append(args, "--start-maximized")
	}
	browserContext, err := pw.Chromium.LaunchPersistentContext(profileDir, playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless:   &headless,
		NoViewport: &noViewport,
		Args:       args,
	})
	if err != nil {
		if ctx.Err() != nil {
			return token.Token{}, ctx.Err()
		}
		return token.Token{}, fmt.Errorf("запустить Chromium: %w", err)
	}
	defer browserContext.Close()

	m.mu.Lock()
	if m.closed || !m.authorizing {
		m.mu.Unlock()
		return token.Token{}, context.Canceled
	}
	// The browser context is closed by Manager.Close if the application exits
	// while the user is still authorizing.
	m.authClose = func() { _ = browserContext.Close() }
	m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return token.Token{}, err
	}

	pages := browserContext.Pages()
	var page playwright.Page
	if len(pages) > 0 {
		page = pages[0]
	} else {
		page, err = browserContext.NewPage()
		if err != nil {
			return token.Token{}, fmt.Errorf("открыть страницу Chromium: %w", err)
		}
	}

	closed := make(chan struct{}, 1)
	notifyClosed := func() {
		for _, candidate := range browserContext.Pages() {
			if !candidate.IsClosed() {
				return
			}
		}
		select {
		case closed <- struct{}{}:
		default:
		}
	}

	captured := make(chan token.Token, 4)
	attach := func(target playwright.Page) {
		target.OnRequest(func(request playwright.Request) {
			value := request.Headers()["authorization"]
			if value == "" {
				return
			}
			const prefix = "bearer "
			if len(value) <= len(prefix) || !strings.EqualFold(value[:len(prefix)], prefix) {
				return
			}
			candidate := token.Token{
				AccessToken: strings.TrimSpace(value[len(prefix):]),
				TokenType:   "Bearer",
			}
			if candidate.AccessToken == "" || candidate.Expired(time.Now()) {
				return
			}
			select {
			case captured <- candidate:
			default:
			}
		})
		target.OnClose(func(playwright.Page) { notifyClosed() })
	}
	for _, existing := range browserContext.Pages() {
		attach(existing)
	}
	browserContext.OnPage(func(created playwright.Page) {
		attach(created)
		notifyClosed()
	})

	if err := openSite(page, siteURL, ctx); err != nil {
		return token.Token{}, err
	}

	ticker := time.NewTicker(authPollInterval)
	defer ticker.Stop()

	var (
		fallback   token.Token
		fallbackAt time.Time
	)
	for {
		select {
		case <-ctx.Done():
			return token.Token{}, ctx.Err()
		case <-closed:
			return token.Token{}, ErrWindowClosed
		case <-ticker.C:
		}

		for _, candidate := range browserContext.Pages() {
			if candidate.IsClosed() {
				continue
			}
			result, err := candidate.Evaluate(authTokenScript)
			if err != nil {
				continue
			}
			raw, ok := result.(string)
			if !ok || raw == "" {
				continue
			}
			parsed, err := token.Parse([]byte(raw))
			if err != nil || parsed.AccessToken == "" || parsed.Expired(time.Now()) {
				continue
			}
			return parsed, nil
		}

		select {
		case candidate := <-captured:
			if fallback.AccessToken == "" {
				fallback = candidate
				fallbackAt = time.Now()
			}
		default:
		}

		// The token storage is the primary source because it also carries the
		// expiration. A captured Bearer token is used only if the site changed
		// its storage layout and no structured value appeared.
		if fallback.AccessToken != "" && time.Since(fallbackAt) > 2*time.Second {
			return fallback, nil
		}
	}
}

// openSite opens the project page and tolerates navigations that the site
// itself supersedes. WaitUntilCommit is deliberate: AnimeLIB's SPA may start a
// redirect as soon as the page is committed, and waiting for DOMContentLoaded
// can surface a spurious ERR_ABORTED (or a closed connection) even though the
// site is already loading.
func openSite(page playwright.Page, target string, ctx context.Context) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
		_, err := page.Goto(target, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateCommit,
			Timeout:   playwright.Float(float64(authNavigationTimeout / time.Millisecond)),
		})
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if isNavigationAbort(err) {
			// The site navigated to another page while Goto was waiting; the
			// polling loop will follow the new page.
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("%w: %s: %v", ErrSiteUnreachable, target, lastErr)
}

func isNavigationAbort(err error) bool {
	message := err.Error()
	for _, marker := range []string{
		"ERR_ABORTED",
		"frame was detached",
		"interrupted by another navigation",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
