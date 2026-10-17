// Package web holds the HTML of the public site and the sales desk, written
// as templ components.
package web

import (
	"strings"
	"time"

	"github.com/jeffgicharu/oriel/internal/assets"
)

// Brand facts used across the site.
const (
	BrandName    = "Oriel"
	CompanyName  = "Oriel Residences Limited"
	Place        = "Riverside, Nairobi"
	DefaultTitle = "Oriel · 86 homes in Riverside, Nairobi"
)

// Site is what every page knows about the site itself.
type Site struct {
	URL     string
	Release string
	Now     time.Time
}

// Sheet is the drawing-sheet number and name shown in a page's title block,
// as on an architect's drawing set.
type Sheet struct {
	Number string
	Name   string
}

// Meta describes one page for the browser, search engines and link previews.
type Meta struct {
	// Title is the page's own title; the brand is added to it. Empty on the home page.
	Title       string
	Description string
	// Path is the canonical path of the page, such as /residences.
	Path string
	// Image is the share image's path; the default is the brand's.
	Image    string
	ImageAlt string
	Type     string
	JSONLD   []any
	Sheet    Sheet
	// Nav names the main navigation item the page belongs to.
	Nav string
	// HeaderOver lays the header over the page's opening picture.
	HeaderOver bool
	BodyClass  string
	// Modules are extra script entries the page needs, by manifest name.
	Modules []string
	// Status is the HTTP status the page is sent with; zero means 200.
	Status int
}

// Page is passed to every public page.
type Page struct {
	Site Site
	Meta Meta
}

// FullTitle is the text for the browser tab.
func (p Page) FullTitle() string {
	if p.Meta.Title == "" {
		return DefaultTitle
	}
	return p.Meta.Title + " · Oriel, Riverside"
}

// Canonical is the page's absolute address.
func (p Page) Canonical() string {
	path := p.Meta.Path
	if path == "" {
		path = "/"
	}
	return p.Site.URL + path
}

// ImageURL is the absolute address of the share image.
func (p Page) ImageURL() string {
	image := p.Meta.Image
	if image == "" {
		image = assets.URL("img/og-oriel.png")
	}
	if strings.HasPrefix(image, "http") {
		return image
	}
	return p.Site.URL + image
}

// ImageAltText describes the share image.
func (p Page) ImageAltText() string {
	if p.Meta.ImageAlt != "" {
		return p.Meta.ImageAlt
	}
	return "Oriel: 86 homes designed around the light, in Riverside, Nairobi. Completing December 2028."
}

// OGType is the Open Graph type of the page.
func (p Page) OGType() string {
	if p.Meta.Type != "" {
		return p.Meta.Type
	}
	return "website"
}

// SheetNumber is the page's sheet number, with a default.
func (p Page) SheetNumber() string {
	if p.Meta.Sheet.Number == "" {
		return "A-000"
	}
	return p.Meta.Sheet.Number
}

// SheetName is the page's sheet name, with a default.
func (p Page) SheetName() string {
	if p.Meta.Sheet.Name == "" {
		return "General arrangement"
	}
	return p.Meta.Sheet.Name
}

// Revision is the release's short id, shown in the title block.
func (p Page) Revision() string {
	if len(p.Site.Release) >= 7 {
		return strings.ToUpper(p.Site.Release[:7])
	}
	return strings.ToUpper(p.Site.Release)
}

// Drawn is today's date in Nairobi, written as on a drawing.
func (p Page) Drawn() string {
	return p.Site.Now.In(Nairobi).Format("02.01.2006")
}

// NavItem is one link of the main navigation.
type NavItem struct {
	Key   string
	Label string
	Href  string
	Sheet string
}

// NavItems are the main sections of the public site.
var NavItems = []NavItem{
	{"residences", "Residences", "/residences", "A-200"},
	{"explore", "Explore in 3D", "/explore", "A-100"},
	{"building", "The building", "/building", "A-300"},
	{"location", "Location", "/location", "A-400"},
	{"progress", "Progress", "/progress", "A-500"},
	{"buying", "Buying", "/buying", "A-600"},
}

// Nairobi is East Africa Time. The server's clock is UTC; every time a person
// reads is shown in Nairobi time.
var Nairobi = func() *time.Location {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}()
