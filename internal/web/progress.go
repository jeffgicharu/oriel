package web

import (
	"fmt"
	"math"
	"time"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/plans"
)

// The construction programme. Dates are fixed; what is done, under way or
// still to come is worked out from today, so the page stays true as the
// months pass.

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, Nairobi) }

// ShowFlatsOpen is when viewings move from the sales suite into the building.
var ShowFlatsOpen = day(2027, 9, 15)

var (
	groundBroken = day(2026, 2, 9)
	frameStarts  = day(2026, 6, 15) // the ground-floor slab is poured
	toppingOut   = day(2027, 4, 20) // the roof slab is poured
)

// Milestone is a step in the programme.
type Milestone struct {
	Start, End time.Time // End is zero for a single day
	Title      string
	Note       string
}

// Milestones are the programme, in order.
var Milestones = []Milestone{
	{Start: day(2025, 11, 20), Title: "Approvals granted", Note: "Planning and environmental approvals for seventeen storeys and 86 homes."},
	{Start: day(2026, 1, 12), Title: "Site hoarded and cleared", Note: "The plot fenced, trees marked for keeping, and the old structures taken down."},
	{Start: groundBroken, Title: "Ground broken", Note: "Work starts on the foundations."},
	{Start: day(2026, 3, 2), End: day(2026, 5, 29), Title: "Piling and the basement dig", Note: "Bored piles, then two basement levels dug for parking and plant."},
	{Start: frameStarts, Title: "Ground-floor slab poured", Note: "The basement is closed in and the frame can start to rise."},
	{Start: frameStarts, End: toppingOut, Title: "The frame rises", Note: "A floor about every eighteen days: columns, then the slab above."},
	{Start: day(2027, 2, 1), End: day(2027, 11, 30), Title: "Façade, bays and windows", Note: "The bays go on from the bottom up, closing in each floor as the frame moves ahead."},
	{Start: toppingOut, Title: "Topping out", Note: "The roof slab poured, seventeen storeys up."},
	{Start: day(2027, 6, 1), End: day(2028, 9, 30), Title: "Services and fit-out", Note: "Power, water and air, then kitchens, bathrooms, floors and joinery."},
	{Start: ShowFlatsOpen, Title: "Show apartments open in the building", Note: "Viewings move from the sales suite to finished homes on level 3."},
	{Start: day(2028, 8, 1), End: day(2028, 10, 31), Title: "Lifts, pool and gardens", Note: "Commissioning and testing, and the planting of the gardens."},
	{Start: plans.Completion, Title: "Completion", Note: "Handover of the first homes begins."},
}

// Status says where a milestone stands on a day: done, now or next.
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

// storeys is how many storeys the frame has, the roof counting as the last.
var storeys = building.TopLevel + 2

// FrameReached is how far the frame has climbed: 0 before the ground slab,
// up to storeys once the roof is poured, with the fraction of the floor under way.
func FrameReached(now time.Time) float64 {
	switch {
	case now.Before(frameStarts):
		return 0
	case !now.Before(toppingOut):
		return float64(storeys)
	}
	return float64(storeys) * now.Sub(frameStarts).Hours() / toppingOut.Sub(frameStarts).Hours()
}

// Progress is where the work stands on a day.
type Progress struct {
	Now      time.Time
	Percent  int
	Frame    float64
	Current  string // what is happening, in a few words
	Complete bool
}

