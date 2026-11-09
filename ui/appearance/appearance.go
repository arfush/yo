package appearance

import "github.com/arfush/yo/ui/resource"

// Options is the constructor input.
type Options struct {
	Resources *resource.Resource
	Language  string // explicit code, e.g. "ru" or "en"; empty panics
}

// New builds the appearance from embedded resources.
func New(opts Options) Appearance {
	if opts.Resources == nil {
		panic("appearance: resources is nil")
	}
	if opts.Language == "" {
		panic("appearance: language is empty")
	}
	return Appearance{
		resources:  opts.Resources,
		palette:    newPalette(),
		typography: newTypography(opts.Resources),
		locale:     newLocale(opts.Resources, opts.Language),
	}
}

// Appearance is one complete look. The zero value is invalid.
type Appearance struct {
	resources  *resource.Resource
	palette    Palette
	typography Typography
	locale     Locale
}

// Palette returns the semantic colors of the application.
func (a Appearance) Palette() Palette { return a.palette }

// Typography returns the text shaper and the role scale.
func (a Appearance) Typography() Typography { return a.typography }

// Locale returns the active language, strings and formatting.
func (a Appearance) Locale() Locale { return a.locale }

// SetPalette replaces the active color set in place. The shell calls it inside
// stdx.Holder.Update when the active site changes. A zero palette is a
// programmer error and panics.
func (a *Appearance) SetPalette(p Palette) {
	if a == nil {
		panic("appearance: nil appearance")
	}
	if p == (Palette{}) {
		panic("appearance: empty palette")
	}
	a.palette = p
}

// SetLanguage replaces the active locale in place. It is for the composition
// root, which calls it inside stdx.Holder.Update. It panics on an empty or
// unknown code.
func (a *Appearance) SetLanguage(code string) {
	if a == nil {
		panic("appearance: nil appearance")
	}
	a.locale = newLocale(a.resources, code)
}
