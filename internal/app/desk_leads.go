package app

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

// ---------- Leads ----------

// leadFilter reads the table's filters; an agent always sees only their own.
func leadFilter(r *http.Request, staff store.Staff) store.LeadFilter {
	q := r.URL.Query()
	f := store.LeadFilter{Query: q.Get("q"), Sort: q.Get("sort")}
	if store.StageTitle(q.Get("stage")) != "" {
		f.Stage = q.Get("stage")
	}
	if staff.Manager() {
		f.AgentID, _ = strconv.ParseInt(q.Get("agent"), 10, 64)
	} else {
		f.AgentID = staff.ID
	}
	return f
}

const boardLimit = 24 // won and lost columns show only their most recent

func (a *App) deskLeads(w http.ResponseWriter, r *http.Request) {
	staff := staffFrom(r.Context())
	db := a.db(r)
	f := leadFilter(r, staff)
	view := "board"
	if r.URL.Query().Get("view") == "table" {
		view = "table"
	}
	d := a.deskPage(r, "leads", "Leads")
	x := web.DeskLeadsData{View: view, Filter: f, Now: a.now()}
	csvQuery := url.Values{}
	for _, k := range []string{"q", "stage", "agent", "sort"} {
		if v := r.URL.Query().Get(k); v != "" {
			csvQuery.Set(k, v)
		}
	}
	x.CSV = "/desk/leads.csv"
	if len(csvQuery) > 0 {
		x.CSV += "?" + csvQuery.Encode()
	}
	team, _ := db.Team(r.Context())
	x.Team = team
	counts, _ := db.LeadCounts(r.Context(), map[bool]int64{true: 0, false: staff.ID}[staff.Manager()])
	for _, n := range counts {
		x.Total += n
	}
	if view == "board" {
		board := store.LeadFilter{AgentID: f.AgentID, Sort: "board"}
		for _, stage := range store.Stages {
			board.Stage = stage
			leads, err := db.Leads(r.Context(), board)
			if err != nil {
				a.serverError(w, r)
				return
			}
			col := web.LeadColumn{Stage: stage, Leads: leads}
			if (stage == "won" || stage == "lost") && len(leads) > boardLimit {
				// The most recently moved, rather than the board's order.
				recent, _ := db.Leads(r.Context(), store.LeadFilter{AgentID: f.AgentID, Stage: stage, Limit: boardLimit})
				col.Leads, col.More = recent, len(leads)-boardLimit
			}
			x.Columns = append(x.Columns, col)
		}
	} else {
		f.Limit = 300
		list, err := db.Leads(r.Context(), f)
		if err != nil {
			a.serverError(w, r)
			return
		}
		x.List = list
	}
	a.render(w, r, http.StatusOK, web.DeskLeads(d, x))
}

