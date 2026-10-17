package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/viewings"
	"github.com/jeffgicharu/oriel/internal/web"
)

// ---------- Viewings ----------

func (a *App) viewingData(r *http.Request, f web.Form, dayValue string) (web.ViewingData, error) {
	db := a.db(r)
	days, err := db.Calendar(r.Context(), a.now())
	if err != nil {
		return web.ViewingData{}, err
	}
	d := web.ViewingData{Form: f, Days: days}
	if id := firstNonEmpty(r.URL.Query().Get("home"), f.Value("home")); id != "" {
		if c, err := db.LoadCatalog(r.Context()); err == nil {
			if h, ok := c.Home(id); ok {
				d.Home = &h
			}
		}
	}
	// The day asked for, or else the first day with a free place.
	for _, day := range days {
		v := web.DateValue(day.Date)
		if v == dayValue || (dayValue == "" && day.Open && day.Free() > 0) {
			d.Day, d.DayValue = day, v
			break
		}
	}
	return d, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (a *App) viewingMeta() web.Meta {
	return web.Meta{
		Title:       "Book a viewing",
		Description: "Book a 45-minute visit to Oriel's sales suite in Riverside, Nairobi: the show apartments, the building model and the team. Open Tuesday to Sunday.",
		Path:        "/viewings/new",
		Sheet:       web.Sheet{Number: "A-700", Name: "Viewings"},
	}
}

func (a *App) viewingForm(w http.ResponseWriter, r *http.Request) {
	d, err := a.viewingData(r, web.Form{Values: map[string]string{}}, r.URL.Query().Get("date"))
	if err != nil {
		a.log.Error("viewing calendar", "err", err)
		a.serverError(w, r)
		return
	}
	a.render(w, r, http.StatusOK, web.Viewing(a.page(a.viewingMeta()), d))
}

func (a *App) viewingSlots(w http.ResponseWriter, r *http.Request) {
	days, err := a.db(r).Calendar(r.Context(), a.now())
	if err != nil {
		a.serverError(w, r)
		return
	}
	want := r.URL.Query().Get("date")
	day := store.Day{}
	for _, d := range days {
		if web.DateValue(d.Date) == want {
			day = d
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	a.render(w, r, http.StatusOK, web.Slots(day, "", ""))
}

func (a *App) bookViewing(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if isBot(r) {
		http.Redirect(w, r, "/viewings/new", http.StatusSeeOther)
		return
	}
	errs := formErrors{}
	c := readContact(r, errs)
	values := map[string]string{
		"name": r.PostFormValue("name"), "email": r.PostFormValue("email"), "phone": r.PostFormValue("phone"),
		"party": r.PostFormValue("party"), "notes": r.PostFormValue("notes"), "slot": r.PostFormValue("slot"),
		"home": r.PostFormValue("home"),
	}
	start, err := time.ParseInLocation("2006-01-02T15:04", r.PostFormValue("slot"), viewings.Nairobi)
	if err != nil {
		errs["slot"] = "Please choose a time."
	}
	notes := strings.TrimSpace(r.PostFormValue("notes"))
	if len(notes) > 600 {
		errs["notes"] = "Please keep this under 600 characters."
	}
	dayValue := r.PostFormValue("date")
	if err == nil {
		dayValue = start.Format("2006-01-02")
	}
	show := func(status int, notice string) {
		d, derr := a.viewingData(r, web.Form{Values: values, Errors: errs}, dayValue)
		if derr != nil {
			a.serverError(w, r)
			return
		}
		d.Notice = notice
		a.render(w, r, status, web.Viewing(a.page(a.viewingMeta()), d))
	}
	if len(errs) > 0 {
		show(http.StatusUnprocessableEntity, "")
		return
	}
	if !a.formAllowed(r) {
		show(http.StatusTooManyRequests, "You have sent several forms in a short time. Please wait a minute and try again.")
		return
	}
	db, err := a.writable(w, r)
	if errors.Is(err, errBusy) {
		a.busy(w, r)
		return
	}
	if err != nil {
		a.log.Error("making a copy", "err", err)
		a.serverError(w, r)
		return
	}
	homeID := r.PostFormValue("home")
	if homeID != "" {
		if c, err := db.LoadCatalog(r.Context()); err != nil {
			homeID = ""
		} else if _, ok := c.Home(homeID); !ok {
			homeID = ""
		}
	}
	c.Interest = "Viewing"
	if homeID != "" {
		c.Interest = "Home " + homeID
	}
	v, err := db.BookViewing(r.Context(), store.ViewingRequest{
		Contact: c, Start: start, PartySize: intField(r, "party", 1, 6, 2), UnitID: homeID, Notes: notes,
	}, a.now())
	switch {
	case errors.Is(err, store.ErrSlotGone):
		errs["slot"] = "That time has just been taken. Please choose another."
		values["slot"] = ""
		show(http.StatusConflict, "")
		return
	case errors.Is(err, store.ErrSlotClosed):
		errs["slot"] = "The sales suite is not open at that time. Please choose another."
		values["slot"] = ""
		show(http.StatusUnprocessableEntity, "")
		return
	case err != nil:
		a.log.Error("booking a viewing", "err", err)
		a.serverError(w, r)
		return
	}
	a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "lead", Text: c.Name + " booked a viewing"})
	http.Redirect(w, r, "/viewings/"+v.Ref+"?booked=1", http.StatusSeeOther)
}

func (a *App) viewingPage(w http.ResponseWriter, r *http.Request) {
	db := a.db(r)
	v, err := db.ViewingByRef(r.Context(), r.PathValue("ref"))
	if errors.Is(err, sql.ErrNoRows) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.serverError(w, r)
		return
	}
	d := web.ViewingDone{Viewing: v, Fresh: r.URL.Query().Get("booked") == "1"}
	if v.UnitID != "" {
		if c, err := db.LoadCatalog(r.Context()); err == nil {
			if h, ok := c.Home(v.UnitID); ok {
				d.Home = &h
			}
		}
	}
	p := a.page(web.Meta{
		Title:       "Viewing " + v.Ref,
		Description: "Your viewing at Oriel's sales suite in Riverside, Nairobi.",
		Path:        "/viewings/" + v.Ref,
		Sheet:       web.Sheet{Number: "A-701", Name: "Viewing " + v.Ref},
	})
	w.Header().Set("Cache-Control", "no-store")
	a.render(w, r, http.StatusOK, web.ViewingBooked(p, d))
}

