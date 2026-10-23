package updates

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

func githubReleaseJSON(version, assetURL, digest string) string {
	assetName := AssetName(version, runtime.GOARCH)
	payload := map[string]any{
		"tag_name": version,
		"body":     "Release notes",
		"assets": []map[string]any{
			{
				"name":                 assetName,
				"size":                 1234,
				"browser_download_url": assetURL,
				"digest":               digest,
			},
		},
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func TestFetchReleaseUsesAssetDigest(t *testing.T) {
	oldBase, oldRepo := apiBase, Repository
	t.Cleanup(func() {
		apiBase = oldBase
		Repository = oldRepo
	})
	apiBase = "https://api.test"
	Repository = "arfush/yonime"

	version := "v1.4.0"
	assetName := AssetName(version, runtime.GOARCH)
	assetURL := "https://github.com/arfush/yonime/releases/download/" + version + "/" + assetName
	client := githubTestClient(map[string]githubTestResponse{
		"api.test/repos/arfush/yonime/releases/latest": {
			status: http.StatusOK,
			body:   githubReleaseJSON(version, assetURL, "sha256:"+testHash),
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

func TestFetchReleaseFallsBackToChecksumAsset(t *testing.T) {
	oldBase, oldRepo := apiBase, Repository
	t.Cleanup(func() {
		apiBase = oldBase
		Repository = oldRepo
	})
	apiBase = "https://api.test"
	Repository = "arfush/yonime"

	version := "v1.4.0"
	assetName := AssetName(version, runtime.GOARCH)
	assetURL := "https://github.com/arfush/yonime/releases/download/" + version + "/" + assetName
	checksumURL := assetURL + ".sha256"
	payload := map[string]any{
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
	}
	apiBody, _ := json.Marshal(payload)
	client := githubTestClient(map[string]githubTestResponse{
		"api.test/repos/arfush/yonime/releases/latest": {
			status: http.StatusOK,
			body:   string(apiBody),
		},
		"github.com/arfush/yonime/releases/download/" + version + "/" + assetName + ".sha256": {
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
	oldBase, oldRepo := apiBase, Repository
	t.Cleanup(func() {
		apiBase = oldBase
		Repository = oldRepo
	})
	apiBase = "https://api.test"
	Repository = "arfush/yonime"

	payload := fmt.Sprintf(`{"tag_name":"v1.4.0","assets":[{"name":"yonime_v1.4.0_other.exe","size":1234,"browser_download_url":"https://github.com/arfush/yonime/releases/download/v1.4.0/other.exe"}]}`)
	client := githubTestClient(map[string]githubTestResponse{
		"api.test/repos/arfush/yonime/releases/latest": {status: http.StatusOK, body: payload},
	})
	_, err := FetchRelease(context.Background(), client)
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchReleaseNotFound(t *testing.T) {
	oldBase, oldRepo := apiBase, Repository
	t.Cleanup(func() {
		apiBase = oldBase
		Repository = oldRepo
	})
	apiBase = "https://api.test"
	Repository = "arfush/yonime"

	client := githubTestClient(map[string]githubTestResponse{
		"api.test/repos/arfush/yonime/releases/latest": {status: http.StatusNotFound, body: ""},
	})
	_, err := FetchRelease(context.Background(), client)
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchReleaseRejectsUntrustedAssetURL(t *testing.T) {
	oldBase, oldRepo := apiBase, Repository
	t.Cleanup(func() {
		apiBase = oldBase
		Repository = oldRepo
	})
	apiBase = "https://api.test"
	Repository = "arfush/yonime"

	client := githubTestClient(map[string]githubTestResponse{
		"api.test/repos/arfush/yonime/releases/latest": {
			status: http.StatusOK,
			body:   githubReleaseJSON("v1.4.0", "https://evil.example/yonime.exe", "sha256:"+testHash),
		},
	})
	_, err := FetchRelease(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error")
	}
}
