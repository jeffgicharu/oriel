package floorplan

import (
	"fmt"
	"html"
	"math"
	"strings"
)

// Options adjust how a plan is drawn.
type Options struct {
	// North is the direction of north on the drawing, in degrees clockwise
	// from straight up.
	North float64
	// ID prefixes the drawing's internal ids, so several plans can share a page.
	ID string
	// Plain leaves out furniture and dimensions, for small drawings.
	Plain bool
	// Title is read out by screen readers.
	Title string
}

// The drawing is in centimetres.
const u = 100.0

func c(v float64) string { return trim(v * u) }

func trim(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	s = strings.TrimSuffix(s, ".0")
	if s == "-0" {
		s = "0"
	}
	return s
}

type svgBuilder struct {
	strings.Builder
}

func (b *svgBuilder) f(format string, args ...any) {
	fmt.Fprintf(&b.Builder, format, args...)
}

func (b *svgBuilder) rect(class string, r Rect, extra string) {
	b.f(`<rect class="%s" x="%s" y="%s" width="%s" height="%s"%s/>`, class, c(r.X0), c(r.Y0), c(r.W()), c(r.H()), extra)
}

func (b *svgBuilder) line(class string, x0, y0, x1, y1 float64) {
	b.f(`<path class="%s" d="M%s %sL%s %s"/>`, class, c(x0), c(y0), c(x1), c(y1))
}

// Bounds is the rectangle the plan and everything outside it occupy.
func (p Plan) Bounds() Rect {
	b := Rect{0, 0, p.W, p.D}
	grow := func(r Rect) {
		if r.W() <= 0 || r.H() <= 0 {
			return
		}
		b.X0 = math.Min(b.X0, r.X0)
		b.Y0 = math.Min(b.Y0, r.Y0)
		b.X1 = math.Max(b.X1, r.X1)
		b.Y1 = math.Max(b.Y1, r.Y1)
	}
	grow(p.Bay)
	grow(p.Outdoor)
	for _, r := range p.OutdoorMore {
		grow(r)
	}
	return b
}

// SVG draws the plan. Layers are grouped (walls, glazing, doors, fittings,
// dimensions, labels) so that a page can bring them in one after another.
func (p Plan) SVG(opt Options) string {
	b := &svgBuilder{}
	id := opt.ID
	if id == "" {
		id = "fp-" + strings.ToLower(p.Type)
	}
	bounds := p.Bounds()
	left, top, right, bottom := 0.7, 1.35, 1.25, 0.9
	if opt.Plain {
		// The north arrow stands in the right margin, clear of the plan.
		left, top, right, bottom = 0.3, 0.3, 1.15, 0.6
	}
	vx := bounds.X0 - left
	vy := bounds.Y0 - top
	vw := bounds.W() + left + right
	vh := bounds.H() + top + bottom
	b.f(`<svg class="floorplan" viewBox="%s %s %s %s" role="img" aria-labelledby="%s-title" xmlns="http://www.w3.org/2000/svg">`, c(vx), c(vy), c(vw), c(vh), id)
	title := opt.Title
	if title == "" {
		title = "Floor plan of type " + p.Type
	}
	b.f(`<title id="%s-title">%s</title>`, id, html.EscapeString(title))
	b.f(`<defs><pattern id="%[1]s-wet" width="14" height="14" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><path d="M0 0V14" class="fp-hatch"/></pattern>`, id)
	b.f(`<pattern id="%[1]s-out" width="22" height="22" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><path d="M0 0V22" class="fp-hatch-out"/></pattern></defs>`, id)

	// Outdoor space and the bay sit outside the walls.
	b.f(`<g class="fp-outside">`)
	outdoor := append([]Rect{p.Outdoor}, p.OutdoorMore...)
	for _, r := range outdoor {
		if r.W() > 0 && r.H() > 0 {
			b.rect("fp-outdoor", r, fmt.Sprintf(` fill="url(#%s-out)"`, id))
		}
	}
	if p.Bay.W() > 0 {
		b.rect("fp-bay", p.Bay, "")
		seat := p.Bay
		inset := 0.14
		seat = Rect{seat.X0 + inset, seat.Y0 + inset, seat.X1 - inset, seat.Y1 - inset}
		b.rect("fp-seat", seat, "")
	}
	b.f(`</g>`)

	// Floors.
	b.f(`<g class="fp-floors">`)
	for _, room := range p.Rooms {
		for _, r := range room.Rects {
			b.rect("fp-floor", r, "")
			if room.Kind == Wet {
				b.rect("fp-floor", r, fmt.Sprintf(` fill="url(#%s-wet)"`, id))
			}
		}
	}
	b.f(`</g>`)

	// Walls: rooms outlined, then the outer walls heavier.
	b.f(`<g class="fp-walls">`)
	for _, room := range p.Rooms {
		for _, r := range room.Rects {
			b.rect("fp-wall", r, ` pathLength="1"`)
		}
	}
	b.rect("fp-outer", Rect{0, 0, p.W, p.D}, ` pathLength="1"`)
	b.f(`</g>`)

	// Openings, windows and doors cut through the walls.
	b.f(`<g class="fp-glazing">`)
	for _, o := range p.Openings {
		b.cut(o, 0.3)
		b.line("fp-open", o.X0, o.Y0, o.X1, o.Y1)
	}
	for _, w := range p.Windows {
		b.cut(w, 0.3)
		dx, dy := normal(w)
		for _, off := range []float64{-0.08, 0, 0.08} {
			class := "fp-window"
			if off == 0 {
				class = "fp-glass"
			}
			b.line(class, w.X0+dx*off, w.Y0+dy*off, w.X1+dx*off, w.Y1+dy*off)
		}
	}
	if p.Bay.W() > 0 {
		b.bayGlass(p)
	}
	b.f(`</g>`)

	b.f(`<g class="fp-doors">`)
	for _, d := range p.Doors {
		b.door(d)
	}
	b.f(`</g>`)

	if !opt.Plain {
		b.f(`<g class="fp-items">`)
		for _, it := range p.Items {
			b.item(p, it)
		}
		b.f(`</g>`)
		b.dimensions(p, bounds)
	}

	b.labels(p, opt.Plain)
	b.north(bounds, opt)
	b.f(`</svg>`)
	return b.String()
}

