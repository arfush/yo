package mpv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
)

const (
	latestReleaseURL = "https://api.github.com/repos/shinchiro/mpv-winbuild-cmake/releases/latest"
	// fallbackAssetURL keeps first install working when the GitHub API is rate
	// limited. The release assets themselves are not rate limited.
	fallbackAssetURL = "https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/20261021/mpv-x86_64-20261021-git-e471496c56.7z"
)

var releaseAssetPattern = regexp.MustCompile(`^mpv-x86_64-\d{8}-git-[0-9a-f]+\.7z$`)

type releaseAsset struct {
	Name string
	URL  string
	Size int64
}

type releaseResponse struct {
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// resolveRelease finds the latest shinchiro Windows build. YOANIME_MPV_URL can
// override the source (used by tests and local mirrors).
func resolveRelease(ctx context.Context, client *http.Client) (releaseAsset, error) {
	if custom := strings.TrimSpace(os.Getenv("YOANIME_MPV_URL")); custom != "" {
		return releaseAsset{Name: path.Base(custom), URL: custom}, nil
	}

	apiCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(apiCtx, http.MethodGet, latestReleaseURL, nil)
	if err == nil {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "yoanime")
		response, requestErr := client.Do(req)
		if requestErr == nil {
			defer response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				var release releaseResponse
				if decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&release); decodeErr == nil {
					assets := make([]releaseAsset, 0, len(release.Assets))
					for _, item := range release.Assets {
						assets = append(assets, releaseAsset{Name: item.Name, URL: item.URL, Size: item.Size})
					}
					if asset, ok := chooseReleaseAsset(assets); ok {
						return asset, nil
					}
				}
			}
		}
	}
	return releaseAsset{
		Name: path.Base(fallbackAssetURL),
		URL:  fallbackAssetURL,
	}, nil
}

// chooseReleaseAsset selects the regular x86_64 build, never a dev/debug/v3
// variant that needs newer CPU instructions.
func chooseReleaseAsset(assets []releaseAsset) (releaseAsset, bool) {
	for _, asset := range assets {
		if releaseAssetPattern.MatchString(asset.Name) && strings.TrimSpace(asset.URL) != "" {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func downloadArchive(ctx context.Context, client *http.Client, asset releaseAsset, dst *os.File, report func(downloaded, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "yoanime")

	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("mpv download: HTTP %d", response.StatusCode)
	}

	total := response.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	if report != nil {
		report(0, total)
	}

	buffer := make([]byte, 128<<10)
	var downloaded int64
	lastPercent := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if report != nil && total > 0 {
				percent := int(downloaded * 100 / total)
				if percent != lastPercent {
					lastPercent = percent
					report(downloaded, total)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if report != nil {
		report(downloaded, total)
	}
	return nil
}

// extractArchive installs mpv.exe and its optional d3dcompiler dependency. Only
// the files needed to run mpv are taken from the release archive.
func extractArchive(ctx context.Context, archivePath, dir string, report func(fraction float64)) error {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open mpv archive: %w", err)
	}
	defer reader.Close()

	wanted := map[string]string{
		"mpv.exe":            "mpv.exe",
		"d3dcompiler_43.dll": "d3dcompiler_43.dll",
	}
	foundMPV := false
	var files []*sevenzip.File
	var total uint64
	for _, file := range reader.File {
		name := strings.ToLower(filepath.Base(file.Name))
		target, ok := wanted[name]
		if !ok || file.FileInfo().IsDir() {
			continue
		}
		if target == "mpv.exe" {
			foundMPV = true
		}
		files = append(files, file)
		total += file.UncompressedSize
	}
	if !foundMPV {
		return errors.New("mpv.exe not found in the downloaded archive")
	}

	var written uint64
	lastPercent := -1
	emit := func(current uint64) {
		if report == nil {
			return
		}
		fraction := 0.0
		if total > 0 {
			fraction = float64(current) / float64(total)
		}
		if fraction > 1 {
			fraction = 1
		}
		percent := int(fraction * 100)
		if percent != lastPercent {
			lastPercent = percent
			report(fraction)
		}
	}

	for _, file := range files {
		name := strings.ToLower(filepath.Base(file.Name))
		target := filepath.Join(dir, wanted[name])
		if err := extractFile(ctx, file, target, &written, emit); err != nil {
			return err
		}
	}
	emit(total)
	return nil
}

func extractFile(ctx context.Context, file *sevenzip.File, target string, written *uint64, emit func(uint64)) error {
	source, err := file.Open()
	if err != nil {
		return fmt.Errorf("open %s in mpv archive: %w", file.Name, err)
	}
	defer source.Close()

	temp, err := os.CreateTemp(filepath.Dir(target), ".mpv-file-*")
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

	buffer := make([]byte, 128<<10)
	for {
		if err := ctx.Err(); err != nil {
			temp.Close()
			return err
		}
		n, readErr := source.Read(buffer)
		if n > 0 {
			if _, writeErr := temp.Write(buffer[:n]); writeErr != nil {
				temp.Close()
				return writeErr
			}
			*written += uint64(n)
			emit(*written)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			temp.Close()
			return readErr
		}
	}
	if err := temp.Close(); err != nil {
		return err
	}
	_ = os.Remove(target)
	if err := os.Rename(tempName, target); err != nil {
		return err
	}
	removeTemp = false
	if filepath.Base(target) == "mpv.exe" {
		_ = os.Chmod(target, 0o755)
	}
	return nil
}
