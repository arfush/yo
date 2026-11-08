package appearance

import (
	"golang.org/x/text/language"

	"github.com/arfush/yo/ui/resource"
)

// Locale owns the active language: code, locale tag, strings and formatting.
type Locale struct {
	lang resource.Lang
	code string
	tag  language.Tag
}

// newLocale resolves code in res. An empty or unknown code panics.
func newLocale(res *resource.Resource, code string) Locale {
	if res == nil {
		panic("appearance: resources is nil")
	}
	if code == "" {
		panic("appearance: language is empty")
	}
	lang := res.Lang(code)
	tag, err := language.Parse(code)
	if err != nil {
		panic("appearance: parse language \"" + code + "\": " + err.Error())
	}
	return Locale{lang: lang, code: code, tag: tag}
}

// Code returns the language code, e.g. "ru".
func (l Locale) Code() string { return l.code }

// Tag returns the tag Gio expects in layout.Context.Locale.
func (l Locale) Tag() language.Tag { return l.tag }

// Text returns the translation for key.
func (l Locale) Text(key resource.TextKey) string { return l.lang.Text(key) }

// Textf returns the translation for key formatted with args.
func (l Locale) Textf(key resource.TextKey, args ...any) string {
	return l.lang.Textf(key, args...)
}

// TextPlural returns the plural form of key for n.
func (l Locale) TextPlural(key resource.TextKey, n int) string {
	return l.lang.TextPlural(key, n)
}

// TextPluralf returns the plural form of key for n formatted with args.
func (l Locale) TextPluralf(key resource.TextKey, n int, args ...any) string {
	return l.lang.TextPluralf(key, n, args...)
}
