package pdf

import (
	"fmt"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
)

// PriceListDoc is what the price list shows.
type PriceListDoc struct {
	Catalog *store.Catalog
	ListID  int64
	AsAt    time.Time
	Nairobi *time.Location
	Site    string
}

// PriceList draws every home for sale with its price, by type.
func PriceList(p PriceListDoc) ([]byte, error) {
	asAt := p.AsAt.In(p.Nairobi)
	d, err := newDoc("Oriel price list", "Prices as at "+asAt.Format("2 January 2006"),
		"Oriel Residences Limited · Riverside, Nairobi · Price list "+fmt.Sprint(p.ListID)+" · Prices change and are confirmed only in a letter of offer")
	if err != nil {
		return nil, err
	}
	available, from := 0, int64(0)
	for _, h := range p.Catalog.Homes {
		if h.Status == store.Available {
			available++
			if from == 0 || h.Price < from {
				from = h.Price
			}
		}
	}
	d.header("Price list", [][2]string{
		{"Price list", fmt.Sprint(p.ListID)},
		{"As at", asAt.Format("2 January 2006")},
		{"Available", fmt.Sprintf("%d of %d", available, len(p.Catalog.Homes))},
	})
	d.text(marginL, d.y, display, 19, graphite, "Homes for sale at Oriel, Riverside")
	d.y += 26
	d.para(marginL, width, sans, 9.5, 14, inkSoft, fmt.Sprintf("Every home still for sale, with its price on the list in force. Homes on hold are waiting for a reservation to be confirmed and may come free. Completion is planned for %s.", plans.Completion.Format("January 2006")))
	d.y += 16

	cols := []float64{marginL, marginL + 52, marginL + 100, marginL + 214, marginL + 270, marginL + 330, right}
	head := func() {
		d.text(cols[0], d.y, sans, 7.5, inkSoft, "Home")
		d.text(cols[1], d.y, sans, 7.5, inkSoft, "Level")
		d.text(cols[2], d.y, sans, 7.5, inkSoft, "Faces")
		d.text(cols[3], d.y, sans, 7.5, inkSoft, "Inside")
		d.text(cols[4], d.y, sans, 7.5, inkSoft, "Outside")
		d.text(cols[5], d.y, sans, 7.5, inkSoft, "Status")
		d.rightText(cols[6], d.y, sans, 7.5, inkSoft, "Price")
		d.y += 6
		d.line(marginL, d.y, right, d.y, 0.8, graphite)
	}
	for _, t := range building.OrderedTypes() {
		var homes []store.Home
		withheld := 0
		for _, h := range p.Catalog.Homes {
			if h.Type.Code != t.Code {
				continue
			}
			switch h.Status {
			case store.Available, store.Held:
				homes = append(homes, h)
			case store.Withheld:
				withheld++
			}
		}
		d.ensure(64)
		d.text(marginL, d.y, mono, 9, cobalt, t.Code)
		d.text(marginL+18, d.y, semi, 11, graphite, t.Name)
		d.rightText(right, d.y, sans, 8.5, inkSoft, fmt.Sprintf("%d m² inside · %s %d m²", t.Internal, t.OutdoorName, t.Outdoor))
		d.y += 18
		if len(homes) == 0 {
			note := "Sold out."
			if withheld > 0 {
				note = "None for sale just now. The next release is coming soon."
			}
			d.text(marginL, d.y, sans, 9, inkSoft, note)
			d.y += 24
			continue
		}
		head()
		for _, h := range homes {
			if d.y+20 > bottom {
				d.newPage()
				head()
			}
			d.y += 13
			d.text(cols[0], d.y, mono, 8.5, graphite, h.ID)
			d.text(cols[1], d.y, mono, 8, graphite, building.LevelName(h.Level))
			d.text(cols[2], d.y, sans, 8.5, graphite, h.Facings())
			d.text(cols[3], d.y, sans, 8.5, graphite, fmt.Sprintf("%d m²", h.Type.Internal))
			d.text(cols[4], d.y, sans, 8.5, graphite, fmt.Sprintf("%d m²", h.Type.Outdoor))
			status := "Available"
			c := cobalt
			if h.Status == store.Held {
				status, c = "On hold", inkSoft
			}
			d.text(cols[5], d.y, sans, 8.5, c, status)
			d.rightText(cols[6], d.y, semi, 9, graphite, kes(h.Price))
			d.y += 7
			d.line(marginL, d.y, right, d.y, 0.4, rule)
		}
		d.y += 26
	}

	d.ensure(120)
	d.label(marginL, "Paying")
	d.y += 14
	for _, pl := range plans.Plans {
		d.ensure(30)
		d.text(marginL, d.y, semi, 9.5, graphite, pl.Name)
		d.para(marginL+110, width-110, sans, 9, 13.5, graphite, pl.Summary)
		d.y += 6
	}
	d.y += 8
	d.para(marginL, width, sans, 8.5, 12.5, inkSoft, "Prices are in Kenyan shillings and change as homes are sold; a price is confirmed only in a letter of offer. Stamp duty, legal fees and other costs of buying are paid by the buyer: see "+p.Site+"/buying. Plans, areas and specification are a guide and may change in detail.")
	return d.bytes()
}