// normal is the unit vector across a wall segment.
func normal(l Line) (float64, float64) {
	if l.X0 == l.X1 {
		return 1, 0
	}
	return 0, 1
}

// cut paints over a wall where an opening, window or door goes.
func (b *svgBuilder) cut(l Line, thickness float64) {
	x0, x1 := math.Min(l.X0, l.X1), math.Max(l.X0, l.X1)
	y0, y1 := math.Min(l.Y0, l.Y1), math.Max(l.Y0, l.Y1)
	if x0 == x1 {
		b.rect("fp-cut", Rect{x0 - thickness/2, y0, x0 + thickness/2, y1}, "")
	} else {
		b.rect("fp-cut", Rect{x0, y0 - thickness/2, x1, y0 + thickness/2}, "")
	}
}

// bayGlass draws the glass on the bay's outward faces.
func (b *svgBuilder) bayGlass(p Plan) {
	r := p.Bay
	faces := []Line{}
	if r.Y0 < 0 {
		faces = append(faces, Line{r.X0, r.Y0, r.X1, r.Y0})
	}
	if r.Y1 > p.D {
		faces = append(faces, Line{r.X0, r.Y1, r.X1, r.Y1})
	}
	if r.X0 < 0 {
		faces = append(faces, Line{r.X0, r.Y0, r.X0, r.Y1})
	}
	if r.X1 > p.W {
		faces = append(faces, Line{r.X1, r.Y0, r.X1, r.Y1})
	}
	if !p.BayCorner {
		// A bay on a face shows glass on its three outward sides.
		faces = append(faces, Line{r.X0, r.Y0, r.X0, r.Y1}, Line{r.X1, r.Y0, r.X1, r.Y1})
	}
	for _, f := range faces {
		dx, dy := normal(f)
		b.line("fp-bayglass", f.X0+dx*0.04, f.Y0+dy*0.04, f.X1+dx*0.04, f.Y1+dy*0.04)
		b.line("fp-bayglass", f.X0-dx*0.04, f.Y0-dy*0.04, f.X1-dx*0.04, f.Y1-dy*0.04)
	}
}

