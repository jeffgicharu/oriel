package light

import (
	"math"
	"testing"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// Checked against the NOAA solar calculator for Nairobi.
func TestSunPosition(t *testing.T) {
	cases := []struct {
		at      string
		az, alt float64
	}{
		// Near the March equinox at solar noon (about 12:40 in Nairobi) the
		// sun is almost overhead.
		{"2026-03-20T12:40:00+03:00", 0, 88},
		// June mornings: the sun rises north of east.
		{"2026-06-21T08:00:00+03:00", 64, 17},
		// December afternoons: the sun sets south of west.
		{"2026-12-21T17:00:00+03:00", 245, 22},
	}
	for _, c := range cases {
		at, _ := time.Parse(time.RFC3339, c.at)
		p := Sun(at, building.Latitude, building.Longitude)
		if !near(p.Altitude, c.alt, 3) {
			t.Errorf("%s: altitude %.1f, want about %.0f", c.at, p.Altitude, c.alt)
		}
		if c.alt < 85 && !near(p.Azimuth, c.az, 4) {
			t.Errorf("%s: azimuth %.1f, want about %.0f", c.at, p.Azimuth, c.az)
		}
	}
}

// Close to the equator the sun is in the north from April to August and in
// the south from October to February, so north- and south-facing homes swap.
func TestNorthAndSouthHomesSwapThroughTheYear(t *testing.T) {
	north, _ := building.UnitByID("0702")
	south, _ := building.UnitByID("0705")
	n := StudyOf(north, 2027)
	s := StudyOf(south, 2027)
	if n.Months[5].Hours() <= s.Months[5].Hours() {
		t.Errorf("June: north home %.1f h, south home %.1f h", n.Months[5].Hours(), s.Months[5].Hours())
	}
	if s.Months[11].Hours() <= n.Months[11].Hours() {
		t.Errorf("December: south home %.1f h, north home %.1f h", s.Months[11].Hours(), n.Months[11].Hours())
	}
}

func TestCornerHomesGetMoreLight(t *testing.T) {
	corner, _ := building.UnitByID("0703") // north and east, with a corner bay
	middle, _ := building.UnitByID("0702") // north only
	if StudyOf(corner, 2027).AverageDay <= StudyOf(middle, 2027).AverageDay {
		t.Error("the corner home gets no more sun than the middle one")
	}
}

func TestEveryHomeGetsSomeSun(t *testing.T) {
	for _, u := range building.Units() {
		s := StudyOf(u, 2027)
		if s.AverageDay <= 0.5 || s.AverageDay > 12 {
			t.Errorf("home %s: %.1f hours a day", u.ID, s.AverageDay)
		}
	}
}
