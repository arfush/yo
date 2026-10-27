package library

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arfush/yoanime/store"
)

const (
	maxCoverBytes  = 12 << 20
	coverUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/154.0.0.0 Safari/537.36"
)

type coverManager struct {
	dir      string
	db       *store.Store
	http     *http.Client
	mu       sync.Mutex
	inflight map[int64]*coverCall
}

type coverCall struct {
	done chan struct{}
	path string
	err  error
}

func newCoverManager(databasePath string, db *store.Store) *coverManager {
	return &coverManager{
		dir:      filepath.Join(filepath.Dir(databasePath), "covers"),
		db:       db,
		http:     &http.Client{Timeout: 20 * time.Second},
		inflight: make(map[int64]*coverCall),
	}
}

// applyCover attaches the local cover file recorded for a title. It is used by
// the history list, where only the cached filename is known.
func (l *Library) applyCover(title *Title) {
	if title == nil || l.covers == nil {
		return
	}
	if path, ok := l.covers.cachedPath(title.ID); ok {
		title.CoverPath = path
		title.CoverNeedsRetry = false
		return
	}
	title.CoverPath = ""
	title.CoverNeedsRetry = false
}

// ensureCover refreshes the poster of a title that was just loaded from
// AnimeLIB. The cover URL is not stored: only the filename of the local file is
// kept. A new file is downloaded only when the filename in the URL differs from
// the cached one.
func (l *Library) ensureCover(ctx context.Context, title *Title) {
	if title == nil || l.covers == nil {
		return
	}
	if title.ID == 0 {
		l.ensurePreviewCover(ctx, title)
		return
	}
	if strings.TrimSpace(title.CoverURL) == "" {
		l.applyCover(title)
		return
	}

	oldPath, hadOld := l.covers.cachedPath(title.ID)
	path, err := l.RefreshCover(ctx, title.ID, title.CoverURL)
	if err != nil {
		if hadOld {
			title.CoverPath = oldPath
		} else {
			title.CoverPath = ""
		}
		title.CoverNeedsRetry = true
		return
	}
	title.CoverPath = path
	title.CoverNeedsRetry = false
}

// PreviewCover downloads the cover of a title that has no local id yet and
// returns the path of the cached file. The file is named after the URL, so a
// saved title can reuse it without downloading the poster again.
func (l *Library) PreviewCover(ctx context.Context, sourceURL string) (string, error) {
	if l == nil || l.covers == nil {
		return "", errors.New("хранилище обложек не открыто")
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return "", errors.New("у тайтла нет ссылки на обложку")
	}
	if path, ok := l.covers.cachedByURL(sourceURL); ok {
		return path, nil
	}
	data, ext, err := l.covers.fetch(ctx, sourceURL)
	if err != nil {
		return "", err
	}
	filename := coverName(sourceURL, ext)
	path := filepath.Join(l.covers.dir, filename)
	if err := l.covers.writeCoverFile(path, data); err != nil {
		return "", err
	}
	return path, nil
}

// ensurePreviewCover attaches a preview cover to an unsaved title. A failure is
// not persisted because there is no title id to record it against; reopening
// the link tries again.
func (l *Library) ensurePreviewCover(ctx context.Context, title *Title) {
	if title == nil || l.covers == nil {
		return
	}
	title.CoverPath = ""
	title.CoverNeedsRetry = false
	if strings.TrimSpace(title.CoverURL) == "" {
		return
	}
	path, err := l.PreviewCover(ctx, title.CoverURL)
	if err != nil {
		title.CoverNeedsRetry = true
		return
	}
	title.CoverPath = path
}

// adoptPreview records the preview file of a freshly saved title as its cover.
// Preview files already use the URL filename, so no copy is needed.
func (l *Library) adoptPreview(tid int64, sourceURL, previewPath string) (string, bool) {
	if l == nil || l.covers == nil || tid == 0 {
		return "", false
	}
	previewPath = strings.TrimSpace(previewPath)
	if previewPath == "" {
		return "", false
	}
	filename := filepath.Base(previewPath)
	if !coverNameMatches(filename, sourceURL) || !validCoverFile(previewPath) {
		return "", false
	}
	if old, err := l.covers.db.GetCoverCache(tid); err == nil && old != nil &&
		old.Filename != "" && filepath.Base(old.Filename) != filename {
		_ = os.Remove(filepath.Join(l.covers.dir, filepath.Base(old.Filename)))
	}
	if err := l.covers.db.SaveCoverSuccess(tid, filename); err != nil {
		return "", false
	}
	return previewPath, true
}

