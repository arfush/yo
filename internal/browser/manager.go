// Package browser manages the Playwright Chromium installation used by Yonime.
//
// The first-run screen asks Playwright to install its Chromium build. Browsers
// are kept in Playwright's regular browser cache, so a later playwright-go
// integration can launch them without extra setup.
package browser

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	playwright "github.com/mxschmitt/playwright-go"
)

// removeChromiumOnExit is a development aid: it makes every application start
// look like a first start. It is disabled so an installed Chromium is reused
// and the application can open the main window on subsequent launches.
const removeChromiumOnExit = false

// Progress reports the overall Chromium installation progress. Preparing is
// true while the Playwright driver itself is being made ready; the driver
// download does not expose byte-level progress.
type Progress struct {
	Percent   float64
	Preparing bool
}

// Manager controls the Playwright driver and Chromium installation.
type Manager struct {
	driver *playwright.PlaywrightDriver
	opts   *playwright.RunOptions

	browsersDir string
	initErr     error

	mu          sync.Mutex
	closed      bool
	downloading bool
	cancel      context.CancelFunc
	cmd         *exec.Cmd
	done        chan struct{}

	authorizing bool
	authCancel  context.CancelFunc
	authClose   func()
}

// New creates a browser manager. Driver files are not touched until Installed
// or Download is called.
func New() *Manager {
	opts := &playwright.RunOptions{
		Verbose:             false,
		Stdout:              io.Discard,
		Stderr:              io.Discard,
		SkipInstallBrowsers: true,
		Browsers:            []string{"chromium"},
	}
	driver, err := playwright.NewDriver(opts)

	m := &Manager{
		driver:  driver,
		opts:    opts,
		initErr: err,
	}
	m.browsersDir = browsersDirectory(opts.DriverDirectory)
	return m
}

// Installed reports whether the Chromium revision expected by the bundled
// Playwright driver is already installed in the browser cache.
func (m *Manager) Installed() bool {
	if m == nil || m.initErr != nil {
		return false
	}

	if revision, err := m.chromiumRevision(); err == nil {
		return markerExists(filepath.Join(m.browsersDir, "chromium-"+revision))
	}

	// The driver may not be downloaded yet. In that case an existing browser
	// directory is still useful: do not ask the user to download it again.
	matches, _ := filepath.Glob(filepath.Join(m.browsersDir, "chromium-*"))
	for _, dir := range matches {
		if markerExists(dir) {
			return true
		}
	}
	return false
}

// Download installs the Playwright driver (if necessary) and Chromium. The
// callback is invoked from a background goroutine and must be safe to call
// repeatedly. Percent is monotonic and only reaches 100 after Chromium has
// actually been installed.
func (m *Manager) Download(ctx context.Context, onProgress func(Progress)) error {
	if m == nil {
		return errors.New("browser manager is nil")
	}
	if m.initErr != nil {
		return fmt.Errorf("prepare Playwright: %w", m.initErr)
	}
	if onProgress == nil {
		onProgress = func(Progress) {}
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		return errors.New("application is closing")
	}
	if m.downloading {
		m.mu.Unlock()
		cancel()
		return errors.New("Chromium download is already running")
	}
	m.downloading = true
	m.cancel = cancel
	m.done = done
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.cancel = nil
		m.cmd = nil
		m.done = nil
		m.mu.Unlock()
		cancel()
		close(done)
	}()

	onProgress(Progress{Preparing: true})
	if err := m.ensureDriver(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.Installed() {
		onProgress(Progress{Percent: 100})
		return nil
	}

	components, err := m.installPlan(ctx)
	if err != nil {
		return err
	}
	missing := components[:0]
	for _, component := range components {
		if !component.installed {
			missing = append(missing, component)
		}
	}
	if len(missing) == 0 {
		if !m.Installed() {
			return errors.New("Playwright did not report a Chromium download")
		}
		onProgress(Progress{Percent: 100})
		return nil
	}

	for i := range missing {
		missing[i].total = downloadSize(ctx, missing[i].urls)
	}
	tracker := newProgressTracker(missing, onProgress)
	onProgress(Progress{Percent: tracker.percent()})

	if err := ctx.Err(); err != nil {
		return err
	}
	writer := &lineWriter{handle: tracker.handleLine}
	cmd := m.driver.Command("install", "chromium")
	var stdout, stderr limitedBuffer
	cmd.Stdout = io.MultiWriter(writer, &stdout)
	cmd.Stderr = &stderr

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("application is closing")
	}
	m.cmd = cmd
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Playwright installer: %w", err)
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-wait:
	case <-ctx.Done():
		killProcessTree(cmd)
		waitErr = <-wait
		if waitErr == nil {
			waitErr = ctx.Err()
		}
	}
	writer.flush()

	m.mu.Lock()
	m.cmd = nil
	m.mu.Unlock()

	if waitErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		detail := stripANSI(strings.TrimSpace(stderr.String()))
		if useful := usefulOutput(stdout.String()); useful != "" {
			if detail != "" {
				detail += "\n"
			}
			detail += useful
		}
		if detail != "" {
			return fmt.Errorf("Playwright could not install Chromium: %w: %s", waitErr, detail)
		}
		return fmt.Errorf("Playwright could not install Chromium: %w", waitErr)
	}
	if !m.Installed() {
		return errors.New("Playwright finished without installing Chromium")
	}

	tracker.finish()
	return nil
}

