package app

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/light"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

func (a *App) catalog(w http.ResponseWriter, r *http.Request) (*store.Catalog, bool) {
	c, err := a.db(r).LoadCatalog(r.Context())
	if err != nil {
		a.log.Error("loading the catalog", "err", err)
		a.serverError(w, r)
		return nil, false
	}
	return c, true
}

func (a *App) residences(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	f := web.ParseFilter(r.URL.Query())
	var homes []store.Home
	for _, h := range c.Homes {
		if f.Matches(h) {
			homes = append(homes, h)
		}
	}
	sortHomes(homes, f.Sort)
	counts := c.Counts()
	d := web.ResidencesData{
		Types:     c.Types(),
		Homes:     homes,
		Filter:    f,
		Available: counts[store.Available],
		Total:     len(c.Homes),
		From:      c.FromPrice(),
	}
	// Filters ask for the list alone.
	if r.Header.Get("HX-Request") == "true" {
		a.render(w, r, http.StatusOK, web.ResidenceResults(d))
		return
	}
	p := a.page(web.Meta{
		Title:       "Residences",
		Description: "Every home at Oriel, Riverside: one, two and three bedrooms and two penthouses, with live availability, prices, plans and the hours of sun each one gets.",
		Path:        "/residences",
		Sheet:       web.Sheet{Number: "A-200", Name: "Residences"},
		Nav:         "residences",
	})
	a.render(w, r, http.StatusOK, web.Residences(p, d))
}

func sortHomes(homes []store.Home, by string) {
	sort.SliceStable(homes, func(i, j int) bool {
		a, b := homes[i], homes[j]
		switch by {
		case "price-desc":
			return a.Price > b.Price
		case "floor":
			if a.Level != b.Level {
				return a.Level > b.Level
			}
			return a.Position.Number < b.Position.Number
		case "size":
			if a.Type.Internal != b.Type.Internal {
				return a.Type.Internal > b.Type.Internal
			}
			return a.Price < b.Price
		default:
			if a.Price != b.Price {
				return a.Price < b.Price
			}
			return a.ID < b.ID
		}
	})
}

// scheduleFor builds the payment schedule asked for in the query.
func (a *App) scheduleFor(r *http.Request, h store.Home) (plans.Schedule, string) {
	code := r.URL.Query().Get("plan")
	plan, ok := plans.ByCode(code)
	if !ok {
		plan = plans.Plans[0]
	}
	opt := plans.Options{From: a.now()}
	if v, err := strconv.ParseInt(r.URL.Query().Get("rate"), 10, 64); err == nil {
		opt.RateBP = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("years")); err == nil {
		opt.Years = v
	}
	s, err := plans.Build(plan, h.Price, opt)
	if err != nil {
		s, _ = plans.Build(plan, h.Price, plans.Options{From: a.now()})
	}
	return s, plan.Code
}

func (a *App) unit(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found {
		a.notFound(w, r)
		return
	}
	schedule, code := a.scheduleFor(r, h)
	study, _ := light.For(h.ID)
	var similar []store.Home
	for _, other := range c.Homes {
		if other.ID != h.ID && other.Type.Code == h.Type.Code && other.Status == store.Available {
			similar = append(similar, other)
		}
	}
	sort.SliceStable(similar, func(i, j int) bool {
		return abs(similar[i].Level-h.Level) < abs(similar[j].Level-h.Level)
	})
	if len(similar) > 3 {
		similar = similar[:3]
	}
	d := web.UnitData{
		Home:     h,
		Plan:     web.PlanFor(h.Unit),
		Light:    study,
		Schedule: schedule,
		PlanCode: code,
		Similar:  similar,
	}
	desc := web.HomeTitle(h) + " at Oriel, Riverside, facing " + h.Facings() + ". " +
		strconv.Itoa(h.Type.Internal) + " m² inside with a " + strconv.Itoa(h.Type.Outdoor) + " m² " + strings.ToLower(h.Type.OutdoorName) + ". "
	if h.ShowsPrice() {
		desc += web.KES(h.Price) + ". "
	} else {
		desc += store.StatusName(h.Status) + ". "
	}
	desc += "Plan, light study and payment schedule."
	p := a.page(web.Meta{
		Title:       "Home " + h.ID + ", " + h.Type.Name,
		Description: desc,
		Path:        web.HomePath(h.ID),
		Sheet:       web.Sheet{Number: "A-2·" + h.ID, Name: "Home " + h.ID},
		Nav:         "residences",
		JSONLD:      []any{apartmentLD(a.cfg.SiteURL, h)},
		BodyClass:   "has-unit-bar",
	})
	a.render(w, r, http.StatusOK, web.Unit(p, d))
}

func (a *App) unitSchedule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	h, found := c.Home(r.PathValue("id"))
	if !found || !h.ShowsPrice() {
		a.notFound(w, r)
		return
	}
	s, _ := a.scheduleFor(r, h)
	a.render(w, r, http.StatusOK, web.Schedule(h, s))
}

func (a *App) homeType(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("code"))
	t, ok := building.Types[code]
	if !ok {
		a.notFound(w, r)
		return
	}
	if r.PathValue("code") != code {
		http.Redirect(w, r, "/residences/types/"+code, http.StatusMovedPermanently)
		return
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	var homes []store.Home
	var summary store.TypeSummary
	for _, s := range c.Types() {
		if s.Code == code {
			summary = s
		}
	}
	for _, h := range c.Homes {
		if h.Type.Code == code {
			homes = append(homes, h)
		}
	}
	sortHomes(homes, "floor")
	p := a.page(web.Meta{
		Title:       "Type " + code + ": " + t.Name,
		Description: t.Summary + " " + strconv.Itoa(t.Internal) + " m² inside, " + strconv.Itoa(summary.Total) + " homes at Oriel, Riverside.",
		Path:        "/residences/types/" + code,
		Sheet:       web.Sheet{Number: "A-21" + strconv.Itoa(strings.Index("ABCDP", code)), Name: "Type " + code},
		Nav:         "residences",
	})
	a.render(w, r, http.StatusOK, web.TypePage(p, summary, homes))
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func apartmentLD(site string, h store.Home) map[string]any {
	ld := map[string]any{
		"@context":               "https://schema.org",
		"@type":                  "Apartment",
		"name":                   "Home " + h.ID + " at Oriel",
		"description":            h.Type.Summary,
		"url":                    site + web.HomePath(h.ID),
		"numberOfRooms":          h.Type.Bedrooms + 1,
		"numberOfBedrooms":       h.Type.Bedrooms,
		"numberOfBathroomsTotal": h.Type.Bathrooms,
		"floorLevel":             strconv.Itoa(h.Level),
		"floorSize": map[string]any{
			"@type":    "QuantitativeValue",
			"value":    h.Type.Internal,
			"unitCode": "MTK",
		},
		"containedInPlace": map[string]any{"@id": site + "/#oriel"},
	}
	if h.ShowsPrice() {
		availability := "https://schema.org/InStock"
		if h.Status == store.Held {
			availability = "https://schema.org/LimitedAvailability"
		}
		ld["offers"] = map[string]any{
			"@type":         "Offer",
			"price":         h.Price,
			"priceCurrency": "KES",
			"availability":  availability,
			"url":           site + web.HomePath(h.ID),
		}
	}
	return ld
}
