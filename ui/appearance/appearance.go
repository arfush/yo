package appearance

import "github.com/arfush/yo/ui/resource"

// Options is the constructor input.
type Options struct {
	Resource *resource.Resource
	Language string // explicit code, e.g. "ru" or "en"; empty panics
}

// New builds the appearance from embedded resources.
func New(opts Options) Appearance {
	if opts.Resource == nil {
		panic("appearance: resources is nil")
	}
	if opts.Language == "" {
		panic("appearance: language is empty")
	}
	return Appearance{
		resource:   opts.Resource,
		palette:    newPalette(),
		appIcon:    resource.AppIconYobase,
		typography: newTypography(),
		locale:     newLocale(opts.Resource, opts.Language),
	}
}

// Appearance is one complete look. The zero value is invalid. The shell
// replaces parts of it in place, so every method uses a pointer receiver.
type Appearance struct {
	resource   *resource.Resource
	palette    Palette
	appIcon    resource.AppIconKey
	typography Typography
	locale     Locale
}

// Resource returns the embedded assets the appearance was built from.
func (a *Appearance) Resource() *resource.Resource { return a.resource }

// Palette returns the semantic colors of the application.
func (a *Appearance) Palette() Palette { return a.palette }

// AppIcon returns the application icon that matches the active palette.
func (a *Appearance) AppIcon() resource.AppIconKey { return a.appIcon }

// Typography returns the font keys and the role scale.
func (a *Appearance) Typography() Typography { return a.typography }

// Locale returns the active language, strings and formatting.
func (a *Appearance) Locale() Locale { return a.locale }

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

// SetAppIcon replaces the application icon in place. The shell sets it together
// with the palette when the active site changes. An empty key is a programmer
// error and panics.
func (a *Appearance) SetAppIcon(key resource.AppIconKey) {
	if a == nil {
		panic("appearance: nil appearance")
	}
	if key == "" {
		panic("appearance: empty app icon")
	}
	a.appIcon = key
}

// SetLanguage replaces the active locale in place. It is for the composition
// root, which calls it inside stdx.Holder.Update. It panics on an empty or
// unknown code.
func (a *Appearance) SetLanguage(code string) {
	if a == nil {
		panic("appearance: nil appearance")
	}
	a.locale = newLocale(a.resource, code)
}
