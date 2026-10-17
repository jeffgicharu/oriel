package floorplan

// The five plans. Each is drawn as the home on the left of its pair would be
// seen with its main façade at the top; handed homes use the mirror image.

var plans = map[string]Plan{
	"A": typeA,
	"B": typeB,
	"C": typeC,
	"D": typeD,
	"P": typeP,
}

func r(x0, y0, x1, y1 float64) Rect { return Rect{x0, y0, x1, y1} }
func l(x0, y0, x1, y1 float64) Line { return Line{x0, y0, x1, y1} }
func rooms(rs ...Room) []Room       { return rs }
func room(name, kind string, rects ...Rect) Room {
	return Room{Name: name, Kind: kind, Rects: rects}
}

// hd is a door in a horizontal wall at y, from x for width metres.
func hd(x, y, width float64, into int) Door {
	return Door{X: x, Y: y, Width: width, Into: into}
}

// vd is a door in a vertical wall at x, from y for width metres.
func vd(x, y, width float64, into int) Door {
	return Door{X: x, Y: y, Width: width, Vertical: true, Into: into}
}

func entrance(x, y, width float64) Door {
	return Door{X: x, Y: y, Width: width, Into: -1, Entrance: true}
}

func it(kind string, x0, y0, x1, y1 float64) Item { return Item{Kind: kind, R: Rect{x0, y0, x1, y1}} }

// One bedroom: 7.2 × 10 m, facing one way.
var typeA = Plan{
	Type: "A", W: 7.2, D: 10,
	Rooms: rooms(
		room("Living", Living, r(3.3, 0, 7.2, 4.5)),
		room("Kitchen & dining", Living, r(3.3, 4.5, 7.2, 7.3)),
		room("Bedroom", Bedroom, r(0, 0, 3.3, 4.5)),
		room("Bathroom", Wet, r(0, 4.5, 2.4, 7.3)),
		room("Hall", Circulation, r(2.4, 7.3, 5.0, 10), r(2.4, 4.5, 3.3, 7.3)),
		room("Laundry", Service, r(0, 7.3, 2.4, 10)),
		room("Store", Service, r(5.0, 7.3, 7.2, 10)),
	),
	Doors: []Door{
		entrance(3.1, 10, 0.95),
		hd(2.45, 4.5, 0.8, -1),
		vd(2.4, 5.3, 0.75, -1),
		vd(2.4, 8.2, 0.8, -1),
		vd(5.0, 8.2, 0.8, 1),
	},
	Openings: []Line{l(3.3, 4.5, 7.2, 4.5), l(3.3, 7.3, 5.0, 7.3), l(4.3, 0, 7.1, 0)},
	Windows:  []Line{l(0.35, 0, 3.0, 0), l(3.55, 0, 4.15, 0)},
	Bay:      r(4.3, -1.2, 7.1, 0),
	Outdoor:  r(0.2, -2.0, 4.2, 0), OutdoorName: "Balcony",
	Items: []Item{
		it("bed", 0, 1.3, 2.05, 2.95),
		it("bedside", 0, 0.8, 0.45, 1.25),
		it("bedside", 0, 3.0, 0.45, 3.45),
		it("wardrobe", 0.35, 3.9, 2.4, 4.5),
		it("sofa", 3.3, 0.9, 4.2, 3.2),
		it("table", 4.75, 1.5, 5.55, 2.6),
		it("table", 4.0, 5.3, 5.45, 6.3),
		it("counter", 6.6, 4.6, 7.2, 7.3),
		it("shower", 0, 4.5, 1.1, 5.6),
		it("wc", 0, 6.55, 0.55, 7.3),
		it("basin", 1.5, 6.8, 2.3, 7.3),
		it("washer", 0, 7.3, 0.65, 7.95),
		it("shelves", 5.0, 9.45, 7.2, 10),
	},
}

