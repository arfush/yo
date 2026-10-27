package browser

import (
	"context"
	"fmt"
	"sync"

	playwright "github.com/mxschmitt/playwright-go"
)

const headlessUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36"

// Headless is a long-lived headless Chromium instance. Kodik's player needs a
// real browser session to produce its signed HLS links; keeping one browser
// alive makes resolving a playlist window much cheaper than starting Playwright
// for every episode.
type Headless struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext

	mu     sync.Mutex
	closed bool
	once   sync.Once
}

// NewHeadless starts Chromium using the already installed Playwright driver.
func (m *Manager) NewHeadless(ctx context.Context) (*Headless, error) {
	if m == nil {
		return nil, ErrManagerNil
	}
	if m.initErr != nil {
		return nil, fmt.Errorf("prepare Playwright: %w", m.initErr)
	}
	if !m.Installed() {
		return nil, ErrChromiumMissing
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pw, err := playwright.Run(m.opts)
	if err != nil {
		return nil, fmt.Errorf("запустить Playwright: %w", err)
	}
	headless := true
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: &headless,
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
		},
	})
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("запустить headless Chromium: %w", err)
	}
	locale := "ru-RU"
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(headlessUserAgent),
		Locale:    &locale,
	})
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, fmt.Errorf("создать контекст Chromium: %w", err)
	}
	return &Headless{pw: pw, browser: browser, context: context}, nil
}

// NewPage opens a new page in the shared browser context.
func (h *Headless) NewPage() (playwright.Page, error) {
	if h == nil {
		return nil, ErrHeadlessNil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrHeadlessClosed
	}
	return h.context.NewPage()
}

// Close stops Chromium and the Playwright driver. It is safe to call more than
// once.
func (h *Headless) Close() error {
	if h == nil {
		return nil
	}
	var firstErr error
	h.once.Do(func() {
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		if h.context != nil {
			if err := h.context.Close(); err != nil {
				firstErr = err
			}
		}
		if h.browser != nil {
			if err := h.browser.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if h.pw != nil {
			if err := h.pw.Stop(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	})
	return firstErr
}
