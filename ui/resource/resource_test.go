package resource

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestTextSequenceCycles(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en")
	for i, want := range []string{"a", "b", "c", "a"} {
		if got := l.Text("phrase"); got != want {
			t.Fatalf("call %d = %q; want %q", i+1, got, want)
		}
	}
}

func TestTextRandomPicksVariant(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en")
	for i := 0; i < 20; i++ {
		if got := l.Text("random.phrase"); got != "a" && got != "b" {
			t.Fatalf("call %d = %q; want one of the variants", i+1, got)
		}
	}
}

func TestTextfFormats(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en")
	if got := l.Textf("greeting", "world"); got != "Hello, world" {
		t.Fatalf("Textf = %q; want %q", got, "Hello, world")
	}
}

func TestTextPluralLooksUpFormKey(t *testing.T) {
	t.Parallel()

	l := testLang(t, "ru")
	tests := []struct {
		n    int
		want string
	}{
		{1, "1 серия"},
		{21, "21 серия"},
		{11, "11 серий"},
		{2, "2 серии"},
		{24, "24 серии"},
		{12, "12 серий"},
		{25, "25 серий"},
		{0, "0 серий"},
		{-3, "-3 серии"},
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

	l := testLang(t, "ru")
	if got := l.TextPluralf("episode", 3, 3); got != "3 серии" {
		t.Fatalf("TextPluralf = %q; want %q", got, "3 серии")
	}

	plain := testLang(t, "en")
	if got := plain.TextPluralf("episode", 1); got != "one episode" {
		t.Fatalf("TextPluralf without args = %q; want %q", got, "one episode")
	}
}

func TestMissingTextKeyPanics(t *testing.T) {
	t.Parallel()

	l := testLang(t, "en")
	assertPanics(t, func() { l.Text("missing") })
}

func TestMissingLangPanics(t *testing.T) {
	t.Parallel()

	assertPanics(t, func() { New().Lang("xx") })
}

func TestMissingFontPanics(t *testing.T) {
	t.Parallel()

	assertPanics(t, func() { New().Font("missing") })
}

func TestMissingIconPanics(t *testing.T) {
	t.Parallel()

	assertPanics(t, func() { New().IconPath("missing") })
}

func TestMissingAppIconPanics(t *testing.T) {
	t.Parallel()

	r := New()
	assertPanics(t, func() { r.AppIcon("missing") })
	assertPanics(t, func() { r.AppIconICO("missing") })
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

func TestAppIconICOReturnsCopy(t *testing.T) {
	t.Parallel()

	r := New()
	first := r.AppIconICO(AppIconYobase)
	original := bytes.Clone(first)
	first[0] ^= 0xff
	if got := r.AppIconICO(AppIconYobase); !bytes.Equal(got, original) {
		t.Fatal("AppIconICO exposed the internal slice")
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

func testLang(t *testing.T, code string) Lang {
	t.Helper()
	return testResource(t).Lang(code)
}

// testResource loads the fixtures from testdata. The production constructor
// embeds assets, so tests use the same loader with a directory instead.
func testResource(t *testing.T) *Resource {
	t.Helper()
	return newResource(os.DirFS("testdata"))
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