// Two bedrooms on a corner: 11.2 × 10 m, facing two ways.
var typeB = Plan{
	Type: "B", W: 11.2, D: 10,
	Rooms: rooms(
		room("Living & dining", Living, r(0, 0, 6.2, 5.4)),
		room("Kitchen", Living, r(3.6, 5.4, 6.2, 8.0)),
		room("Principal bedroom", Bedroom, r(6.2, 0, 11.2, 4.6)),
		room("Dressing", Service, r(8.4, 4.6, 11.2, 6.4)),
		room("En suite", Wet, r(8.4, 6.4, 11.2, 9.0)),
		room("Bedroom 2", Bedroom, r(0, 5.4, 3.6, 8.8)),
		room("Bathroom", Wet, r(0, 8.8, 3.6, 10)),
		room("Hall", Circulation, r(3.6, 8.0, 8.4, 10), r(6.2, 4.6, 8.4, 8.0)),
		room("Laundry", Service, r(8.4, 9.0, 11.2, 10)),
	),
	Doors: []Door{
		entrance(6.95, 10, 0.95),
		hd(6.45, 4.6, 0.85, -1),
		hd(9.1, 4.6, 0.8, 1),
		hd(9.45, 6.4, 0.75, 1),
		vd(8.4, 9.12, 0.75, 1),
		vd(3.6, 8.05, 0.7, -1),
		vd(3.6, 9.12, 0.75, -1),
	},
	Openings: []Line{l(3.6, 5.4, 6.2, 5.4), l(6.2, 4.6, 6.2, 5.4), l(4.5, 8.0, 6.2, 8.0), l(0, 0, 1.2, 0), l(0, 0, 0, 1.2)},
	Windows: []Line{
		l(1.3, 0, 5.8, 0), l(6.6, 0, 10.8, 0),
		l(0, 1.3, 0, 5.0), l(0, 5.8, 0, 8.4), l(0, 9.1, 0, 9.7),
	},
	Bay: r(-1.2, -1.2, 1.2, 1.2), BayCorner: true,
	Outdoor: r(2.8, -1.6, 10.3, 0), OutdoorName: "Balcony",
	Items: []Item{
		it("sofa", 1.6, 3.0, 4.0, 3.9),
		it("table", 2.2, 1.7, 3.4, 2.5),
		it("table", 4.5, 1.1, 5.6, 2.9),
		it("counter", 5.6, 5.5, 6.2, 8.0),
		it("island", 4.1, 6.2, 5.0, 7.6),
		it("bed", 8.1, 2.45, 9.9, 4.6),
		it("bedside", 7.55, 4.1, 8.05, 4.6),
		it("bedside", 9.95, 4.1, 10.45, 4.6),
		it("wardrobe", 8.6, 5.85, 11.2, 6.4),
		it("basin", 8.4, 7.0, 8.9, 7.9),
		it("shower", 10.1, 6.4, 11.2, 7.5),
		it("wc", 10.55, 8.2, 11.2, 8.95),
		it("bed", 0.9, 6.75, 2.5, 8.8),
		it("shower", 0, 8.8, 1.0, 10),
		it("wc", 1.35, 9.35, 1.9, 10),
		it("basin", 2.25, 9.5, 3.05, 10),
		it("washer", 10.5, 9.05, 11.2, 10),
	},
}

