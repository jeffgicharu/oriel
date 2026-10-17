package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/jeffgicharu/oriel/internal/copies"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

const deskCookie = "oriel_desk"

type staffKey struct{}

func staffFrom(ctx context.Context) store.Staff {
	s, _ := ctx.Value(staffKey{}).(store.Staff)
	return s
}

// desk wraps a desk page: only a signed-in member of the team gets through.
func (a *App) desk(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		c, err := r.Cookie(deskCookie)
		var staff store.Staff
		if err == nil {
			staff, err = a.db(r).SessionStaff(r.Context(), c.Value, a.now())
		}
		if err != nil {
			if r.Method == http.MethodGet && r.Header.Get("HX-Request") == "" {
				http.Redirect(w, r, "/desk/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
				return
			}
			w.Header().Set("HX-Redirect", "/desk/login")
			http.Error(w, "Please sign in again.", http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), staffKey{}, staff)))
	}
}

// managers wraps a desk action only the sales manager may take.
func (a *App) managers(next http.HandlerFunc) http.HandlerFunc {
	return a.desk(func(w http.ResponseWriter, r *http.Request) {
		if !staffFrom(r.Context()).Manager() {
			a.deskForbidden(w, r)
			return
		}
		next(w, r)
	})
}

func (a *App) deskForbidden(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Only the sales manager can do that.", http.StatusForbidden)
}

// deskPage is the frame every desk page shares.
func (a *App) deskPage(r *http.Request, nav, title string) web.DeskPage {
	db := a.db(r)
	if err := db.LapseHolds(r.Context(), a.now()); err != nil {
		a.log.Error("lapsing holds", "err", err)
	}
	var waiting int
	db.QueryRowContext(r.Context(), `SELECT count(*) FROM reservations WHERE status = 'requested'`).Scan(&waiting)
	p := a.page(web.Meta{Title: title, Path: r.URL.Path})
	notice := ""
	if q := r.URL.Query(); q.Get("done") != "" && hmac.Equal([]byte(q.Get("ds")), []byte(a.sign("done", q.Get("done")))) {
		notice = q.Get("done")
	}
	return web.DeskPage{Page: p, Staff: staffFrom(r.Context()), Nav: nav, Waiting: waiting, Notice: notice}
}

// doneQuery is the query that carries a notice to the next page, signed so
// that only the desk itself can put words there.
func (a *App) doneQuery(msg string) string {
	return "done=" + url.QueryEscape(msg) + "&ds=" + url.QueryEscape(a.sign("done", msg))
}

