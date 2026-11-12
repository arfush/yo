package appearance

import "github.com/arfush/yo/ui/resource"

// TextRole selects a point on the type scale.
type TextRole uint8

// Text roles from the display heading down to the caption.
const (
	RoleDisplay TextRole = iota
	RoleTitle
	RoleSubtitle
	RoleBody
	RoleLabel
	RoleCaption
)

// Typography owns the font keys and the role scale.
type Typography struct {
	fonts [6]resource.FontKey
	sizes [6]float32
}

// newTypography builds the role scale. The Qt backend resolves the font keys
// against the embedded faces in resource.
func newTypography() Typography {
	roles := [...]struct {
		key  resource.FontKey
		size float32
	}{
		RoleDisplay:  {resource.FontBold, 26},
		RoleTitle:    {resource.FontSemiBold, 23.5},
		RoleSubtitle: {resource.FontMedium, 15},
		RoleBody:     {resource.FontRegular, 12.5},
		RoleLabel:    {resource.FontBold, 13.2},
		RoleCaption:  {resource.FontMedium, 11},
	}

	var fonts [6]resource.FontKey
	var sizes [6]float32
	for role, spec := range roles {
		fonts[role] = spec.key
		sizes[role] = spec.size
	}
	return Typography{fonts: fonts, sizes: sizes}
}

// Font returns the embedded face key for role. An unknown role is a programmer
// error and panics.
func (t Typography) Font(role TextRole) resource.FontKey {
	return t.fonts[t.index(role)]
}

// TextSize returns the size for role, in logical pixels. An unknown role is a
// programmer error and panics.
func (t Typography) TextSize(role TextRole) float32 {
	return t.sizes[t.index(role)]
}

func (t Typography) index(role TextRole) int {
	if role > RoleCaption {
		panic("appearance: unknown text role")
	}
	return int(role)
}
