package web

import (
	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/store"
)

// ExploreHome is what the explorer knows about each home.
type ExploreHome struct {
	ID     string  `json:"id"`
	Level  int     `json:"l"`
	Pos    int     `json:"p"`
	Type   string  `json:"t"`
	Beds   int     `json:"b"`
	Size   int     `json:"m"`
	Status string  `json:"s"`
	Price  int64   `json:"pr,omitempty"`
	Faces  string  `json:"f"`
	Sun    float64 `json:"h"`
	River  bool    `json:"r,omitempty"`
}

// ExploreData is the explorer's starting state, embedded in the page.
type ExploreData struct {
	Model  building.Model `json:"model"`
	Homes  []ExploreHome  `json:"homes"`
	Filter Filter         `json:"-"`
	Start  string         `json:"start,omitempty"`
	Counts map[string]int `json:"-"`
}

// ExploreHomes lists every home for the explorer.
func ExploreHomes(c *store.Catalog) []ExploreHome {
	out := make([]ExploreHome, 0, len(c.Homes))
	for _, h := range c.Homes {
		e := ExploreHome{
			ID:     h.ID,
			Level:  h.Level,
			Pos:    h.Position.Number,
			Type:   h.Type.Code,
			Beds:   h.Type.Bedrooms,
			Size:   h.Type.Internal,
			Status: h.Status,
			Faces:  h.Facings(),
			River:  h.Position.RiverView,
		}
		if h.ShowsPrice() {
			e.Price = h.Price
		}
		if s, ok := lightFor(h.ID); ok {
			e.Sun = s.AverageDay
		}
		out = append(out, e)
	}
	return out
}

// LevelRow is one row of the explorer's list of floors: a small stacking plan.
type LevelRow struct {
	Level     int
	Name      string
	Homes     []store.Home
	Available int
}

// LevelRows lists the floors with homes, from the top down.
func LevelRows(c *store.Catalog) []LevelRow {
	var rows []LevelRow
	for _, level := range building.HomeLevels() {
		row := LevelRow{Level: level, Name: LevelName(level)}
		for _, h := range c.Homes {
			if h.Level == level {
				row.Homes = append(row.Homes, h)
				if h.Status == store.Available {
					row.Available++
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}