func (a *App) deskCookieFor(token string) *http.Cookie {
	return &http.Cookie{
		Name:     deskCookie,
		Value:    token,
		Path:     "/desk",
		MaxAge:   int(store.SessionLength.Seconds()),
		HttpOnly: true,
		Secure:   strings.HasPrefix(a.cfg.SiteURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

// safeNext keeps a sign-in's onward address on the desk.
func safeNext(next string) string {
	if strings.HasPrefix(next, "/desk") && !strings.HasPrefix(next, "//") && !strings.ContainsAny(next, "\\\r\n") {
		return next
	}
	return "/desk"
}

func (a *App) deskLoginForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	a.render(w, r, http.StatusOK, web.DeskLogin("Sign in · Oriel sales desk", "", "", safeNext(r.URL.Query().Get("next"))))
}

func (a *App) deskLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username, next := r.PostFormValue("username"), safeNext(r.PostFormValue("next"))
	show := func(status int, problem string) {
		w.Header().Set("Cache-Control", "no-store")
		a.render(w, r, status, web.DeskLogin("Sign in · Oriel sales desk", username, problem, next))
	}
	if !a.formAllowed(r) {
		show(http.StatusTooManyRequests, "Too many attempts. Please wait a minute and try again.")
		return
	}
	staff, err := a.db(r).CheckSignIn(r.Context(), username, r.PostFormValue("password"))
	if errors.Is(err, store.ErrBadSignIn) {
		show(http.StatusUnauthorized, "That username and password don’t match.")
		return
	}
	if err != nil {
		a.serverError(w, r)
		return
	}
	db, err := a.writable(w, r)
	if errors.Is(err, errBusy) {
		a.busy(w, r)
		return
	}
	if err != nil {
		a.serverError(w, r)
		return
	}
	token, err := db.StartSession(r.Context(), staff.ID, a.now())
	if err != nil {
		a.serverError(w, r)
		return
	}
	http.SetCookie(w, a.deskCookieFor(token))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (a *App) deskLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(deskCookie); err == nil && !a.db(r).ReadOnly() {
		a.db(r).EndSession(r.Context(), c.Value)
	}
	c := a.deskCookieFor("")
	c.MaxAge = -1
	http.SetCookie(w, c)
	http.Redirect(w, r, "/desk/login", http.StatusSeeOther)
}

// EnterSignature is the signature in a one-click sign-in link for a role.
func EnterSignature(deskKey []byte, role string) string {
	mac := hmac.New(sha256.New, deskKey)
	mac.Write([]byte("enter:" + role))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// deskEnter is a one-click sign-in: a fresh copy of the business, signed in
// as the manager or an agent.
func (a *App) deskEnter(w http.ResponseWriter, r *http.Request) {
	role, sig := r.PathValue("role"), r.PathValue("sig")
	if (role != "manager" && role != "agent") || !hmac.Equal([]byte(sig), []byte(EnterSignature(a.cfg.DeskKey, role))) {
		a.notFound(w, r)
		return
	}
	check := a.isReleaseCheck(r)
	if !check && !a.copyLimiter.allow(clientIP(r), a.now()) {
		a.busy(w, r)
		return
	}
	prefix := ""
	if check {
		prefix = "chk"
	}
	h, err := a.copies.Create(prefix)
	if errors.Is(err, copies.ErrFull) {
		a.busy(w, r)
		return
	}
	if err != nil {
		a.log.Error("making a copy for the desk", "err", err)
		a.serverError(w, r)
		return
	}
	defer h.Release()
	staff, err := h.DB().FirstWithRole(r.Context(), role)
	if err != nil {
		a.serverError(w, r)
		return
	}
	token, err := h.DB().StartSession(r.Context(), staff.ID, a.now())
	if err != nil {
		a.serverError(w, r)
		return
	}
	http.SetCookie(w, a.copyCookieFor(h.ID()))
	http.SetCookie(w, a.deskCookieFor(token))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/desk", http.StatusSeeOther)
}

// ---------- Dashboard ----------

func (a *App) deskDashboard(w http.ResponseWriter, r *http.Request) {
	staff := staffFrom(r.Context())
	db := a.db(r)
	d := a.deskPage(r, "dashboard", "Dashboard")
	agent := int64(0)
	if !staff.Manager() {
		agent = staff.ID
	}
	dash, err := db.DashboardOn(r.Context(), a.now(), agent)
	if err != nil {
		a.log.Error("dashboard", "err", err)
		a.serverError(w, r)
		return
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	x := web.DeskDashboardData{Dash: dash, Counts: c.Counts(), Total: len(c.Homes), Funnel: web.Funnel(dash.Funnel)}
	for _, h := range c.Homes {
		if h.Status == store.Available || h.Status == store.Held || h.Status == store.Withheld {
			x.Stock += h.Price
		}
	}
	waiting, err := db.Reservations(r.Context(), "requested", 6)
	if err == nil {
		x.Waiting = waiting
	}
	leads, err := db.Leads(r.Context(), store.LeadFilter{AgentID: agent, Sort: "follow", Limit: 40})
	if err == nil {
		for _, l := range leads {
			if l.Overdue(a.now()) && len(x.FollowUps) < 6 {
				x.FollowUps = append(x.FollowUps, l)
			}
		}
	}
	a.render(w, r, http.StatusOK, web.DeskDashboard(d, x))
}
