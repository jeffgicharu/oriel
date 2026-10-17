package app

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

var homeID = regexp.MustCompile(`^[0-9]{4}$`)

// homesFromQuery reads home numbers given as homes=0704,0806 or as repeated
// homes= values, in order, without repeats, up to max.
func homesFromQuery(r *http.Request, max int) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range r.URL.Query()["homes"] {
		for _, id := range strings.Split(v, ",") {
			id = strings.TrimSpace(id)
			if homeID.MatchString(id) && !seen[id] && len(out) < max {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

func (a *App) homesIn(w http.ResponseWriter, r *http.Request, ids []string) ([]store.Home, bool) {
	c, ok := a.catalog(w, r)
	if !ok {
		return nil, false
	}
	var homes []store.Home
	for _, id := range ids {
		if h, found := c.Home(id); found {
			homes = append(homes, h)
		}
	}
	return homes, true
}

func (a *App) shortlistPage(w http.ResponseWriter, r *http.Request) {
	ids := homesFromQuery(r, 12)
	homes, ok := a.homesIn(w, r, ids)
	if !ok {
		return
	}
	p := a.page(web.Meta{
		Title:       "Your shortlist",
		Description: "The Oriel homes you have shortlisted, kept in your browser. Choose up to three to compare side by side.",
		Path:        "/shortlist",
		Sheet:       web.Sheet{Number: "A-210", Name: "Shortlist"},
		Nav:         "residences",
	})
	a.render(w, r, http.StatusOK, web.ShortlistPage(p, web.ShortlistData{Homes: homes, IDs: strings.Join(ids, ",")}))
}

func (a *App) comparePage(w http.ResponseWriter, r *http.Request) {
	ids := homesFromQuery(r, web.MaxCompare)
	homes, ok := a.homesIn(w, r, ids)
	if !ok {
		return
	}
	if len(homes) == 0 {
		http.Redirect(w, r, "/shortlist", http.StatusSeeOther)
		return
	}
	var found []string
	for _, h := range homes {
		found = append(found, h.ID)
	}
	p := a.page(web.Meta{
		Title:       "Compare homes " + strings.Join(found, ", "),
		Description: "Oriel homes side by side: plans to the same scale, aspect, size, sunlight, status and price.",
		Path:        "/compare",
		Sheet:       web.Sheet{Number: "A-220", Name: "Compare"},
		Nav:         "residences",
	})
	a.render(w, r, http.StatusOK, web.ComparePage(p, web.CompareData{Homes: homes, IDs: found}))
}
