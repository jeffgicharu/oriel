package app

import (
	"net/http"

	"github.com/jeffgicharu/oriel/internal/assets"
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
	a.render(w, r, http.StatusOK, web.Home(p))
}
