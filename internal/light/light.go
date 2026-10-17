// Package light works out how much direct sun reaches each home, from the
// sun's real path over Riverside and the shading the building gives itself:
// the slab edge over every window, the fins on the east and west faces, and
// the bay, which has no fins and catches light from two or three sides.
package light

import (
	"math"
	"sync"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
)

const rad = math.Pi / 180

// Position is where the sun is: azimuth clockwise from north, altitude above
// the horizon, both in degrees.
type Position struct {
	Azimuth  float64
	Altitude float64
}

// Sun is the NOAA solar position calculation, the same one the 3D view uses.
func Sun(t time.Time, lat, lon float64) Position {
	t = t.UTC()
	jd := float64(t.UnixMilli())/86400000 + 2440587.5
	jc := (jd - 2451545) / 36525
	meanLong := math.Mod(280.46646+jc*(36000.76983+jc*0.0003032), 360)
	meanAnom := 357.52911 + jc*(35999.05029-0.0001537*jc)
	ecc := 0.016708634 - jc*(0.000042037+0.0000001267*jc)
	centre := math.Sin(rad*meanAnom)*(1.914602-jc*(0.004817+0.000014*jc)) +
		math.Sin(rad*2*meanAnom)*(0.019993-0.000101*jc) +
		math.Sin(rad*3*meanAnom)*0.000289
	trueLong := meanLong + centre
	omega := 125.04 - 1934.136*jc
	apparentLong := trueLong - 0.00569 - 0.00478*math.Sin(rad*omega)
	meanObliquity := 23 + (26+(21.448-jc*(46.815+jc*(0.00059-jc*0.001813)))/60)/60
	obliquity := meanObliquity + 0.00256*math.Cos(rad*omega)
	declination := math.Asin(math.Sin(rad*obliquity)*math.Sin(rad*apparentLong)) / rad
	y := math.Pow(math.Tan(rad*obliquity/2), 2)
	eqTime := 4 / rad * (y*math.Sin(2*rad*meanLong) -
		2*ecc*math.Sin(rad*meanAnom) +
		4*ecc*y*math.Sin(rad*meanAnom)*math.Cos(2*rad*meanLong) -
		0.5*y*y*math.Sin(4*rad*meanLong) -
		1.25*ecc*ecc*math.Sin(2*rad*meanAnom))
	minutes := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60
	solar := math.Mod(minutes+eqTime+4*lon, 1440)
	if solar < 0 {
		solar += 1440
	}
	hourAngle := solar/4 - 180
	cosZenith := math.Sin(rad*lat)*math.Sin(rad*declination) + math.Cos(rad*lat)*math.Cos(rad*declination)*math.Cos(rad*hourAngle)
	zenith := math.Acos(math.Max(-1, math.Min(1, cosZenith))) / rad
	denom := math.Cos(rad*lat) * math.Sin(rad*zenith)
	azimuth := 0.0
	if math.Abs(denom) > 1e-6 {
		a := math.Max(-1, math.Min(1, (math.Sin(rad*lat)*math.Cos(rad*zenith)-math.Sin(rad*declination))/denom))
		base := math.Acos(a) / rad
		if hourAngle > 0 {
			azimuth = math.Mod(base+180, 360)
		} else {
			azimuth = math.Mod(540-base, 360)
		}
	}
	return Position{Azimuth: azimuth, Altitude: 90 - zenith}
}

// Window is a glazed face with its shading.
type Window struct {
	Facing building.Facing
	// Overhang is how far the shade above projects, in metres; Height is the
	// glass height below it.
	Overhang, Height float64
	// FinDepth and FinSpacing describe vertical fins, if any.
	FinDepth, FinSpacing float64
}

// lit reports whether the sun at p shines on this window's glass.
func (w Window) lit(p Position) bool {
	if p.Altitude < 4 {
		return false
	}
	diff := math.Mod(p.Azimuth-w.Facing.Azimuth()+540, 360) - 180
	if math.Abs(diff) >= 88 {
		return false
	}
	// The slab edge above: the sun's profile angle must clear it.
	profile := math.Atan(math.Tan(p.Altitude*rad)/math.Cos(diff*rad)) / rad
	if w.Overhang > 0 && math.Tan(profile*rad) > w.Height/w.Overhang {
		return false
	}
	// Fins cut off sun that comes in at a slant along the face.
	if w.FinDepth > 0 && math.Abs(math.Tan(diff*rad)) > w.FinSpacing/w.FinDepth {
		return false
	}
	return true
}