// Three bedrooms on the river corner: 15.8 × 10 m.
var typeC = Plan{
	Type: "C", W: 15.8, D: 10,
	Rooms: rooms(
		room("Living & dining", Living, r(0, 0, 7.0, 5.0)),
		room("Kitchen", Living, r(3.6, 5.0, 7.0, 7.8)),
		room("Principal bedroom", Bedroom, r(11.2, 0, 15.8, 4.8)),
		room("Dressing", Service, r(12.6, 4.8, 15.8, 6.4)),
		room("En suite", Wet, r(12.6, 6.4, 15.8, 8.8)),
		room("Bedroom 2", Bedroom, r(7.0, 0, 11.2, 4.2)),
		room("En suite", Wet, r(9.0, 4.2, 11.2, 6.6)),
		room("Bedroom 3", Bedroom, r(0, 5.0, 3.6, 8.6)),
		room("Bathroom", Wet, r(0, 8.6, 3.6, 10)),
		room("Hall", Circulation, r(3.6, 7.8, 12.6, 10), r(7.0, 4.2, 9.0, 7.8), r(11.2, 4.8, 12.6, 7.8)),
		room("Linen", Service, r(9.0, 6.6, 11.2, 7.8)),
		room("Laundry", Service, r(12.6, 8.8, 15.8, 10)),
	),
	Doors: []Door{
		entrance(5.0, 10, 0.95),
		vd(3.6, 7.85, 0.7, -1),
		vd(3.6, 8.92, 0.75, -1),
		hd(7.2, 4.2, 0.8, -1),
		hd(9.7, 4.2, 0.75, 1),
		hd(9.6, 7.8, 0.7, -1),
		hd(11.4, 4.8, 0.85, -1),
		hd(13.6, 4.8, 0.8, 1),
		hd(14.4, 6.4, 0.75, 1),
		vd(12.6, 9.02, 0.75, 1),
	},
	Openings: []Line{l(3.6, 5.0, 7.0, 5.0), l(5.6, 7.8, 7.0, 7.8), l(7.0, 4.2, 7.0, 5.0), l(0, 0, 1.2, 0), l(0, 0, 0, 1.2)},
	Windows: []Line{
		l(1.3, 0, 6.6, 0), l(7.4, 0, 10.8, 0), l(11.6, 0, 15.4, 0),
		l(0, 1.3, 0, 4.6), l(0, 5.4, 0, 8.2), l(0, 9.0, 0, 9.6),
	},
	Bay: r(-1.2, -1.2, 1.2, 1.2), BayCorner: true,
	Outdoor: r(4.3, -1.6, 14.3, 0), OutdoorName: "Balcony",
	Items: []Item{
		it("sofa", 1.4, 3.15, 4.0, 4.05),
		it("table", 2.1, 1.85, 3.3, 2.75),
		it("table", 4.6, 0.9, 5.6, 2.8),
		it("counter", 3.6, 7.2, 5.6, 7.8),
		it("counter", 3.6, 5.0, 4.2, 7.2),
		it("island", 4.9, 5.7, 6.3, 6.6),
		it("bed", 8.2, 2.1, 10.0, 4.2),
		it("bed", 12.6, 2.6, 14.4, 4.8),
		it("wardrobe", 12.8, 5.85, 15.6, 6.4),
		it("bed", 1.0, 6.55, 2.6, 8.6),
		it("shower", 14.75, 6.4, 15.8, 7.5),
		it("bath", 12.6, 7.95, 14.3, 8.8),
		it("wc", 15.2, 7.95, 15.8, 8.75),
		it("basin", 12.6, 6.55, 13.1, 7.45),
		it("shower", 10.15, 4.2, 11.2, 5.2),
		it("wc", 10.6, 5.85, 11.2, 6.6),
		it("basin", 9.0, 5.6, 9.5, 6.45),
		it("shower", 0, 8.6, 1.0, 10),
		it("wc", 1.4, 9.35, 1.95, 10),
		it("basin", 2.4, 9.5, 3.2, 10),
		it("washer", 15.1, 8.8, 15.8, 9.5),
		it("washer", 14.4, 8.8, 15.1, 9.5),
	},
}

