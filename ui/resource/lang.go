package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"strings"
)

// TextKey is a translation key: its value is the key in lang/<code>.json.
// Plural keys use the "<key>.<form>" suffix, e.g. "episode.one".
type TextKey string

// Lang is a translation language bound to a Resource. Use Resource.Lang to get
// a value; the zero value is invalid and its methods panic.
type Lang struct {
	res  *Resource
	code string
}

// Lang returns the embedded language for code. An unknown code panics: the
// language catalog is part of the build, not runtime data.
func (r *Resource) Lang(code string) Lang {
	if _, ok := r.lexicons[code]; !ok {
		panic("resource: lang \"" + code + "\" is not embedded")
	}
	return Lang{res: r, code: code}
}

// Text returns the translation for key in lang. A plain entry has one variant;
// a variant entry is served in order or randomly, depending on its sort. Call
// Text once per presentation, not every frame: sequential entries advance per
// call. A missing key panics.
func (r *Resource) Text(lang Lang, key TextKey) string {
	if lang.res == nil {
		panic("resource: lang is zero")
	}
	if key == "" {
		panic("resource: text key is empty")
	}
	code := lang.Code()
	lex, ok := r.lexicons[code]
	if !ok {
		panic("resource: lang \"" + code + "\" is not embedded")
	}
	entry, ok := lex[string(key)]
	if !ok {
		panic("resource: text key \"" + string(key) + "\" is missing in " + code)
	}
	switch len(entry.variants) {
	case 0:
		return ""
	case 1:
		return entry.variants[0]
	}
	if entry.sort == textSortRandom {
		return entry.variants[rand.IntN(len(entry.variants))]
	}
	cursor := variantCursor{lang: code, key: string(key)}
	r.mu.Lock()
	index := r.next[cursor]
	r.next[cursor] = index + 1
	r.mu.Unlock()
	return entry.variants[int(index%uint64(len(entry.variants)))]
}

// Code returns the language code.
func (l Lang) Code() string { return l.code }

// Text returns the translation for key.
func (l Lang) Text(key TextKey) string { return l.res.Text(l, key) }

// Textf returns the translation for key formatted with args.
func (l Lang) Textf(key TextKey, args ...any) string {
	value := l.Text(key)
	if len(args) == 0 {
		return value
	}
	return fmt.Sprintf(value, args...)
}

// TextPlural returns the translation for the CLDR form key "<key>.<form>". The
// translation carries its own placeholder, so the caller formats the result.
func (l Lang) TextPlural(key TextKey, n int) string {
	return l.Text(TextKey(string(key) + "." + pluralForm(l.code, n)))
}

// TextPluralf returns the plural form and formats it with args. n selects the
// form; pass n in args when the translation contains a count placeholder.
func (l Lang) TextPluralf(key TextKey, n int, args ...any) string {
	return fmt.Sprintf(l.TextPlural(key, n), args...)
}

// pluralForm returns the CLDR plural category for n in code. Unknown languages
// use the English one/other rule.
func pluralForm(code string, n int) string {
	if n < 0 {
		n = -n
	}
	switch code {
	case "ru":
		switch {
		case n%10 == 1 && n%100 != 11:
			return "one"
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
			return "few"
		default:
			return "many"
		}
	default:
		if n == 1 {
			return "one"
		}
		return "other"
	}
}

type lexicon map[string]textEntry

type textSort uint8

const (
	textSortSequence textSort = iota
	textSortRandom
)

type textEntry struct {
	variants []string
	sort     textSort
}

// UnmarshalJSON accepts a plain string or a variant object:
//
//	"title": "Home"
//	"phrase": {"sort": "random", "variants": ["a", "b"]}
func (e *textEntry) UnmarshalJSON(data []byte) error {
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		e.variants = []string{plain}
		e.sort = textSortSequence
		return nil
	}

	var object struct {
		Sort     string   `json:"sort"`
		Variants []string `json:"variants"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("translation must be a string or an object: %w", err)
	}
	if len(object.Variants) == 0 {
		return errors.New("translation object has no variants")
	}
	sort, err := parseTextSort(object.Sort)
	if err != nil {
		return err
	}
	e.variants = object.Variants
	e.sort = sort
	return nil
}

func parseTextSort(value string) (textSort, error) {
	switch value {
	case "", "sequence", "sequential":
		return textSortSequence, nil
	case "random":
		return textSortRandom, nil
	default:
		return textSortSequence, fmt.Errorf("unknown translation sort %q", value)
	}
}

type variantCursor struct {
	lang string
	key  string
}

func (r *Resource) loadLangs(fsys fs.FS) {
	for _, entry := range readDir(fsys, "lang") {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(entry.Name(), ".json")
		data := readFile(fsys, "lang/"+entry.Name())
		var lex lexicon
		if err := json.Unmarshal(data, &lex); err != nil {
			panic("resource: parse lang " + entry.Name() + ": " + err.Error())
		}
		if lex == nil {
			lex = lexicon{}
		}
		r.lexicons[code] = lex
	}
}
