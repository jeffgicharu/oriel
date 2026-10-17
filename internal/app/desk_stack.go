package app

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

// ---------- Stacking plan ----------

func (a *App) deskStack(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	d := a.deskPage(r, "stack", "Stacking plan")
	story := web.StoryFor(c, a.now())
	x := web.DeskStackData{Rows: story.Stack, Homes: map[string]store.Home{}, Counts: c.Counts()}
	for _, h := range c.Homes {
		x.Homes[h.ID] = h
	}
	if id := r.URL.Query().Get("home"); id != "" {
		if h, ok := c.Home(id); ok {
			open := a.homeDrawer(r, h, "", "")
			x.Open = &open
		}
	}
	a.render(w, r, http.StatusOK, web.DeskStack(d, x))
}

func (a *App) homeDrawer(r *http.Request, h store.Home, done, problem string) web.DeskHomeData {
	db := a.db(r)
	x := web.DeskHomeData{Home: h, Done: done, Problem: problem, CanChange: staffFrom(r.Context()).Manager(), Now: a.now()}
	if hist, err := db.UnitHistory(r.Context(), h.ID); err == nil {
		x.History = hist
	}
	if res, err := db.UnitReservation(r.Context(), h.ID); err == nil {
		x.Reservation = res
	}
	return x
}

// deskHome is a home's drawer; opened directly, it is the stacking plan with
// the drawer open.
func (a *App) deskHome(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "" {
		http.Redirect(w, r, "/desk/stack?home="+r.PathValue("id"), http.StatusSeeOther)
		return
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found {
		http.Error(w, "No such home.", http.StatusNotFound)
		return
	}
	a.render(w, r, http.StatusOK, web.DeskHome(a.homeDrawer(r, h, "", "")))
}

// homeChange runs a change to a home from its drawer and shows the drawer again.
func (a *App) homeChange(w http.ResponseWriter, r *http.Request, change func(db *store.DB, staff store.Staff, id string) (string, error)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	staff := staffFrom(r.Context())
	db := a.db(r)
	done, err := change(db, staff, id)
	problem := ""
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "No such home.", http.StatusNotFound)
		return
	}
	if err != nil {
		if errors.Is(err, store.ErrManagersOnly) || !isInternal(err) {
			problem = err.Error()
		} else {
			a.log.Error("changing a home", "home", id, "err", err)
			a.serverError(w, r)
			return
		}
		done = ""
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(id)
	if !found {
		http.Error(w, "No such home.", http.StatusNotFound)
		return
	}
	if problem == "" {
		a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "unit", ID: h.ID, Status: h.Status})
	}
	a.render(w, r, http.StatusOK, web.DeskHome(a.homeDrawer(r, h, done, capitalFirst(problem))))
}

func (a *App) deskHomeStatus(w http.ResponseWriter, r *http.Request) {
	a.homeChange(w, r, func(db *store.DB, staff store.Staff, id string) (string, error) {
		to := r.PostFormValue("status")
		if err := db.SetUnitStatus(r.Context(), id, to, r.PostFormValue("reason"), staff, a.now()); err != nil {
			return "", err
		}
		if to == store.Withheld {
			return "Home " + id + " is withheld from sale.", nil
		}
		return "Home " + id + " is for sale again.", nil
	})
}

func (a *App) deskHomePrice(w http.ResponseWriter, r *http.Request) {
	a.homeChange(w, r, func(db *store.DB, staff store.Staff, id string) (string, error) {
		var price int64
		if r.PostFormValue("clear") != "1" {
			p, err := web.ParseShillings(r.PostFormValue("price"))
			if err != nil || p <= 0 {
				return "", userError("please give the price in shillings, such as 18,480,000")
			}
			price = p
		}
		if err := db.SetPriceOverride(r.Context(), id, price, r.PostFormValue("reason"), staff, a.now()); err != nil {
			return "", err
		}
		if price == 0 {
			return "Home " + id + " is back on the list price.", nil
		}
		return "Home " + id + " now has its own price of " + web.KES(price) + ".", nil
	})
}

// deskHomeCell is one cell of the stacking plan, fetched when a change arrives.
func (a *App) deskHomeCell(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found {
		http.Error(w, "No such home.", http.StatusNotFound)
		return
	}
	a.render(w, r, http.StatusOK, web.DeskStackCell(h))
}
