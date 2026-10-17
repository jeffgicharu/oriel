package web

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/floorplan"
	"github.com/jeffgicharu/oriel/internal/light"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
)

// Filter is the homes list's filters, read from and written to the address.
type Filter struct {
	Beds   int
	Max    int64
	Floors string
	Side   string
	All    bool
	Sort   string
}

// ParseFilter reads the filters from a query string.
func ParseFilter(q url.Values) Filter {
	f := Filter{Sort: "price"}
	if n, err := strconv.Atoi(q.Get("beds")); err == nil && n >= 1 && n <= 4 {
		f.Beds = n
	}
	if n, err := strconv.ParseInt(q.Get("max"), 10, 64); err == nil && n > 0 {
		f.Max = n
	}
	switch q.Get("floors") {
	case "lower", "middle", "upper":
		f.Floors = q.Get("floors")
	}
	switch q.Get("side") {
	case "river", "garden":
		f.Side = q.Get("side")
	}
	f.All = q.Get("show") == "all"
	switch q.Get("sort") {
	case "price-desc", "floor", "size":
		f.Sort = q.Get("sort")
	}
	return f
}

// Query writes the filters back as a query string, leaving out defaults.
func (f Filter) Query() string {
	q := url.Values{}
	if f.Beds > 0 {
		q.Set("beds", strconv.Itoa(f.Beds))
	}
	if f.Max > 0 {
		q.Set("max", strconv.FormatInt(f.Max, 10))
	}
	if f.Floors != "" {
		q.Set("floors", f.Floors)
	}
	if f.Side != "" {
		q.Set("side", f.Side)
	}
	if f.All {
		q.Set("show", "all")
	}
	if f.Sort != "price" && f.Sort != "" {
		q.Set("sort", f.Sort)
	}
	return q.Encode()
}

// Active reports whether any filter narrows the list.
func (f Filter) Active() bool {
	return f.Beds > 0 || f.Max > 0 || f.Floors != "" || f.Side != "" || f.All
}

// Bedrooms is the number of bedrooms a home of this type has.
func Bedrooms(code string) int {
	return building.Types[code].Bedrooms
}

// Matches reports whether a home passes the filters.
func (f Filter) Matches(h store.Home) bool {
	if !f.All && h.Status != store.Available {
		return false
	}
	if f.Beds > 0 && Bedrooms(h.Type.Code) != f.Beds {
		return false
	}
	if f.Max > 0 && h.Price > f.Max {
		return false
	}
	switch f.Floors {
	case "lower":
		if h.Level > 6 {
			return false
		}
	case "middle":
		if h.Level < 7 || h.Level > 11 {
			return false
		}
	case "upper":
		if h.Level < 12 {
			return false
		}
	}
	switch f.Side {
	case "river":
		if !h.Position.RiverView {
			return false
		}
	case "garden":
		if h.Position.RiverView {
			return false
		}
	}
	return true
}

// PriceSteps are the choices for the highest price.
var PriceSteps = []int64{12_000_000, 15_000_000, 20_000_000, 25_000_000, 30_000_000, 40_000_000, 60_000_000}

// ResidencesData is everything the homes list shows.
type ResidencesData struct {
	Types     []store.TypeSummary
	Homes     []store.Home
	Filter    Filter
	Available int
	Total     int
	From      int64
}

// UnitData is everything a home's page shows.
type UnitData struct {
	Home      store.Home
	Plan      floorplan.Plan
	Light     light.Study
	Schedule  plans.Schedule
	PlanCode  string
	Similar   []store.Home
	Shortlist bool
}

// statusClass is the CSS modifier for a status.
func statusClass(status string) string {
	return "status status--" + status
}

// PlanFor is the drawing of a home as it really sits: mirrored where it is
// handed, with north the right way.
func PlanFor(u building.Unit) floorplan.Plan {
	p, _ := floorplan.For(u.Type.Code)
	if u.Position.Handed {
		p = p.Mirrored()
	}
	return p
}

// NorthFor is the direction of north on a home's plan drawing.
func NorthFor(u building.Unit) float64 {
	return float64(u.Position.PlanRotation)
}

func planSVG(p floorplan.Plan, opt floorplan.Options) templ.Component {
	return templ.Raw(p.SVG(opt))
}

// HomeTitle is a home's headline: "Two bedrooms on level 7".
func HomeTitle(h store.Home) string {
	if h.Type.Code == "P" {
		return "Penthouse on the top floor"
	}
	return fmt.Sprintf("%s on level %d", h.Type.Name, h.Level)
}

