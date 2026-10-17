package app

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

// ---------- Reservations ----------

func (a *App) deskReservations(w http.ResponseWriter, r *http.Request) {
	a.showReservations(w, r, "", "")
}

func (a *App) showReservations(w http.ResponseWriter, r *http.Request, focus, problem string) {
	staff := staffFrom(r.Context())
	db := a.db(r)
	tab := r.URL.Query().Get("tab")
	if _, ok := store.ReservationGroups[tab]; !ok {
		tab = "waiting"
	}
	agent := int64(0)
	if !staff.Manager() {
		agent = staff.ID
	}
	d := a.deskPage(r, "reservations", "Reservations")
	list, err := db.ReservationsIn(r.Context(), tab, agent)
	if err != nil {
		a.log.Error("reservations", "err", err)
		a.serverError(w, r)
		return
	}
	counts, _ := db.ReservationGroupCounts(r.Context(), agent)
	status := http.StatusOK
	if problem != "" {
		status = http.StatusUnprocessableEntity
	}
	a.render(w, r, status, web.DeskReservations(d, web.DeskReservationsData{Tab: tab, Counts: counts, List: list, Now: a.now(), Problem: problem, Focus: focus}))
}

// reservationAction runs a manager's decision on a reservation, then shows
// the tab the reservation has moved to.
func (a *App) reservationAction(w http.ResponseWriter, r *http.Request, run func(db *store.DB, ref string, staff store.Staff) (tab, done string, err error)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ref := r.PathValue("ref")
	db := a.db(r)
	staff := staffFrom(r.Context())
	before, err := db.ReservationByRef(r.Context(), ref)
	if errors.Is(err, sql.ErrNoRows) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.serverError(w, r)
		return
	}
	tab, done, err := run(db, ref, staff)
	if err != nil {
		if isInternal(err) {
			a.log.Error("deciding a reservation", "ref", ref, "err", err)
			a.serverError(w, r)
			return
		}
		r.URL.RawQuery = "tab=" + map[string]string{"requested": "waiting", "confirmed": "confirmed"}[before.Status]
		a.showReservations(w, r, ref, capitalFirst(err.Error()))
		return
	}
	after, _ := db.ReservationByRef(r.Context(), ref)
	c, err := db.LoadCatalog(r.Context())
	if err == nil {
		if h, ok := c.Home(after.UnitID); ok {
			a.broker.publish(visitorFrom(r.Context()).copyID,
				event{Type: "unit", ID: h.ID, Status: h.Status},
				event{Type: "reservation", ID: ref, Status: after.Status},
			)
		}
	}
	http.Redirect(w, r, "/desk/reservations?tab="+tab+"&"+a.doneQuery(done)+"#"+ref, http.StatusSeeOther)
}

func (a *App) deskConfirm(w http.ResponseWriter, r *http.Request) {
	a.reservationAction(w, r, func(db *store.DB, ref string, staff store.Staff) (string, string, error) {
		if err := db.ConfirmReservation(r.Context(), ref, staff.ID, a.now()); err != nil {
			return "", "", err
		}
		res, _ := db.ReservationByRef(r.Context(), ref)
		return "confirmed", "Reservation " + ref + " confirmed. Letter of offer " + res.OfferNumber + " issued, and home " + res.UnitID + " is reserved.", nil
	})
}

func (a *App) deskDecline(w http.ResponseWriter, r *http.Request) {
	a.reservationAction(w, r, func(db *store.DB, ref string, staff store.Staff) (string, string, error) {
		reason := r.PostFormValue("reason")
		if len([]rune(reason)) < 3 {
			return "", "", userError("please give the buyer a reason")
		}
		if err := db.DeclineReservation(r.Context(), ref, reason, staff.ID, a.now()); err != nil {
			return "", "", err
		}
		res, _ := db.ReservationByRef(r.Context(), ref)
		return "closed", "Reservation " + ref + " declined. Home " + res.UnitID + " is for sale again.", nil
	})
}

func (a *App) deskDeposit(w http.ResponseWriter, r *http.Request) {
	a.reservationAction(w, r, func(db *store.DB, ref string, staff store.Staff) (string, string, error) {
		amount, err := web.ParseShillings(r.PostFormValue("amount"))
		if err != nil || amount <= 0 {
			return "", "", userError("please give the deposit in shillings, such as 1,848,000")
		}
		res, _ := db.ReservationByRef(r.Context(), ref)
		if amount > res.Price {
			return "", "", userError("the deposit is more than the price of the home")
		}
		if err := db.RecordDeposit(r.Context(), ref, amount, staff.ID, a.now()); err != nil {
			return "", "", err
		}
		return "completed", "Deposit of " + web.KES(amount) + " recorded. Home " + res.UnitID + " is sold.", nil
	})
}