// ProgressOn works out the state of the site on a day.
func ProgressOn(now time.Time) Progress {
	p := Progress{Now: now, Frame: FrameReached(now)}
	total := plans.Completion.Sub(groundBroken).Hours()
	done := math.Max(0, math.Min(1, now.Sub(groundBroken).Hours()/total))
	p.Percent = int(math.Round(done * 100))
	switch {
	case !now.Before(plans.Completion):
		p.Current, p.Complete = "Complete", true
	case now.Before(groundBroken):
		p.Current = "Preparing the site"
	case now.Before(frameStarts):
		p.Current = "Foundations and basement"
	case now.Before(toppingOut):
		p.Current = fmt.Sprintf("Frame at %s", FrameLevelName(p.Frame))
	case now.Before(day(2027, 11, 30)):
		p.Current = "Façade and fit-out"
	default:
		p.Current = "Fit-out and commissioning"
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

// SitePhoto is a dated picture of the site.
type SitePhoto struct {
	Image   string
	Taken   time.Time
	Caption string
}

// SitePhotos are the progress pictures, newest last; only those already
// taken are shown.
var SitePhotos = []SitePhoto{
	{Image: "construction-1", Taken: day(2026, 6, 26), Caption: "The ground-floor slab, with the basement closed in below."},
	{Image: "construction-2", Taken: day(2026, 8, 21), Caption: "The frame at level three, from the street."},
	{Image: "construction-3", Taken: day(2026, 10, 9), Caption: "The frame passing level six, with the deck for level seven being set."},
}

// PhotosBy lists the pictures taken by a day, newest first.
func PhotosBy(now time.Time) []SitePhoto {
	var out []SitePhoto
	for i := len(SitePhotos) - 1; i >= 0; i-- {
		if !SitePhotos[i].Taken.After(now) {
			out = append(out, SitePhotos[i])
		}
	}
	return out
}

// ProgressDrawing is the south elevation drawn as the frame stands: storeys
// built solid, the one under way hatched, the rest in dashed outline, and the
// crane above.
func ProgressDrawing(frame float64) templ.Component {
	const (
		k     = 6.0 // pixels a metre
		left  = 70.0
		width = building.PlateWidth * k
	)
	top := building.RoofElevation() + 1.2
	H := top*k + 70
	ground := H - 40
	y := func(h float64) float64 { return ground - h*k }
	var p pen
	p.f(`<svg class="progress-drawing" viewBox="0 0 %s %s" role="img" aria-labelledby="pd-title" xmlns="http://www.w3.org/2000/svg">`, n1(left+width+120), n1(H))
	p.f(`<title id="pd-title">The frame today: %s</title>`, FrameLevelName(frame))
	p.f(`<defs><pattern id="pd-hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><line x1="0" y1="0" x2="0" y2="6"/></pattern></defs>`)
	p.f(`<line class="pd-ground" x1="10" y1="%s" x2="%s" y2="%s"/>`, n1(ground), n1(left+width+110), n1(ground))
	built := int(frame)
	for level := 0; level <= building.TopLevel+1; level++ {
		y0 := building.LevelElevation(level)
		y1 := building.RoofElevation() + 1.2
		if level <= building.TopLevel {
			y1 = building.LevelElevation(level) + map[bool]float64{true: building.GroundHeight, false: building.FloorHeight}[level == 0]
			if level == 1 {
				y1 = building.LevelElevation(1) + building.AmenityHeight
			}
		} else {
			y0 = building.RoofElevation()
		}
		inset := 0.0
		if level == building.TopLevel || level == building.TopLevel+1 {
			inset = building.PenthouseSetback * k
		}
		class := "pd-future"
		switch {
		case level < built:
			class = "pd-built"
		case level == built && frame < float64(storeys):
			class = "pd-now"
		}
		p.f(`<rect class="%s" x="%s" y="%s" width="%s" height="%s"/>`, class, n1(left+inset), n1(y(y1)), n1(width-2*inset), n1((y1-y0)*k))
		if level <= building.TopLevel && level%2 == 0 {
			p.f(`<text class="pd-level" x="%s" y="%s" text-anchor="end" dominant-baseline="middle">%s</text>`, n1(left-10), n1(y((y0+y1)/2)), building.LevelName(level))
		}
	}
	// The tower crane stands in the lift core and climbs with the frame.
	if frame > 0 && frame < float64(storeys) {
		reach := building.LevelElevation(int(math.Min(frame, float64(building.TopLevel)))) + 14
		cx := left + width*0.6
		jib := y(reach - 2)
		p.f(`<g class="pd-crane"><line x1="%s" y1="%s" x2="%s" y2="%s"/><line x1="%s" y1="%s" x2="%s" y2="%s"/><line x1="%s" y1="%s" x2="%s" y2="%s"/><line x1="%s" y1="%s" x2="%s" y2="%s"/><line class="pd-hook" x1="%s" y1="%s" x2="%s" y2="%s"/></g>`,
			n1(cx), n1(ground), n1(cx), n1(y(reach+4)),
			n1(cx-150), n1(jib), n1(cx+60), n1(jib),
			n1(cx), n1(y(reach+4)), n1(cx-150), n1(jib),
			n1(cx), n1(y(reach+4)), n1(cx+60), n1(jib),
			n1(cx-100), n1(jib), n1(cx-100), n1(jib+40))
	}
	// Today's marker.
	if frame > 0 && frame < float64(storeys) {
		ty := y(building.LevelElevation(int(frame)) + 1.6)
		p.f(`<g class="pd-today"><line x1="%s" y1="%s" x2="%s" y2="%s"/><text x="%s" y="%s" dominant-baseline="middle">Today</text></g>`,
			n1(left+width+4), n1(ty), n1(left+width+26), n1(ty), n1(left+width+30), n1(ty))
	}
	p.f(`</svg>`)
	return templ.Raw(p.String())
}

func progressTitle(g Progress) string {
	switch {
	case g.Complete:
		return "Complete."
	case g.Frame > 0 && g.Frame < float64(storeys):
		return "Rising on Riverside."
	case g.Frame >= float64(storeys):
		return "Topped out."
	}
	return "Breaking ground."
}

func progressWidth(percent int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("width: %d%%", percent))
}

func sitePhotoSizes(i int) string {
	if i == 0 {
		return "(min-width: 1520px) 1408px, calc(100vw - 2 * var(--gutter))"
	}
	return "(min-width: 1000px) 50vw, 100vw"
}