// ensureDriver downloads the driver once. A driver directory left half-written
// by a previous interrupted first run is removed and recreated instead of
// permanently failing the version check.
func (m *Manager) ensureDriver() error {
	err := m.driver.DownloadDriver()
	if err == nil {
		return nil
	}
	if strings.TrimSpace(os.Getenv("PLAYWRIGHT_CLI_PATH")) != "" {
		return fmt.Errorf("prepare Playwright driver: %w", err)
	}
	if removeErr := os.RemoveAll(m.opts.DriverDirectory); removeErr != nil {
		return fmt.Errorf("prepare Playwright driver: %w", err)
	}
	if retryErr := m.driver.DownloadDriver(); retryErr != nil {
		return fmt.Errorf("prepare Playwright driver: %w (retry: %v)", err, retryErr)
	}
	return nil
}

// Close stops an in-flight installation and removes Chromium for development.
// The Playwright driver and unrelated Playwright browsers are left in place.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	cancel := m.cancel
	cmd := m.cmd
	done := m.done
	authCancel := m.authCancel
	authClose := m.authClose
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	// Closing the context ends the authorization browser immediately instead
	// of waiting for the next protocol call to time out.
	if authCancel != nil {
		authCancel()
	}
	if authClose != nil {
		authClose()
	}
	if cmd != nil {
		killProcessTree(cmd)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	// Playwright's installer uses a directory lock. A killed installer cannot
	// release it, so clear it here to keep the next attempt from waiting.
	if cmd != nil {
		_ = os.RemoveAll(filepath.Join(m.browsersDir, "__dirlock"))
	}

	if !removeChromiumOnExit {
		return nil
	}

	var firstErr error
	remove := func(dir string) {
		if dir == "" {
			return
		}
		if err := os.RemoveAll(dir); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if revision, err := m.chromiumRevision(); err == nil {
		remove(filepath.Join(m.browsersDir, "chromium-"+revision))
		remove(filepath.Join(m.browsersDir, "chromium_headless_shell-"+revision))
	} else {
		// Best effort if the driver is missing or was removed while running.
		for _, prefix := range []string{"chromium-*", "chromium_headless_shell-*"} {
			matches, _ := filepath.Glob(filepath.Join(m.browsersDir, prefix))
			for _, dir := range matches {
				remove(dir)
			}
		}
	}
	return firstErr
}

// chromiumRevision reads the browser revision from the Playwright driver's
// browsers.json. Reading the manifest is cheaper and safer than launching the
// driver just to ask where its Chromium lives.
func (m *Manager) chromiumRevision() (string, error) {
	manifest := m.browsersJSONPath()
	data, err := os.ReadFile(manifest)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Browsers []struct {
			Name     string          `json:"name"`
			Revision json.RawMessage `json:"revision"`
		} `json:"browsers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("read %s: %w", manifest, err)
	}
	for _, browser := range parsed.Browsers {
		if browser.Name != "chromium" {
			continue
		}
		revision := strings.TrimSpace(string(browser.Revision))
		revision = strings.Trim(revision, `"`)
		if revision != "" && revision != "null" {
			return revision, nil
		}
	}
	return "", fmt.Errorf("chromium revision not found in %s", manifest)
}

