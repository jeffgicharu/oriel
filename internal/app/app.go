// Package app is Oriel's web server: the public site, the sales desk and the
// small endpoints around them.
package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/config"
	"github.com/jeffgicharu/oriel/internal/copies"
	"github.com/jeffgicharu/oriel/internal/sample"
	"github.com/jeffgicharu/oriel/internal/web"
)

// App holds what every request needs.
type App struct {
	cfg     config.Config
	release string
	log     *slog.Logger
	now     func() time.Time

	copies      *copies.Store
	broker      *broker
	copyLimiter *limiter
	formLimiter *limiter

	// streams ends every live-update stream and the background work when the
	// server shuts down, so browsers reconnect to the release taking over.
	streams context.Context
	stop    context.CancelFunc
}

// New prepares the server: it opens the data folder, building the shared
// sample if there is none, and starts the hourly rebuild and the sweep of
// expired copies.
func New(cfg config.Config, release string, log *slog.Logger) (*App, error) {
	streams, stop := context.WithCancel(context.Background())
	opt := copies.DefaultOptions()
	opt.SampleVersion = sample.Version
	store, err := copies.Open(streams, cfg.DataDir, opt, sample.Build, log)
	if err != nil {
		stop()
		return nil, err
	}
	a := &App{
		cfg:         cfg,
		release:     release,
		log:         log,
		now:         time.Now,
		copies:      store,
		broker:      newBroker(),
		copyLimiter: newLimiter(20, 6),
		formLimiter: newLimiter(60, 12),
		streams:     streams,
		stop:        stop,
	}
	go store.Run(streams)
	return a, nil
}

// Close ends live-update streams and background work. Call it when the
// server starts shutting down.
func (a *App) Close() {
	a.stop()
}

// Shutdown closes the databases once requests have finished.
func (a *App) Shutdown() {
	a.copies.Close()
}

// Handler is every route of the site, wrapped in the middleware all of them share.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	a.routes(mux)
	// Forms may only be posted from Oriel's own pages.
	cross := http.NewCrossOriginProtection()
	return a.recoverer(a.headers(cross.Handler(a.withVisitor(mux))))
}

func (a *App) site() web.Site {
	return web.Site{URL: a.cfg.SiteURL, Release: a.release, Now: a.now()}
}

func (a *App) page(meta web.Meta) web.Page {
	return web.Page{Site: a.site(), Meta: meta}
}

// render writes a page in full or not at all: it is rendered into memory
// first, so a failure part-way through never sends half a page.
func (a *App) render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		a.log.Error("render failed", "path", r.URL.Path, "err", err)
		a.serverError(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (a *App) notFound(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Title:       "Page not found",
		Description: "This page could not be found at Oriel, Riverside.",
		Path:        r.URL.Path,
		Sheet:       web.Sheet{Number: "A-404", Name: "Not found"},
	})
	a.render(w, r, http.StatusNotFound, web.NotFound(p))
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{
		Title:       "Something went wrong",
		Description: "Something went wrong at Oriel, Riverside.",
		Path:        r.URL.Path,
		Sheet:       web.Sheet{Number: "A-500", Name: "Error"},
	})
	var buf bytes.Buffer
	if err := web.ServerError(p).Render(context.Background(), &buf); err != nil {
		http.Error(w, "Something went wrong on our side. Please try again in a moment.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(buf.Bytes())
}