// HomePath is the address of a home's page.
func HomePath(id string) string { return "/residences/" + id }

// Hours writes hours of sun to one decimal: "5.2".
func Hours(h float64) string {
	return strconv.FormatFloat(math.Round(h*10)/10, 'f', 1, 64)
}

// MonthLetter is a month's initial, for chart axes.
func MonthLetter(m int) string {
	return string("JFMAMJJASOND"[m-1])
}

// MonthName is a month written out.
func MonthName(m int) string {
	return []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}[m-1]
}

// Ordinal writes a level as "7th".
func Ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

// lightChart lays out the light study's bars.
type lightBar struct {
	Month                int
	X, W                 float64
	MorningY, MorningH   float64
	AfternoonY, AfterH   float64
	Total, Morning, Aftn float64
	Label                bool
}

const (
	chartW    = 640.0
	chartH    = 210.0
	chartTop  = 18.0
	chartBase = 180.0
	chartLeft = 34.0
)

func lightBars(s light.Study) ([]lightBar, float64) {
	max := 8.0
	for _, m := range s.Months {
		if m.Hours() > max {
			max = math.Ceil(m.Hours()/2) * 2
		}
	}
	scale := (chartBase - chartTop) / max
	slot := (chartW - chartLeft - 8) / 12
	var bars []lightBar
	for i, m := range s.Months {
		b := lightBar{
			Month:   i + 1,
			X:       chartLeft + float64(i)*slot + slot*0.2,
			W:       slot * 0.6,
			Total:   m.Hours(),
			Morning: m.Morning,
			Aftn:    m.Afternoon,
			Label:   m.Month == s.Best || m.Month == s.Worst,
		}
		b.MorningH = m.Morning * scale
		b.MorningY = chartBase - b.MorningH
		b.AfterH = m.Afternoon * scale
		// A 2 px gap separates the two parts of a bar.
		b.AfternoonY = b.MorningY - b.AfterH - 2
		if m.Morning == 0 {
			b.AfternoonY = chartBase - b.AfterH
		}
		bars = append(bars, b)
	}
	return bars, max
}

func gridLines(max float64) []float64 {
	var out []float64
	for v := 0.0; v <= max; v += 2 {
		out = append(out, v)
	}
	return out
}

func chartY(v, max float64) float64 {
	return chartBase - v*(chartBase-chartTop)/max
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// barPath draws a bar with 4 px rounded ends at the top only.
func barPath(x, y, w, h float64, roundTop bool) string {
	if h <= 0 {
		return ""
	}
	r := math.Min(4, math.Min(w/2, h))
	if !roundTop {
		return fmt.Sprintf("M%s %sh%sv%sh-%sz", f1(x), f1(y), f1(w), f1(h), f1(w))
	}
	return fmt.Sprintf("M%s %sv-%sq0 -%s %s -%sh%sq%s 0 %s %sv%sz",
		f1(x), f1(y+h), f1(h-r), f1(r), f1(r), f1(r), f1(w-2*r), f1(r), f1(r), f1(r), f1(h-r))
}

// LightSummary is the light study in one sentence.
func LightSummary(s light.Study) string {
	return fmt.Sprintf("About %s hours of direct sun on an average day, most in %s and least in %s.", Hours(s.AverageDay), MonthName(int(s.Best)), MonthName(int(s.Worst)))
}

// MorningOrEvening says when the home's sun comes.
func MorningOrEvening(s light.Study) string {
	switch {
	case s.MorningShare >= 0.62:
		return "Mostly morning light"
	case s.MorningShare <= 0.38:
		return "Mostly afternoon and evening light"
	default:
		return "Light through the morning and the afternoon"
	}
}

func joinComma(parts []string) string { return strings.Join(parts, ", ") }

func lightFor(id string) (light.Study, bool) { return light.For(id) }

func bedroomsText(h store.Home) string {
	t := h.Type
	s := strconv.Itoa(t.Bedrooms)
	if t.Study {
		s += " and a study"
	}
	if t.DSQ {
		s += ", with DSQ"
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func riverNote(h store.Home) string {
	if h.Position.RiverView {
		return ", on the river side"
	}
	return ", on the garden side"
}

func percent(bp int64) string {
	whole, frac := bp/100, bp%100
	switch {
	case frac == 0:
		return strconv.FormatInt(whole, 10) + "%"
	case frac%10 == 0:
		return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(frac/10, 10) + "%"
	default:
		return fmt.Sprintf("%d.%02d%%", whole, frac)
	}
}

func minF(a, b float64) float64 { return math.Min(a, b) }
