package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/jeffgicharu/oriel/internal/pdf"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

// The letter of offer and the price list, as PDFs.

func (a *App) siteHost() string {
	return strings.TrimPrefix(strings.TrimPrefix(a.cfg.SiteURL, "https://"), "http://")
}

func parkingFor(code string) int {
	if code == "A" || code == "B" {
		return 1
	}
	return 2
}

// offerPDF is a confirmed reservation's letter of offer.
func (a *App) offerPDF(w http.ResponseWriter, r *http.Request) {
	db := a.db(r)
	res, err := db.ReservationByRef(r.Context(), r.PathValue("ref"))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && res.OfferNumber == "") {
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
	h, found := c.Home(res.UnitID)
	if !found {
		a.notFound(w, r)
		return
	}
	plan, _ := plans.ByCode(res.Plan)
	from := res.DecidedAt
	if from.IsZero() {
		from = res.RequestedAt
	}
	s, err := plans.Build(plan, res.Price, plans.Options{From: from})
	if err != nil {
		a.serverError(w, r)
		return
	}
	manager, err := db.FirstWithRole(r.Context(), "manager")
	if err != nil {
		a.serverError(w, r)
		return
	}
	body, err := pdf.OfferLetter(pdf.Offer{
		Reservation: res, Home: h, HomeTitle: web.HomeTitle(h), Schedule: s, Plan: web.PlanFor(h.Unit),
		Manager: manager.Name, Parking: parkingFor(h.Type.Code), Issued: from, Nairobi: store.Nairobi(), Site: a.siteHost(),
	})
	if err != nil {
		a.log.Error("drawing a letter of offer", "ref", res.Ref, "err", err)
		a.serverError(w, r)
		return
	}
	writePDF(w, "oriel-letter-of-offer-"+strings.ToLower(res.OfferNumber)+".pdf", body)
}

// priceListPDF is every home for sale with its price.
func (a *App) priceListPDF(w http.ResponseWriter, r *http.Request) {
	c, ok := a.catalog(w, r)
	if !ok {
		return
	}
	var listID int64
	a.db(r).QueryRowContext(r.Context(), `SELECT COALESCE(MAX(id), 1) FROM price_lists`).Scan(&listID)
	body, err := pdf.PriceList(pdf.PriceListDoc{Catalog: c, ListID: listID, AsAt: a.now(), Nairobi: store.Nairobi(), Site: a.siteHost()})
	if err != nil {
		a.log.Error("drawing the price list", "err", err)
		a.serverError(w, r)
		return
	}
	writePDF(w, "oriel-price-list.pdf", body)
}

func writePDF(w http.ResponseWriter, name string, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/pdf")
	h.Set("Content-Disposition", `inline; filename="`+name+`"`)
	h.Set("Cache-Control", "private, no-cache")
	w.Write(body)
}
