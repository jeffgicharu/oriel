package web

import (
	"sort"
	"strconv"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
)

// HomeStory is what the home page tells beyond its opening.
type HomeStory struct {
	Types     []store.TypeSummary
	From      int64
	Available int
	Total     int
	Stack     []StackRow
	Progress  Progress
}

// StackRow is one floor of the stacking plan.
type StackRow struct {
	Level int
	Cells []StackCell
}

// StackCell is one home on the stacking plan.
type StackCell struct {
	ID, Status, Type string
	Wide             bool // a penthouse, which takes half the floor
}

// StoryFor gathers the home page's figures from the catalogue.
func StoryFor(c *store.Catalog, now time.Time) HomeStory {
	s := HomeStory{Types: c.Types(), From: c.FromPrice(), Progress: ProgressOn(now), Total: len(c.Homes)}
	byLevel := map[int][]StackCell{}
	for _, h := range c.Homes {
		if h.Status == store.Available {
			s.Available++
		}
		byLevel[h.Level] = append(byLevel[h.Level], StackCell{ID: h.ID, Status: h.Status, Type: h.Type.Code, Wide: h.Level == building.PenthouseLevel})
	}
	for _, level := range building.HomeLevels() {
		cells := byLevel[level]
		sort.Slice(cells, func(i, j int) bool { return cells[i].ID < cells[j].ID })
		s.Stack = append(s.Stack, StackRow{Level: level, Cells: cells})
	}
	return s
}

// StackTitle describes a stacking plan cell for a screen reader and a tooltip.
func StackTitle(c StackCell) string {
	return "Home " + c.ID + ", type " + c.Type + ": " + store.StatusName(c.Status)
}

func stackLabel(s HomeStory) string {
	return "Stacking plan: " + strconv.Itoa(s.Available) + " of " + strconv.Itoa(s.Total) + " homes are available."
}

func plansList() []plans.Plan { return plans.Plans }