func (b *svgBuilder) door(d Door) {
	var l Line
	if d.Vertical {
		l = Line{d.X, d.Y, d.X, d.Y + d.Width}
	} else {
		l = Line{d.X, d.Y, d.X + d.Width, d.Y}
	}
	b.cut(l, 0.3)
	// Hinge, and the leaf swung open at right angles to the wall.
	hx, hy := l.X0, l.Y0
	ex, ey := l.X1, l.Y1
	if d.HingeAtEnd {
		hx, hy, ex, ey = l.X1, l.Y1, l.X0, l.Y0
	}
	into := float64(d.Into)
	var lx, ly float64
	if d.Vertical {
		lx, ly = hx+into*d.Width, hy
	} else {
		lx, ly = hx, hy+into*d.Width
	}
	class := "fp-door"
	if d.Entrance || d.Service {
		class = "fp-door fp-door--main"
	}
	b.line(class, hx, hy, lx, ly)
	// The arc from the open leaf back to the closed position.
	sweep := 0
	cross := (lx-hx)*(ey-hy) - (ly-hy)*(ex-hx)
	if cross > 0 {
		sweep = 1
	}
	b.f(`<path class="fp-swing" d="M%s %sA%s %s 0 0 %d %s %s"/>`, c(lx), c(ly), c(d.Width), c(d.Width), sweep, c(ex), c(ey))
	if d.Entrance || d.Service {
		label := "Entrance"
		if d.Service {
			label = "Service door"
		}
		mx, my := (l.X0+l.X1)/2, (l.Y0+l.Y1)/2
		if d.Vertical {
			b.f(`<text class="fp-note" x="%s" y="%s" text-anchor="middle" transform="rotate(90 %s %s)">%s</text>`, c(mx+0.42), c(my), c(mx+0.42), c(my), label)
		} else {
			b.f(`<path class="fp-arrow" d="M%s %sl-9 -16h18z"/>`, c(mx), c(my+0.24))
			b.f(`<text class="fp-note" x="%s" y="%s" text-anchor="middle">%s</text>`, c(mx), c(my+0.55), label)
		}
	}
}

// roomAt finds the room containing a point.
func (p Plan) roomAt(x, y float64) (Room, Rect, bool) {
	for _, room := range p.Rooms {
		for _, r := range room.Rects {
			if x >= r.X0 && x <= r.X1 && y >= r.Y0 && y <= r.Y1 {
				return room, r, true
			}
		}
	}
	return Room{}, Rect{}, false
}