// deskLeadsCSV is the same list as a spreadsheet.
func (a *App) deskLeadsCSV(w http.ResponseWriter, r *http.Request) {
	staff := staffFrom(r.Context())
	f := leadFilter(r, staff)
	list, err := a.db(r).Leads(r.Context(), f)
	if err != nil {
		a.serverError(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="oriel-leads-`+a.now().In(store.Nairobi()).Format("2006-01-02")+`.csv"`)
	out := csv.NewWriter(w)
	out.Write([]string{"Name", "Email", "Phone", "Stage", "Agent", "Source", "Interest", "Budget (KES)", "Buying", "Follow up on", "Lost because", "First contact", "Last activity"})
	for _, l := range list {
		budget := ""
		if l.Budget > 0 {
			budget = strconv.FormatInt(l.Budget, 10)
		}
		out.Write([]string{
			csvSafe(l.Name), l.Email, l.Phone, store.StageTitle(l.Stage), l.AgentName, csvSafe(l.Source), csvSafe(l.Interest), budget, l.BuyingFor, l.FollowUpOn, csvSafe(l.LostReason),
			l.CreatedAt.In(store.Nairobi()).Format("2006-01-02 15:04"), l.UpdatedAt.In(store.Nairobi()).Format("2006-01-02 15:04"),
		})
	}
	out.Flush()
}

// csvSafe keeps a spreadsheet from reading a cell as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (a *App) leadFromPath(w http.ResponseWriter, r *http.Request) (store.Lead, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		a.notFound(w, r)
		return store.Lead{}, false
	}
	l, err := a.db(r).LeadByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		a.notFound(w, r)
		return store.Lead{}, false
	}
	if err != nil {
		a.serverError(w, r)
		return store.Lead{}, false
	}
	staff := staffFrom(r.Context())
	if !staff.Manager() && l.AgentID != staff.ID {
		http.Error(w, "This lead belongs to another agent.", http.StatusForbidden)
		return store.Lead{}, false
	}
	return l, true
}

func (a *App) deskLead(w http.ResponseWriter, r *http.Request) {
	a.showLead(w, r, "")
}

func (a *App) showLead(w http.ResponseWriter, r *http.Request, problem string) {
	l, ok := a.leadFromPath(w, r)
	if !ok {
		return
	}
	db := a.db(r)
	d := a.deskPage(r, "leads", l.Name)
	x := web.DeskLeadData{Lead: l, Now: a.now(), Problem: problem, CanEdit: true}
	x.Timeline, _ = db.Timeline(r.Context(), l.ID)
	x.Viewings, _ = db.LeadViewings(r.Context(), l.ID)
	x.Reservations, _ = db.LeadReservations(r.Context(), l.ID)
	team, _ := db.Team(r.Context())
	x.Team = team
	status := http.StatusOK
	if problem != "" {
		status = http.StatusUnprocessableEntity
	}
	a.render(w, r, status, web.DeskLead(d, x))
}

// leadAction runs a change to a lead from its page and goes back to it.
func (a *App) leadAction(w http.ResponseWriter, r *http.Request, run func(db *store.DB, l store.Lead, staff store.Staff) (string, error)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	l, ok := a.leadFromPath(w, r)
	if !ok {
		return
	}
	done, err := run(a.db(r), l, staffFrom(r.Context()))
	if err != nil {
		if isInternal(err) {
			a.log.Error("changing a lead", "lead", l.ID, "err", err)
			a.serverError(w, r)
			return
		}
		a.showLead(w, r, capitalFirst(err.Error()))
		return
	}
	a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "lead-changed", ID: strconv.FormatInt(l.ID, 10)})
	http.Redirect(w, r, "/desk/leads/"+strconv.FormatInt(l.ID, 10)+"?"+a.doneQuery(done), http.StatusSeeOther)
}

func (a *App) deskLeadNote(w http.ResponseWriter, r *http.Request) {
	a.leadAction(w, r, func(db *store.DB, l store.Lead, staff store.Staff) (string, error) {
		return "Note saved.", db.AddNote(r.Context(), l.ID, r.PostFormValue("body"), staff, a.now())
	})
}

func (a *App) deskLeadStage(w http.ResponseWriter, r *http.Request) {
	a.leadAction(w, r, func(db *store.DB, l store.Lead, staff store.Staff) (string, error) {
		stage := r.PostFormValue("stage")
		if err := db.MoveLead(r.Context(), l.ID, stage, r.PostFormValue("reason"), 0, 0, staff, a.now()); err != nil {
			return "", err
		}
		return "Moved to " + store.StageTitle(stage) + ".", nil
	})
}

func (a *App) deskLeadFollowUp(w http.ResponseWriter, r *http.Request) {
	a.leadAction(w, r, func(db *store.DB, l store.Lead, staff store.Staff) (string, error) {
		day := r.PostFormValue("day")
		if r.PostFormValue("clear") == "1" {
			day = ""
		}
		if err := db.SetFollowUp(r.Context(), l.ID, day, staff, a.now()); err != nil {
			return "", err
		}
		if day == "" {
			return "Follow-up cleared.", nil
		}
		return "Follow-up set for " + web.DayLabel(day) + ".", nil
	})
}

func (a *App) deskLeadAssign(w http.ResponseWriter, r *http.Request) {
	a.leadAction(w, r, func(db *store.DB, l store.Lead, staff store.Staff) (string, error) {
		agent, err := strconv.ParseInt(r.PostFormValue("agent"), 10, 64)
		if err != nil {
			return "", userError("please choose an agent")
		}
		if err := db.AssignLead(r.Context(), l.ID, agent, staff, a.now()); err != nil {
			return "", err
		}
		m, _ := db.StaffByID(r.Context(), agent)
		return "Assigned to " + m.Name + ".", nil
	})
}

// deskLeadMove is a card dropped on the board.
func (a *App) deskLeadMove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "No such lead.", http.StatusNotFound)
		return
	}
	after, _ := strconv.ParseInt(r.PostFormValue("after"), 10, 64)
	before, _ := strconv.ParseInt(r.PostFormValue("before"), 10, 64)
	err = a.db(r).MoveLead(r.Context(), id, r.PostFormValue("stage"), r.PostFormValue("reason"), after, before, staffFrom(r.Context()), a.now())
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "No such lead.", http.StatusNotFound)
	case errors.Is(err, store.ErrNotYours):
		http.Error(w, err.Error(), http.StatusForbidden)
	case err != nil && isInternal(err):
		a.log.Error("moving a lead", "lead", id, "err", err)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
	case err != nil:
		http.Error(w, capitalFirst(err.Error()), http.StatusUnprocessableEntity)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *App) deskNewLeadForm(w http.ResponseWriter, r *http.Request) {
	team, _ := a.db(r).Team(r.Context())
	a.render(w, r, http.StatusOK, web.DeskNewLead(a.deskPage(r, "leads", "Add a lead"), web.DeskNewLeadData{Form: web.Form{Values: map[string]string{}}, Team: team}))
}

func (a *App) deskAddLead(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	staff := staffFrom(r.Context())
	db := a.db(r)
	errs := formErrors{}
	c := readContact(r, errs)
	values := map[string]string{}
	for _, k := range []string{"name", "email", "phone", "source", "interest", "buying", "agent", "note"} {
		values[k] = r.PostFormValue(k)
	}
	team, _ := db.Team(r.Context())
	show := func(status int) {
		a.render(w, r, status, web.DeskNewLead(a.deskPage(r, "leads", "Add a lead"), web.DeskNewLeadData{Form: web.Form{Values: values, Errors: errs}, Team: team}))
	}
	if len(errs) > 0 {
		show(http.StatusUnprocessableEntity)
		return
	}
	c.Source = strings.TrimSpace(r.PostFormValue("source"))
	c.Interest = strings.TrimSpace(r.PostFormValue("interest"))
	c.Buying = "home"
	if r.PostFormValue("buying") == "investment" {
		c.Buying = "investment"
	}
	agent, _ := strconv.ParseInt(r.PostFormValue("agent"), 10, 64)
	id, err := db.AddLead(r.Context(), store.NewLead{Contact: c, Note: strings.TrimSpace(r.PostFormValue("note")), AgentID: agent}, staff, a.now())
	var exists store.LeadExistsError
	if errors.As(err, &exists) {
		errs["email"] = "A lead with this email address is already on the desk."
		show(http.StatusConflict)
		return
	}
	if err != nil {
		a.log.Error("adding a lead", "err", err)
		a.serverError(w, r)
		return
	}
	a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "lead-changed", ID: strconv.FormatInt(id, 10)})
	http.Redirect(w, r, "/desk/leads/"+strconv.FormatInt(id, 10)+"?"+a.doneQuery(c.Name+" added."), http.StatusSeeOther)
}