// RefreshCover returns a cached cover or downloads it. A download happens only
// when the filename in sourceURL differs from the cached filename. It is safe
// to call from several windows at once: one title has at most one download in
// flight.
func (l *Library) RefreshCover(ctx context.Context, tid int64, sourceURL string) (string, error) {
	if l == nil || l.covers == nil {
		return "", errors.New("хранилище обложек не открыто")
	}
	if tid == 0 {
		return "", errors.New("у тайтла нет локального id")
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return "", errors.New("у тайтла нет ссылки на обложку")
	}

	cache, err := l.covers.db.GetCoverCache(tid)
	if err != nil {
		return "", err
	}
	if cache != nil && cache.Filename != "" && coverNameMatches(cache.Filename, sourceURL) {
		name := filepath.Base(cache.Filename)
		cached := filepath.Join(l.covers.dir, name)
		if validCoverFile(cached) {
			return cached, nil
		}
		_ = os.Remove(cached)
	}

	l.covers.mu.Lock()
	if call, ok := l.covers.inflight[tid]; ok {
		l.covers.mu.Unlock()
		select {
		case <-call.done:
			return call.path, call.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	call := &coverCall{done: make(chan struct{})}
	l.covers.inflight[tid] = call
	l.covers.mu.Unlock()

	path, err := l.covers.download(ctx, tid, sourceURL)
	call.path, call.err = path, err
	close(call.done)

	l.covers.mu.Lock()
	delete(l.covers.inflight, tid)
	l.covers.mu.Unlock()
	return path, err
}

func (c *coverManager) cachedPath(tid int64) (string, bool) {
	cache, err := c.db.GetCoverCache(tid)
	if err != nil || cache == nil || cache.Filename == "" {
		return "", false
	}
	name := filepath.Base(cache.Filename)
	if name == "." || name == "" {
		return "", false
	}
	path := filepath.Join(c.dir, name)
	if !validCoverFile(path) {
		_ = os.Remove(path)
		_ = c.db.SaveCoverFailure(tid)
		return "", false
	}
	return path, true
}

// cachedByURL finds a local cover whose name matches the filename in the URL.
// The extension may have been appended after the image format was detected.
func (c *coverManager) cachedByURL(sourceURL string) (string, bool) {
	name := coverNameFromURL(sourceURL)
	if name == "" {
		return c.findByPrefix(fallbackNamePrefix(sourceURL))
	}
	if filepath.Ext(name) != "" {
		path := filepath.Join(c.dir, name)
		if validCoverFile(path) {
			return path, true
		}
		return "", false
	}
	return c.findByPrefix(name)
}

func (c *coverManager) findByPrefix(prefix string) (string, bool) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name != prefix && !(strings.HasPrefix(name, prefix+".") && filepath.Ext(name) != "") {
			continue
		}
		path := filepath.Join(c.dir, name)
		if validCoverFile(path) {
			return path, true
		}
	}
	return "", false
}

func validCoverFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	return err == nil && config.Width > 0 && config.Height > 0
}

// fetch downloads and validates a cover image, returning its bytes together
// with the extension matching the detected format.
func (c *coverManager) fetch(ctx context.Context, sourceURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/jpeg,image/png,image/*;q=0.8")
	req.Header.Set("Referer", "https://v5.animelib.org/")
	req.Header.Set("User-Agent", coverUserAgent)

	response, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("обложка: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCoverBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > maxCoverBytes {
		return nil, "", errors.New("обложка: пустой или слишком большой файл")
	}

	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("обложка: повреждённое изображение: %w", err)
	}
	ext := coverExtension(format)
	if ext == "" {
		return nil, "", fmt.Errorf("обложка: неподдерживаемый формат %q", format)
	}
	return data, ext, nil
}

// writeCoverFile stores data at path through a temporary file, so a partially
// written cover is never left behind.
func (c *coverManager) writeCoverFile(path string, data []byte) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(c.dir, ".cover-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func (c *coverManager) download(ctx context.Context, tid int64, sourceURL string) (string, error) {
	data, ext, err := c.fetch(ctx, sourceURL)
	if err != nil {
		c.markFailure(tid)
		return "", err
	}

	filename := coverName(sourceURL, ext)
	path := filepath.Join(c.dir, filename)
	if err := c.writeCoverFile(path, data); err != nil {
		c.markFailure(tid)
		return "", err
	}

	if old, err := c.db.GetCoverCache(tid); err == nil && old != nil &&
		old.Filename != "" && filepath.Base(old.Filename) != filename {
		_ = os.Remove(filepath.Join(c.dir, filepath.Base(old.Filename)))
	}
	_ = c.db.SaveCoverSuccess(tid, filename)
	return path, nil
}

func (c *coverManager) markFailure(tid int64) {
	_ = c.db.SaveCoverFailure(tid)
}

// removeCover deletes the cached cover file and its cache row. A title that was
// never downloaded, or whose row already disappeared, is ignored: deleting a
// history entry must not fail because the cover is missing.
func (l *Library) removeCover(tid int64) {
	if l == nil || l.covers == nil || tid == 0 {
		return
	}
	cache, err := l.covers.db.GetCoverCache(tid)
	if err == nil && cache != nil && cache.Filename != "" {
		name := filepath.Base(cache.Filename)
		if name != "" && name != "." {
			_ = os.Remove(filepath.Join(l.covers.dir, name))
		}
	}
	_ = l.covers.db.DeleteCoverCache(tid)
}

// coverName builds the local filename from the filename in the cover URL. The
// detected extension is appended only when the URL does not carry one.
func coverName(sourceURL, ext string) string {
	if name := coverNameFromURL(sourceURL); name != "" {
		if filepath.Ext(name) == "" && ext != "" {
			name += ext
		}
		return name
	}
	return fallbackNamePrefix(sourceURL) + ext
}

// coverNameMatches reports whether a cached filename was produced by sourceURL.
func coverNameMatches(filename, sourceURL string) bool {
	if filename == "" {
		return false
	}
	name := filepath.Base(filename)
	if expected := coverNameFromURL(sourceURL); expected != "" {
		if name == expected {
			return true
		}
		return filepath.Ext(expected) == "" && strings.HasPrefix(name, expected+".")
	}
	prefix := fallbackNamePrefix(sourceURL)
	return name == prefix || strings.HasPrefix(name, prefix+".")
}

func coverNameFromURL(sourceURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil {
		return ""
	}
	name := path.Base(parsed.Path)
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" || name == `\` {
		return ""
	}
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if r < 32 {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" {
		return ""
	}
	return name
}

func fallbackNamePrefix(sourceURL string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(sourceURL)))
	return fmt.Sprintf("%x", sum[:8])
}

func coverExtension(format string) string {
	switch format {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	default:
		return ""
	}
}
