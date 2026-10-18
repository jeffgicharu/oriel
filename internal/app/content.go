package app

import (
	"net/http"

	"github.com/jeffgicharu/oriel/internal/web"
)

// The building, location, progress, buying, privacy and terms pages.

func (a *App) buildingPage(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	p := a.page(web.Meta{
		Title:       "The building",
		Description: "How Oriel works: a glazed bay at the corner of every home, shading shaped by the equator’s sun, a rooftop pool, gym and garden by the river, and the full specification.",
		Path:        "/building",
		Sheet:       web.Sheet{Number: "A-300", Name: "The building"},
		Nav:         "building",
		JSONLD:      []any{complexLD(a.cfg.SiteURL)},
	})
	a.render(w, r, http.StatusOK, web.BuildingPage(p, c.Types()))
}

func (a *App) locationPage(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Title:       "Location",
		Description: "Oriel is in Riverside, Nairobi: five minutes from Westlands, twelve from the city centre, fifteen from Karura Forest and thirty-five from the airport. How to find the sales suite.",
		Path:        "/location",
		Sheet:       web.Sheet{Number: "A-400", Name: "Location"},
		Nav:         "location",
	})
	a.render(w, r, http.StatusOK, web.LocationPage(p, !a.now().Before(web.ShowFlatsOpen)))
}

func (a *App) progressPage(w http.ResponseWriter, r *http.Request) {
	g := web.ProgressOn(a.now())
	p := a.page(web.Meta{
		Title:       "Construction progress",
		Description: "Oriel is going up in Riverside, Nairobi. Follow the programme from the first pile to completion in December 2028, with photographs from the site.",
		Path:        "/progress",
		Sheet:       web.Sheet{Number: "A-500", Name: "Progress"},
		Nav:         "progress",
	})
	a.render(w, r, http.StatusOK, web.ProgressPage(p, g))
}

func (a *App) buyingPage(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Title:       "Buying off-plan",
		Description: "How to buy a home at Oriel: reserving, the 10% deposit, three payment plans, the costs beyond the price, and answers to the questions buyers ask most.",
		Path:        "/buying",
		Sheet:       web.Sheet{Number: "A-600", Name: "Buying"},
		Nav:         "buying",
		JSONLD:      []any{faqLD()},
	})
	a.render(w, r, http.StatusOK, web.BuyingPage(p, web.PlanExamples(a.now())))
}

func (a *App) privacyPage(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Title:       "Privacy",
		Description: "What the Oriel website keeps about you, where and for how long: a private workspace for what you type, deleted after a day unused, and no tracking cookies.",
		Path:        "/privacy",
		Sheet:       web.Sheet{Number: "A-900", Name: "Privacy"},
	})
	v := visitorFrom(r.Context())
	x := web.PrivacyData{HasCopy: v != nil && v.copyID != ""}
	x.Deleted = !x.HasCopy && r.URL.Query().Get("deleted") == "1"
	a.render(w, r, http.StatusOK, web.PrivacyPage(p, x))
}

func (a *App) termsPage(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Title:       "Terms of use",
		Description: "The terms for using the Oriel website: plans, prices and pictures are a guide, and a home is bought under its own letter of offer and agreement for sale.",
		Path:        "/terms",
		Sheet:       web.Sheet{Number: "A-901", Name: "Terms"},
	})
	a.render(w, r, http.StatusOK, web.TermsPage(p))
}

func faqLD() map[string]any {
	var entities []map[string]any
	for _, q := range web.Questions {
		entities = append(entities, map[string]any{
			"@type":          "Question",
			"name":           q.Q,
			"acceptedAnswer": map[string]any{"@type": "Answer", "text": q.A},
		})
	}
	return map[string]any{
		"@context":   "https://schema.org",
		"@type":      "FAQPage",
		"mainEntity": entities,
	}
}
