// Package floorplan holds the plan of each home type: its rooms, doors,
// windows, bay, balcony and furniture. The same plan draws the drawing on a
// home's page, the plan printed on a letter of offer and the floor seen from
// above in the 3D explorer.
//
// Plans are in metres. The main façade runs along the top edge (y = 0), so
// the drawing reads the way a buyer stands in the living room looking out;
// the corridor is along the bottom edge. A north arrow shows which way the
// home really faces.
package floorplan

import (
	"fmt"
	"math"
)

// Rect is a rectangle in plan, in metres.
type Rect struct{ X0, Y0, X1, Y1 float64 }

// W is the rectangle's width.
func (r Rect) W() float64 { return r.X1 - r.X0 }

// H is the rectangle's depth.
func (r Rect) H() float64 { return r.Y1 - r.Y0 }

// Area is the rectangle's area.
func (r Rect) Area() float64 { return r.W() * r.H() }

// Kinds of room, which decide how a room is drawn.
const (
	Living      = "living"
	Bedroom     = "bedroom"
	Wet         = "wet"
	Service     = "service"
	Circulation = "circulation"
)

// Room is one room, made of one rectangle or a few (an L-shaped hall).
type Room struct {
	Name  string
	Kind  string
	Rects []Rect
}

// Area is the room's floor area.
func (r Room) Area() float64 {
	a := 0.0
	for _, rect := range r.Rects {
		a += rect.Area()
	}
	return a
}

// Size describes a rectangular room as "4.6 × 4.8 m"; L-shaped rooms by area.
func (r Room) Size() string {
	if len(r.Rects) == 1 {
		w, h := r.Rects[0].W(), r.Rects[0].H()
		return fmt.Sprintf("%s × %s m", metres(w), metres(h))
	}
	return fmt.Sprintf("%.0f m²", math.Round(r.Area()))
}

func metres(v float64) string {
	return fmt.Sprintf("%.1f", math.Round(v*10)/10)
}

// Door is an opening in a wall with a door in it. It sits on the wall at
// x = X (Vertical) or y = Y, from its start along the wall for Width metres.
type Door struct {
	X, Y     float64
	Width    float64
	Vertical bool
	// Into is +1 when the door swings towards +x or +y, -1 the other way.
	Into int
	// HingeAtEnd puts the hinge at the far end of the opening.
	HingeAtEnd bool
	Entrance   bool
	Service    bool
}

// Line is a segment along a wall: an opening without a wall, or a window.
type Line struct{ X0, Y0, X1, Y1 float64 }

// Item is a piece of furniture or a fitting, drawn in outline.
type Item struct {
	Kind string
	R    Rect
}

// Plan is a home type's plan.
type Plan struct {
	Type     string
	W, D     float64
	Rooms    []Room
	Doors    []Door
	Openings []Line
	Windows  []Line
	// Bay is the glazed bay; on corner homes it straddles the corner.
	Bay       Rect
	BayCorner bool
	Outdoor   Rect
	// OutdoorName is "Balcony" or "Terrace".
	OutdoorName string
	// OutdoorShape lists extra rectangles of a terrace that wraps round.
	OutdoorMore []Rect
	Items       []Item
}

// For returns the plan of a home type.
func For(code string) (Plan, bool) {
	p, ok := plans[code]
	return p, ok
}

// Mirrored is the plan flipped left to right, for homes that are handed.
func (p Plan) Mirrored() Plan {
	fx := func(x float64) float64 { return p.W - x }
	fr := func(r Rect) Rect { return Rect{fx(r.X1), r.Y0, fx(r.X0), r.Y1} }
	fl := func(l Line) Line { return Line{fx(l.X1), l.Y1, fx(l.X0), l.Y0} }
	out := p
	out.Rooms = nil
	for _, room := range p.Rooms {
		m := Room{Name: room.Name, Kind: room.Kind}
		for _, r := range room.Rects {
			m.Rects = append(m.Rects, fr(r))
		}
		out.Rooms = append(out.Rooms, m)
	}
	out.Doors = nil
	for _, d := range p.Doors {
		if d.Vertical {
			d.X = fx(d.X)
			d.Into = -d.Into
		} else {
			d.X = fx(d.X + d.Width)
			d.HingeAtEnd = !d.HingeAtEnd
		}
		out.Doors = append(out.Doors, d)
	}
	out.Openings = nil
	for _, l := range p.Openings {
		out.Openings = append(out.Openings, fl(l))
	}
	out.Windows = nil
	for _, l := range p.Windows {
		out.Windows = append(out.Windows, fl(l))
	}
	out.Bay = fr(p.Bay)
	out.Outdoor = fr(p.Outdoor)
	out.OutdoorMore = nil
	for _, r := range p.OutdoorMore {
		out.OutdoorMore = append(out.OutdoorMore, fr(r))
	}
	out.Items = nil
	for _, it := range p.Items {
		out.Items = append(out.Items, Item{Kind: it.Kind, R: fr(it.R)})
	}
	return out
}

// Bedrooms counts the rooms named as bedrooms.
func (p Plan) Bedrooms() int {
	n := 0
	for _, r := range p.Rooms {
		if r.Kind == Bedroom {
			n++
		}
	}
	return n
}
