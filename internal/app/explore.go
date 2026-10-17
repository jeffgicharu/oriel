package app

import (
	"net/http"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/web"
)

func (a *App) explore(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	d := web.ExploreData{
		Model:  building.ModelData(),
		Homes:  web.ExploreHomes(c),
		Filter: web.ParseFilter(r.URL.Query()),
	}
	if id := r.URL.Query().Get("home"); id != "" {
		if _, found := c.Home(id); found {
			d.Start = id
		}
	}
	p := a.page(web.Meta{
		Title:       "Explore the building in 3D",
		Description: "Turn Oriel in 3D, open up any floor to see which homes are still available, and move the sun through the day to see the light reach each one.",
		Path:        "/explore",
		Sheet:       web.Sheet{Number: "A-100", Name: "Explore"},
		Nav:         "explore",
		BodyClass:   "is-explorer",
	})
	a.render(w, r, http.StatusOK, web.Explore(p, d, web.LevelRows(c)))
}

func (a *App) exploreCard(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	a.render(w, r, http.StatusOK, web.ExploreCard(h))
}
