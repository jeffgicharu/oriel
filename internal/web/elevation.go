package web

import "fmt"

// elevationLine is one stroke of the tower's south elevation, in drawing units.
type elevationLine struct {
	D     string
	Class string
}

// Elevation geometry, in the drawing's own units (one unit is about 8 cm).
const (
	elevGround = 880.0
	elevLeft   = 110.0
	elevRight  = 310.0
	elevFloorH = 40.0
)

// elevationLines draws the south face of the building: the ground, the
// glazed ground floor, sixteen floors of homes with their bays, the
// penthouse set back on top, and the pergola on the roof.
func elevationLines() []elevationLine {
	var lines []elevationLine
	add := func(class, format string, args ...any) {
		lines = append(lines, elevationLine{D: fmt.Sprintf(format, args...), Class: class})
	}

	// Ground and the site line.
	add("", "M20 %.0fH400", elevGround)

	// Ground floor: columns behind a glass line, set back under the slab.
	groundTop := elevGround - 60
	add("", "M%.0f %.0fH%.0f", elevLeft, groundTop, elevRight)
	for x := elevLeft + 10; x <= elevRight-10; x += 45 {
		add("", "M%.0f %.0fV%.0f", x, elevGround, groundTop)
	}

	// Level 1: amenities, a band of glass.
	l1Top := groundTop - 50
	add("", "M%.0f %.0fV%.0fH%.0fV%.0f", elevLeft, groundTop, l1Top, elevRight, groundTop)

	// Levels 2 to 15: the homes. Each floor carries a corner bay at both
	// ends and the bay of the one-bedroom home in the middle.
	top := l1Top
	for level := 2; level <= 15; level++ {
		y := top - elevFloorH
		add("", "M%.0f %.0fH%.0f", elevLeft, y, elevRight)
		add("bay", "M%.0f %.0fH%.0fV%.0fH%.0f", elevLeft, y+6, elevLeft-14, y+34, elevLeft)
		add("bay", "M%.0f %.0fH%.0fV%.0fH%.0f", elevRight, y+6, elevRight+14, y+34, elevRight)
		add("bay", "M%.0f %.0fh30v28h-30z", 214.0, y+6)
		top = y
	}
	add("", "M%.0f %.0fV%.0f", elevLeft, l1Top, top)
	add("", "M%.0f %.0fV%.0f", elevRight, l1Top, top)

	// Vertical fins that shade the façade from the high equatorial sun.
	for x := elevLeft + 25; x < elevRight; x += 25 {
		if x > 205 && x < 252 {
			continue
		}
		add("fin", "M%.0f %.0fV%.0f", x, l1Top, top)
	}

	// Level 16: the two penthouses, set back behind terraces.
	phTop := top - elevFloorH
	add("", "M%.0f %.0fV%.0fH%.0fV%.0f", elevLeft+18, top, phTop, elevRight-18, top)
	add("", "M%.0f %.0fH%.0f", elevLeft, top-12, elevRight)

	// The roof pergola over the pool.
	pergola := phTop - 18
	add("", "M%.0f %.0fV%.0fH%.0fV%.0f", elevLeft+40, phTop, pergola, elevRight-40, phTop)
	for x := elevLeft + 52; x < elevRight-40; x += 12 {
		add("fin", "M%.0f %.0fv6", x, pergola)
	}
	return lines
}
