package appearance

import (
	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/arfush/yo/ui/resource"
)

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

// Typography owns the shared text shaper and the role scale. The shaper must
// stay on the UI goroutine.
type Typography struct {
	shaper *text.Shaper
	fonts  [6]font.Font
	sizes  [6]unit.Sp
}

// newTypography loads the four Inter faces and builds the shaper. A missing
// embedded font panics through resource.Font.
func newTypography(res *resource.Resource) Typography {
	roles := [...]struct {
		key  resource.FontKey
		size unit.Sp
	}{
		RoleDisplay:  {resource.FontBold, 26},
		RoleTitle:    {resource.FontSemiBold, 23.5},
		RoleSubtitle: {resource.FontMedium, 15},
		RoleBody:     {resource.FontRegular, 12.5},
		RoleLabel:    {resource.FontMedium, 13},
		RoleCaption:  {resource.FontRegular, 11},
	}

	faces := make([]font.FontFace, 0, len(roles))
	var fonts [6]font.Font
	var sizes [6]unit.Sp
	for role, spec := range roles {
		face := res.Font(spec.key)
		faces = append(faces, face)
		fonts[role] = face.Font
		sizes[role] = spec.size
	}
	return Typography{
		shaper: text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces)),
		fonts:  fonts,
		sizes:  sizes,
	}
}

// Shaper returns the shared shaper. It must stay on the UI goroutine.
func (t Typography) Shaper() *text.Shaper { return t.shaper }

// Font returns the face for role. An unknown role is a programmer error and
// panics.
func (t Typography) Font(role TextRole) font.Font {
	return t.fonts[t.index(role)]
}

// TextSize returns the size for role. An unknown role is a programmer error
// and panics.
func (t Typography) TextSize(role TextRole) unit.Sp {
	return t.sizes[t.index(role)]
}

func (t Typography) index(role TextRole) int {
	if role > RoleCaption {
		panic("appearance: unknown text role")
	}
	return int(role)
}