// Three bedrooms, a study and a DSQ on the morning corner: 17.6 × 10 m.
var typeD = Plan{
	Type: "D", W: 17.6, D: 10,
	Rooms: rooms(
		room("Living & dining", Living, r(11.4, 0, 17.6, 6.0)),
		room("Kitchen", Living, r(11.4, 6.0, 14.6, 8.4)),
		room("Study", Living, r(14.6, 6.0, 17.6, 8.0)),
		room("Principal bedroom", Bedroom, r(0, 0, 4.6, 4.8)),
		room("En suite", Wet, r(0, 4.8, 2.6, 7.6)),
		room("Dressing", Service, r(2.6, 4.8, 4.6, 6.6)),
		room("Bedroom 2", Bedroom, r(4.6, 0, 8.2, 4.2)),
		room("En suite", Wet, r(6.2, 4.2, 8.2, 6.6)),
		room("Bedroom 3", Bedroom, r(8.2, 0, 11.4, 4.2)),
		room("Bathroom", Wet, r(8.2, 4.2, 10.2, 6.6)),
		room("Hall", Circulation, r(2.6, 6.6, 11.4, 8.6), r(7.2, 8.6, 9.6, 10), r(4.6, 4.2, 6.2, 6.6), r(10.2, 4.2, 11.4, 6.6)),
		room("Linen", Service, r(0, 7.6, 2.6, 10)),
		room("Store", Service, r(2.6, 8.6, 7.2, 10)),
		room("WC", Wet, r(9.6, 8.6, 11.4, 10)),
		room("DSQ", Bedroom, r(14.6, 8.0, 17.6, 10)),
		room("Shower", Wet, r(11.4, 8.4, 13.0, 10)),
		room("Laundry", Service, r(13.0, 8.4, 14.6, 10)),
	),
	Doors: []Door{
		entrance(7.95, 10, 0.95),
		{X: 13.35, Y: 10, Width: 0.85, Into: -1, Service: true},
		hd(3.2, 6.6, 0.8, -1),
		hd(3.3, 4.8, 0.8, -1),
		vd(2.6, 5.25, 0.75, -1),
		vd(2.6, 7.75, 0.7, -1),
		hd(5.4, 8.6, 0.8, 1),
		hd(4.9, 4.2, 0.8, -1),
		hd(6.85, 4.2, 0.75, 1),
		hd(10.4, 4.2, 0.8, -1),
		hd(8.6, 6.6, 0.75, -1),
		vd(9.6, 8.95, 0.7, 1),
		hd(13.35, 8.4, 0.8, -1),
		vd(14.6, 8.75, 0.75, 1),
		vd(13.0, 8.75, 0.7, -1),
		hd(14.9, 6.0, 0.85, 1),
	},
	Openings: []Line{l(11.4, 6.0, 14.6, 6.0), l(11.4, 4.2, 11.4, 6.0), l(11.4, 6.8, 11.4, 8.2), l(16.4, 0, 17.6, 0), l(17.6, 0, 17.6, 1.2)},
	Windows: []Line{
		l(0.4, 0, 4.2, 0), l(5.0, 0, 7.8, 0), l(8.6, 0, 11.0, 0), l(11.8, 0, 16.3, 0),
		l(17.6, 1.3, 17.6, 5.6), l(17.6, 6.3, 17.6, 7.7), l(17.6, 8.4, 17.6, 9.6),
		l(15.85, 6.0, 17.6, 6.0),
	},
	Bay: r(16.4, -1.2, 18.8, 1.2), BayCorner: true,
	Outdoor: r(2.3, -1.6, 13.3, 0), OutdoorName: "Balcony",
	Items: []Item{
		it("bed", 1.4, 2.7, 3.2, 4.8),
		it("bed", 5.5, 2.15, 7.1, 4.2),
		it("bed", 8.95, 2.15, 10.55, 4.2),
		it("sofa", 12.2, 3.35, 15.0, 4.25),
		it("table", 13.0, 1.85, 14.3, 2.75),
		it("table", 15.75, 2.3, 16.75, 4.4),
		it("counter", 11.4, 7.8, 13.2, 8.4),
		it("island", 12.3, 6.6, 13.8, 7.3),
		it("desk", 16.0, 7.35, 17.6, 8.0),
		it("bed", 15.75, 8.05, 17.6, 9.0),
		it("wardrobe", 2.6, 6.05, 4.6, 6.6),
		it("shower", 0, 4.8, 1.05, 5.85),
		it("wc", 0, 6.8, 0.6, 7.55),
		it("basin", 1.9, 6.65, 2.6, 7.45),
		it("shower", 7.15, 5.6, 8.2, 6.6),
		it("basin", 6.2, 4.95, 6.7, 5.75),
		it("shower", 9.15, 4.2, 10.2, 5.25),
		it("basin", 8.2, 5.8, 8.75, 6.6),
		it("wc", 10.75, 9.3, 11.35, 10),
		it("shower", 11.4, 8.4, 12.4, 9.4),
		it("washer", 13.95, 9.3, 14.6, 10),
	},
}