func (m *Manager) browsersJSONPath() string {
	if cli := strings.TrimSpace(os.Getenv("PLAYWRIGHT_CLI_PATH")); cli != "" {
		return filepath.Join(filepath.Dir(cli), "browsers.json")
	}
	return filepath.Join(m.opts.DriverDirectory, "package", "browsers.json")
}

type installComponent struct {
	title     string
	dir       string
	urls      []string
	total     int64
	installed bool
}

// installPlan asks Playwright which files "install chromium" would need. The
// dry-run output is stable and human-readable, and includes the install
// locations and fallback URLs.
func (m *Manager) installPlan(ctx context.Context) ([]installComponent, error) {
	cmd := m.driver.Command("install", "chromium", "--dry-run")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := stripANSI(strings.TrimSpace(stderr.String()))
		if detail != "" {
			return nil, fmt.Errorf("read Playwright install plan: %w: %s", err, detail)
		}
		return nil, fmt.Errorf("read Playwright install plan: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	components := parseInstallPlan(stripANSI(string(out)))
	if len(components) == 0 {
		return nil, errors.New("Playwright returned an empty Chromium install plan")
	}
	for i := range components {
		components[i].installed = markerExists(components[i].dir)
	}
	return components, nil
}

func parseInstallPlan(out string) []installComponent {
	var components []installComponent
	var current *installComponent

	flush := func() {
		if current != nil && current.title != "" {
			components = append(components, *current)
		}
		current = nil
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			flush()
			current = &installComponent{title: trimmed}
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "Install location:"):
			current.dir = strings.TrimSpace(strings.TrimPrefix(trimmed, "Install location:"))
		case strings.HasPrefix(trimmed, "Download url:"):
			if url := strings.TrimSpace(strings.TrimPrefix(trimmed, "Download url:")); url != "" {
				current.urls = append(current.urls, url)
			}
		case strings.HasPrefix(trimmed, "Download fallback"):
			if i := strings.IndexByte(trimmed, ':'); i >= 0 {
				if url := strings.TrimSpace(trimmed[i+1:]); url != "" {
					current.urls = append(current.urls, url)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil
	}
	flush()
	return components
}

func markerExists(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "INSTALLATION_COMPLETE"))
	return err == nil && !info.IsDir()
}

