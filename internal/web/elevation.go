package web

import (
	"fmt"
	"sort"
	"strings"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/building"
)

// elevationLine is one stroke or shape of the tower's south elevation.
type elevationLine struct {
	D     string
	Class string
}

// The south elevation is drawn at ten units to the metre, with the ground at
// 640 and the plate's centre at 300.
const (
	elevScale  = 10.0
	elevGround = 640.0
	elevCentre = 300.0
)

func ex(x float64) float64 { return elevCentre + x*elevScale }
func ey(y float64) float64 { return elevGround - y*elevScale }

type elevPen struct {
	lines []elevationLine
}

func (p *elevPen) path(class, format string, args ...any) {
	p.lines = append(p.lines, elevationLine{D: fmt.Sprintf(format, args...), Class: class})
}

// rect draws a rectangle between two corners given in metres.
func (p *elevPen) rect(class string, x0, y0, x1, y1 float64) {
	p.path(class, "M%.1f %.1fH%.1fV%.1fH%.1fZ", ex(x0), ey(y0), ex(x1), ey(y1), ex(x0))
}

func (p *elevPen) hline(class string, x0, x1, y float64) {
	p.path(class, "M%.1f %.1fH%.1f", ex(x0), ey(y), ex(x1))
}

func (p *elevPen) vline(class string, x, y0, y1 float64) {
	p.path(class, "M%.1f %.1fV%.1f", ex(x), ey(y0), ey(y1))
}

// elevationLines draws Oriel's south face from the building's own geometry:
// the ground floor on its columns, the amenity floor, fourteen floors of
// homes with slab edges, glass, mullions, fins, bays and balconies, the
// penthouse floor set back behind its planters, the roof pergola, and trees
// and people for scale.
func elevationLines() []elevationLine {
	p := &elevPen{}
	x0, x1 := building.XMin, building.XMax

	// Ground.
	p.path("", "M%.1f %.1fH%.1f", ex(-29), elevGround, ex(29))
	p.path("faint", "M%.1f %.1fH%.1f", ex(-29), elevGround+6, ex(29))

	// Ground floor: columns in front, the lobby glass set back behind them.
	gTop := building.GroundHeight
	p.rect("faint", x0+2.4, 0, x1-2.4, gTop-0.05)
	for x := x0 + 2.4; x <= x1-2.4+0.01; x += (x1 - x0 - 4.8) / 8 {
		p.vline("faint", x, 0, gTop-0.05)
	}
	span := (x1 - x0 - 1.8) / 6
	for i := 0; i <= 6; i++ {
		x := x0 + 0.9 + float64(i)*span
		p.rect("", x-0.34, 0, x+0.34, gTop)
	}
	// Entrance canopy with its cobalt edge.
	p.rect("", -7, gTop-0.6, 7, gTop-0.32)
	p.rect("fill", -7, gTop-0.68, 7, gTop-0.6)
	p.vline("", -6.4, 0, gTop-0.68)
	p.vline("", 6.4, 0, gTop-0.68)

	// Level 1: the amenity floor behind its slab.
	l1 := building.LevelElevation(1)
	p.rect("", x0-0.45, l1, x1+0.45, l1+0.38)
	p.rect("", x0+0.3, l1+0.38, x1-0.3, l1+building.AmenityHeight)
	for x := x0 + 3; x < x1-1; x += 3 {
		p.vline("faint", x, l1+0.38, l1+building.AmenityHeight)
	}

	// Levels 2 to 15.
	for level := building.FirstHomeLevel; level <= building.LastTypical; level++ {
		y := building.LevelElevation(level)
		h := building.FloorHeight
		yg := y + 0.32
		top := y + h
		p.rect("", x0-0.45, y, x1+0.45, yg)
		p.rect("", x0+0.2, yg, x1-0.2, top)
		// Mullions every 1.5 m, and two strokes of reflection across the glass.
		for x := x0 + 1.5; x < x1-1.2; x += 1.5 {
			p.vline("faint", x, yg, top)
		}
		off := float64(level%3) * 4
		p.path("faint", "M%.1f %.1fl18 -22M%.1f %.1fl10 -12", ex(-13+off), ey(yg+0.2), ex(-10+off), ey(yg+0.2))
		// Fins on the east and west faces, seen edge-on.
		p.vline("", x0-0.6, yg, top)
		p.vline("", x1+0.6, yg, top)
		for _, u := range building.HomesOnLevel(level) {
			pos := u.Position
			hasSouth := false
			for _, f := range pos.Facing {
				if f == building.South {
					hasSouth = true
				}
			}
			bx0, bx1 := pos.Bay.Plan.X0, pos.Bay.Plan.X1
			seat := yg + building.BaySeatHeight
			head := yg + building.BayHeadHeight
			// The south-facing bays: the corner bays at each end hide the
			// north ones behind them, and the one-bedroom bay sits on the face.
			if hasSouth {
				p.rect("bay", bx0, yg, bx1, top-0.02)
				p.hline("bay", bx0, bx1, seat)
				p.rect("fill", bx0, head, bx1, top-0.02)
				p.vline("bay", (bx0+bx1)/2, seat, head)
			}
			// Balconies on the south face: slab edge and glass balustrade.
			if hasSouth && pos.Balcony.X1-pos.Balcony.X0 > 0.5 {
				k := pos.Balcony
				p.rect("", k.X0, y, k.X1, y+0.26)
				p.rect("glass", k.X0, y+0.26, k.X1, y+1.32)
				p.hline("", k.X0, k.X1, y+1.32)
			}
		}
	}

	// Level 16: the penthouses, set back behind planted terraces.
	ph := building.LevelElevation(building.PenthouseLevel)
	s := building.PenthouseSetback
	p.rect("", x0-0.45, ph, x1+0.45, ph+0.32)
	p.rect("", x0+s, ph+0.32, x1-s, ph+building.FloorHeight)
	for x := x0 + s + 1.5; x < x1-s-1.2; x += 1.5 {
		p.vline("faint", x, ph+0.32, ph+building.FloorHeight)
	}
	p.rect("", x0, ph+0.32, x1, ph+1.07)
	for x := x0 + 0.6; x < x1; x += 1.1 {
		p.path("leaf", "M%.1f %.1fq3 -5 6 0", ex(x), ey(ph+1.07))
	}
	p.rect("", -0.2, ph+0.32, 0.2, ph+building.FloorHeight)
	for _, u := range building.HomesOnLevel(building.PenthouseLevel) {
		b := u.Position.Bay.Plan
		yg := ph + 0.32
		p.rect("bay", b.X0, yg, b.X1, ph+building.FloorHeight-0.02)
		p.rect("fill", b.X0, yg+building.BayHeadHeight, b.X1, ph+building.FloorHeight-0.02)
	}

	// Roof slab, balustrade, pergola and the lift overrun.
	r := building.RoofElevation()
	p.rect("", x0+s-0.6, r, x1-s+0.6, r+0.45)
	p.rect("glass", x0+s-0.5, r+0.45, x1-s+0.5, r+1.55)
	p.rect("", -8, r+0.45, 0.8, r+3.55)
	pg := r + 0.45 + 3.0
	p.hline("", -15, 13, pg)
	p.hline("", -15, 13, pg+0.22)
	for x := -15.0; x <= 13.01; x += 4 {
		p.vline("", x, r+0.45, pg)
	}
	for x := -14.7; x < 13; x += 0.6 {
		p.vline("faint", x, pg+0.22, pg+0.42)
	}

	// Trees either side, and people at the door, for scale.
	for _, t := range [][3]float64{{-26.5, 7.5, 3.4}, {-23, 6.2, 2.8}, {25.5, 7.8, 3.6}, {22.4, 6, 2.6}} {
		x, crown, r := t[0], t[1], t[2]
		p.vline("", x, 0, crown-r*0.6)
		p.path("tree", "M%.1f %.1fa%.1f %.1f 0 1 1 0.1 0Z", ex(x)-r*elevScale, ey(crown), r*elevScale, r*elevScale*0.9)
	}
	for _, x := range []float64{-4.2, -3.6, 2.8, 9.5} {
		person(p, x)
	}
	return p.lines
}

