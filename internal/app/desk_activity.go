package app

import (
	"net/http"
	"strconv"

	"github.com/jeffgicharu/oriel/internal/web"
)

const activityPage = 60

func (a *App) deskActivity(w http.ResponseWriter, r *http.Request) {
	actor := r.URL.Query().Get("actor")
	if actor != "staff" && actor != "visitor" && actor != "system" {
		actor = ""
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	entries, err := a.db(r).ActivityLog(r.Context(), actor, (page-1)*activityPage, activityPage+1)
	if err != nil {
		a.serverError(w, r)
		return
	}
	x := web.DeskActivityData{Actor: actor, Page: page, Now: a.now()}
	if len(entries) > activityPage {
		x.More, entries = true, entries[:activityPage]
	}
	x.Entries = entries
	a.render(w, r, http.StatusOK, web.DeskActivity(a.deskPage(r, "activity", "Activity"), x))
}
