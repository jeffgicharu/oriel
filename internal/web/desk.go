package web

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
)

// DeskPage is passed to every page of the sales desk.
type DeskPage struct {
	Page
	Staff   store.Staff
	Nav     string
	Waiting int    // reservation requests waiting, shown on the menu
	Notice  string // what the last action did
}

// DeskNavItem is an entry in the desk's menu.
type DeskNavItem struct {
	Key, Label, Href string
	ManagerOnly      bool
}

// DeskNav is the desk's menu.
var DeskNav = []DeskNavItem{
	{Key: "dashboard", Label: "Dashboard", Href: "/desk"},
	{Key: "stack", Label: "Stacking plan", Href: "/desk/stack"},
	{Key: "leads", Label: "Leads", Href: "/desk/leads"},
	{Key: "viewings", Label: "Viewings", Href: "/desk/viewings"},
	{Key: "reservations", Label: "Reservations", Href: "/desk/reservations"},
	{Key: "pricing", Label: "Pricing", Href: "/desk/pricing", ManagerOnly: true},
	{Key: "activity", Label: "Activity", Href: "/desk/activity"},
}

// DeskTitle is the browser tab's text on the desk.
func (d DeskPage) DeskTitle() string {
	if d.Meta.Title == "" {
		return "Sales desk · Oriel"
	}
	return d.Meta.Title + " · Oriel sales desk"
}

// When writes a time on the desk: "Today 14:05", "Yesterday 09:30", "Tue 21 Oct 10:00".
func When(t, now time.Time) string {
	t, now = t.In(Nairobi), now.In(Nairobi)
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, Nairobi) }
	switch diff := int(day(now).Sub(day(t)).Hours() / 24); {
	case diff == 0:
		return "Today " + t.Format("15:04")
	case diff == 1:
		return "Yesterday " + t.Format("15:04")
	case diff == -1:
		return "Tomorrow " + t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("Mon 2 Jan 15:04")
	}
	return t.Format("2 Jan 2006")
}

// Ago writes how long ago something was: "3 days", "5 hours".
func Ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m <= 1 {
			return "a minute"
		}
		return strconv.Itoa(m) + " minutes"
	case d < 48*time.Hour:
		return strconv.Itoa(int(d.Hours())) + " hours"
	}
	return strconv.Itoa(int(d.Hours()/24)) + " days"
}

// DayLabel writes a calendar date from the database: "Tue 21 Oct".
func DayLabel(day string) string {
	t, err := time.ParseInLocation("2006-01-02", day, Nairobi)
	if err != nil {
		return day
	}
	return t.Format("Mon 2 Jan")
}

// ---------- Charts, drawn in SVG from the desk's figures ----------

// StatusBar is every home as one bar, divided by status.
func StatusBar(counts map[string]int, total int) templ.Component {
	var p pen
	const w, h = 1000.0, 44.0
	p.f(`<svg class="chart chart--status" viewBox="0 0 %s %s" preserveAspectRatio="none" role="img" aria-label="%s" xmlns="http://www.w3.org/2000/svg">`, n1(w), n1(h), statusSummary(counts, total))
	p.f(`<defs><pattern id="cs-held" width="7" height="7" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><line x1="0" y1="0" x2="0" y2="7"/></pattern>`)
	p.f(`<pattern id="cs-reserved" width="7" height="7" patternUnits="userSpaceOnUse"><path d="M0 0L7 7M7 0L0 7"/></pattern></defs>`)
	x := 0.0
	for _, st := range []string{store.Sold, store.Reserved, store.Held, store.Available, store.Withheld} {
		n := counts[st]
		if n == 0 || total == 0 {
			continue
		}
		bw := w * float64(n) / float64(total)
		p.f(`<rect class="cs-%s" x="%s" y="0" width="%s" height="%s"><title>%s: %d</title></rect>`, st, n1(x), n1(math.Max(bw-2, 1)), n1(h), store.StatusName(st), n)
		x += bw
	}
	p.f(`</svg>`)
	return templ.Raw(p.String())
}