// windows lists the glazed faces of a home.
func windows(u building.Unit) []Window {
	var out []Window
	for _, f := range u.Position.Facing {
		w := Window{Facing: f, Overhang: 0.45, Height: 2.75}
		if (f == building.East || f == building.West) && u.Level != building.PenthouseLevel {
			w.FinDepth, w.FinSpacing = 0.6, 1.4
		}
		if u.Level == building.PenthouseLevel {
			// The roof overhang over the terrace shades deeper.
			w.Overhang, w.Height = 1.2, 2.8
		}
		out = append(out, w)
	}
	// The bay looks out on its own faces, under its cobalt head only.
	for _, f := range u.Position.Bay.Facing {
		out = append(out, Window{Facing: f, Overhang: 0.25, Height: 2.3})
	}
	return out
}

// Month is the light a home gets on a typical day in one month.
type Month struct {
	Month     time.Month
	Morning   float64 // hours of direct sun before solar noon
	Afternoon float64 // hours after
}

// Hours is the total for the day.
func (m Month) Hours() float64 { return m.Morning + m.Afternoon }

// Study is the light study of one home.
type Study struct {
	Months []Month
	// Best and Worst are the months with the most and least direct sun.
	Best         time.Month
	Worst        time.Month
	AverageDay   float64
	MorningShare float64
}

var nairobi = func() *time.Location {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}()

// StudyOf works out the light study for a home, for the year given.
func StudyOf(u building.Unit, year int) Study {
	ws := windows(u)
	var s Study
	total, morning := 0.0, 0.0
	for m := time.January; m <= time.December; m++ {
		mo := Month{Month: m}
		day := time.Date(year, m, 15, 5, 30, 0, 0, nairobi)
		noon := solarNoon(day)
		const step = 10 * time.Minute
		for t := day; t.Hour() < 19 || (t.Hour() == 19 && t.Minute() <= 30); t = t.Add(step) {
			p := Sun(t, building.Latitude, building.Longitude)
			sunny := false
			for _, w := range ws {
				if w.lit(p) {
					sunny = true
					break
				}
			}
			if !sunny {
				continue
			}
			h := step.Hours()
			if t.Before(noon) {
				mo.Morning += h
			} else {
				mo.Afternoon += h
			}
		}
		mo.Morning = math.Round(mo.Morning*10) / 10
		mo.Afternoon = math.Round(mo.Afternoon*10) / 10
		total += mo.Hours()
		morning += mo.Morning
		s.Months = append(s.Months, mo)
	}
	best, worst := s.Months[0], s.Months[0]
	for _, mo := range s.Months {
		if mo.Hours() > best.Hours() {
			best = mo
		}
		if mo.Hours() < worst.Hours() {
			worst = mo
		}
	}
	s.Best, s.Worst = best.Month, worst.Month
	s.AverageDay = math.Round(total/12*10) / 10
	if total > 0 {
		s.MorningShare = morning / total
	}
	return s
}

// solarNoon is when the sun is highest on the day of t.
func solarNoon(t time.Time) time.Time {
	best := t
	bestAlt := -90.0
	for h := 10.0; h <= 14; h += 1.0 / 12 {
		at := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, nairobi).Add(time.Duration(h * float64(time.Hour)))
		if p := Sun(at, building.Latitude, building.Longitude); p.Altitude > bestAlt {
			bestAlt, best = p.Altitude, at
		}
	}
	return best
}

var (
	studiesOnce sync.Once
	studies     map[string]Study
)

// For returns the light study of a home, worked out once for every home the
// first time it is asked for.
func For(id string) (Study, bool) {
	studiesOnce.Do(func() {
		studies = map[string]Study{}
		for _, u := range building.Units() {
			studies[u.ID] = StudyOf(u, 2027)
		}
	})
	s, ok := studies[id]
	return s, ok
}
