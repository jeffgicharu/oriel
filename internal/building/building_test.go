package building

import (
	"math"
	"testing"
)

func TestThereAreEightySixHomes(t *testing.T) {
	if got := len(Units()); got != 86 {
		t.Fatalf("%d homes, want 86", got)
	}
	want := map[string]int{"A": 28, "B": 28, "C": 14, "D": 14, "P": 2}
	for code, n := range want {
		if got := CountOfType(code); got != n {
			t.Errorf("type %s: %d homes, want %d", code, got, n)
		}
	}
}

func TestFootprintsMatchTheTypeAreas(t *testing.T) {
	for _, p := range Positions {
		got := p.Footprint.Area()
		want := float64(Types[p.Type].Internal)
		if math.Abs(got-want) > 0.5 {
			t.Errorf("position %d (%s): footprint %.1f m², type says %.0f m²", p.Number, p.Type, got, want)
		}
	}
}

func TestTypicalFloorIsFullyUsed(t *testing.T) {
	total := Core.Area() + Corridor.Area()
	for _, p := range Positions {
		total += p.Footprint.Area()
	}
	plate := PlateWidth * PlateDepth
	if math.Abs(total-plate) > 0.01 {
		t.Fatalf("homes, core and corridor cover %.2f m² of a %.2f m² plate", total, plate)
	}
}

func TestFootprintsDoNotOverlap(t *testing.T) {
	rects := []Rect{Core, Corridor}
	for _, p := range Positions {
		rects = append(rects, p.Footprint)
	}
	for i := range rects {
		for j := i + 1; j < len(rects); j++ {
			a, b := rects[i], rects[j]
			overlapX := math.Min(a.X1, b.X1) - math.Max(a.X0, b.X0)
			overlapZ := math.Min(a.Z1, b.Z1) - math.Max(a.Z0, b.Z0)
			if overlapX > 0.001 && overlapZ > 0.001 {
				t.Errorf("rectangles %d and %d overlap", i, j)
			}
		}
	}
}

func TestEveryBayReachesPastItsFacade(t *testing.T) {
	for _, u := range Units() {
		b := u.Position.Bay.Plan
		f := u.Position.Footprint
		outside := b.X0 < f.X0-0.01 || b.X1 > f.X1+0.01 || b.Z0 < f.Z0-0.01 || b.Z1 > f.Z1+0.01
		if !outside {
			t.Errorf("home %s: bay does not project from the façade", u.ID)
		}
	}
}

func TestUnitNumbers(t *testing.T) {
	for _, id := range []string{"0201", "0704", "1506", "1601", "1602"} {
		if _, ok := UnitByID(id); !ok {
			t.Errorf("home %s missing", id)
		}
	}
	for _, id := range []string{"0101", "1507", "1603", "0200"} {
		if _, ok := UnitByID(id); ok {
			t.Errorf("home %s should not exist", id)
		}
	}
	u, _ := UnitByID("0603")
	if u.Type.Code != "D" || u.Level != 6 || u.Facings() != "north and east" {
		t.Errorf("0603 is %s on level %d facing %s", u.Type.Code, u.Level, u.Facings())
	}
}

func TestLevelElevations(t *testing.T) {
	if LevelElevation(2) != 8.5 {
		t.Errorf("level 2 at %.2f m", LevelElevation(2))
	}
	if math.Abs(LevelElevation(16)-53.3) > 1e-9 {
		t.Errorf("level 16 at %.2f m", LevelElevation(16))
	}
}
