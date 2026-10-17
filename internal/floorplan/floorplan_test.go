package floorplan

import (
	"math"
	"strings"
	"testing"

	"github.com/jeffgicharu/oriel/internal/building"
)

func allPlans() []Plan {
	var out []Plan
	for _, code := range building.TypeOrder {
		p, ok := For(code)
		if !ok {
			panic("no plan for " + code)
		}
		out = append(out, p, p.Mirrored())
	}
	return out
}

func overlap(a, b Rect) float64 {
	w := math.Min(a.X1, b.X1) - math.Max(a.X0, b.X0)
	h := math.Min(a.Y1, b.Y1) - math.Max(a.Y0, b.Y0)
	if w <= 1e-6 || h <= 1e-6 {
		return 0
	}
	return w * h
}

// Rooms fill the home exactly: no gaps, no overlaps.
func TestRoomsFillTheHome(t *testing.T) {
	for _, p := range allPlans() {
		var rects []Rect
		total := 0.0
		for _, room := range p.Rooms {
			for _, r := range room.Rects {
				if r.X0 < -1e-9 || r.Y0 < -1e-9 || r.X1 > p.W+1e-9 || r.Y1 > p.D+1e-9 {
					t.Errorf("type %s: %s reaches outside the home", p.Type, room.Name)
				}
				rects = append(rects, r)
				total += r.Area()
			}
		}
		for i := range rects {
			for j := i + 1; j < len(rects); j++ {
				if o := overlap(rects[i], rects[j]); o > 0 {
					t.Errorf("type %s: rooms overlap by %.2f m² (%v and %v)", p.Type, o, rects[i], rects[j])
				}
			}
		}
		if math.Abs(total-p.W*p.D) > 0.01 {
			t.Errorf("type %s: rooms cover %.2f m² of %.2f m²", p.Type, total, p.W*p.D)
		}
	}
}

// The typical plans match the area each type is sold at.
func TestPlansMatchTheirTypes(t *testing.T) {
	for _, code := range []string{"A", "B", "C", "D"} {
		p, _ := For(code)
		want := float64(building.Types[code].Internal)
		if math.Abs(p.W*p.D-want) > 0.5 {
			t.Errorf("type %s plan is %.1f m², sold as %.0f m²", code, p.W*p.D, want)
		}
		if got := p.Bedrooms(); got < building.Types[code].Bedrooms {
			t.Errorf("type %s plan has %d bedrooms, sold with %d", code, got, building.Types[code].Bedrooms)
		}
	}
}

// sideOf says which rooms lie either side of a short piece of wall.
func (p Plan) roomsBeside(x0, y0, x1, y1 float64) []int {
	mx, my := (x0+x1)/2, (y0+y1)/2
	var probes [][2]float64
	if x0 == x1 {
		probes = [][2]float64{{mx - 0.05, my}, {mx + 0.05, my}}
	} else {
		probes = [][2]float64{{mx, my - 0.05}, {mx, my + 0.05}}
	}
	var out []int
	for _, pr := range probes {
		for i, room := range p.Rooms {
			for _, r := range room.Rects {
				if pr[0] > r.X0 && pr[0] < r.X1 && pr[1] > r.Y0 && pr[1] < r.Y1 {
					out = append(out, i)
				}
			}
		}
	}
	return out
}

// Every room can be reached from the front door, through doors and openings,
// and every door joins two rooms or leads in from outside.
func TestEveryRoomCanBeReached(t *testing.T) {
	for _, p := range allPlans() {
		links := map[int][]int{}
		start := -1
		join := func(x0, y0, x1, y1 float64, what string) {
			beside := p.roomsBeside(x0, y0, x1, y1)
			switch len(beside) {
			case 2:
				a, b := beside[0], beside[1]
				links[a] = append(links[a], b)
				links[b] = append(links[b], a)
			case 1:
				if start < 0 && strings.HasPrefix(what, "entrance") {
					start = beside[0]
				}
			default:
				t.Errorf("type %s: %s at (%.2f, %.2f) is not on a wall between rooms", p.Type, what, x0, y0)
			}
		}
		for _, d := range p.Doors {
			what := "door"
			if d.Entrance {
				what = "entrance"
			}
			if d.Vertical {
				join(d.X, d.Y, d.X, d.Y+d.Width, what)
			} else {
				join(d.X, d.Y, d.X+d.Width, d.Y, what)
			}
		}
		for _, o := range p.Openings {
			if o.X0 == 0 && o.X1 == 0 || o.Y0 == 0 && o.Y1 == 0 || o.X0 == p.W && o.X1 == p.W {
				continue // the opening into a bay
			}
			join(o.X0, o.Y0, o.X1, o.Y1, "opening")
		}
		if start < 0 {
			t.Errorf("type %s: no entrance", p.Type)
			continue
		}
		seen := map[int]bool{start: true}
		queue := []int{start}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			for _, m := range links[n] {
				if !seen[m] {
					seen[m] = true
					queue = append(queue, m)
				}
			}
		}
		for i, room := range p.Rooms {
			if !seen[i] {
				t.Errorf("type %s: %s cannot be reached from the entrance", p.Type, room.Name)
			}
		}
	}
}

// Bedrooms and living rooms have daylight, except the penthouse's inner rooms.
func TestMainRoomsHaveWindows(t *testing.T) {
	for _, p := range allPlans() {
		for _, room := range p.Rooms {
			if room.Kind != Bedroom && room.Kind != Living {
				continue
			}
			if p.Type == "P" && (room.Name == "DSQ" || room.Name == "Study") {
				continue
			}
			if p.Type == "D" && room.Name == "Kitchen" || p.Type == "B" && room.Name == "Kitchen" || p.Type == "C" && room.Name == "Kitchen" || p.Type == "A" && room.Name == "Kitchen & dining" {
				continue // open to the living room
			}
			lit := false
			for _, w := range append(append([]Line{}, p.Windows...), p.Openings...) {
				for _, r := range room.Rects {
					onEdge := (w.Y0 == w.Y1 && (w.Y0 == r.Y0 || w.Y0 == r.Y1) && math.Min(w.X0, w.X1) < r.X1 && math.Max(w.X0, w.X1) > r.X0) ||
						(w.X0 == w.X1 && (w.X0 == r.X0 || w.X0 == r.X1) && math.Min(w.Y0, w.Y1) < r.Y1 && math.Max(w.Y0, w.Y1) > r.Y0)
					exterior := w.Y0 == 0 && w.Y1 == 0 || w.Y0 == p.D && w.Y1 == p.D || w.X0 == 0 && w.X1 == 0 || w.X0 == p.W && w.X1 == p.W
					if onEdge && exterior {
						lit = true
					}
				}
			}
			if !lit {
				t.Errorf("type %s: %s has no window", p.Type, room.Name)
			}
		}
	}
}

func TestSVGDrawsEveryRoom(t *testing.T) {
	for _, p := range allPlans() {
		svg := p.SVG(Options{North: 180})
		if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
			t.Fatalf("type %s: not an SVG", p.Type)
		}
		if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
			t.Fatalf("type %s: drawing has bad numbers", p.Type)
		}
		if !strings.Contains(svg, "Entrance") {
			t.Errorf("type %s: entrance not labelled", p.Type)
		}
	}
}