func (a *App) cancelViewing(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("ref")
	db := a.db(r)
	if db.ReadOnly() {
		http.Redirect(w, r, "/viewings/"+ref, http.StatusSeeOther)
		return
	}
	if err := db.CancelViewing(r.Context(), ref, a.now(), 0); err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.log.Error("cancelling a viewing", "err", err)
		a.serverError(w, r)
		return
	}
	a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "lead", Text: "A viewing was cancelled"})
	http.Redirect(w, r, "/viewings/"+ref, http.StatusSeeOther)
}

// viewingCalendar is the booking as a calendar file.
func (a *App) viewingCalendar(w http.ResponseWriter, r *http.Request) {
	v, err := a.db(r).ViewingByRef(r.Context(), r.PathValue("ref"))
	if err != nil {
		a.notFound(w, r)
		return
	}
	stamp := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }
	desc := "Your viewing at Oriel with the sales team. Reference " + v.Ref + "."
	if v.UnitID != "" {
		desc += " Home " + v.UnitID + "."
	}
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Oriel Residences//Viewings//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"BEGIN:VEVENT",
		"UID:" + v.Ref + "@" + strings.TrimPrefix(strings.TrimPrefix(a.cfg.SiteURL, "https://"), "http://"),
		"DTSTAMP:" + stamp(a.now()),
		"DTSTART:" + stamp(v.StartsAt),
		"DTEND:" + stamp(v.StartsAt.Add(viewings.Length)),
		"SUMMARY:Viewing at Oriel",
		"LOCATION:Oriel sales suite\\, Riverside\\, Nairobi",
		"DESCRIPTION:" + desc,
		"URL:" + a.cfg.SiteURL + "/viewings/" + v.Ref,
		"STATUS:CONFIRMED",
		"BEGIN:VALARM",
		"TRIGGER:-PT2H",
		"ACTION:DISPLAY",
		"DESCRIPTION:Viewing at Oriel in two hours",
		"END:VALARM",
		"END:VEVENT",
		"END:VCALENDAR",
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="oriel-viewing-%s.ics"`, strings.ToLower(v.Ref)))
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, strings.Join(lines, "\r\n")+"\r\n")
}

// ---------- Reservations ----------

func (a *App) reserveData(r *http.Request, c *store.Catalog, h store.Home, f web.Form, plan string) web.ReserveData {
	d := web.ReserveData{Form: f, Home: h, Plan: plan}
	for _, pl := range plans.Plans {
		if s, err := plans.Build(pl, h.Price, plans.Options{From: a.now()}); err == nil {
			d.Previews = append(d.Previews, s)
		}
	}
	if !h.OnSale() {
		for _, other := range c.Homes {
			if other.Type.Code == h.Type.Code && other.OnSale() && len(d.Similar) < 3 {
				d.Similar = append(d.Similar, other)
			}
		}
	}
	return d
}

func (a *App) reserveMeta(h store.Home) web.Meta {
	return web.Meta{
		Title:       "Reserve home " + h.ID,
		Description: "Request a reservation on home " + h.ID + " at Oriel, Riverside. The home is held for 72 hours while the sales team confirms it with a letter of offer.",
		Path:        "/reserve/" + h.ID,
		Sheet:       web.Sheet{Number: "A-710", Name: "Reserve " + h.ID},
	}
}

func (a *App) reserveForm(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found {
		a.notFound(w, r)
		return
	}
	plan := r.URL.Query().Get("plan")
	if _, ok := plans.ByCode(plan); !ok {
		plan = plans.Plans[0].Code
	}
	d := a.reserveData(r, c, h, web.Form{Values: map[string]string{}}, plan)
	a.render(w, r, http.StatusOK, web.Reserve(a.page(a.reserveMeta(h)), d))
}

func (a *App) reserve(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found {
		a.notFound(w, r)
		return
	}
	if isBot(r) {
		http.Redirect(w, r, "/reserve/"+h.ID, http.StatusSeeOther)
		return
	}
	errs := formErrors{}
	contact := readContact(r, errs)
	plan := r.PostFormValue("plan")
	if _, ok := plans.ByCode(plan); !ok {
		errs["plan"] = "Please choose how you will pay."
	}
	if r.PostFormValue("agree") != "yes" {
		errs["agree"] = "Please confirm you understand how a reservation works."
	}
	note := strings.TrimSpace(r.PostFormValue("note"))
	if len(note) > 600 {
		errs["note"] = "Please keep this under 600 characters."
	}
	values := map[string]string{
		"name": r.PostFormValue("name"), "email": r.PostFormValue("email"), "phone": r.PostFormValue("phone"),
		"buying": r.PostFormValue("buying"), "note": note, "agree": r.PostFormValue("agree"),
	}
	show := func(status int) {
		d := a.reserveData(r, c, h, web.Form{Values: values, Errors: errs}, firstNonEmpty(plan, plans.Plans[0].Code))
		a.render(w, r, status, web.Reserve(a.page(a.reserveMeta(h)), d))
	}
	if len(errs) > 0 {
		show(http.StatusUnprocessableEntity)
		return
	}
	if !a.formAllowed(r) {
		errs["agree"] = "You have sent several forms in a short time. Please wait a minute and try again."
		show(http.StatusTooManyRequests)
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
	contact.Buying = "home"
	if r.PostFormValue("buying") == "investment" {
		contact.Buying = "investment"
	}
	contact.Interest = h.Type.Short + ", home " + h.ID
	res, err := db.RequestReservation(r.Context(), store.ReservationRequest{Contact: contact, UnitID: h.ID, Plan: plan, Note: note}, h.Price, a.now())
	if errors.Is(err, store.ErrNotAvailable) {
		if fresh, err := db.LoadCatalog(r.Context()); err == nil {
			if again, ok := fresh.Home(h.ID); ok {
				h, c = again, fresh
			}
		}
		show(http.StatusConflict)
		return
	}
	if err != nil {
		a.log.Error("requesting a reservation", "err", err)
		a.serverError(w, r)
		return
	}
	copyID := visitorFrom(r.Context()).copyID
	a.broker.publish(copyID,
		event{Type: "unit", ID: h.ID, Status: store.Held},
		event{Type: "reservation", ID: res.Ref, Status: res.Status, Text: contact.Name + " requested " + h.ID},
	)
	http.Redirect(w, r, "/reservations/"+res.Ref+"?new=1", http.StatusSeeOther)
}

func (a *App) reservationPage(w http.ResponseWriter, r *http.Request) {
	db := a.db(r)
	if err := db.LapseHolds(r.Context(), a.now()); err != nil {
		a.log.Error("lapsing holds", "err", err)
	}
	res, err := db.ReservationByRef(r.Context(), r.PathValue("ref"))
	if errors.Is(err, sql.ErrNoRows) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.serverError(w, r)
		return
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, _ := c.Home(res.UnitID)
	plan, _ := plans.ByCode(res.Plan)
	s, err := plans.Build(plan, res.Price, plans.Options{From: res.RequestedAt})
	if err != nil {
		s, _ = plans.Build(plan, res.Price, plans.Options{From: a.now()})
	}
	d := web.ReservationData{Reservation: res, Home: h, Schedule: s, Plan: plan, Fresh: r.URL.Query().Get("new") == "1", Now: a.now()}
	p := a.page(web.Meta{
		Title:       "Reservation " + res.Ref,
		Description: "Your reservation on home " + res.UnitID + " at Oriel, Riverside.",
		Path:        "/reservations/" + res.Ref,
		Sheet:       web.Sheet{Number: "A-711", Name: "Reservation " + res.Ref},
	})
	w.Header().Set("Cache-Control", "no-store")
	a.render(w, r, http.StatusOK, web.Reservation(p, d))
}

// ---------- Questions ----------

func (a *App) enquireMeta() web.Meta {
	return web.Meta{
		Title:       "Ask a question",
		Description: "Ask Oriel's sales team about a home, the building, prices or buying off-plan in Riverside, Nairobi.",
		Path:        "/enquire",
		Sheet:       web.Sheet{Number: "A-720", Name: "Questions"},
	}
}

func (a *App) enquireForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, web.Enquire(a.page(a.enquireMeta()), web.EnquireData{Form: web.Form{Values: map[string]string{}}}))
}

func (a *App) enquire(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if isBot(r) {
		http.Redirect(w, r, "/enquire", http.StatusSeeOther)
		return
	}
	errs := formErrors{}
	c := readContact(r, errs)
	message := strings.TrimSpace(r.PostFormValue("message"))
	switch {
	case message == "":
		errs["message"] = "Please write your question."
	case len(message) > 1500:
		errs["message"] = "Please keep your question under 1,500 characters."
	}
	values := map[string]string{"name": r.PostFormValue("name"), "email": r.PostFormValue("email"), "phone": r.PostFormValue("phone"), "interest": r.PostFormValue("interest"), "message": message}
	if len(errs) > 0 || !a.formAllowed(r) {
		if len(errs) == 0 {
			errs["message"] = "You have sent several forms in a short time. Please wait a minute and try again."
		}
		a.render(w, r, http.StatusUnprocessableEntity, web.Enquire(a.page(a.enquireMeta()), web.EnquireData{Form: web.Form{Values: values, Errors: errs}}))
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
	c.Interest = r.PostFormValue("interest")
	if _, err := db.Enquire(r.Context(), c, message, a.now()); err != nil {
		a.log.Error("saving an enquiry", "err", err)
		a.serverError(w, r)
		return
	}
	a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "lead", Text: c.Name + " sent a question"})
	a.render(w, r, http.StatusOK, web.Enquire(a.page(a.enquireMeta()), web.EnquireData{Sent: strings.Fields(c.Name)[0]}))
}