func statusSummary(counts map[string]int, total int) string {
	var parts []string
	for _, st := range []string{store.Sold, store.Reserved, store.Held, store.Available, store.Withheld} {
		if counts[st] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[st], strings.ToLower(store.StatusName(st))))
		}
	}
	return fmt.Sprintf("%d homes: %s", total, strings.Join(parts, ", "))
}

// MonthBars is sales by month, the latest month drawn solid.
func MonthBars(months []store.Count) templ.Component {
	var p pen
	const w, h, top, base = 560.0, 220.0, 26.0, 190.0
	max := 1
	for _, m := range months {
		if m.N > max {
			max = m.N
		}
	}
	step := w / float64(len(months))
	bar := step * 0.56
	var labels []string
	for _, m := range months {
		labels = append(labels, fmt.Sprintf("%s %d", m.Label, m.N))
	}
	p.f(`<svg class="chart chart--months" viewBox="0 0 %s %s" role="img" aria-label="Homes sold each month: %s" xmlns="http://www.w3.org/2000/svg">`, n1(w), n1(h), strings.Join(labels, ", "))
	p.f(`<line class="chart__base" x1="0" y1="%s" x2="%s" y2="%s"/>`, n1(base), n1(w), n1(base))
	for i, m := range months {
		bh := (base - top) * float64(m.N) / float64(max)
		x := step*float64(i) + (step-bar)/2
		cls := "chart__bar"
		if i == len(months)-1 {
			cls += " is-current"
		}
		p.f(`<g><rect class="%s" x="%s" y="%s" width="%s" height="%s"><title>%s: %d sold, KES %s</title></rect>`, cls, n1(x), n1(base-bh), n1(bar), n1(math.Max(bh, 0)), m.Label, m.N, thousandsWeb(m.Value))
		p.f(`<text class="chart__value" x="%s" y="%s" text-anchor="middle">%d</text>`, n1(x+bar/2), n1(base-bh-8), m.N)
		p.f(`<text class="chart__label" x="%s" y="%s" text-anchor="middle">%s</text></g>`, n1(x+bar/2), n1(base+20), m.Label)
	}
	p.f(`</svg>`)
	return templ.Raw(p.String())
}

// HBars is a horizontal bar for each label, longest first.
func HBars(items []store.Count, unit string) templ.Component {
	var p pen
	const w, row, labelW = 560.0, 34.0, 150.0
	max := 1
	for _, it := range items {
		if it.N > max {
			max = it.N
		}
	}
	h := row * float64(len(items))
	var labels []string
	for _, it := range items {
		labels = append(labels, fmt.Sprintf("%s %d", it.Label, it.N))
	}
	p.f(`<svg class="chart chart--hbars" viewBox="0 0 %s %s" role="img" aria-label="%s" xmlns="http://www.w3.org/2000/svg">`, n1(w), n1(h), strings.Join(labels, ", "))
	for i, it := range items {
		y := row * float64(i)
		bw := (w - labelW - 50) * float64(it.N) / float64(max)
		p.f(`<g><text class="chart__label chart__label--left" x="0" y="%s" dominant-baseline="middle">%s</text>`, n1(y+row/2), templEscape(it.Label))
		p.f(`<rect class="chart__bar" x="%s" y="%s" width="%s" height="14"><title>%s: %d %s</title></rect>`, n1(labelW), n1(y+row/2-7), n1(math.Max(bw, 1)), templEscape(it.Label), it.N, unit)
		p.f(`<text class="chart__value" x="%s" y="%s" dominant-baseline="middle">%d</text></g>`, n1(labelW+bw+8), n1(y+row/2), it.N)
	}
	p.f(`</svg>`)
	return templ.Raw(p.String())
}

// FunnelRows are the pipeline's stages with the share that reached each.
type FunnelRow struct {
	Label   string
	N       int
	Reached int // leads that got at least this far
	Percent int // of all leads, that got this far
	Lost    bool
}

