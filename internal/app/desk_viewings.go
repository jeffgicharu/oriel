package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/viewings"
	"github.com/jeffgicharu/oriel/internal/web"
)

// ---------- Viewings ----------

func (a *App) deskViewings(w http.ResponseWriter, r *http.Request) {
	a.showViewings(w, r, "")
}

func (a *App) showViewings(w http.ResponseWriter, r *http.Request, problem string) {
	staff := staffFrom(r.Context())
	db := a.db(r)
	nairobi := store.Nairobi()
	now := a.now().In(nairobi)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, nairobi)
	if t, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("day"), nairobi); err == nil {
		day = t
	}
	agent := int64(0)
	if !staff.Manager() {
		agent = staff.ID
	}
	cfg := db.ViewingConfig(r.Context())
	monday := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	blocks, _ := db.Blocks(r.Context(), monday.AddDate(0, 0, -7).Format("2006-01-02"))
	blocked := map[string]string{}
	for _, b := range blocks {
		blocked[b.Day] = b.Reason
	}
	week, err := db.ViewingsBetween(r.Context(), monday, monday.AddDate(0, 0, 7), agent)
	if err != nil {
		a.serverError(w, r)
		return
	}
	x := web.DeskViewingsData{Day: day, Config: cfg, Now: a.now(), Problem: problem,
		PrevWeek: monday.AddDate(0, 0, -7).Format("2006-01-02"), NextWeek: monday.AddDate(0, 0, 7).Format("2006-01-02")}
	for i := 0; i < 7; i++ {
		date := monday.AddDate(0, 0, i)
		key := date.Format("2006-01-02")
		ad := web.AgendaDay{Date: date, Current: key == day.Format("2006-01-02")}
		_, isBlocked := blocked[key]
		ad.Closed = !cfg.Opens(date) || isBlocked
		for _, v := range week {
			if v.StartsAt.In(nairobi).Format("2006-01-02") == key && v.Status != "cancelled" {
				ad.Count++
			}
		}
		x.Week = append(x.Week, ad)
	}
	for _, v := range week {
		if v.StartsAt.In(nairobi).Format("2006-01-02") == day.Format("2006-01-02") {
			x.List = append(x.List, v)
		}
	}
	x.Blocked = blocked[day.Format("2006-01-02")]
	for _, b := range blocks {
		if b.Day >= now.Format("2006-01-02") {
			x.Blocks = append(x.Blocks, b)
		}
	}
	status := http.StatusOK
	if problem != "" {
		status = http.StatusUnprocessableEntity
	}
	a.render(w, r, status, web.DeskViewings(a.deskPage(r, "viewings", "Viewings"), x))
}

// viewingsAction runs a change from the viewings page and goes back to it.
func (a *App) viewingsAction(w http.ResponseWriter, r *http.Request, run func(db *store.DB, staff store.Staff) (string, error)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	done, err := run(a.db(r), staffFrom(r.Context()))
	if err != nil {
		if isInternal(err) {
			a.log.Error("changing viewings", "err", err)
			a.serverError(w, r)
			return
		}
		a.showViewings(w, r, capitalFirst(err.Error()))
		return
	}
	back := "/desk/viewings?"
	if day := r.URL.Query().Get("day"); day != "" {
		back += "day=" + day + "&"
	}
	http.Redirect(w, r, back+a.doneQuery(done), http.StatusSeeOther)
}

func (a *App) deskViewingOutcome(w http.ResponseWriter, r *http.Request) {
	a.viewingsAction(w, r, func(db *store.DB, staff store.Staff) (string, error) {
		outcome := r.PostFormValue("outcome")
		if err := db.SetViewingOutcome(r.Context(), r.PathValue("ref"), outcome, r.PostFormValue("notes"), staff, a.now()); err != nil {
			return "", err
		}
		if outcome == "attended" {
			return "Marked as came.", nil
		}
		return "Marked as did not come.", nil
	})
}

func (a *App) deskViewingHours(w http.ResponseWriter, r *http.Request) {
	a.viewingsAction(w, r, func(db *store.DB, staff store.Staff) (string, error) {
		var cfg viewings.Config
		for _, v := range r.PostForm["days"] {
			n, err := strconv.Atoi(v)
			if err == nil && n >= 0 && n <= 6 {
				cfg.Days = append(cfg.Days, time.Weekday(n))
			}
		}
		for _, t := range strings.FieldsFunc(r.PostFormValue("times"), func(c rune) bool { return c == ',' || c == ';' || unicode.IsSpace(c) }) {
			h, m, err := viewings.ParseClock(t)
			if err != nil {
				return "", userError("“" + t + "” is not a time; write times like 10:00, 14:45")
			}
			cfg.Times = append(cfg.Times, twoDigits(h)+":"+twoDigits(m))
		}
		cfg.Capacity, _ = strconv.Atoi(r.PostFormValue("capacity"))
		if err := db.SaveViewingConfig(r.Context(), cfg, staff, a.now()); err != nil {
			return "", err
		}
		return "Opening hours saved. The booking page shows them at once.", nil
	})
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func (a *App) deskBlockDay(w http.ResponseWriter, r *http.Request) {
	a.viewingsAction(w, r, func(db *store.DB, staff store.Staff) (string, error) {
		day := r.PostFormValue("day")
		if err := db.BlockDay(r.Context(), day, r.PostFormValue("reason"), staff, a.now()); err != nil {
			return "", err
		}
		return web.DayLabel(day) + " is closed for viewings.", nil
	})
}

func (a *App) deskUnblockDay(w http.ResponseWriter, r *http.Request) {
	a.viewingsAction(w, r, func(db *store.DB, staff store.Staff) (string, error) {
		day := r.PathValue("day")
		if err := db.UnblockDay(r.Context(), day, staff, a.now()); err != nil {
			return "", err
		}
		return web.DayLabel(day) + " is open for viewings again.", nil
	})
}
