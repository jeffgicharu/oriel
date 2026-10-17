package app

import (
	"net/http"

	"github.com/jeffgicharu/oriel/internal/assets"
	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/web"
)

func (a *App) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.Handle("GET /assets/", assets.Handler())
	for _, name := range []string{"favicon.ico", "favicon.svg", "apple-touch-icon.png"} {
		mux.HandleFunc("GET /"+name, a.publicFile(name))
	}
	mux.HandleFunc("GET /robots.txt", a.robots)
	mux.HandleFunc("GET /sitemap.xml", a.sitemap)
	mux.HandleFunc("GET /site.webmanifest", a.webManifest)
	mux.HandleFunc("GET /events", a.events)

	mux.HandleFunc("GET /{$}", a.home)
	mux.HandleFunc("GET /residences", a.residences)
	mux.HandleFunc("GET /residences/{id}", a.unit)
	mux.HandleFunc("GET /schedule/{id}", a.unitSchedule)
	mux.HandleFunc("GET /residences/types/{code}", a.homeType)
	mux.HandleFunc("GET /viewings/new", a.viewingForm)
	mux.HandleFunc("GET /viewings/slots", a.viewingSlots)
	mux.HandleFunc("POST /viewings", a.bookViewing)
	mux.HandleFunc("GET /viewings/{ref}", a.viewingPage)
	mux.HandleFunc("POST /viewings/{ref}/cancel", a.cancelViewing)
	mux.HandleFunc("GET /viewings/{ref}/calendar.ics", a.viewingCalendar)
	mux.HandleFunc("GET /reserve/{id}", a.reserveForm)
	mux.HandleFunc("POST /reserve/{id}", a.reserve)
	mux.HandleFunc("GET /reservations/{ref}", a.reservationPage)
	mux.HandleFunc("GET /enquire", a.enquireForm)
	mux.HandleFunc("POST /enquire", a.enquire)
	mux.HandleFunc("GET /building", a.buildingPage)
	mux.HandleFunc("GET /location", a.locationPage)
	mux.HandleFunc("GET /progress", a.progressPage)
	mux.HandleFunc("GET /buying", a.buyingPage)
	mux.HandleFunc("GET /privacy", a.privacyPage)
	mux.HandleFunc("GET /terms", a.termsPage)
	mux.HandleFunc("GET /explore", a.explore)
	mux.HandleFunc("GET /explore/card/{id}", a.exploreCard)
	mux.HandleFunc("POST /internal/forget", a.forget)

	// The sales desk.
	mux.HandleFunc("GET /desk/login", a.deskLoginForm)
	mux.HandleFunc("POST /desk/login", a.deskLogin)
	mux.HandleFunc("POST /desk/logout", a.deskLogout)
	mux.HandleFunc("GET /desk/enter/{role}/{sig}", a.deskEnter)
	mux.HandleFunc("GET /desk/events", a.desk(a.events))
	mux.HandleFunc("GET /desk", a.desk(a.deskDashboard))
	mux.HandleFunc("GET /desk/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/desk", http.StatusMovedPermanently)
	})

	mux.HandleFunc("/", a.notFound)
}

func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Release", a.release)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (a *App) publicFile(name string) http.HandlerFunc {
	body, contentType, ok := assets.PublicFile(name)
	return func(w http.ResponseWriter, r *http.Request) {
		if !ok {
			a.notFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(body)
	}
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Description: "Eighty-six homes in Riverside, Nairobi, each with a glazed bay that catches the morning light. Explore the building in 3D, find a home and book a viewing. Completing December 2028.",
		Path:        "/",
		Sheet:       web.Sheet{Number: "A-000", Name: "General arrangement"},
		HeaderOver:  true,
		JSONLD:      []any{organizationLD(a.cfg.SiteURL), websiteLD(a.cfg.SiteURL), complexLD(a.cfg.SiteURL)},
	})
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	a.render(w, r, http.StatusOK, web.Home(p, building.ModelData(), web.StoryFor(c, a.now())))
}