// Funnel works out how far leads have got. A lead at a later stage has
// passed through every earlier one; a lost lead is known only to have started.
func Funnel(stages []store.Count) []FunnelRow {
	total := 0
	for _, s := range stages {
		total += s.N
	}
	var out []FunnelRow
	open := []string{"new", "contacted", "viewing", "reservation", "won"}
	for i, key := range open {
		reached := 0
		n := 0
		for _, s := range stages {
			for _, later := range open[i:] {
				if s.Key == later {
					reached += s.N
				}
			}
			if s.Key == key {
				n = s.N
			}
		}
		// Every lead starts as new, the lost ones included.
		if i == 0 {
			reached = total
		}
		pct := 0
		if total > 0 {
			pct = int(math.Round(100 * float64(reached) / float64(total)))
		}
		out = append(out, FunnelRow{Label: store.StageTitle(key), N: n, Reached: reached, Percent: pct})
	}
	for _, s := range stages {
		if s.Key == "lost" {
			pct := 0
			if total > 0 {
				pct = int(math.Round(100 * float64(s.N) / float64(total)))
			}
			out = append(out, FunnelRow{Label: "Lost", N: s.N, Reached: s.N, Percent: pct, Lost: true})
		}
	}
	return out
}

func funnelWidth(pct int) templ.SafeCSS {
	if pct < 1 {
		pct = 1
	}
	return templ.SafeCSS(fmt.Sprintf("width: %d%%", pct))
}

func thousandsWeb(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func templEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// Trend writes a change against before: "+3 on the week before".
func Trend(now, before int, period string) string {
	switch d := now - before; {
	case d > 0:
		return fmt.Sprintf("%d more than the %s before", d, period)
	case d < 0:
		return fmt.Sprintf("%d fewer than the %s before", -d, period)
	}
	return "the same as the " + period + " before"
}

// Lapses says when a hold runs out: "lapses in 5 hours".
func Lapses(now, at time.Time) string {
	if !at.After(now) {
		return "lapsing now"
	}
	return "lapses " + HoursUntil(now, at)
}

func dotColour(c string) templ.SafeCSS {
	if c == "" || !strings.HasPrefix(c, "#") || len(c) > 7 {
		c = "#81878c"
	}
	return templ.SafeCSS("background-color: " + c)
}

func partOfDay(now time.Time) string {
	switch h := now.In(Nairobi).Hour(); {
	case h < 12:
		return "morning"
	case h < 17:
		return "afternoon"
	}
	return "evening"
}

// lastN is the count back months from the latest.
func lastN(months []store.Count, back int) int {
	i := len(months) - 1 - back
	if i < 0 || i >= len(months) {
		return 0
	}
	return months[i].N
}

func planName(code string) string {
	if p, ok := plans.ByCode(code); ok {
		return p.Name
	}
	return code
}

// priceField is a home's own price for its form, or nothing for the list price.
func priceField(h store.Home) string {
	if !h.Overridden {
		return ""
	}
	return thousandsWeb(h.Price)
}

// ParseShillings reads an amount typed with or without commas: "18,480,000".
func ParseShillings(s string) (int64, error) {
	clean := strings.NewReplacer(",", "", " ", "", "KES", "", "kes", "", "Ksh", "", "_", "").Replace(strings.TrimSpace(s))
	return strconv.ParseInt(clean, 10, 64)
}

func leadsLede(s store.Staff) string {
	if s.Manager() {
		return "Everyone the team is talking to, from the first enquiry to the keys."
	}
	return "Your leads, from the first enquiry to the keys."
}

func phoneLink(p string) string { return strings.ReplaceAll(p, " ", "") }

func buyingName(b string) string {
	if b == "investment" {
		return "To let"
	}
	return "To live in"
}

func viewingStatus(s string) string {
	switch s {
	case "booked":
		return "Booked"
	case "attended":
		return "Came"
	case "no_show":
		return "Did not come"
	case "cancelled":
		return "Cancelled"
	}
	return s
}

func reservationTab(status string) string {
	switch status {
	case "requested":
		return "waiting"
	case "confirmed", "completed":
		return status
	}
	return "closed"
}
