package web

import (
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
)

// Form carries what a visitor typed and what was wrong with it, so a form can
// be shown again without losing anything.
type Form struct {
	Values map[string]string
	Errors map[string]string
}

// Value is what the visitor typed in a field.
func (f Form) Value(name string) string { return f.Values[name] }

// Error is what is wrong with a field.
func (f Form) Error(name string) string { return f.Errors[name] }

// HasErrors reports whether anything needs fixing.
func (f Form) HasErrors() bool { return len(f.Errors) > 0 }

// ViewingData is the booking page.
type ViewingData struct {
	Form     Form
	Days     []store.Day
	Day      store.Day
	DayValue string
	Home     *store.Home
	Notice   string
}

// ViewingDone is the page a booked visitor keeps.
type ViewingDone struct {
	Viewing store.Viewing
	Home    *store.Home
	Fresh   bool
}

// ReserveData is the reservation request page.
type ReserveData struct {
	Form     Form
	Home     store.Home
	Plan     string
	Previews []plans.Schedule
	Similar  []store.Home
}

// ReservationData is a buyer's reservation page.
type ReservationData struct {
	Reservation store.Reservation
	Home        store.Home
	Schedule    plans.Schedule
	Plan        plans.Plan
	Fresh       bool
	Now         time.Time
}

// EnquireData is the question form.
type EnquireData struct {
	Form Form
	Sent string
	// Home is the home the question is about, when it was asked from its page.
	Home *store.Home
}

// Interests are the choices for what a question is about.
func (d EnquireData) Interests() []string {
	out := []string{}
	if d.Home != nil {
		out = append(out, "Home "+d.Home.ID)
	}
	out = append(out, "Not sure yet")
	for _, code := range []string{"A", "B", "C", "D", "P"} {
		out = append(out, building.Types[code].Name)
	}
	return out
}

// errorAnchor is where an error in the summary links to.
func errorAnchor(name string) string {
	switch name {
	case "slot":
		return "#slots"
	case "plan":
		return "#plans"
	}
	return "#f-" + name
}

// DateValue is how a day is written in a form: 2026-10-20.
func DateValue(t time.Time) string { return t.In(Nairobi).Format("2006-01-02") }

// SlotValue is how a slot is written in a form: 2026-10-20T14:45.
func SlotValue(t time.Time) string { return t.In(Nairobi).Format("2006-01-02T15:04") }

func dayName(t time.Time) string    { return t.In(Nairobi).Format("Mon") }
func dayNumber(t time.Time) string  { return t.In(Nairobi).Format("2") }
func monthShort(t time.Time) string { return t.In(Nairobi).Format("Jan") }
func clock(t time.Time) string      { return t.In(Nairobi).Format("15:04") }

// dayNote says why a day can't be chosen.
func dayNote(d store.Day) string {
	switch {
	case !d.Open:
		return d.Reason
	case d.Free() == 0:
		return "Fully booked"
	}
	return ""
}

// DayNote is a closure worth pointing out under the days: a holiday, a day
// the desk has blocked, or a day already full. The weekly day off is not
// listed; the opening hours say it.
type DayNote struct{ When, Text string }

func dayNotes(days []store.Day) []DayNote {
	var out []DayNote
	for _, d := range days {
		if d.Blocked || (d.Open && d.Free() == 0) {
			out = append(out, DayNote{When: d.Date.In(Nairobi).Format("Mon 2 Jan"), Text: dayNote(d)})
		}
	}
	return out
}

// LongDate is "Tuesday 21 October 2026".
func LongDate(t time.Time) string { return t.In(Nairobi).Format("Monday 2 January 2006") }

// LongDateTime is "Tuesday 21 October, 14:45".
func LongDateTime(t time.Time) string { return t.In(Nairobi).Format("Monday 2 January, 15:04") }

func firstName(full string) string {
	for i, r := range full {
		if r == ' ' {
			return full[:i]
		}
	}
	return full
}

func placesLeft(n int) string {
	switch n {
	case 0:
		return "Full"
	case 1:
		return "1 place"
	default:
		return itoa(n) + " places"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// HoursUntil writes the time left on a hold: "in 71 hours".
func HoursUntil(from, to time.Time) string {
	d := to.Sub(from)
	if d <= 0 {
		return "now"
	}
	h := int(d.Hours() + 0.5)
	if h >= 2 {
		return "in " + itoa(h) + " hours"
	}
	m := int(d.Minutes())
	if m <= 1 {
		return "in a minute"
	}
	return "in " + itoa(m) + " minutes"
}

// ReservationSteps are the stages a reservation passes through.
type ReservationStep struct {
	Title string
	When  string
	State string // done, current, todo, stopped
	Note  string
}

// StepsFor lays out a reservation's progress.
func StepsFor(r store.Reservation, now time.Time) []ReservationStep {
	steps := []ReservationStep{
		{Title: "Requested", When: LongDateTime(r.RequestedAt), State: "done", Note: "Home held while the sales team checks your request."},
		{Title: "Letter of offer", State: "todo", Note: "We confirm the home, the price and your payment plan."},
		{Title: "Deposit", State: "todo", Note: "Pay 10% by bank transfer within 7 days to secure the home."},
	}
	switch r.Status {
	case "requested":
		steps[1].State = "current"
		steps[1].Note = "Held until " + LongDateTime(r.HoldExpiresAt) + ", " + HoursUntil(now, r.HoldExpiresAt) + "."
	case "confirmed":
		steps[1].State, steps[1].When = "done", LongDateTime(r.DecidedAt)
		steps[1].Note = "Letter of offer " + r.OfferNumber + " issued."
		steps[2].State = "current"
	case "completed":
		steps[1].State, steps[1].When = "done", LongDateTime(r.DecidedAt)
		steps[1].Note = "Letter of offer " + r.OfferNumber + " issued."
		steps[2].State, steps[2].When = "done", LongDateTime(r.DepositAt)
		steps[2].Note = "Deposit of " + KES(r.DepositAmount) + " received. The home is yours."
	case "declined":
		steps[1].State, steps[1].When = "stopped", LongDateTime(r.DecidedAt)
		steps[1].Note = "Declined: " + r.DeclineReason
	case "lapsed":
		steps[1].State, steps[1].When = "stopped", LongDateTime(r.HoldExpiresAt)
		steps[1].Note = "The hold lapsed before it was confirmed, and the home was released."
	}
	return steps
}
