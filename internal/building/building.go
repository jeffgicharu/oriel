// Package building is the one description of Oriel that everything else is
// drawn from: the floors and their heights, where each home sits on its floor,
// the home types, which way each home faces, and its bay.
//
// Coordinates are in metres. x runs east, y runs up from the ground, and z
// runs south, so north is -z. The footprint is centred on x = 0, z = 0.
package building

import (
	"fmt"
	"sort"
)

// Compass directions a façade can face.
type Facing string

const (
	North Facing = "N"
	East  Facing = "E"
	South Facing = "S"
	West  Facing = "W"
)

// Name is the direction written out.
func (f Facing) Name() string {
	switch f {
	case North:
		return "north"
	case East:
		return "east"
	case South:
		return "south"
	case West:
		return "west"
	}
	return string(f)
}

// Azimuth is the direction the façade faces, in degrees clockwise from north.
func (f Facing) Azimuth() float64 {
	switch f {
	case North:
		return 0
	case East:
		return 90
	case South:
		return 180
	case West:
		return 270
	}
	return 0
}

// Floor plate and heights.
const (
	PlateWidth = 40.6 // east to west
	PlateDepth = 22.0 // north to south
	XMin       = -PlateWidth / 2
	XMax       = PlateWidth / 2
	ZMin       = -PlateDepth / 2 // the north face
	ZMax       = PlateDepth / 2  // the south face

	GroundHeight  = 4.5
	AmenityHeight = 4.0
	FloorHeight   = 3.2

	FirstHomeLevel = 2
	LastTypical    = 15
	PenthouseLevel = 16
	TopLevel       = PenthouseLevel

	// The penthouse floor is set back behind a terrace on every side.
	PenthouseSetback = 1.8

	// Bays project this far from the façade.
	BayProjection = 1.2
	// Balconies on the long façades project this far.
	BalconyDepth = 1.6
	// The glass of a bay starts above a window seat and stops under a cobalt
	// band about 0.4 m deep that runs up to the slab above.
	BaySeatHeight = 0.45
	BayHeadHeight = 2.45
)

// LevelElevation is the height of a level's floor above the ground.
func LevelElevation(level int) float64 {
	switch {
	case level <= 0:
		return 0
	case level == 1:
		return GroundHeight
	default:
		return GroundHeight + AmenityHeight + float64(level-2)*FloorHeight
	}
}

// RoofElevation is the height of the main roof, where the pool is.
func RoofElevation() float64 {
	return LevelElevation(PenthouseLevel) + FloorHeight
}

// LevelName is how a level is written on the site: "L07".
func LevelName(level int) string {
	if level == 0 {
		return "G"
	}
	return fmt.Sprintf("L%02d", level)
}

// Rect is an axis-aligned rectangle in plan.
type Rect struct {
	X0, Z0, X1, Z1 float64
}

// Width is the rectangle's east-west size.
func (r Rect) Width() float64 { return r.X1 - r.X0 }

// Depth is the rectangle's north-south size.
func (r Rect) Depth() float64 { return r.Z1 - r.Z0 }

// Area is the rectangle's area in square metres.
func (r Rect) Area() float64 { return r.Width() * r.Depth() }

// Bay is a glazed box that projects from the façade. Corner bays wrap a
// corner and look two ways; the others look one way.
type Bay struct {
	Plan   Rect
	Facing []Facing
	Corner bool
}

// Position is one of the places a home can sit on a typical floor.
type Position struct {
	Number    int
	Type      string
	Footprint Rect
	Facing    []Facing
	Bay       Bay
	// Balcony is the projecting balcony, in plan.
	Balcony   Rect
	RiverView bool
	// Handed is true where the plan is the mirror image of its type's drawing.
	Handed bool
	// PlanRotation turns the type's drawing so its top faces the right way:
	// 0 when the main façade faces north, 180 when it faces south.
	PlanRotation int
}

// The core holds the lifts, the stairs and the risers.
var Core = Rect{X0: -9.1, Z0: 1.0, X1: 1.9, Z1: ZMax}

// Corridor runs the length of each typical floor.
var Corridor = Rect{X0: XMin, Z0: -1.0, X1: XMax, Z1: 1.0}

// Positions on levels 2 to 15, numbered clockwise from the north-west corner.
var Positions = []Position{
	{
		Number: 1, Type: "C",
		Footprint: Rect{XMin, ZMin, -4.5, -1.0},
		Facing:    []Facing{North, West},
		Bay:       cornerBay(XMin, ZMin, North, West),
		Balcony:   Rect{-16.0, ZMin - BalconyDepth, -6.0, ZMin},
		RiverView: true,
	},
	{
		Number: 2, Type: "A",
		Footprint: Rect{-4.5, ZMin, 2.7, -1.0},
		Facing:    []Facing{North},
		Bay:       faceBay(1.2, ZMin, North),
		Balcony:   Rect{-4.3, ZMin - 2.0, -0.3, ZMin},
		RiverView: true,
	},
	{
		Number: 3, Type: "D",
		Footprint: Rect{2.7, ZMin, XMax, -1.0},
		Facing:    []Facing{North, East},
		Bay:       cornerBay(XMax, ZMin, North, East),
		Balcony:   Rect{5.0, ZMin - BalconyDepth, 16.0, ZMin},
		RiverView: true,
	},
	{
		Number: 4, Type: "B",
		Footprint:    Rect{9.1, 1.0, XMax, ZMax},
		Facing:       []Facing{South, East},
		Bay:          cornerBay(XMax, ZMax, South, East),
		Balcony:      Rect{10.0, ZMax, 17.5, ZMax + BalconyDepth},
		PlanRotation: 180,
	},
	{
		Number: 5, Type: "A",
		Footprint:    Rect{1.9, 1.0, 9.1, ZMax},
		Facing:       []Facing{South},
		Bay:          faceBay(3.3, ZMax, South),
		Balcony:      Rect{5.0, ZMax, 9.0, ZMax + 2.0},
		PlanRotation: 180,
	},
	{
		Number: 6, Type: "B",
		Footprint:    Rect{XMin, 1.0, -9.1, ZMax},
		Facing:       []Facing{South, West},
		Bay:          cornerBay(XMin, ZMax, South, West),
		Balcony:      Rect{-17.5, ZMax, -10.0, ZMax + BalconyDepth},
		Handed:       true,
		PlanRotation: 180,
	},
}

