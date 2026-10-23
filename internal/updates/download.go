package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const downloadBufferSize = 64 << 10

var allowedDownloadHosts = map[string]bool{
	"github.com":                            true,
	"objects.githubusercontent.com":         true,
	"release-assets.githubusercontent.com":  true,
	"github-releases.githubusercontent.com": true,
}

// Download fetches a release, verifies its size, SHA-256 and MZ header, and
// stores it at destination. Progress is reported with the expected total size
// taken from the release metadata, so the bar does not depend on
// Content-Length.
func Download(ctx context.Context, client *http.Client, release Release, destination string, report func(downloaded, total int64)) (string, error) {
	if !release.Valid() {
		return "", fmt.Errorf("invalid release")
	}
	if destination == "" {
		return "", fmt.Errorf("empty download destination")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", fmt.Errorf("create update directory: %w", err)
	}

	temporary := destination + ".part"
	_ = os.Remove(temporary)
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("create download file: %w", err)
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(temporary)
		}
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.URL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Yonime")
	request.Header.Set("Accept-Encoding", "identity")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download update: %w", err)
	}
	defer response.Body.Close()
	if response.Request == nil || response.Request.URL == nil || !allowedDownloadHosts[response.Request.URL.Hostname()] {
		return "", fmt.Errorf("download update: untrusted download host")
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download update: HTTP %d", response.StatusCode)
	}
	if response.ContentLength >= 0 && response.ContentLength != release.Size {
		return "", fmt.Errorf(
			"download update: server sent %d bytes, release says %d",
			response.ContentLength,
			release.Size,
		)
	}

	if report != nil {
		report(0, release.Size)
	}

	hasher := sha256.New()
	header := make([]byte, 2)
	if _, err := io.ReadFull(response.Body, header); err != nil {
		return "", fmt.Errorf("download update: read executable header: %w", err)
	}
	if header[0] != 'M' || header[1] != 'Z' {
		return "", fmt.Errorf("download update: file is not a Windows executable")
	}
	if _, err := file.Write(header); err != nil {
		return "", fmt.Errorf("write update: %w", err)
	}
	_, _ = hasher.Write(header)
	written := int64(len(header))

	buffer := make([]byte, downloadBufferSize)
	lastReport := time.Time{}

	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			if written+int64(read) > maxReleaseRaw {
				return "", fmt.Errorf("download update: file is too large")
			}
			if _, writeErr := file.Write(buffer[:read]); writeErr != nil {
				return "", fmt.Errorf("write update: %w", writeErr)
			}
			if _, hashErr := hasher.Write(buffer[:read]); hashErr != nil {
				return "", hashErr
			}
			written += int64(read)
			if report != nil && time.Since(lastReport) >= 100*time.Millisecond {
				report(written, release.Size)
				lastReport = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", fmt.Errorf("download update: %w", readErr)
		}
	}
	if written != release.Size {
		return "", fmt.Errorf("download update: got %d bytes, expected %d", written, release.Size)
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(digest, release.SHA256) {
		return "", fmt.Errorf("download update: SHA-256 mismatch")
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("flush update: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close update: %w", err)
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("replace previous update: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		return "", fmt.Errorf("finish update download: %w", err)
	}
	committed = true
	if report != nil {
		report(release.Size, release.Size)
	}
	return destination, nil
}
