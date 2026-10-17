package app

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/pricing"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

// ---------- Pricing ----------

func (a *App) pricingData(r *http.Request) (web.DeskPricingData, error) {
	lists, err := a.db(r).PriceLists(r.Context())
	if err != nil || len(lists) == 0 {
		return web.DeskPricingData{}, err
	}
	x := web.DeskPricingData{Current: lists[0], History: lists, Now: a.now(), Form: web.Form{Values: map[string]string{}}}
	for _, code := range building.TypeOrder {
		x.Form.Values[code] = web.KES(lists[0].List.Base[code])[4:]
	}
	x.Form.Values["floor"] = strings.TrimSuffix(pricing.Percent(lists[0].List.FloorBP), "%")
	x.Form.Values["river"] = strings.TrimSuffix(pricing.Percent(lists[0].List.RiverBP), "%")
	return x, nil
}

func (a *App) deskPricing(w http.ResponseWriter, r *http.Request) {
	x, err := a.pricingData(r)
	if err != nil {
		a.serverError(w, r)
		return
	}
	a.render(w, r, http.StatusOK, web.DeskPricing(a.deskPage(r, "pricing", "Pricing"), x))
}

// readPriceList reads a price list from the form, noting what is wrong.
func readPriceList(r *http.Request, errs formErrors) pricing.List {
	l := pricing.List{Base: map[string]int64{}}
	for _, code := range building.TypeOrder {
		p, err := web.ParseShillings(r.PostFormValue(code))
		switch {
		case err != nil || p <= 0:
			errs[code] = "Type " + code + ": please give the base price in shillings."
		case p%pricing.Rounding != 0:
			errs[code] = "Type " + code + ": prices are set to the nearest KES 10,000."
		}
		l.Base[code] = p
	}
	var ok bool
	if l.FloorBP, ok = web.ParsePercentBP(r.PostFormValue("floor")); !ok {
		errs["floor"] = "The floor premium should be a percentage, such as 1 or 0.8."
	}
	if l.RiverBP, ok = web.ParsePercentBP(r.PostFormValue("river")); !ok {
		errs["river"] = "The river premium should be a percentage, such as 4."
	}
	if len(errs) == 0 {
		if err := l.Validate(); err != nil {
			errs["floor"] = capitalFirst(err.Error()) + "."
		}
	}
	return l
}

func (a *App) deskPricingPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	x, err := a.pricingData(r)
	if err != nil {
		a.serverError(w, r)
		return
	}
	errs := formErrors{}
	list := readPriceList(r, errs)
	for _, k := range append(append([]string{}, building.TypeOrder...), "floor", "river", "note") {
		x.Form.Values[k] = r.PostFormValue(k)
	}
	x.Form.Errors = errs
	if len(errs) > 0 {
		a.render(w, r, http.StatusUnprocessableEntity, web.DeskPricing(a.deskPage(r, "pricing", "Pricing"), x))
		return
	}
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	p := &web.PricePreview{List: list, Note: strings.TrimSpace(r.PostFormValue("note"))}
	for _, h := range c.Homes {
		if h.Status == store.Reserved || h.Status == store.Sold {
			continue
		}
		row := web.PriceRow{Home: h, Old: h.Price, New: list.Price(h.Unit), KeepsOwn: h.Overridden}
		if row.KeepsOwn {
			row.New = h.Price
			p.KeptOwn++
		} else if row.New != row.Old {
			p.Changed++
		}
		p.OldTotal += row.Old
		p.NewTotal += row.New
		p.Rows = append(p.Rows, row)
	}
	// The homes that change come first.
	sort.SliceStable(p.Rows, func(i, j int) bool {
		ci, cj := p.Rows[i].Change() != 0, p.Rows[j].Change() != 0
		if ci != cj {
			return ci
		}
		if p.Rows[i].Home.Type.Code != p.Rows[j].Home.Type.Code {
			return strings.Index("ABCDP", p.Rows[i].Home.Type.Code) < strings.Index("ABCDP", p.Rows[j].Home.Type.Code)
		}
		return p.Rows[i].Home.ID < p.Rows[j].Home.ID
	})
	x.Preview = p
	a.render(w, r, http.StatusOK, web.DeskPricing(a.deskPage(r, "pricing", "Pricing"), x))
}

func (a *App) deskPublishPrices(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	errs := formErrors{}
	list := readPriceList(r, errs)
	if len(errs) > 0 {
		x, _ := a.pricingData(r)
		x.Form.Errors = errs
		a.render(w, r, http.StatusUnprocessableEntity, web.DeskPricing(a.deskPage(r, "pricing", "Pricing"), x))
		return
	}
	id, err := a.db(r).PublishPriceList(r.Context(), list, r.PostFormValue("note"), staffFrom(r.Context()), a.now())
	if err != nil {
		if isInternal(err) {
			a.log.Error("publishing prices", "err", err)
			a.serverError(w, r)
			return
		}
		x, _ := a.pricingData(r)
		x.Form.Errors = formErrors{"floor": capitalFirst(err.Error())}
		a.render(w, r, http.StatusUnprocessableEntity, web.DeskPricing(a.deskPage(r, "pricing", "Pricing"), x))
		return
	}
	a.broker.publish(visitorFrom(r.Context()).copyID, event{Type: "prices", ID: strconv.FormatInt(id, 10)})
	http.Redirect(w, r, "/desk/pricing?"+a.doneQuery("Price list "+strconv.FormatInt(id, 10)+" is in force. The website shows the new prices."), http.StatusSeeOther)
}
