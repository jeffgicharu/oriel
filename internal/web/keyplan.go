package web

import (
	"fmt"
	"strings"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/building"
)

// KeyPlan draws the floor the home is on, seen from above with north up,
// with the home picked out and the river to the north: the small plan an
// architect puts in the corner of a sheet to show where a drawing belongs.
func KeyPlan(u building.Unit) templ.Component {
	const s = 9.0 // pixels to the metre
	ox, oz := -building.XMin+3, -building.ZMin+8
	x := func(v float64) float64 { return (v + ox) * s }
	z := func(v float64) float64 { return (v + oz) * s }
	w := (building.PlateWidth + 6) * s
	h := (building.PlateDepth + 14) * s
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="keyplan" viewBox="0 0 %.0f %.0f" role="img" aria-label="Where home %s sits on level %d">`, w, h, u.ID, u.Level)
	// The river beyond the north edge.
	fmt.Fprintf(&b, `<path class="keyplan__river" d="M0 %.1fC%.1f %.1f %.1f %.1f %.1f %.1f"/>`, z(building.ZMin-5.5), w*0.3, z(building.ZMin-3.5), w*0.65, z(building.ZMin-7), w, z(building.ZMin-5))
	fmt.Fprintf(&b, `<text class="keyplan__note" x="%.1f" y="%.1f">River</text>`, x(building.XMin), z(building.ZMin-6.6))
	positions := building.Positions
	if u.Level == building.PenthouseLevel {
		positions = building.PenthousePositions
	}
	// The core and corridor on typical floors.
	if u.Level != building.PenthouseLevel {
		c := building.Core
		fmt.Fprintf(&b, `<rect class="keyplan__core" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`, x(c.X0), z(c.Z0), c.Width()*s, c.Depth()*s)
	}
	for _, p := range positions {
		f := p.Footprint
		class := "keyplan__home"
		if p.Number == u.Position.Number {
			class += " is-this"
		}
		fmt.Fprintf(&b, `<rect class="%s" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`, class, x(f.X0), z(f.Z0), f.Width()*s, f.Depth()*s)
		bay := p.Bay.Plan
		fmt.Fprintf(&b, `<rect class="%s keyplan__bay" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`, class, x(bay.X0), z(bay.Z0), bay.Width()*s, bay.Depth()*s)
		numClass := "keyplan__num"
		if p.Number == u.Position.Number {
			numClass += " is-this"
		}
		fmt.Fprintf(&b, `<text class="%s" x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="middle">%02d</text>`, numClass, x((f.X0+f.X1)/2), z((f.Z0+f.Z1)/2), p.Number)
	}
	// North.
	fmt.Fprintf(&b, `<g class="keyplan__north" transform="translate(%.1f %.1f)"><path d="M0 -12L5 6L0 2L-5 6Z"/><text y="-16" text-anchor="middle">N</text></g>`, w-16, 26.0)
	b.WriteString(`</svg>`)
	return templ.Raw(b.String())
}

// MiniElevation draws the tower's levels as bands with the home's level
// picked out, for the side of a home's page.
func MiniElevation(level int) templ.Component {
	var b strings.Builder
	const w, floor = 120.0, 9.0
	top := 12.0
	h := top + float64(building.TopLevel+1)*floor + 14
	fmt.Fprintf(&b, `<svg class="minielev" viewBox="0 0 %.0f %.0f" role="img" aria-label="Level %d of %d">`, w+70, h, level, building.TopLevel)
	for l := 0; l <= building.TopLevel; l++ {
		y := top + float64(building.TopLevel-l)*floor
		x0, x1 := 10.0, w
		if l == building.PenthouseLevel {
			x0, x1 = 16, w-6
		}
		class := "minielev__floor"
		if l == level {
			class += " is-this"
		}
		if l < building.FirstHomeLevel {
			class += " is-shared"
		}
		fmt.Fprintf(&b, `<rect class="%s" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`, class, x0, y+1, x1-x0, floor-2)
		// Labels for the top, the ground and the home's own level, never two
		// close enough to touch.
		near := l != level && (l-level == 1 || level-l == 1 || (level <= 2 && l == 0))
		if (l == level || l == building.TopLevel || l == 0) && !(near && l != level) {
			label := LevelName(l)
			if l == 0 {
				label = "G · Lobby"
			}
			fmt.Fprintf(&b, `<text class="minielev__label%s" x="%.1f" y="%.1f" dominant-baseline="middle">%s</text>`, map[bool]string{true: " is-this", false: ""}[l == level], w+8, y+floor/2, label)
		}
	}
	fmt.Fprintf(&b, `<path class="minielev__ground" d="M0 %.1fH%.1f"/>`, top+float64(building.TopLevel+1)*floor+1, w+10)
	b.WriteString(`</svg>`)
	return templ.Raw(b.String())
}

// LevelName is "L07".
func LevelName(l int) string { return building.LevelName(l) }