// PenthousePositions are the two homes on level 16.
var PenthousePositions = []Position{
	{
		Number: 1, Type: "P",
		Footprint: Rect{XMin + PenthouseSetback, ZMin + PenthouseSetback, -0.4, ZMax - PenthouseSetback},
		Facing:    []Facing{North, West, South},
		Bay:       cornerBay(XMin+PenthouseSetback, ZMin+PenthouseSetback, North, West),
		RiverView: true,
	},
	{
		Number: 2, Type: "P",
		Footprint: Rect{0.4, ZMin + PenthouseSetback, XMax - PenthouseSetback, ZMax - PenthouseSetback},
		Facing:    []Facing{North, East, South},
		Bay:       cornerBay(XMax-PenthouseSetback, ZMin+PenthouseSetback, North, East),
		RiverView: true,
		Handed:    true,
	},
}

// cornerBay is a 2.4 m square box centred on a corner, so it reaches 1.2 m
// past both façades.
func cornerBay(x, z float64, a, b Facing) Bay {
	const half = 1.2
	return Bay{Plan: Rect{x - half, z - half, x + half, z + half}, Facing: []Facing{a, b}, Corner: true}
}

// faceBay is a 2.8 m wide box centred at x on a north or south façade.
func faceBay(x, z float64, f Facing) Bay {
	const half = 1.4
	if f == North {
		return Bay{Plan: Rect{x - half, z - BayProjection, x + half, z}, Facing: []Facing{f}}
	}
	return Bay{Plan: Rect{x - half, z, x + half, z + BayProjection}, Facing: []Facing{f}}
}

// Unit is one home in the building.
type Unit struct {
	// ID is the home's number: the level and the position, such as "0704".
	ID       string
	Level    int
	Position Position
	Type     Type
}

// Facings lists the directions the home looks, written out: "north and west".
func (u Unit) Facings() string {
	names := make([]string, len(u.Position.Facing))
	for i, f := range u.Position.Facing {
		names[i] = f.Name()
	}
	return JoinAnd(names)
}

// JoinAnd writes a list as "a, b and c".
func JoinAnd(items []string) string {
	out := ""
	for i, item := range items {
		switch {
		case i == 0:
			out = item
		case i == len(items)-1:
			out += " and " + item
		default:
			out += ", " + item
		}
	}
	return out
}

// Aspect is the short compass form: "N · W".
func (u Unit) Aspect() string {
	out := ""
	for i, f := range u.Position.Facing {
		if i > 0 {
			out += " · "
		}
		out += string(f)
	}
	return out
}

// Elevation is the height of the home's floor above the ground.
func (u Unit) Elevation() float64 {
	return LevelElevation(u.Level)
}

// Units lists every home, from the lowest floor up and clockwise round each.
func Units() []Unit {
	return units
}

// UnitByID finds a home by its number.
func UnitByID(id string) (Unit, bool) {
	u, ok := unitIndex[id]
	return u, ok
}

var (
	units     []Unit
	unitIndex = map[string]Unit{}
)

func init() {
	for level := FirstHomeLevel; level <= LastTypical; level++ {
		for _, p := range Positions {
			add(level, p)
		}
	}
	for _, p := range PenthousePositions {
		add(PenthouseLevel, p)
	}
	sort.SliceStable(units, func(i, j int) bool {
		if units[i].Level != units[j].Level {
			return units[i].Level < units[j].Level
		}
		return units[i].Position.Number < units[j].Position.Number
	})
}

func add(level int, p Position) {
	t, ok := Types[p.Type]
	if !ok {
		panic("building: unknown type " + p.Type)
	}
	u := Unit{ID: fmt.Sprintf("%02d%02d", level, p.Number), Level: level, Position: p, Type: t}
	units = append(units, u)
	unitIndex[u.ID] = u
}

// HomesOnLevel lists the homes on one level.
func HomesOnLevel(level int) []Unit {
	var out []Unit
	for _, u := range units {
		if u.Level == level {
			out = append(out, u)
		}
	}
	return out
}

// HomeLevels lists the levels with homes, from the top down, as the level
// rail shows them.
func HomeLevels() []int {
	var out []int
	for level := TopLevel; level >= FirstHomeLevel; level-- {
		out = append(out, level)
	}
	return out
}
