package resource

import (
	"fmt"
	"testing"
	"testing/fstest"
)

func TestPluralForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		n    int
		want string
	}{
		{"en one", "en", 1, "one"},
		{"en zero", "en", 0, "other"},
		{"en few", "en", 3, "other"},
		{"en negative one", "en", -1, "one"},
		{"ru one", "ru", 1, "one"},
		{"ru twenty one", "ru", 21, "one"},
		{"ru eleven", "ru", 11, "many"},
		{"ru two", "ru", 2, "few"},
		{"ru twenty four", "ru", 24, "few"},
		{"ru twelve", "ru", 12, "many"},
		{"ru twenty five", "ru", 25, "many"},
		{"ru zero", "ru", 0, "many"},
		{"ru negative three", "ru", -3, "few"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := pluralForm(tt.code, tt.n); got != tt.want {
				t.Fatalf("pluralForm(%q, %d) = %q; want %q", tt.code, tt.n, got, tt.want)
			}
		})
	}
}

func TestTextSequenceCycles(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en", `{"phrase":{"variants":["a","b","c"]}}`)
	for i, want := range []string{"a", "b", "c", "a"} {
		if got := l.Text("phrase"); got != want {
			t.Fatalf("call %d = %q; want %q", i+1, got, want)
		}
	}
}

func TestTextRandomPicksVariant(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en", `{"phrase":{"sort":"random","variants":["a","b"]}}`)
	for i := 0; i < 20; i++ {
		if got := l.Text("phrase"); got != "a" && got != "b" {
			t.Fatalf("call %d = %q; want one of the variants", i+1, got)
		}
	}
}

func TestTextfFormats(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en", `{"greeting":"Hello, %s"}`)
	if got := l.Textf("greeting", "world"); got != "Hello, world" {
		t.Fatalf("Textf = %q; want %q", got, "Hello, world")
	}
}

func TestTextPluralLooksUpFormKey(t *testing.T) {
	t.Parallel()

	l := testLang(t, "ru", `{
		"episode.one":"%d серия",
		"episode.few":"%d серии",
		"episode.many":"%d серий"
	}`)
	tests := []struct {
		n    int
		want string
	}{
		{1, "1 серия"},
		{3, "3 серии"},
		{5, "5 серий"},
	}
	for _, tt := range tests {
		got := fmt.Sprintf(l.TextPlural("episode", tt.n), tt.n)
		if got != tt.want {
			t.Fatalf("TextPlural(episode, %d) = %q; want %q", tt.n, got, tt.want)
		}
	}
}

func TestTextPluralfFormats(t *testing.T) {
	t.Parallel()

	l := testLang(t, "ru", `{
		"episode.one":"%d серия",
		"episode.few":"%d серии",
		"episode.many":"%d серий"
	}`)
	if got := l.TextPluralf("episode", 3, 3); got != "3 серии" {
		t.Fatalf("TextPluralf = %q; want %q", got, "3 серии")
	}

	plain := testLang(t, "en", `{"episode.one":"one episode","episode.other":"episodes"}`)
	if got := plain.TextPluralf("episode", 1); got != "one episode" {
		t.Fatalf("TextPluralf without args = %q; want %q", got, "one episode")
	}
}

func TestMissingTextKeyPanics(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en", `{}`)
	assertPanics(t, func() { l.Text("missing") })
}

func TestMissingLangPanics(t *testing.T) {
	t.Parallel()

	r := newResource(fstest.MapFS{})
	assertPanics(t, func() { r.Lang("en") })
}

func TestMissingFontPanics(t *testing.T) {
	t.Parallel()

	r := newResource(fstest.MapFS{})
	assertPanics(t, func() { r.Font(FontRegular) })
}

func TestMissingIconPanics(t *testing.T) {
	t.Parallel()

	r := newResource(fstest.MapFS{})
	assertPanics(t, func() { r.Icon(IconPlay) })
}

func TestMissingAppIconPanics(t *testing.T) {
	t.Parallel()

	r := newResource(fstest.MapFS{})
	assertPanics(t, func() { r.AppIcon(AppIconYobase) })
	assertPanics(t, func() { r.AppIconICO(AppIconYobase) })
}

func TestEmbeddedAppIcons(t *testing.T) {
	t.Parallel()

	r := New()
	for _, key := range []AppIconKey{
		AppIconYobase,
		AppIconYoanime,
		AppIconYomanga,
		AppIconYoranobe,
	} {
		if got := r.AppIcon(key); got.Bounds().Dx() <= 0 {
			t.Fatalf("AppIcon(%q) is empty", key)
		}
		if got := r.AppIconICO(key); len(got) == 0 {
			t.Fatalf("AppIconICO(%q) is empty", key)
		}
	}
}

func TestEmbeddedLangs(t *testing.T) {
	t.Parallel()

	r := New()
	for _, code := range []string{"en", "ru"} {
		if got := r.Lang(code); got.Code() != code {
			t.Fatalf("Lang(%q).Code() = %q", code, got.Code())
		}
	}
}

func testLang(t *testing.T, code, body string) Lang {
	t.Helper()
	r := newResource(fstest.MapFS{
		"lang/" + code + ".json": &fstest.MapFile{Data: []byte(body)},
	})
	return r.Lang(code)
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("want panic")
		}
	}()
	fn()
}
