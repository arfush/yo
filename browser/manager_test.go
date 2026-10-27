package browser

import (
	"math"
	"reflect"
	"testing"
)

const sampleInstallPlan = `Chrome for Testing 151.0.7922.34 (playwright chromium v1234)
  Install location:    C:\cache\chromium-1234
  Download url:        https://cdn.example/chromium.zip

FFmpeg (playwright ffmpeg v1011)
  Install location:    C:\cache\ffmpeg-1011
  Download url:        https://cdn.example/ffmpeg.zip
  Download fallback 1: https://fallback.example/ffmpeg.zip

Chrome Headless Shell 151.0.7922.34 (playwright chromium-headless-shell v1234)
  Install location:    C:\cache\chromium_headless_shell-1234
  Download url:        https://cdn.example/headless.zip
`

func TestParseInstallPlan(t *testing.T) {
	components := parseInstallPlan(sampleInstallPlan)
	if len(components) != 3 {
		t.Fatalf("got %d components, want 3", len(components))
	}

	if got, want := components[0].title, "Chrome for Testing 151.0.7922.34 (playwright chromium v1234)"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if got, want := components[0].dir, `C:\cache\chromium-1234`; got != want {
		t.Errorf("dir = %q, want %q", got, want)
	}
	if want := []string{"https://cdn.example/chromium.zip"}; !reflect.DeepEqual(components[0].urls, want) {
		t.Errorf("urls = %#v, want %#v", components[0].urls, want)
	}
	if want := []string{"https://cdn.example/ffmpeg.zip", "https://fallback.example/ffmpeg.zip"}; !reflect.DeepEqual(components[1].urls, want) {
		t.Errorf("fallback urls = %#v, want %#v", components[1].urls, want)
	}
}

func TestDownloadingTitle(t *testing.T) {
	title, ok := downloadingTitle("Downloading Chrome for Testing 151.0.7922.34 (playwright chromium v1234) from https://cdn.example/chromium.zip")
	if !ok {
		t.Fatal("line was not recognized")
	}
	if want := "Chrome for Testing 151.0.7922.34 (playwright chromium v1234)"; title != want {
		t.Fatalf("title = %q, want %q", title, want)
	}
}

func TestProgressPercent(t *testing.T) {
	percent, ok := progressPercent("|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■|  26% of 201.1 MiB")
	if !ok || percent != 26 {
		t.Fatalf("got %d, %v; want 26, true", percent, ok)
	}
}

func TestProgressTrackerAggregatesFiles(t *testing.T) {
	components := []installComponent{
		{title: "Chromium", total: 100},
		{title: "Headless", total: 300},
	}
	var got []float64
	tracker := newProgressTracker(components, func(progress Progress) {
		got = append(got, progress.Percent)
	})

	tracker.handleLine("Downloading Chromium from https://cdn.example/chromium.zip")
	tracker.handleLine("|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■|  50% of 100.0 MiB")
	tracker.handleLine("|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■| 100% of 100.0 MiB")
	tracker.handleLine("Downloading Headless from https://cdn.example/headless.zip")
	tracker.handleLine("|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■|  50% of 300.0 MiB")
	tracker.finish()

	if len(got) == 0 {
		t.Fatal("no progress was reported")
	}
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Fatalf("progress went backwards: %v", got)
		}
	}
	if got[len(got)-1] != 100 {
		t.Fatalf("final progress = %v, want 100", got[len(got)-1])
	}
	// 50% of 100 plus 0 of 300 is 12.5%; 100% of 100 is 25%; 100 of 100
	// plus 150 of 300 is 62.5%.
	if !containsPercent(got, 12) || !containsPercent(got, 25) || !containsPercent(got, 62) {
		t.Fatalf("progress sequence %v does not contain expected values", got)
	}
}

func TestProgressTrackerWithoutSizes(t *testing.T) {
	components := []installComponent{{title: "A"}, {title: "B"}}
	var got []float64
	tracker := newProgressTracker(components, func(progress Progress) {
		got = append(got, progress.Percent)
	})

	tracker.handleLine("Downloading A from https://cdn.example/a.zip")
	tracker.handleLine("|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■|  50% of 1.0 MiB")
	tracker.handleLine("Downloading B from https://cdn.example/b.zip")
	tracker.handleLine("|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■|  50% of 1.0 MiB")
	tracker.finish()

	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Fatalf("progress went backwards: %v", got)
		}
	}
	if got[len(got)-1] != 100 {
		t.Fatalf("final progress = %v, want 100", got[len(got)-1])
	}
	if len(got) < 4 {
		t.Fatalf("too few progress updates: %v", got)
	}
}

func TestStripANSI(t *testing.T) {
	if got, want := stripANSI("\x1b[2mfrom https://cdn.example\x1b[22m"), "from https://cdn.example"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUsefulOutput(t *testing.T) {
	input := "Downloading Chromium from https://cdn.example/chromium.zip\n|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■|  26% of 201.1 MiB\nFailed to install browsers\nError: ENOSPC: no space left on device\n"
	want := "Failed to install browsers\nError: ENOSPC: no space left on device"
	if got := usefulOutput(input); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func containsPercent(values []float64, want int) bool {
	for _, value := range values {
		if int(math.Round(value)) == want {
			return true
		}
	}
	return false
}
