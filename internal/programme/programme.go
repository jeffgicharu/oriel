// Package programme is Oriel's construction programme: when each stage of
// the building happens, how far the work has gone on a day, and the
// photographs from the site.
//
// The dates are written as they stood on 18 October 2026, and they move on
// with the calendar a day each day, as the sample's sales story does. So the
// building always stands where it stood then: the frame at level 7, seven
// months after the sales launch, and completion a little over two years
// away. The pages, the payment schedules and the documents all read their
// dates from here, so they always agree.
package programme

import (
	"fmt"
	"math"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
)

var nairobi = func() *time.Location {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}()

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, nairobi) }

// written is the day the dates below stand as of.
var written = day(2026, 10, 18)

var (
	groundBroken  = day(2026, 2, 9)
	frameStarts   = day(2026, 6, 15)  // the ground-floor slab is poured
	toppingOut    = day(2027, 4, 20)  // the roof slab is poured
	closedIn      = day(2027, 11, 30) // the last of the façade is on
	showFlatsOpen = day(2027, 9, 15)  // viewings move from the sales suite into the building
	completion    = day(2028, 12, 15)
)

// Milestone is a step in the programme.
type Milestone struct {
	Start, End time.Time // End is zero for a single day
	Title      string
	Note       string
}

var milestones = []Milestone{
	{Start: day(2025, 11, 20), Title: "Approvals granted", Note: "Planning and environmental approvals for seventeen storeys and 86 homes."},
	{Start: day(2026, 1, 12), Title: "Site hoarded and cleared", Note: "The plot fenced, trees marked for keeping, and the old structures taken down."},
	{Start: groundBroken, Title: "Ground broken", Note: "Work starts on the foundations."},
	{Start: day(2026, 3, 2), End: day(2026, 5, 29), Title: "Piling and the basement dig", Note: "Bored piles, then two basement levels dug for parking and plant."},
	{Start: frameStarts, Title: "Ground-floor slab poured", Note: "The basement is closed in and the frame can start to rise."},
	{Start: frameStarts, End: toppingOut, Title: "The frame rises", Note: "A floor about every eighteen days: columns, then the slab above."},
	{Start: day(2027, 2, 1), End: closedIn, Title: "Façade, bays and windows", Note: "The bays go on from the bottom up, closing in each floor as the frame moves ahead."},
	{Start: toppingOut, Title: "Topping out", Note: "The roof slab poured, seventeen storeys up."},
	{Start: day(2027, 6, 1), End: day(2028, 9, 30), Title: "Services and fit-out", Note: "Power, water and air, then kitchens, bathrooms, floors and joinery."},
	{Start: showFlatsOpen, Title: "Show apartments open in the building", Note: "Viewings move from the sales suite to finished homes on level 3."},
	{Start: day(2028, 8, 1), End: day(2028, 10, 31), Title: "Lifts, pool and gardens", Note: "Commissioning and testing, and the planting of the gardens."},
	{Start: completion, Title: "Completion", Note: "Handover of the first homes begins."},
}

// Photo is a dated picture of the site.
type Photo struct {
	Image   string
	Taken   time.Time
	Caption string
}

// photos are the progress pictures, oldest first.
var photos = []Photo{
	{Image: "construction-1", Taken: day(2026, 6, 26), Caption: "The ground-floor slab, with the basement closed in below."},
	{Image: "construction-2", Taken: day(2026, 8, 21), Caption: "The frame at level three, from the street."},
	{Image: "construction-3", Taken: day(2026, 10, 9), Caption: "The frame passing level six, with the deck for level seven being set."},
}

// moved is how many days the calendar has moved on since the dates were
// written, on the day of now in Nairobi.
func moved(now time.Time) int {
	n := now.In(nairobi)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, nairobi)
	return int(math.Round(today.Sub(written).Hours() / 24))
}

func move(t time.Time, days int) time.Time {
	if t.IsZero() {
		return t
	}
	return t.AddDate(0, 0, days)
}

// Completion is when the building is due to be handed over, as the
// programme stands on the day of now.
func Completion(now time.Time) time.Time { return move(completion, moved(now)) }

// ShowFlatsOpen is when viewings move from the sales suite into the
// building, as the programme stands on the day of now.
func ShowFlatsOpen(now time.Time) time.Time { return move(showFlatsOpen, moved(now)) }

// Status says where a milestone stands at a time: done, now or next.
func (m Milestone) Status(now time.Time) string {
	end := m.End
	if end.IsZero() {
		end = m.Start
	}
	switch {
	case !now.Before(end.Add(24 * time.Hour)):
		return "done"
	case !now.Before(m.Start):
		return "now"
	}
	return "next"
}

// When is how a milestone's date is written.
func (m Milestone) When() string {
	if m.End.IsZero() {
		return m.Start.Format("2 Jan 2006")
	}
	if m.Start.Year() == m.End.Year() {
		return m.Start.Format("Jan") + " to " + m.End.Format("Jan 2006")
	}
	return m.Start.Format("Jan 2006") + " to " + m.End.Format("Jan 2006")
}

// Storeys is how many storeys the frame has, the roof counting as the last.
const Storeys = building.TopLevel + 2

// frameReached is how far the frame has climbed at a time in the calendar
// the dates are written in: 0 before the ground slab, up to Storeys once the
// roof is poured, with the fraction of the floor under way.
func frameReached(t time.Time) float64 {
	switch {
	case t.Before(frameStarts):
		return 0
	case !t.Before(toppingOut):
		return Storeys
	}
	return Storeys * t.Sub(frameStarts).Hours() / toppingOut.Sub(frameStarts).Hours()
}

// Progress is where the work stands on a day.
type Progress struct {
	Now        time.Time
	Percent    int
	Frame      float64
	Current    string // what is happening, in a few words
	Complete   bool
	Completion time.Time
	Milestones []Milestone // the programme, in order
	Photos     []Photo     // the pictures taken by now, newest first
}

// On works out the state of the site at now.
func On(now time.Time) Progress {
	days := moved(now)
	// then is now in the calendar the dates are written in.
	then := now.AddDate(0, 0, -days)
	p := Progress{Now: now, Frame: frameReached(then), Completion: move(completion, days)}
	total := completion.Sub(groundBroken).Hours()
	done := math.Max(0, math.Min(1, then.Sub(groundBroken).Hours()/total))
	p.Percent = int(math.Round(done * 100))
	switch {
	case !then.Before(completion):
		p.Current, p.Complete = "Complete", true
	case then.Before(groundBroken):
		p.Current = "Preparing the site"
	case then.Before(frameStarts):
		p.Current = "Foundations and basement"
	case then.Before(toppingOut):
		p.Current = fmt.Sprintf("Frame at %s", FrameLevelName(p.Frame))
	case then.Before(closedIn):
		p.Current = "Façade and fit-out"
	default:
		p.Current = "Fit-out and commissioning"
	}
	for _, m := range milestones {
		m.Start, m.End = move(m.Start, days), move(m.End, days)
		p.Milestones = append(p.Milestones, m)
	}
	for i := len(photos) - 1; i >= 0; i-- {
		if ph := photos[i]; !ph.Taken.After(then) {
			ph.Taken = move(ph.Taken, days)
			p.Photos = append(p.Photos, ph)
		}
	}
	return p
}

// FrameLevelName names the floor being built.
func FrameLevelName(frame float64) string {
	level := int(frame)
	if level > building.TopLevel {
		return "the roof"
	}
	if level == 0 {
		return "the ground floor"
	}
	return "level " + fmt.Sprint(level)
}