// The penthouse: 18.1 × 18.4 m, open on three sides, with a terrace round them.
var typeP = Plan{
	Type: "P", W: 18.1, D: 18.4,
	Rooms: rooms(
		room("Living & dining", Living, r(0, 0, 9.8, 7.4)),
		room("Kitchen", Living, r(9.8, 0, 13.6, 5.4)),
		room("Family room", Living, r(13.6, 0, 18.1, 5.4)),
		room("Study", Living, r(13.6, 5.4, 18.1, 8.2)),
		room("DSQ", Bedroom, r(9.8, 5.4, 12.4, 7.4)),
		room("Shower", Wet, r(12.4, 5.4, 13.6, 7.4)),
		room("Lift lobby", Circulation, r(13.6, 8.2, 18.1, 11.0)),
		room("Hall", Circulation, r(4.4, 7.4, 13.6, 8.6), r(6.8, 8.6, 9.6, 13.4), r(9.6, 8.6, 13.6, 10.4), r(9.6, 12.6, 15.4, 13.4)),
		room("Bedroom 4", Bedroom, r(0, 7.4, 4.4, 11.4)),
		room("En suite", Wet, r(4.4, 8.6, 6.8, 11.4)),
		room("Principal bedroom", Bedroom, r(0, 11.4, 6.8, 18.4)),
		room("Dressing", Service, r(6.8, 13.4, 9.6, 14.6)),
		room("En suite", Wet, r(6.8, 14.6, 9.6, 18.4)),
		room("Bathroom", Wet, r(9.6, 10.4, 12.0, 12.6)),
		room("Powder", Wet, r(12.0, 10.4, 13.6, 12.6)),
		room("Laundry & store", Service, r(13.6, 11.0, 15.4, 12.6)),
		room("Bedroom 2", Bedroom, r(9.6, 13.4, 13.8, 18.4)),
		room("Bedroom 3", Bedroom, r(13.8, 13.4, 18.1, 18.4)),
		room("En suite", Wet, r(15.4, 11.0, 18.1, 13.4)),
	),
	Doors: []Door{
		{X: 18.1, Y: 9.05, Width: 1.1, Vertical: true, Into: -1, Entrance: true},
		vd(13.6, 9.0, 1.0, -1),
		vd(4.4, 7.6, 0.8, -1),
		vd(4.4, 9.4, 0.75, 1),
		vd(6.8, 12.45, 0.85, -1),
		vd(6.8, 13.6, 0.8, 1),
		hd(7.6, 14.6, 0.8, 1),
		hd(10.2, 13.4, 0.85, 1),
		hd(10.4, 12.6, 0.75, -1),
		hd(14.2, 13.4, 0.85, 1),
		hd(16.4, 13.4, 0.8, -1),
		hd(12.4, 10.4, 0.7, 1),
		hd(14.2, 12.6, 0.8, -1),
		hd(14.2, 8.2, 0.8, -1),
		hd(10.4, 5.4, 0.8, 1),
		vd(12.4, 5.9, 0.7, 1),
	},
	Openings: []Line{l(9.8, 0.6, 9.8, 5.0), l(13.6, 1.0, 13.6, 4.4), l(5.0, 7.4, 9.6, 7.4), l(0, 0, 1.2, 0), l(0, 0, 0, 1.2)},
	Windows: []Line{
		l(1.3, 0, 9.4, 0), l(10.2, 0, 13.2, 0), l(14.0, 0, 17.7, 0),
		l(0, 1.3, 0, 7.0), l(0, 7.8, 0, 11.0), l(0, 11.8, 0, 18.0),
		l(0.4, 18.4, 6.4, 18.4), l(7.2, 18.4, 9.2, 18.4), l(10.0, 18.4, 13.4, 18.4), l(14.2, 18.4, 17.7, 18.4),
		l(13.9, 5.4, 18.1, 5.4),
	},
	Bay: r(-1.2, -1.2, 1.2, 1.2), BayCorner: true,
	Outdoor: r(-1.8, -1.8, 18.1, 0), OutdoorName: "Terrace",
	OutdoorMore: []Rect{r(-1.8, 0, 0, 18.4), r(-1.8, 18.4, 18.1, 20.2)},
	Items: []Item{
		it("sofa", 2.0, 3.6, 5.2, 4.6),
		it("sofa", 2.0, 1.6, 2.9, 3.6),
		it("table", 3.4, 2.0, 4.8, 3.1),
		it("table", 6.6, 1.4, 8.6, 3.0),
		it("counter", 9.8, 0, 13.6, 0.6),
		it("island", 10.6, 2.0, 12.8, 3.0),
		it("sofa", 14.2, 3.4, 17.4, 4.4),
		it("desk", 16.2, 7.5, 18.1, 8.2),
		it("bed", 15.5, 7.1-1.7, 17.4, 7.4),
		it("bed", 1.2, 9.2, 3.2, 11.4),
		it("bed", 2.0, 15.9, 4.2, 18.0),
		it("bed", 10.6, 16.0, 12.6, 18.0),
		it("bed", 15.0, 16.0, 17.0, 18.0),
		it("bath", 6.8, 17.6, 8.6, 18.4),
		it("shower", 8.5, 14.6, 9.6, 15.7),
		it("basin", 6.8, 15.3, 7.35, 16.4),
		it("shower", 4.4, 8.6, 5.45, 9.65),
		it("shower", 17.0, 11.0, 18.1, 12.1),
		it("shower", 9.6, 11.5, 10.65, 12.6),
		it("wc", 13.0, 11.85, 13.6, 12.6),
		it("washer", 13.6, 11.95, 14.25, 12.6),
	},
}