// person is a figure 1.75 m tall, as architects draw them.
func person(p *elevPen, x float64) {
	head := 1.62
	p.path("", "M%.1f %.1fa1.3 1.3 0 1 1 0.1 0Z", ex(x)-1.3, ey(head))
	var b strings.Builder
	fmt.Fprintf(&b, "M%.1f %.1fL%.1f %.1fL%.1f %.1f", ex(x-0.22), ey(0), ex(x), ey(0.85), ex(x+0.22), ey(0))
	fmt.Fprintf(&b, "M%.1f %.1fV%.1f", ex(x), ey(0.85), ey(1.45))
	fmt.Fprintf(&b, "M%.1f %.1fL%.1f %.1fL%.1f %.1f", ex(x-0.28), ey(0.95), ex(x), ey(1.42), ex(x+0.28), ey(0.95))
	p.path("", "%s", b.String())
}

// elevationBays counts the bays drawn, for the test.
func elevationBays() int {
	n := 0
	for _, l := range elevationLines() {
		if l.Class == "bay" && strings.HasSuffix(l.D, "Z") {
			n++
		}
	}
	return n
}

// elevationStroke is the lines of one kind on one storey, drawn as a single
// path, so the drawing traces itself in a few dozen strokes, not hundreds.
type elevationStroke struct {
	D     string
	Class string
	Order int // the storey, counted up from the ground: strokes draw in this order
}

// elevationStrokes groups the drawing's lines by the storey they start on and
// their kind. Every kind on a storey is drawn at once, storey by storey upward.
func elevationStrokes() []elevationStroke {
	type key struct {
		storey int
		class  string
	}
	var order []key
	groups := map[key][]string{}
	storeyHeight := building.FloorHeight * elevScale
	for _, l := range elevationLines() {
		var x, y float64
		fmt.Sscanf(l.D, "M%f %f", &x, &y)
		storey := int((elevGround - y) / storeyHeight)
		if storey < 0 {
			storey = 0
		}
		k := key{storey, l.Class}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], l.D)
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].storey < order[j].storey })
	out := make([]elevationStroke, 0, len(order))
	for _, k := range order {
		out = append(out, elevationStroke{D: strings.Join(groups[k], ""), Class: k.class, Order: k.storey})
	}
	return out
}

func strokeOrder(i int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("--i: %d", i))
}
