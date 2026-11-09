package catalog

// Site identifies one library site.
type Site string

// Library sites in display order.
const (
	SiteYoanime  Site = "yoanime"
	SiteYomanga  Site = "yomanga"
	SiteYoranobe Site = "yoranobe"
)

// Sites returns every library site in display order.
func Sites() []Site {
	return []Site{SiteYoanime, SiteYomanga, SiteYoranobe}
}

// String returns the display name of the site. Brand names are lowercase and
// are not translated, so the result does not depend on the active locale.
func (s Site) String() string {
	return string(s)
}