func (b *svgBuilder) item(p Plan, it Item) {
	r := it.R
	_, room, _ := p.roomAt((r.X0+r.X1)/2, (r.Y0+r.Y1)/2)
	// The side of the item against a wall: a bed's head, a sofa's back.
	side := "top"
	best := math.Abs(r.Y0 - room.Y0)
	for _, s := range []struct {
		name string
		d    float64
	}{{"bottom", math.Abs(room.Y1 - r.Y1)}, {"left", math.Abs(r.X0 - room.X0)}, {"right", math.Abs(room.X1 - r.X1)}} {
		if s.d < best {
			side, best = s.name, s.d
		}
	}
	switch it.Kind {
	case "bed":
		b.rect("fp-item", r, ` rx="6"`)
		// Two pillows at the head, and the turned-down sheet.
		var p1, p2 Rect
		const d = 0.4
		switch side {
		case "top", "bottom":
			y0 := r.Y0 + 0.08
			if side == "bottom" {
				y0 = r.Y1 - 0.08 - d
			}
			mid := (r.X0 + r.X1) / 2
			p1 = Rect{r.X0 + 0.1, y0, mid - 0.05, y0 + d}
			p2 = Rect{mid + 0.05, y0, r.X1 - 0.1, y0 + d}
			fy := y0 + d + 0.25
			if side == "bottom" {
				fy = y0 - 0.25
			}
			b.line("fp-item", r.X0, fy, r.X1, fy)
		default:
			x0 := r.X0 + 0.08
			if side == "right" {
				x0 = r.X1 - 0.08 - d
			}
			mid := (r.Y0 + r.Y1) / 2
			p1 = Rect{x0, r.Y0 + 0.1, x0 + d, mid - 0.05}
			p2 = Rect{x0, mid + 0.05, x0 + d, r.Y1 - 0.1}
			fx := x0 + d + 0.25
			if side == "right" {
				fx = x0 - 0.25
			}
			b.line("fp-item", fx, r.Y0, fx, r.Y1)
		}
		b.rect("fp-item", p1, ` rx="5"`)
		b.rect("fp-item", p2, ` rx="5"`)
	case "sofa":
		b.rect("fp-item", r, ` rx="8"`)
		back := 0.22
		switch side {
		case "top":
			b.line("fp-item", r.X0, r.Y0+back, r.X1, r.Y0+back)
		case "bottom":
			b.line("fp-item", r.X0, r.Y1-back, r.X1, r.Y1-back)
		case "left":
			b.line("fp-item", r.X0+back, r.Y0, r.X0+back, r.Y1)
		default:
			b.line("fp-item", r.X1-back, r.Y0, r.X1-back, r.Y1)
		}
	case "table", "desk", "island", "bedside":
		rx := ` rx="4"`
		if it.Kind == "table" {
			rx = ` rx="10"`
		}
		b.rect("fp-item", r, rx)
	case "counter":
		b.rect("fp-item", r, "")
		// A hob of four rings and a sink on a long run.
		long := r.W() > r.H()
		length := math.Max(r.W(), r.H())
		if length >= 1.8 {
			for i := 0; i < 4; i++ {
				ox, oy := 0.25+float64(i%2)*0.3, 0.15+float64(i/2)*0.27
				if !long {
					ox, oy = 0.15+float64(i%2)*0.27, 0.25+float64(i/2)*0.3
				}
				cx, cy := r.X0+ox, r.Y0+oy
				if long {
					cx = r.X1 - ox
				}
				b.f(`<circle class="fp-item" cx="%s" cy="%s" r="8"/>`, c(cx), c(cy))
			}
			sx, sy := (r.X0+r.X1)/2, (r.Y0+r.Y1)/2
			b.f(`<rect class="fp-item" x="%s" y="%s" width="%s" height="%s" rx="6"/>`, c(sx-0.22), c(sy-0.17), c(0.44), c(0.34))
		}
	case "wardrobe", "shelves":
		b.rect("fp-item", r, "")
		if r.W() > r.H() {
			b.f(`<path class="fp-item fp-dash" d="M%s %sH%s"/>`, c(r.X0+0.05), c((r.Y0+r.Y1)/2), c(r.X1-0.05))
		} else {
			b.f(`<path class="fp-item fp-dash" d="M%s %sV%s"/>`, c((r.X0+r.X1)/2), c(r.Y0+0.05), c(r.Y1-0.05))
		}
	case "shower":
		b.rect("fp-item", r, "")
		b.line("fp-item", r.X0, r.Y0, r.X1, r.Y1)
		b.line("fp-item", r.X1, r.Y0, r.X0, r.Y1)
	case "bath":
		b.rect("fp-item", r, ` rx="8"`)
		b.f(`<rect class="fp-item" x="%s" y="%s" width="%s" height="%s" rx="14"/>`, c(r.X0+0.08), c(r.Y0+0.08), c(r.W()-0.16), c(r.H()-0.16))
	case "wc":
		cx, cy := (r.X0+r.X1)/2, (r.Y0+r.Y1)/2
		b.f(`<ellipse class="fp-item" cx="%s" cy="%s" rx="%s" ry="%s"/>`, c(cx), c(cy), c(math.Min(r.W(), r.H())*0.4), c(math.Max(r.W(), r.H())*0.32))
		b.rect("fp-item", r, ` rx="5"`)
	case "basin":
		b.rect("fp-item", r, ` rx="4"`)
		cx, cy := (r.X0+r.X1)/2, (r.Y0+r.Y1)/2
		b.f(`<ellipse class="fp-item" cx="%s" cy="%s" rx="%s" ry="%s"/>`, c(cx), c(cy), c(r.W()*0.32), c(r.H()*0.32))
	case "washer":
		b.rect("fp-item", r, ` rx="4"`)
		b.f(`<circle class="fp-item" cx="%s" cy="%s" r="%s"/>`, c((r.X0+r.X1)/2), c((r.Y0+r.Y1)/2), c(math.Min(r.W(), r.H())*0.32))
	default:
		b.rect("fp-item", r, "")
	}
}

