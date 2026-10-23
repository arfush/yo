package browser

import "testing"

func TestSiteURLDefaultsToAnimeLIB(t *testing.T) {
	got, err := SiteURL("")
	if err != nil {
		t.Fatalf("SiteURL: %v", err)
	}
	if got != DefaultSiteURL {
		t.Fatalf("got %q, want %q", got, DefaultSiteURL)
	}
}

func TestSiteURLFromDomain(t *testing.T) {
	for _, input := range []string{"v5.animelib.org", "https://v5.animelib.org", "https://v5.animelib.org/"} {
		got, err := SiteURL(input)
		if err != nil {
			t.Fatalf("SiteURL(%q): %v", input, err)
		}
		if want := "https://v5.animelib.org/"; got != want {
			t.Fatalf("SiteURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSiteURLKeepsExplicitPath(t *testing.T) {
	got, err := SiteURL("https://anilib.me/custom/page?next=%2F")
	if err != nil {
		t.Fatalf("SiteURL: %v", err)
	}
	if want := "https://anilib.me/custom/page?next=%2F"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSiteURLMatchesDefaultDomain(t *testing.T) {
	got, err := SiteURL(DefaultDomain)
	if err != nil {
		t.Fatalf("SiteURL: %v", err)
	}
	if got != DefaultSiteURL {
		t.Fatalf("SiteURL(%q) = %q, want %q", DefaultDomain, got, DefaultSiteURL)
	}
}

func TestSiteURLRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"ftp://anilib.me", "https://", "://bad"} {
		if _, err := SiteURL(input); err == nil {
			t.Fatalf("SiteURL(%q) succeeded", input)
		}
	}
}

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"anilib.me":                             "anilib.me",
		"  v5.animelib.org  ":                   "v5.animelib.org",
		"https://V5.AnimeLib.org/ru/anime/1--x": "v5.animelib.org",
		"http://anilib.me:8080/title/2?foo=bar": "anilib.me:8080",
	}
	for input, want := range cases {
		got, err := NormalizeDomain(input)
		if err != nil {
			t.Fatalf("NormalizeDomain(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeDomain(%q) = %q, want %q", input, got, want)
		}
	}
	for _, input := range []string{"", "ftp://anilib.me", "https://"} {
		if _, err := NormalizeDomain(input); err == nil {
			t.Fatalf("NormalizeDomain(%q) succeeded", input)
		}
	}
}
