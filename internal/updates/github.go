package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

// FetchRelease reads the latest published GitHub release and returns the asset
// for the running architecture. ErrNoRelease means there is no release or no
// matching asset.
func FetchRelease(ctx context.Context, client *http.Client) (*Release, error) {
	repository := strings.Trim(strings.TrimSpace(Repository), "/")
	if repository == "" || !strings.Contains(repository, "/") {
		return nil, ErrNotConfigured
	}
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}

	endpoint := strings.TrimRight(apiBase, "/") + "/repos/" + repository + "/releases/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", userAgent)

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNoRelease
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch GitHub release: HTTP %d", response.StatusCode)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxAPIBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read GitHub release: %w", err)
	}
	if len(payload) > maxAPIBytes {
		return nil, fmt.Errorf("read GitHub release: response is too large")
	}

	var latest githubRelease
	if err := json.Unmarshal(payload, &latest); err != nil {
		return nil, fmt.Errorf("decode GitHub release: %w", err)
	}
	version := NormalizeVersion(latest.TagName)
	if version == "" {
		return nil, fmt.Errorf("decode GitHub release: invalid tag %q", latest.TagName)
	}

	name := AssetName(version, runtime.GOARCH)
	asset, ok := findAsset(latest.Assets, name)
	if !ok {
		return nil, fmt.Errorf("%w: no %s asset in %s", ErrNoRelease, runtime.GOARCH, version)
	}
	if asset.Size <= 0 || asset.Size > maxReleaseRaw {
		return nil, fmt.Errorf("GitHub release asset %q has invalid size %d", name, asset.Size)
	}
	if !validAssetURL(asset.URL, repository) {
		return nil, fmt.Errorf("GitHub release asset %q has an untrusted URL", name)
	}

	digest := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(asset.Digest)), "sha256:")
	if !validSHA256(digest) {
		digest, err = fetchChecksum(ctx, client, latest.Assets, name, repository)
		if err != nil {
			return nil, err
		}
	}

	release := &Release{
		Version:   version,
		Size:      asset.Size,
		URL:       asset.URL,
		SHA256:    digest,
		AssetName: name,
		Notes:     latest.Body,
	}
	if !release.Valid() {
		return nil, fmt.Errorf("GitHub release %s is incomplete", version)
	}
	return release, nil
}

func findAsset(assets []githubAsset, name string) (githubAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return githubAsset{}, false
}

func fetchChecksum(ctx context.Context, client *http.Client, assets []githubAsset, name, repository string) (string, error) {
	checksum, ok := findAsset(assets, name+".sha256")
	if !ok {
		return "", fmt.Errorf("GitHub release asset %q has no SHA-256", name)
	}
	if !validAssetURL(checksum.URL, repository) {
		return "", fmt.Errorf("GitHub checksum asset %q has an untrusted URL", checksum.Name)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksum.URL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/plain")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch GitHub checksum: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch GitHub checksum: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumSize+1))
	if err != nil {
		return "", fmt.Errorf("read GitHub checksum: %w", err)
	}
	if len(data) > maxChecksumSize {
		return "", fmt.Errorf("read GitHub checksum: file is too large")
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("GitHub checksum file is empty")
	}
	digest := strings.ToLower(fields[0])
	if !validSHA256(digest) {
		return "", fmt.Errorf("GitHub checksum file has an invalid digest")
	}
	return digest, nil
}

func validAssetURL(value, repository string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" &&
		strings.EqualFold(parsed.Hostname(), "github.com") &&
		parsed.User == nil &&
		parsed.Fragment == "" &&
		strings.HasPrefix(parsed.Path, "/"+repository+"/releases/download/")
}
