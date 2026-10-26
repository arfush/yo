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

	"github.com/arfush/yonime/log"
)

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Body       string        `json:"body"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

const (
	releasesPerPage = 100
	maxReleasePages = 10
)

// FetchRelease reads the newest published stable GitHub release and returns the
// asset for the running architecture. ErrNoRelease means there is no release or
// no matching asset.
func FetchRelease(ctx context.Context, client *http.Client) (*Release, error) {
	return FetchReleaseChannel(ctx, client, ChannelStable)
}

// FetchReleaseChannel reads the newest published release accepted by channel.
// A beta installation also sees patch releases (v1.2.3), while a stable one
// only accepts releases with one or two version components (v1.2).
func FetchReleaseChannel(ctx context.Context, client *http.Client, channel Channel) (*Release, error) {
	repository := strings.Trim(strings.TrimSpace(Repository), "/")
	if repository == "" || !strings.Contains(repository, "/") {
		return nil, ErrNotConfigured
	}
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}

	log.Debug("checking GitHub releases", "repository", repository, "channel", channel)
	latest, err := fetchBestRelease(ctx, client, repository, channel)
	if err != nil {
		log.Warn("GitHub release check failed", "repository", repository, "error", err)
		return nil, err
	}
	if latest == nil {
		log.Info("no matching GitHub release", "repository", repository, "channel", channel)
		return nil, ErrNoRelease
	}
	release, err := releaseFromGitHub(ctx, client, repository, latest)
	if err != nil {
		log.Warn("cannot read GitHub release", "repository", repository, "tag", latest.TagName, "error", err)
		return nil, err
	}
	log.Info("GitHub release found", "version", release.Version, "channel", channel)
	return release, nil
}

// fetchBestRelease scans published releases page by page and returns the
// highest version accepted by the channel that has an asset for this
// architecture.
func fetchBestRelease(ctx context.Context, client *http.Client, repository string, channel Channel) (*githubRelease, error) {
	var best *githubRelease
	bestVersion := ""
	for page := 1; page <= maxReleasePages; page++ {
		endpoint := fmt.Sprintf(
			"%s/repos/%s/releases?per_page=%d&page=%d",
			strings.TrimRight(apiBase, "/"),
			repository,
			releasesPerPage,
			page,
		)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		request.Header.Set("User-Agent", userAgent)

		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("fetch GitHub releases: %w", err)
		}
		if response.StatusCode == http.StatusNotFound {
			response.Body.Close()
			return nil, ErrNoRelease
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("fetch GitHub releases: HTTP %d", response.StatusCode)
		}

		payload, readErr := io.ReadAll(io.LimitReader(response.Body, maxAPIBytes+1))
		response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read GitHub releases: %w", readErr)
		}
		if len(payload) > maxAPIBytes {
			return nil, fmt.Errorf("read GitHub releases: response is too large")
		}

		var releases []githubRelease
		if err := json.Unmarshal(payload, &releases); err != nil {
			return nil, fmt.Errorf("decode GitHub releases: %w", err)
		}
		for index := range releases {
			release := &releases[index]
			if release.Draft {
				continue
			}
			version := NormalizeVersion(release.TagName)
			if version == "" || !channel.Accepts(version) {
				continue
			}
			// Stable builds never come from a prerelease flag. Beta builds
			// may, but the version still has to follow the patch scheme.
			if release.Prerelease && channel != ChannelBeta {
				continue
			}
			if _, ok := findAsset(release.Assets, AssetName(version, runtime.GOARCH)); !ok {
				continue
			}
			if best == nil || IsNewer(version, bestVersion) {
				best = release
				bestVersion = version
			}
		}
		if len(releases) < releasesPerPage {
			break
		}
	}
	return best, nil
}

// releaseFromGitHub validates one selected release and adds the checksum.
func releaseFromGitHub(ctx context.Context, client *http.Client, repository string, latest *githubRelease) (*Release, error) {
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
	var err error
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
