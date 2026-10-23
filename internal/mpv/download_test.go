package mpv

import "testing"

func TestChooseReleaseAssetSkipsDevelopmentBuilds(t *testing.T) {
	assets := []releaseAsset{
		{Name: "mpv-dev-x86_64-20261021-git-e471496c56.7z", URL: "dev"},
		{Name: "mpv-debug-x86_64-20261021-git-e471496c56.7z", URL: "debug"},
		{Name: "mpv-x86_64-v3-20261021-git-e471496c56.7z", URL: "v3"},
		{Name: "mpv-x86_64-20261021-git-e471496c56.7z", URL: "regular"},
	}
	asset, ok := chooseReleaseAsset(assets)
	if !ok {
		t.Fatal("regular build was not found")
	}
	if asset.URL != "regular" {
		t.Fatalf("selected %q, want regular build", asset.URL)
	}
}

func TestChooseReleaseAssetRejectsUnexpectedNames(t *testing.T) {
	assets := []releaseAsset{
		{Name: "mpv-aarch64-20261021-git-e471496c56.7z", URL: "arm"},
		{Name: "ffmpeg-x86_64-git-9a861e968.7z", URL: "ffmpeg"},
	}
	if _, ok := chooseReleaseAsset(assets); ok {
		t.Fatal("unexpected asset was selected")
	}
}
