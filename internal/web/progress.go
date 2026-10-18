package web

import (
	"fmt"
	"math"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/programme"
)

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
	p.f(`<title id="pd-title">The frame today: %s</title>`, programme.FrameLevelName(frame))
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
		case level == built && frame < programme.Storeys:
			class = "pd-now"
		}
		p.f(`<rect class="%s" x="%s" y="%s" width="%s" height="%s"/>`, class, n1(left+inset), n1(y(y1)), n1(width-2*inset), n1((y1-y0)*k))
		if level <= building.TopLevel && level%2 == 0 {
			p.f(`<text class="pd-level" x="%s" y="%s" text-anchor="end" dominant-baseline="middle">%s</text>`, n1(left-10), n1(y((y0+y1)/2)), building.LevelName(level))
		}
	}
	// The tower crane stands in the lift core and climbs with the frame.
	if frame > 0 && frame < programme.Storeys {
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
	if frame > 0 && frame < programme.Storeys {
		ty := y(building.LevelElevation(int(frame)) + 1.6)
		p.f(`<g class="pd-today"><line x1="%s" y1="%s" x2="%s" y2="%s"/><text x="%s" y="%s" dominant-baseline="middle">Today</text></g>`,
			n1(left+width+4), n1(ty), n1(left+width+26), n1(ty), n1(left+width+30), n1(ty))
	}
	p.f(`</svg>`)
	return templ.Raw(p.String())
}

func progressTitle(g programme.Progress) string {
	switch {
	case g.Complete:
		return "Complete."
	case g.Frame > 0 && g.Frame < programme.Storeys:
		return "Rising on Riverside."
	case g.Frame >= programme.Storeys:
		return "Topped out."
	}
	return "Breaking ground."
}

func progressWidth(percent int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("width: %d%%", percent))
}

func sitePhotoSizes(i int) string {
	if i == 0 {
		return "(min-width: 1520px) 1408px, 92vw"
	}
	return "(min-width: 1000px) 50vw, 100vw"
}
