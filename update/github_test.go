package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

type githubTestResponse struct {
	status int
	body   string
}

func githubTestClient(responses map[string]githubTestResponse) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		key := request.URL.Host + request.URL.Path
		response, ok := responses[key]
		if !ok {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     http.StatusText(http.StatusNotFound),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode:    response.status,
			Status:        http.StatusText(response.status),
			ContentLength: int64(len(response.body)),
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader([]byte(response.body))),
			Request:       request,
		}, nil
	})}
}

func testRepository(t *testing.T) {
	t.Helper()
	oldBase, oldRepo := apiBase, Repository
	t.Cleanup(func() {
		apiBase = oldBase
		Repository = oldRepo
	})
	apiBase = "https://api.test"
	Repository = "arfush/yo"
}

func githubAssetURL(version string) string {
	name := AssetName(version, runtime.GOARCH)
	return "https://github.com/arfush/yo/releases/download/" + version + "/" + name
}

func githubReleaseEntry(version, assetURL, digest string) map[string]any {
	return map[string]any{
		"tag_name": version,
		"body":     "Release notes",
		"assets": []map[string]any{
			{
				"name":                 AssetName(version, runtime.GOARCH),
				"size":                 1234,
				"browser_download_url": assetURL,
				"digest":               digest,
			},
		},
	}
}

func githubReleaseListJSON(entries ...map[string]any) string {
	data, _ := json.Marshal(entries)
	return string(data)
}

func releasesPath() string {
	return "api.test/repos/arfush/yo/releases"
}

func TestFetchReleaseUsesAssetDigest(t *testing.T) {
	testRepository(t)

	version := "v1.4"
	assetName := AssetName(version, runtime.GOARCH)
	assetURL := githubAssetURL(version)
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body:   githubReleaseListJSON(githubReleaseEntry(version, assetURL, "sha256:"+testHash)),
		},
	})

	release, err := FetchRelease(context.Background(), client)
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}
	if release.Version != version || release.AssetName != assetName {
		t.Fatalf("release = %+v", release)
	}
	if release.Size != 1234 || release.SHA256 != testHash {
		t.Fatalf("release metadata = %+v", release)
	}
	if release.URL != assetURL {
		t.Fatalf("url = %q", release.URL)
	}
}

func TestFetchReleaseStableSkipsPatchVersions(t *testing.T) {
	testRepository(t)

	stable := "v1.2"
	beta := "v1.2.1"
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body: githubReleaseListJSON(
				githubReleaseEntry(beta, githubAssetURL(beta), "sha256:"+testHash),
				githubReleaseEntry(stable, githubAssetURL(stable), "sha256:"+testHash),
			),
		},
	})

	release, err := FetchRelease(context.Background(), client)
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}
	if release.Version != stable {
		t.Fatalf("stable release = %q, want %q", release.Version, stable)
	}

	release, err = FetchReleaseChannel(context.Background(), client, ChannelBeta)
	if err != nil {
		t.Fatalf("FetchReleaseChannel: %v", err)
	}
	if release.Version != beta {
		t.Fatalf("beta release = %q, want %q", release.Version, beta)
	}
}

func TestFetchReleaseStableRejectsPatchOnlyFeed(t *testing.T) {
	testRepository(t)

	version := "v1.2.1"
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body:   githubReleaseListJSON(githubReleaseEntry(version, githubAssetURL(version), "sha256:"+testHash)),
		},
	})

	_, err := FetchRelease(context.Background(), client)
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchReleaseBetaIncludesStableAndPatch(t *testing.T) {
	testRepository(t)

	stable := "v1.3"
	patch := "v1.2.9"
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body: githubReleaseListJSON(
				githubReleaseEntry(patch, githubAssetURL(patch), "sha256:"+testHash),
				githubReleaseEntry(stable, githubAssetURL(stable), "sha256:"+testHash),
			),
		},
	})

	release, err := FetchReleaseChannel(context.Background(), client, ChannelBeta)
	if err != nil {
		t.Fatalf("FetchReleaseChannel: %v", err)
	}
	if release.Version != stable {
		t.Fatalf("beta release = %q, want the newer stable %q", release.Version, stable)
	}
}

func TestFetchReleaseIgnoresDraftsPrereleasesAndInvalidTags(t *testing.T) {
	testRepository(t)

	stable := "v1.4"
	prerelease := githubReleaseEntry("v1.5", githubAssetURL("v1.5"), "sha256:"+testHash)
	prerelease["prerelease"] = true
	draft := githubReleaseEntry("v1.6", githubAssetURL("v1.6"), "sha256:"+testHash)
	draft["draft"] = true
	invalid := githubReleaseEntry("nightly", "https://github.com/arfush/yo/releases/download/nightly/nightly.exe", "sha256:"+testHash)
	fourParts := githubReleaseEntry("v1.7.0.1", githubAssetURL("v1.7.0.1"), "sha256:"+testHash)
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body: githubReleaseListJSON(
				draft,
				prerelease,
				fourParts,
				invalid,
				githubReleaseEntry(stable, githubAssetURL(stable), "sha256:"+testHash),
			),
		},
	})

	release, err := FetchRelease(context.Background(), client)
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}
	if release.Version != stable {
		t.Fatalf("release = %q, want %q", release.Version, stable)
	}
}

func TestFetchReleaseFallsBackToChecksumAsset(t *testing.T) {
	testRepository(t)

	version := "v1.4"
	assetName := AssetName(version, runtime.GOARCH)
	assetURL := githubAssetURL(version)
	checksumURL := assetURL + ".sha256"
	payload := githubReleaseListJSON(map[string]any{
		"tag_name": version,
		"assets": []map[string]any{
			{
				"name":                 assetName,
				"size":                 1234,
				"browser_download_url": assetURL,
			},
			{
				"name":                 assetName + ".sha256",
				"size":                 70,
				"browser_download_url": checksumURL,
			},
		},
	})
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body:   payload,
		},
		"github.com/arfush/yo/releases/download/" + version + "/" + assetName + ".sha256": {
			status: http.StatusOK,
			body:   testHash + "  " + assetName + "\n",
		},
	})

	release, err := FetchRelease(context.Background(), client)
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}
	if release.SHA256 != testHash {
		t.Fatalf("sha256 = %q", release.SHA256)
	}
}

func TestFetchReleaseNoMatchingAsset(t *testing.T) {
	testRepository(t)

	payload := fmt.Sprintf(`[{"tag_name":"v1.4","assets":[{"name":"yo_v1.4_other.exe","size":1234,"browser_download_url":"https://github.com/arfush/yo/releases/download/v1.4/other.exe"}]}]`)
	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {status: http.StatusOK, body: payload},
	})
	_, err := FetchRelease(context.Background(), client)
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchReleaseNotFound(t *testing.T) {
	testRepository(t)

	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {status: http.StatusNotFound, body: ""},
	})
	_, err := FetchRelease(context.Background(), client)
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchReleaseRejectsUntrustedAssetURL(t *testing.T) {
	testRepository(t)

	client := githubTestClient(map[string]githubTestResponse{
		releasesPath(): {
			status: http.StatusOK,
			body:   githubReleaseListJSON(githubReleaseEntry("v1.4", "https://evil.example/yo.exe", "sha256:"+testHash)),
		},
	})
	_, err := FetchRelease(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error")
	}
}
