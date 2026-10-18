package app

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/jeffgicharu/oriel/internal/assets"
	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/web"
)

// publicPaths are the pages listed in the sitemap.
func (a *App) publicPaths() []string {
	paths := []string{"/", "/explore", "/residences", "/building", "/location", "/progress", "/buying", "/viewings/new", "/enquire", "/privacy", "/terms"}
	for _, code := range building.TypeOrder {
		paths = append(paths, "/residences/types/"+code)
	}
	for _, u := range building.Units() {
		paths = append(paths, "/residences/"+u.ID)
	}
	return paths
}

func (a *App) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	var b strings.Builder
	b.WriteString("# Every page also carries noindex.\n")
	b.WriteString("User-agent: *\n")
	for _, path := range []string{"/desk", "/viewings/", "/reservations/", "/reserve/", "/events", "/compare", "/shortlist"} {
		b.WriteString("Disallow: " + path + "\n")
	}
	b.WriteString("\nSitemap: " + a.cfg.SiteURL + "/sitemap.xml\n")
	_, _ = w.Write([]byte(b.String()))
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (a *App) sitemap(w http.ResponseWriter, r *http.Request) {
	today := a.now().In(web.Nairobi).Format("2006-01-02")
	set := sitemapSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, path := range a.publicPaths() {
		set.URLs = append(set.URLs, sitemapURL{Loc: a.cfg.SiteURL + path, LastMod: today})
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		a.serverError(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}

func (a *App) webManifest(w http.ResponseWriter, r *http.Request) {
	type icon struct {
		Src     string `json:"src"`
		Sizes   string `json:"sizes"`
		Type    string `json:"type"`
		Purpose string `json:"purpose,omitempty"`
	}
	manifest := map[string]any{
		"name":             "Oriel, Riverside",
		"short_name":       "Oriel",
		"description":      "Eighty-six homes designed around the light in Riverside, Nairobi.",
		"start_url":        "/",
		"scope":            "/",
		"display":          "standalone",
		"background_color": "#ecedea",
		"theme_color":      "#ecedea",
		"lang":             "en-KE",
		"icons": []icon{
			{Src: assets.URL("icons/icon-192.png"), Sizes: "192x192", Type: "image/png"},
			{Src: assets.URL("icons/icon-512.png"), Sizes: "512x512", Type: "image/png"},
			{Src: assets.URL("icons/icon-maskable-512.png"), Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(manifest)
}

// Structured data for search engines.

func organizationLD(site string) map[string]any {
	return map[string]any{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"@id":      site + "/#organization",
		"name":     web.CompanyName,
		"brand":    web.BrandName,
		"url":      site + "/",
		"logo":     site + assets.URL("icons/icon-512.png"),
		"address": map[string]any{
			"@type":           "PostalAddress",
			"addressLocality": "Nairobi",
			"addressRegion":   "Nairobi",
			"addressCountry":  "KE",
		},
	}
}

func websiteLD(site string) map[string]any {
	return map[string]any{
		"@context":   "https://schema.org",
		"@type":      "WebSite",
		"@id":        site + "/#website",
		"name":       "Oriel, Riverside",
		"url":        site + "/",
		"inLanguage": "en-KE",
		"publisher":  map[string]any{"@id": site + "/#organization"},
	}
}

func complexLD(site string, now time.Time) map[string]any {
	return map[string]any{
		"@context":                   "https://schema.org",
		"@type":                      "ApartmentComplex",
		"@id":                        site + "/#oriel",
		"name":                       "Oriel",
		"description":                "A seventeen-storey building of 86 homes, two of them penthouses, in Riverside, Nairobi, completing in " + web.CompletionMonth(now) + ".",
		"url":                        site + "/",
		"numberOfAccommodationUnits": 86,
		"petsAllowed":                false,
		"address": map[string]any{
			"@type":           "PostalAddress",
			"addressLocality": "Riverside, Nairobi",
			"addressCountry":  "KE",
		},
		"amenityFeature": []map[string]any{
			{"@type": "LocationFeatureSpecification", "name": "Rooftop pool", "value": true},
			{"@type": "LocationFeatureSpecification", "name": "Gym", "value": true},
			{"@type": "LocationFeatureSpecification", "name": "Residents' lounge and work room", "value": true},
			{"@type": "LocationFeatureSpecification", "name": "Backup power and water", "value": true},
		},
	}
}
