package building

import "math"

// Model is the building's geometry for the 3D view, kept small: about 14 KB
// of JSON for the whole tower. Rectangles are [x0, z0, x1, z1] in metres.
type Model struct {
	Plate     [4]float64   `json:"plate"`
	Core      [4]float64   `json:"core"`
	Levels    []ModelLevel `json:"levels"`
	Roof      float64      `json:"roof"`
	Setback   float64      `json:"setback"`
	Units     []ModelUnit  `json:"units"`
	SeatH     float64      `json:"seat"`
	HeadH     float64      `json:"head"`
	Latitude  float64      `json:"lat"`
	Longitude float64      `json:"lon"`
}

// ModelLevel is one floor of the tower.
type ModelLevel struct {
	Level int     `json:"level"`
	Y     float64 `json:"y"`
	H     float64 `json:"h"`
	Kind  string  `json:"kind"`
}

// ModelUnit is one home, with its footprint, bay and balcony.
type ModelUnit struct {
	ID      string     `json:"id"`
	Level   int        `json:"l"`
	Pos     int        `json:"p"`
	Type    string     `json:"t"`
	Foot    [4]float64 `json:"f"`
	Bay     [4]float64 `json:"b"`
	Corner  bool       `json:"c,omitempty"`
	Balcony [4]float64 `json:"k"`
	Facing  string     `json:"o"`
	River   bool       `json:"r,omitempty"`
}

// The site, a little north of Riverside Drive.
const (
	Latitude  = -1.2701
	Longitude = 36.8031
)

func rect(r Rect) [4]float64 {
	round := func(v float64) float64 { return math.Round(v*100) / 100 }
	return [4]float64{round(r.X0), round(r.Z0), round(r.X1), round(r.Z1)}
}

// ModelData is the geometry the 3D view draws.
func ModelData() Model {
	m := Model{
		Plate:     [4]float64{XMin, ZMin, XMax, ZMax},
		Core:      rect(Core),
		Roof:      math.Round(RoofElevation()*100) / 100,
		Setback:   PenthouseSetback,
		SeatH:     BaySeatHeight,
		HeadH:     BayHeadHeight,
		Latitude:  Latitude,
		Longitude: Longitude,
	}
	for level := 0; level <= TopLevel; level++ {
		kind := "homes"
		h := FloorHeight
		switch {
		case level == 0:
			kind, h = "ground", GroundHeight
		case level == 1:
			kind, h = "amenity", AmenityHeight
		case level == PenthouseLevel:
			kind = "penthouse"
		}
		m.Levels = append(m.Levels, ModelLevel{Level: level, Y: math.Round(LevelElevation(level)*100) / 100, H: h, Kind: kind})
	}
	for _, u := range units {
		facing := ""
		for _, f := range u.Position.Facing {
			facing += string(f)
		}
		m.Units = append(m.Units, ModelUnit{
			ID:      u.ID,
			Level:   u.Level,
			Pos:     u.Position.Number,
			Type:    u.Type.Code,
			Foot:    rect(u.Position.Footprint),
			Bay:     rect(u.Position.Bay.Plan),
			Corner:  u.Position.Bay.Corner,
			Balcony: rect(u.Position.Balcony),
			Facing:  facing,
			River:   u.Position.RiverView,
		})
	}
	return m
}
