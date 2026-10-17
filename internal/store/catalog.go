package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/pricing"
)

// Statuses a home can have.
const (
	Available = "available"
	Held      = "held"
	Reserved  = "reserved"
	Sold      = "sold"
	Withheld  = "withheld"
)

// StatusName is a status as a buyer reads it.
func StatusName(status string) string {
	switch status {
	case Available:
		return "Available"
	case Held:
		return "On hold"
	case Reserved:
		return "Reserved"
	case Sold:
		return "Sold"
	case Withheld:
		return "Not yet released"
	}
	return status
}

// PriceList is the price list in force.
type PriceList struct {
	ID          int64
	PublishedAt time.Time
	PublishedBy int64
	List        pricing.List
	Note        string
}

// Home is one home with its current status and price.
type Home struct {
	building.Unit
	Status          string
	StatusChangedAt time.Time
	// Price is what the home costs now: its override, or the price list's figure.
	Price      int64
	ListPrice  int64
	Overridden bool
}

// OnSale reports whether a buyer can still reserve the home.
func (h Home) OnSale() bool { return h.Status == Available }

// ShowsPrice reports whether the public site shows the home's price.
func (h Home) ShowsPrice() bool { return h.Status == Available || h.Status == Held }

// Catalog is every home as it stands, with the price list in force.
type Catalog struct {
	Homes     []Home
	byID      map[string]int
	PriceList PriceList
}

// Home finds a home by its number.
func (c *Catalog) Home(id string) (Home, bool) {
	i, ok := c.byID[id]
	if !ok {
		return Home{}, false
	}
	return c.Homes[i], true
}

// TypeSummary is what the site says about a type: how many homes are left
// and the lowest price among them.
type TypeSummary struct {
	building.Type
	Total     int
	Available int
	From      int64
}

// Types summarises each type, in order.
func (c *Catalog) Types() []TypeSummary {
	var out []TypeSummary
	for _, t := range building.OrderedTypes() {
		s := TypeSummary{Type: t}
		for _, h := range c.Homes {
			if h.Type.Code != t.Code {
				continue
			}
			s.Total++
			if h.Status == Available {
				s.Available++
				if s.From == 0 || h.Price < s.From {
					s.From = h.Price
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// Counts is how many homes have each status.
func (c *Catalog) Counts() map[string]int {
	out := map[string]int{}
	for _, h := range c.Homes {
		out[h.Status]++
	}
	return out
}

// FromPrice is the lowest price of any available home.
func (c *Catalog) FromPrice() int64 {
	var from int64
	for _, h := range c.Homes {
		if h.Status == Available && (from == 0 || h.Price < from) {
			from = h.Price
		}
	}
	return from
}

// CurrentPriceList reads the newest published price list.
func (db *DB) CurrentPriceList(ctx context.Context) (PriceList, error) {
	var pl PriceList
	var at int64
	var by sql.NullInt64
	var a, b, cc, d, p int64
	err := db.QueryRowContext(ctx, `SELECT id, published_at, published_by, floor_bp, river_bp, base_a, base_b, base_c, base_d, base_p, note
		FROM price_lists ORDER BY published_at DESC, id DESC LIMIT 1`).
		Scan(&pl.ID, &at, &by, &pl.List.FloorBP, &pl.List.RiverBP, &a, &b, &cc, &d, &p, &pl.Note)
	if err != nil {
		return pl, err
	}
	pl.PublishedAt = time.Unix(at, 0)
	pl.PublishedBy = by.Int64
	pl.List.Base = map[string]int64{"A": a, "B": b, "C": cc, "D": d, "P": p}
	return pl, nil
}

// LoadCatalog reads every home's status and works out its price.
func (db *DB) LoadCatalog(ctx context.Context) (*Catalog, error) {
	pl, err := db.CurrentPriceList(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, status, price_override, status_changed_at FROM units`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type state struct {
		status   string
		override sql.NullInt64
		changed  int64
	}
	states := map[string]state{}
	for rows.Next() {
		var id string
		var s state
		if err := rows.Scan(&id, &s.status, &s.override, &s.changed); err != nil {
			return nil, err
		}
		states[id] = s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	c := &Catalog{PriceList: pl, byID: map[string]int{}}
	for _, u := range building.Units() {
		s, ok := states[u.ID]
		if !ok {
			s = state{status: Available}
		}
		list := pl.List.Price(u)
		h := Home{Unit: u, Status: s.status, StatusChangedAt: time.Unix(s.changed, 0), Price: list, ListPrice: list}
		if s.override.Valid {
			h.Price = s.override.Int64
			h.Overridden = true
		}
		c.byID[u.ID] = len(c.Homes)
		c.Homes = append(c.Homes, h)
	}
	return c, nil
}
