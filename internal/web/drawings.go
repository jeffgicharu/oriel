package web

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/light"
)

// The drawings on the content pages, made in Go so they come from the same
// numbers as the building and the sun study. Each is drawn once and kept.

type pen struct{ strings.Builder }

func (p *pen) f(format string, args ...any) { fmt.Fprintf(&p.Builder, format, args...) }

// metres writes a length as a person would read it off a drawing: 1.2, 0.45.
func metres(v float64) string {
	s := strings.TrimRight(fmt.Sprintf("%.2f", v), "0")
	return strings.TrimSuffix(s, ".")
}

func n1(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	s = strings.TrimSuffix(s, ".0")
	if s == "-0" {
		return "0"
	}
	return s
}

var (
	sunPathOnce, baySectionOnce, mapOnce sync.Once
	sunPathSVG, baySectionSVG, mapSVG    string
)

// ---------- The sun's path over Riverside ----------

// SunPath is a sky chart: the horizon as a circle, the zenith at its centre,
// and the sun's track across it on the solstices and the equinox.
func SunPath() templ.Component {
	sunPathOnce.Do(func() { sunPathSVG = drawSunPath() })
	return templ.Raw(sunPathSVG)
}

func drawSunPath() string {
	const R = 250.0
	var p pen
	p.f(`<svg class="sunpath" viewBox="-320 -320 640 640" role="img" aria-labelledby="sunpath-title sunpath-desc" xmlns="http://www.w3.org/2000/svg">`)
	p.f(`<title id="sunpath-title">The sun's path over Riverside</title>`)
	p.f(`<desc id="sunpath-desc">A sky chart with north at the top. The sun passes close to overhead all year: north of the zenith in June, south of it in December, and almost straight through it at the equinoxes.</desc>`)
	// Horizon and altitude rings.
	p.f(`<g class="sp-grid">`)
	p.f(`<circle class="sp-horizon" r="%s"/>`, n1(R))
	for _, alt := range []float64{30, 60} {
		p.f(`<circle class="sp-ring" r="%s"/>`, n1(R*(90-alt)/90))
		// On the south side, clear of the June path's hours.
		p.f(`<text class="sp-ring-label" x="5" y="%s" dominant-baseline="hanging">%d°</text>`, n1(R*(90-alt)/90+4), int(alt))
	}
	for az := 0; az < 360; az += 15 {
		a := float64(az) * math.Pi / 180
		inner := R
		if az%90 == 0 {
			inner = 0
		}
		outer := R + 8
		if az%45 == 0 {
			outer = R + 14
		}
		p.f(`<line class="%s" x1="%s" y1="%s" x2="%s" y2="%s"/>`, map[bool]string{true: "sp-axis", false: "sp-tick"}[az%90 == 0],
			n1(inner*math.Sin(a)), n1(-inner*math.Cos(a)), n1(outer*math.Sin(a)), n1(-outer*math.Cos(a)))
	}
	for _, c := range []struct {
		az   float64
		name string
	}{{0, "N"}, {90, "E"}, {180, "S"}, {270, "W"}} {
		a := c.az * math.Pi / 180
		p.f(`<text class="sp-compass" x="%s" y="%s" text-anchor="middle" dominant-baseline="middle">%s</text>`, n1((R+34)*math.Sin(a)), n1(-(R+34)*math.Cos(a)), c.name)
	}
	p.f(`</g>`)

	// The building in plan at the centre, long faces north and south, with
	// fins drawn on the east and west faces.
	const s = 2.1
	w, d := building.PlateWidth*s, building.PlateDepth*s
	p.f(`<g class="sp-building"><rect x="%s" y="%s" width="%s" height="%s"/>`, n1(-w/2), n1(-d/2), n1(w), n1(d))
	for i := 0; i <= 6; i++ {
		y := -d/2 + d*float64(i)/6
		p.f(`<line x1="%s" y1="%s" x2="%s" y2="%s"/><line x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(-w/2-7), n1(y), n1(-w/2), n1(y), n1(w/2), n1(y), n1(w/2+7), n1(y))
	}
	p.f(`</g>`)

	project := func(pos light.Position) (float64, float64) {
		r := R * (90 - pos.Altitude) / 90
		a := pos.Azimuth * math.Pi / 180
		return r * math.Sin(a), -r * math.Cos(a)
	}
	loc := Nairobi
	days := []struct {
		date  time.Time
		class string
		label string
	}{
		{time.Date(2026, 6, 21, 0, 0, 0, 0, loc), "sp-june", "21 June"},
		{time.Date(2026, 3, 20, 0, 0, 0, 0, loc), "sp-equinox", "20 March and 22 September"},
		{time.Date(2026, 12, 21, 0, 0, 0, 0, loc), "sp-december", "21 December"},
	}
	for _, day := range days {
		var pts []string
		for m := 5 * 60; m <= 20*60; m += 5 {
			t := day.date.Add(time.Duration(m) * time.Minute)
			pos := light.Sun(t, building.Latitude, building.Longitude)
			if pos.Altitude < 0 {
				continue
			}
			x, y := project(pos)
			pts = append(pts, n1(x)+","+n1(y))
		}
		p.f(`<polyline class="sp-path %s" points="%s"/>`, day.class, strings.Join(pts, " "))
		for h := 7; h <= 17; h++ {
			pos := light.Sun(day.date.Add(time.Duration(h)*time.Hour), building.Latitude, building.Longitude)
			if pos.Altitude < 0 {
				continue
			}
			x, y := project(pos)
			p.f(`<circle class="sp-hour %s" cx="%s" cy="%s" r="3.5"/>`, day.class, n1(x), n1(y))
			if day.class == "sp-june" && h%2 == 0 {
				p.f(`<text class="sp-hour-label" x="%s" y="%s" text-anchor="middle">%02d:00</text>`, n1(x), n1(y-12), h)
			}
		}
	}
	p.f(`</svg>`)
	return p.String()
}

// ---------- A section through a bay ----------

// BaySection is a vertical section through a bay: the seat, the glass, the
// cobalt band and the slabs, dimensioned, with the morning and midday sun.
func BaySection() templ.Component {
	baySectionOnce.Do(func() { baySectionSVG = drawBaySection() })
	return templ.Raw(baySectionSVG)
}

func drawBaySection() string {
	const k = 100.0 // pixels a metre
	floor := 420.0
	y := func(h float64) float64 { return floor - h*k }
	facade := 330.0
	outer := facade + building.BayProjection*k
	slab := 0.32 // as in the 3D model
	ceiling := building.FloorHeight - slab
	seatTop, seatLeft := y(building.BaySeatHeight), facade-40
	var p pen
	p.f(`<svg class="baysection" viewBox="0 0 680 500" role="img" aria-labelledby="bs-title bs-desc" xmlns="http://www.w3.org/2000/svg">`)
	p.f(`<title id="bs-title">Section through a bay</title>`)
	p.f(`<desc id="bs-desc">The bay projects %.2f metres from the façade. Its glass runs from a window seat %.2f metres above the floor to %.2f metres, under a cobalt band %.2f metres deep. Low morning sun reaches deep into the room; the high midday sun is held off by the band and the slab.</desc>`,
		building.BayProjection, building.BaySeatHeight, building.BayHeadHeight, ceiling-building.BayHeadHeight)

	// The morning sun, low in the east: two rays through the glass, one to
	// the floor deep in the room and one onto the seat, and the patch of
	// light between them on the floor.
	low := 30.0 * math.Pi / 180
	cot := 1 / math.Tan(low)
	land := func(h float64) (float64, float64) {
		gy := y(h)
		if x := outer - 3 - (seatTop-gy)*cot; x >= seatLeft {
			return x, seatTop
		}
		return outer - 3 - (floor-gy)*cot, floor
	}
	p.f(`<g class="bs-sun">`)
	var fx float64
	for i, h := range []float64{2.0, 0.8} {
		gy := y(h)
		x, ly := land(h)
		if i == 0 {
			fx = x
		}
		p.f(`<line class="bs-ray bs-ray--low" x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(outer+150), n1(gy-150*math.Tan(low)), n1(outer-3), n1(gy))
		p.f(`<line class="bs-ray bs-ray--low bs-ray--in" x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(outer-3), n1(gy), n1(x), n1(ly))
	}
	p.f(`<rect class="bs-patch" x="%s" y="%s" width="%s" height="6"/>`, n1(fx), n1(floor-6), n1(seatLeft-fx))
	p.f(`<text class="bs-note bs-note--sun" x="676" y="%s" text-anchor="end">Morning sun, low</text>`, n1(y(2.0)-150*math.Tan(low)-12))
	// The midday sun, almost overhead, stopped by the band and the slab.
	high := 80.0 * math.Pi / 180
	ray := func(x, yy float64) {
		p.f(`<line class="bs-ray bs-ray--high" x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(x+90*math.Cos(high)), n1(yy-90*math.Sin(high)), n1(x), n1(yy))
	}
	ray(outer-20, y(building.FloorHeight))              // onto the slab above
	ray(outer+1, y((ceiling+building.BayHeadHeight)/2)) // onto the band
	p.f(`<text class="bs-note" x="%s" y="28">Midday sun, high</text>`, n1(outer+24))
	p.f(`</g>`)

	p.f(`<g class="bs-build">`)
	// Slabs: the floor of this level and the one above, carried out under and over the bay.
	p.f(`<rect class="bs-cut" x="20" y="%s" width="%s" height="%s"/>`, n1(floor), n1(outer-20), n1(slab*k))
	p.f(`<rect class="bs-cut" x="20" y="%s" width="%s" height="%s"/>`, n1(y(building.FloorHeight)), n1(outer-20), n1(slab*k))
	// The line of the wall the bay steps out of.
	p.f(`<line class="bs-hidden" x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(facade), n1(y(ceiling)), n1(facade), n1(floor))
	// Oak window seat.
	p.f(`<rect class="bs-oak" x="%s" y="%s" width="%s" height="%s"/>`, n1(seatLeft), n1(seatTop), n1(outer-10-seatLeft), n1(building.BaySeatHeight*k))
	// Cobalt band outside, and the lowered soffit inside.
	p.f(`<rect class="bs-band" x="%s" y="%s" width="14" height="%s"/>`, n1(outer-14), n1(y(ceiling)), n1((ceiling-building.BayHeadHeight)*k))
	p.f(`<rect class="bs-soffit" x="%s" y="%s" width="%s" height="%s"/>`, n1(seatLeft), n1(y(ceiling)), n1(outer-14-seatLeft), n1((ceiling-building.BayHeadHeight)*k*0.55))
	// Glass, double, in a graphite frame.
	gt, gb := y(building.BayHeadHeight), seatTop
	p.f(`<rect class="bs-frame" x="%s" y="%s" width="12" height="9"/>`, n1(outer-12), n1(gt))
	p.f(`<rect class="bs-frame" x="%s" y="%s" width="12" height="9"/>`, n1(outer-12), n1(gb-9))
	p.f(`<line class="bs-glass" x1="%s" y1="%s" x2="%s" y2="%s"/><line class="bs-glass" x1="%s" y1="%s" x2="%s" y2="%s"/>`,
		n1(outer-9), n1(gt+9), n1(outer-9), n1(gb-9), n1(outer-3), n1(gt+9), n1(outer-3), n1(gb-9))
	// The oak floor inside.
	p.f(`<line class="bs-finish" x1="20" y1="%s" x2="%s" y2="%s"/>`, n1(floor-1), n1(seatLeft), n1(floor-1))
	p.f(`</g>`)

	// Dimensions, outside the bay.
	dimX := outer + 52.0
	dim := func(h0, h1 float64) {
		y0, y1 := y(h0), y(h1)
		p.f(`<g class="bs-dim"><line x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(dimX), n1(y0), n1(dimX), n1(y1))
		for _, yy := range []float64{y0, y1} {
			p.f(`<line x1="%s" y1="%s" x2="%s" y2="%s"/><line class="bs-tick" x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(outer+6), n1(yy), n1(dimX+6), n1(yy), n1(dimX-5), n1(yy+5), n1(dimX+5), n1(yy-5))
		}
		p.f(`<text x="%s" y="%s" dominant-baseline="middle">%.2f</text></g>`, n1(dimX+12), n1((y0+y1)/2), h1-h0)
	}
	dim(0, building.BaySeatHeight)
	dim(building.BaySeatHeight, building.BayHeadHeight)
	dim(building.BayHeadHeight, ceiling)
	by := floor + slab*k + 26
	p.f(`<g class="bs-dim"><line x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(facade), n1(by), n1(outer), n1(by))
	for _, xx := range []float64{facade, outer} {
		p.f(`<line x1="%s" y1="%s" x2="%s" y2="%s"/><line class="bs-tick" x1="%s" y1="%s" x2="%s" y2="%s"/>`, n1(xx), n1(floor+slab*k+4), n1(xx), n1(by+6), n1(xx-5), n1(by+5), n1(xx+5), n1(by-5))
	}
	p.f(`<text x="%s" y="%s" text-anchor="middle">%.2f</text></g>`, n1((facade+outer)/2), n1(by+18), building.BayProjection)

	// Labels with leaders, inside the room.
	label := func(x, yy, tx, ty float64, text string) {
		p.f(`<g class="bs-label"><line x1="%s" y1="%s" x2="%s" y2="%s"/><circle cx="%s" cy="%s" r="2.5"/><text x="%s" y="%s" text-anchor="end" dominant-baseline="middle">%s</text></g>`,
			n1(tx+6), n1(ty), n1(x), n1(yy), n1(x), n1(yy), n1(tx), n1(ty), text)
	}
	label(facade+40, y(building.BaySeatHeight/2), 200, y(0.75), "Oak window seat")
	label(outer-6, y(1.75), 200, y(1.75), "Double glazing, graphite frame")
	label(outer-7, y((ceiling+building.BayHeadHeight)/2), 200, y(2.6), "Cobalt band")
	label(facade-60, y(building.FloorHeight-slab/2), 200, y(3.42), "Slab edge")
	p.f(`<text class="bs-room" x="40" y="%s">Living room</text>`, n1(y(1.3)))
	p.f(`<text class="bs-room" x="%s" y="%s" text-anchor="end">Line of façade</text>`, n1(facade-8), n1(by+4))
	p.f(`</svg>`)
	return p.String()
}

// ---------- Where Oriel is ----------

// MapPlace is somewhere on the location map, with the time to get there.
type MapPlace struct {
	Name     string
	Lat, Lon float64
	Minutes  string
	How      string
	// Anchor is where the label sits against the dot: "start" or "end".
	Anchor string
	// DY moves the label up or down, to keep it off a road or the river.
	DY float64
	// Edge places are off the map; they are drawn as an arrow on its edge.
	Edge bool
}

// Places are on the location map and listed beside it.
var Places = []MapPlace{
	{Name: "Westlands", Lat: -1.2650, Lon: 36.8105, Minutes: "5", How: "by car", Anchor: "start"},
	{Name: "City centre", Lat: -1.2850, Lon: 36.8235, Minutes: "12", How: "by car", Anchor: "start"},
	{Name: "Nairobi Arboretum", Lat: -1.2752, Lon: 36.8088, Minutes: "10", How: "on foot", Anchor: "end"},
	{Name: "University of Nairobi", Lat: -1.2797, Lon: 36.8172, Minutes: "8", How: "by car", Anchor: "end", DY: 8},
	{Name: "Karura Forest", Lat: -1.2420, Lon: 36.8335, Minutes: "15", How: "by car", Anchor: "start"},
	{Name: "Gigiri", Lat: -1.2335, Lon: 36.8150, Minutes: "18", How: "by car", Anchor: "end"},
	{Name: "Kileleshwa", Lat: -1.2850, Lon: 36.7860, Minutes: "8", How: "by car", Anchor: "end", DY: -10},
	{Name: "Lavington", Lat: -1.2790, Lon: 36.7700, Minutes: "12", How: "by car", Anchor: "start", DY: 6},
	{Name: "Upper Hill", Lat: -1.2990, Lon: 36.8150, Minutes: "15", How: "by car", Anchor: "start"},
	{Name: "Wilson Airport", Lat: -1.3215, Lon: 36.8148, Minutes: "25", How: "by car", Anchor: "start", Edge: true},
	{Name: "JKIA", Lat: -1.3192, Lon: 36.9275, Minutes: "35", How: "by the Expressway", Anchor: "end", Edge: true},
}

// The site, as a point on a diagram; no plot is named.
const siteLat, siteLon = -1.2705, 36.8030

// LocationMap is a diagram of the neighbourhood, not to scale in detail.
func LocationMap() templ.Component {
	mapOnce.Do(func() { mapSVG = drawLocationMap() })
	return templ.Raw(mapSVG)
}

func drawLocationMap() string {
	const (
		W, H           = 800.0, 620.0
		lonMin, lonMax = 36.755, 36.865
		latMax, latMin = -1.222, -1.312
		kmPerDegLon    = 111.32 // at the equator; Nairobi is close enough
		edgePad        = 26.0
		siteMarker     = 9.0
	)
	px := func(lat, lon float64) (float64, float64) {
		return (lon - lonMin) / (lonMax - lonMin) * W, (latMax - lat) / (latMax - latMin) * H
	}
	perKm := W / ((lonMax - lonMin) * kmPerDegLon)
	path := func(pts [][2]float64) string {
		var parts []string
		for i, pt := range pts {
			x, y := px(pt[0], pt[1])
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			parts = append(parts, cmd+n1(x)+" "+n1(y))
		}
		return strings.Join(parts, " ")
	}
	var p pen
	p.f(`<svg class="locmap" viewBox="0 0 %s %s" role="img" aria-labelledby="map-title map-desc" xmlns="http://www.w3.org/2000/svg">`, n1(W), n1(H))
	p.f(`<title id="map-title">Where Oriel is</title>`)
	p.f(`<desc id="map-desc">A diagram of Riverside and the places around it: Westlands five minutes away, the city centre twelve, Karura Forest fifteen and the airport thirty-five by the Expressway.</desc>`)
	p.f(`<defs><pattern id="map-forest" width="9" height="9" patternUnits="userSpaceOnUse"><circle cx="2" cy="2" r="1.1"/><circle cx="6.5" cy="6.5" r="1.1"/></pattern><clipPath id="map-clip"><rect width="%s" height="%s"/></clipPath></defs>`, n1(W), n1(H))
	p.f(`<g clip-path="url(#map-clip)">`)
	// Karura Forest and the Arboretum as stippled areas.
	p.f(`<path class="map-green" d="%s Z"/>`, path([][2]float64{{-1.2250, 36.8180}, {-1.2290, 36.8420}, {-1.2480, 36.8520}, {-1.2600, 36.8440}, {-1.2560, 36.8290}, {-1.2440, 36.8200}, {-1.2330, 36.8170}}))
	p.f(`<path class="map-green" d="%s Z"/>`, path([][2]float64{{-1.2725, 36.8058}, {-1.2728, 36.8115}, {-1.2775, 36.8120}, {-1.2780, 36.8065}}))
	// The river Riverside is named after, and the roads.
	p.f(`<path class="map-river" d="%s"/>`, path([][2]float64{{-1.2805, 36.7550}, {-1.2770, 36.7700}, {-1.2745, 36.7860}, {-1.2728, 36.7980}, {-1.2722, 36.8060}, {-1.2748, 36.8140}, {-1.2795, 36.8220}, {-1.2835, 36.8330}, {-1.2860, 36.8480}, {-1.2880, 36.8650}}))
	p.f(`<path class="map-road map-road--major" d="%s"/>`, path([][2]float64{{-1.2585, 36.7550}, {-1.2625, 36.7800}, {-1.2660, 36.8000}, {-1.2676, 36.8100}, {-1.2735, 36.8160}, {-1.2830, 36.8195}, {-1.2950, 36.8230}, {-1.3080, 36.8310}, {-1.3150, 36.8450}, {-1.3200, 36.8650}}))
	p.f(`<path class="map-road" d="%s"/>`, path([][2]float64{{-1.2676, 36.8100}, {-1.2600, 36.8160}, {-1.2450, 36.8165}, {-1.2220, 36.8160}}))
	p.f(`<path class="map-road" d="%s"/>`, path([][2]float64{{-1.2735, 36.8160}, {-1.2690, 36.8085}, {-1.2680, 36.7980}, {-1.2695, 36.7850}, {-1.2760, 36.7700}}))
	p.f(`<path class="map-road" d="%s"/>`, path([][2]float64{{-1.2830, 36.8195}, {-1.2880, 36.8000}, {-1.2870, 36.7800}, {-1.2840, 36.7550}}))
	p.f(`<path class="map-road" d="%s"/>`, path([][2]float64{{-1.2830, 36.8195}, {-1.3020, 36.8100}, {-1.3120, 36.8080}}))
	p.f(`</g>`)
	// Road names along the lines.
	rx, ry := px(-1.2622, 36.7790)
	p.f(`<text class="map-roadname" x="%s" y="%s" transform="rotate(8 %s %s)">Waiyaki Way</text>`, n1(rx), n1(ry-6), n1(rx), n1(ry-6))
	ex, ey := px(-1.3050, 36.8270)
	p.f(`<text class="map-roadname" x="%s" y="%s" transform="rotate(-38 %s %s)">Expressway</text>`, n1(ex+8), n1(ey), n1(ex+8), n1(ey))
	// The river's name sits above its open stretch east of the city centre.
	v0x, v0y := px(-1.2835, 36.8330)
	v1x, v1y := px(-1.2860, 36.8480)
	vt := 0.3
	vx, vy := v0x+(v1x-v0x)*vt, v0y+(v1y-v0y)*vt-7
	va := math.Atan2(v1y-v0y, v1x-v0x) * 180 / math.Pi
	p.f(`<text class="map-rivername" x="%s" y="%s" transform="rotate(%s %s %s)">Nairobi River</text>`, n1(vx), n1(vy), n1(va), n1(vx), n1(vy))
	kx, ky := px(-1.2380, 36.8330)
	p.f(`<text class="map-area" x="%s" y="%s" text-anchor="middle">Karura Forest</text>`, n1(kx), n1(ky))

	// Distance rings from the site.
	sx, sy := px(siteLat, siteLon)
	for _, km := range []float64{2, 5} {
		r := km * perKm
		p.f(`<circle class="map-ring" cx="%s" cy="%s" r="%s"/>`, n1(sx), n1(sy), n1(r))
		p.f(`<text class="map-ringlabel" x="%s" y="%s" text-anchor="middle">%s km</text>`, n1(sx), n1(sy+r-6), n1(km))
	}
	// Places.
	for _, pl := range Places {
		if pl.Name == "Karura Forest" {
			x, y := px(pl.Lat, pl.Lon)
			p.f(`<circle class="map-dot" cx="%s" cy="%s" r="4"/>`, n1(x), n1(y+18))
			p.f(`<text class="map-time" x="%s" y="%s" text-anchor="middle">%s min</text>`, n1(x), n1(y+38), pl.Minutes)
			continue
		}
		x, y := px(pl.Lat, pl.Lon)
		if pl.Edge {
			// An arrow on the map's edge pointing the way.
			dx, dy := x-sx, y-sy
			l := math.Hypot(dx, dy)
			ux, uy := dx/l, dy/l
			t := math.Inf(1)
			if ux > 0 {
				t = math.Min(t, (W-edgePad-sx)/ux)
			} else if ux < 0 {
				t = math.Min(t, (edgePad-sx)/ux)
			}
			if uy > 0 {
				t = math.Min(t, (H-edgePad-sy)/uy)
			} else if uy < 0 {
				t = math.Min(t, (edgePad-sy)/uy)
			}
			ax, ay := sx+ux*t, sy+uy*t
			ang := math.Atan2(uy, ux) * 180 / math.Pi
			p.f(`<g class="map-edge" transform="translate(%s %s) rotate(%s)"><path d="M-12 -6 L0 0 L-12 6"/></g>`, n1(ax), n1(ay), n1(ang))
			anchor, lx := "end", ax-18
			if ux < 0 {
				anchor, lx = "start", ax+18
			}
			ly := ay - 12
			p.f(`<text class="map-place" x="%s" y="%s" text-anchor="%s">%s</text>`, n1(lx), n1(ly), anchor, pl.Name)
			p.f(`<text class="map-time" x="%s" y="%s" text-anchor="%s">%s min</text>`, n1(lx), n1(ly+16), anchor, pl.Minutes)
			continue
		}
		p.f(`<circle class="map-dot" cx="%s" cy="%s" r="4"/>`, n1(x), n1(y))
		lx := x + 10
		if pl.Anchor == "end" {
			lx = x - 10
		}
		p.f(`<text class="map-place" x="%s" y="%s" text-anchor="%s">%s</text>`, n1(lx), n1(y-2+pl.DY), pl.Anchor, pl.Name)
		p.f(`<text class="map-time" x="%s" y="%s" text-anchor="%s">%s min</text>`, n1(lx), n1(y+14+pl.DY), pl.Anchor, pl.Minutes)
	}
	// Oriel itself.
	p.f(`<g class="map-site"><rect x="%s" y="%s" width="%s" height="%s"/><text x="%s" y="%s" text-anchor="end">Oriel</text></g>`,
		n1(sx-siteMarker), n1(sy-siteMarker), n1(siteMarker*2), n1(siteMarker*2), n1(sx-16), n1(sy+5))
	// North arrow and scale bar.
	p.f(`<g class="map-north" transform="translate(%s 46)"><circle r="20"/><path d="M0 -16 L7 9 L0 4 L-7 9 Z"/><text y="-26" text-anchor="middle">N</text></g>`, n1(W-46))
	bar := 2 * perKm
	p.f(`<g class="map-scale" transform="translate(28 %s)"><rect width="%s" height="6"/><rect class="map-scale-half" width="%s" height="6"/><text y="-8">0</text><text x="%s" y="-8" text-anchor="middle">1</text><text x="%s" y="-8" text-anchor="middle">2 km</text></g>`,
		n1(H-28), n1(bar), n1(bar/2), n1(bar/2), n1(bar))
	p.f(`</svg>`)
	return p.String()
}
