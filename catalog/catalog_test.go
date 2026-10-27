package catalog

import "testing"

func TestIsReadingSource(t *testing.T) {
	for source, want := range map[string]bool{
		SourceAnimeLIB:  false,
		SourceKinopoisk: false,
		SourceMangaLib:  true,
		SourceYaoiLib:   true,
		SourceRanobeLib: true,
		SourceHentaiLib: true,
		"":              false,
	} {
		if got := IsReadingSource(source); got != want {
			t.Fatalf("IsReadingSource(%q) = %v, want %v", source, got, want)
		}
	}
}

func TestIsRanobeSource(t *testing.T) {
	if !IsRanobeSource(SourceRanobeLib) {
		t.Fatal("ranobelib must be a ranobe source")
	}
	if IsRanobeSource(SourceMangaLib) || IsRanobeSource(SourceHentaiLib) {
		t.Fatal("manga sources must not be ranobe sources")
	}
}

func TestReadingChapterKeyAndSplit(t *testing.T) {
	cases := []struct {
		volume, number string
		key            string
	}{
		{"1", "0", "1:0"},
		{"", "12", "0:12"},
		{"2", "0.5", "2:0.5"},
	}
	for _, tc := range cases {
		key := ReadingChapterKey(tc.volume, tc.number)
		if key != tc.key {
			t.Fatalf("ReadingChapterKey(%q, %q) = %q, want %q", tc.volume, tc.number, key, tc.key)
		}
		volume, number := SplitReadingChapterKey(key)
		wantVolume := tc.volume
		if wantVolume == "" {
			wantVolume = "0"
		}
		if volume != wantVolume || number != tc.number {
			t.Fatalf("SplitReadingChapterKey(%q) = %q, %q; want %q, %q", key, volume, number, wantVolume, tc.number)
		}
	}
}