// downloadSize obtains the archive size without downloading it. HEAD is tried
// first; a one-byte ranged GET covers CDNs that reject HEAD.
func downloadSize(ctx context.Context, urls []string) int64 {
	if len(urls) == 0 {
		return 0
	}
	client := &http.Client{Timeout: 25 * time.Second}
	for _, rawURL := range urls {
		if ctx.Err() != nil {
			return 0
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
		if err == nil {
			if size := responseSize(client, req); size > 0 {
				return size
			}
		}

		req, err = http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Range", "bytes=0-0")
		if size := responseSize(client, req); size > 0 {
			return size
		}
	}
	return 0
}

func responseSize(client *http.Client, req *http.Request) int64 {
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPartialContent {
		// Content-Range: bytes 0-0/123456
		value := resp.Header.Get("Content-Range")
		if i := strings.LastIndexByte(value, '/'); i >= 0 {
			if size, err := strconv.ParseInt(value[i+1:], 10, 64); err == nil && size > 0 {
				return size
			}
		}
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if resp.ContentLength > 0 {
			return resp.ContentLength
		}
	}
	return 0
}

type progressTracker struct {
	components []installComponent
	byTitle    map[string]*installComponent
	report     func(Progress)

	useBytes bool
	total    int64

	completedBytes int64
	completedCount int

	current     *installComponent
	bytes       int64
	filePercent int

	last     int
	finished bool
}

func newProgressTracker(components []installComponent, report func(Progress)) *progressTracker {
	t := &progressTracker{
		components: components,
		byTitle:    make(map[string]*installComponent, len(components)),
		report:     report,
		last:       -1,
	}
	var total int64
	allSizes := true
	for i := range components {
		t.byTitle[components[i].title] = &components[i]
		if components[i].total <= 0 {
			allSizes = false
			continue
		}
		total += components[i].total
	}
	if allSizes && total > 0 {
		t.useBytes = true
		t.total = total
	}
	return t
}

func (t *progressTracker) percent() float64 {
	if t.finished {
		return 100
	}
	if t.useBytes && t.total > 0 {
		value := float64(t.completedBytes+t.bytes) * 100 / float64(t.total)
		if value > 99 {
			value = 99
		}
		if value < 0 {
			value = 0
		}
		return value
	}
	if len(t.components) == 0 {
		return 0
	}
	value := (float64(t.completedCount) + float64(t.filePercent)/100) * 100 / float64(len(t.components))
	if value > 99 {
		value = 99
	}
	if value < 0 {
		value = 0
	}
	return value
}

func (t *progressTracker) emit() {
	if t.report == nil || t.finished {
		return
	}
	value := int(t.percent())
	if value <= t.last {
		return
	}
	t.last = value
	t.report(Progress{Percent: float64(value)})
}

func (t *progressTracker) handleLine(line string) {
	line = strings.TrimSpace(stripANSI(line))
	if line == "" {
		return
	}

	if title, ok := downloadingTitle(line); ok {
		t.startComponent(title)
		return
	}
	if percent, ok := progressPercent(line); ok {
		t.update(percent)
	}
}

func (t *progressTracker) startComponent(title string) {
	next, ok := t.byTitle[title]
	if !ok {
		return
	}
	if t.current == next {
		return
	}
	if t.current != nil {
		t.completeCurrent()
	}
	t.current = next
	t.bytes = 0
	t.filePercent = 0
	t.emit()
}

func (t *progressTracker) completeCurrent() {
	if t.current == nil {
		return
	}
	if t.useBytes {
		t.completedBytes += t.bytes
		if t.completedBytes > t.total {
			t.completedBytes = t.total
		}
	} else {
		t.completedCount++
	}
}

func (t *progressTracker) update(percent int) {
	if t.current == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	if percent < t.filePercent {
		return
	}
	t.filePercent = percent
	if t.useBytes {
		done := t.current.total * int64(percent) / 100
		if done > t.bytes {
			t.bytes = done
		}
	}
	t.emit()
}

func (t *progressTracker) finish() {
	if t.finished {
		return
	}
	t.finished = true
	if t.report != nil {
		t.report(Progress{Percent: 100})
	}
}

func downloadingTitle(line string) (string, bool) {
	const prefix = "Downloading "
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	if i := strings.LastIndex(rest, " from "); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	return rest, rest != ""
}

var progressLineRE = regexp.MustCompile(`^\|.*\|\s+([0-9]+)% of `)

func progressPercent(line string) (int, bool) {
	match := progressLineRE.FindStringSubmatch(line)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return value, true
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	return ansiRE.ReplaceAllString(s, "")
}

// lineWriter turns a stream of process output into complete lines.
type lineWriter struct {
	mu     sync.Mutex
	buf    []byte
	handle func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:i]), "\r")
		w.buf = w.buf[i+1:]
		if line != "" && w.handle != nil {
			w.handle(line)
		}
	}
	// A pathological line without a newline must not retain memory forever.
	if len(w.buf) > 1<<20 {
		w.buf = w.buf[len(w.buf)-64<<10:]
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return
	}
	line := strings.TrimRight(string(w.buf), "\r")
	w.buf = nil
	if line != "" && w.handle != nil {
		w.handle(line)
	}
}

type limitedBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	const limit = 8 << 10
	if len(p) >= limit {
		b.buf = append(b.buf[:0], p[len(p)-limit:]...)
		return n, nil
	}
	if overflow := len(b.buf) + len(p) - limit; overflow > 0 {
		b.buf = b.buf[overflow:]
	}
	b.buf = append(b.buf, p...)
	return n, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// usefulOutput removes the repetitive progress rows so an installer failure
// can be logged with its actual error text.
func usefulOutput(out string) string {
	out = stripANSI(out)
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "Downloading ") {
			continue
		}
		lines = append(lines, line)
	}
	const keep = 12
	if len(lines) > keep {
		lines = lines[len(lines)-keep:]
	}
	return strings.Join(lines, "\n")
}

func browsersDirectory(driverDirectory string) string {
	if env := strings.TrimSpace(os.Getenv("PLAYWRIGHT_BROWSERS_PATH")); env != "" {
		if env == "0" {
			return filepath.Join(driverDirectory, "package", ".local-browsers")
		}
		if filepath.IsAbs(env) {
			return env
		}
		if cwd, err := os.Getwd(); err == nil {
			return filepath.Join(cwd, env)
		}
		return env
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cache, "ms-playwright")
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "ms-playwright")
	}
	return "ms-playwright"
}