// dimensions draws the overall width above the plan and the depth beside it.
func (b *svgBuilder) dimensions(p Plan, bounds Rect) {
	b.f(`<g class="fp-dims">`)
	y := bounds.Y0 - 0.55
	b.line("fp-dim", 0, y, p.W, y)
	b.line("fp-dim", 0, y-0.12, 0, y+0.12)
	b.line("fp-dim", p.W, y-0.12, p.W, y+0.12)
	b.line("fp-dim fp-ext", 0, y+0.12, 0, bounds.Y0-0.08)
	b.line("fp-dim fp-ext", p.W, y+0.12, p.W, bounds.Y0-0.08)
	b.f(`<text class="fp-dimtext" x="%s" y="%s" text-anchor="middle">%s m</text>`, c(p.W/2), c(y-0.14), metres(p.W))
	x := bounds.X1 + 0.5
	b.line("fp-dim", x, 0, x, p.D)
	b.line("fp-dim", x-0.12, 0, x+0.12, 0)
	b.line("fp-dim", x-0.12, p.D, x+0.12, p.D)
	b.f(`<text class="fp-dimtext" x="%s" y="%s" text-anchor="middle" transform="rotate(90 %s %s)">%s m</text>`, c(x+0.16), c(p.D/2), c(x+0.16), c(p.D/2), metres(p.D))
	b.f(`</g>`)
}

// labels names each room and gives its size, inside its largest rectangle.
func (b *svgBuilder) labels(p Plan, plain bool) {
	b.f(`<g class="fp-labels">`)
	for _, room := range p.Rooms {
		best := room.Rects[0]
		for _, r := range room.Rects[1:] {
			if r.Area() > best.Area() {
				best = r
			}
		}
		cx, cy := (best.X0+best.X1)/2, (best.Y0+best.Y1)/2
		name := room.Name
		size := 25.0
		textW := func(s string, fs float64) float64 { return float64(len([]rune(s))) * fs * 0.0058 }
		if textW(name, size) > best.W()-0.2 {
			size = 19
		}
		rotate := false
		if textW(name, size) > best.W()-0.15 {
			if best.H() > best.W() && textW(name, size) < best.H()-0.2 {
				rotate = true
			} else {
				continue
			}
		}
		showSize := !plain && room.Kind != Circulation && best.H() > 1.15 && textW(room.Size(), 17) < best.W()-0.15 && !rotate
		if showSize {
			cy -= 0.12
		}
		transform := ""
		if rotate {
			transform = fmt.Sprintf(` transform="rotate(-90 %s %s)"`, c(cx), c(cy))
		}
		b.f(`<text class="fp-name" x="%s" y="%s" text-anchor="middle" dominant-baseline="middle" font-size="%s"%s>%s</text>`, c(cx), c(cy), trim(size), transform, html.EscapeString(name))
		if showSize {
			b.f(`<text class="fp-size" x="%s" y="%s" text-anchor="middle" dominant-baseline="middle">%s</text>`, c(cx), c(cy+0.3), room.Size())
		}
	}
	if p.Bay.W() > 0 {
		b.f(`<text class="fp-note fp-note--bay" x="%s" y="%s" text-anchor="middle">Oriel bay</text>`, c((p.Bay.X0+p.Bay.X1)/2), c(p.Bay.Y0-0.12))
	}
	if p.Outdoor.W() > 1.5 && p.Outdoor.H() > 0.6 {
		label := p.OutdoorName
		b.f(`<text class="fp-note" x="%s" y="%s" text-anchor="middle" dominant-baseline="middle">%s</text>`, c((p.Outdoor.X0+p.Outdoor.X1)/2), c((p.Outdoor.Y0+p.Outdoor.Y1)/2), label)
	}
	b.f(`</g>`)
}

// north draws the north arrow at the top right.
func (b *svgBuilder) north(bounds Rect, opt Options) {
	x := bounds.X1 + 0.55
	y := bounds.Y0 - 0.85
	if opt.Plain {
		x, y = bounds.X1+0.6, bounds.Y0+0.7
	}
	b.f(`<g class="fp-north" transform="translate(%s %s) rotate(%s)"><circle r="34"/><path d="M0 -30L11 14L0 6L-11 14Z"/><text y="-40" text-anchor="middle" transform="rotate(%s 0 -48)">N</text></g>`,
		c(x), c(y), trim(opt.North), trim(-opt.North))
}
