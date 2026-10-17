package web

import (
	"strconv"
	"strings"

	"github.com/jeffgicharu/oriel/internal/assets"
)

// Photographs are served at three widths in AVIF, with WebP for browsers
// that lack it. A picture missing from the build is left out of the page.

var photoWidths = []int{800, 1400, 2000}

// photoShapes are each picture's width and height as made.
var photoShapes = map[string][2]int{
	"living-bay-b": {2400, 1600}, "living-bay-a": {2400, 1600}, "kitchen-b": {2400, 1600},
	"bedroom-principal": {2400, 1600}, "bathroom-ensuite": {1856, 2304}, "study-d": {2400, 1600},
	"dining-river": {2400, 1350}, "penthouse-terrace": {2400, 1350}, "penthouse-living": {2400, 1350},
	"rooftop-pool": {2400, 1350}, "gym": {2400, 1600}, "lounge-workroom": {2400, 1350},
	"lobby": {1856, 2304}, "balcony-morning": {2400, 1600}, "materials": {2400, 1600},
	"river-walk": {2400, 1350}, "forest-path": {1856, 2304}, "neighbourhood-street": {2400, 1350},
	"sales-suite": {2400, 1600}, "facade-detail-day": {1856, 2304}, "facade-bays-dusk": {1856, 2304},
	"construction-1": {2400, 1600}, "construction-2": {2400, 1600}, "construction-3": {2400, 1600},
}

// PhotoSource is a picture ready for the page.
type PhotoSource struct {
	AVIF, WebP    string
	Fallback      string
	Width, Height int
}

// PhotoFor finds a picture's files in the build.
func PhotoFor(name string) (PhotoSource, bool) {
	shape, ok := photoShapes[name]
	if !ok {
		return PhotoSource{}, false
	}
	// Every picture has 800 and 1400 in both formats; landscapes also have
	// a 2000 AVIF for wide, sharp screens.
	var avif, webp []string
	for _, w := range photoWidths {
		a := assets.URL("img/photos/" + name + "-" + strconv.Itoa(w) + ".avif")
		b := assets.URL("img/photos/" + name + "-" + strconv.Itoa(w) + ".webp")
		if w <= 1400 && (a == "" || b == "") {
			return PhotoSource{}, false
		}
		if a != "" {
			avif = append(avif, a+" "+strconv.Itoa(w)+"w")
		}
		if b != "" {
			webp = append(webp, b+" "+strconv.Itoa(w)+"w")
		}
	}
	return PhotoSource{
		AVIF:     strings.Join(avif, ", "),
		WebP:     strings.Join(webp, ", "),
		Fallback: assets.URL("img/photos/" + name + "-1400.webp"),
		Width:    shape[0],
		Height:   shape[1],
	}, true
}

// HasPhoto reports whether a picture is in the build.
func HasPhoto(name string) bool {
	_, ok := PhotoFor(name)
	return ok
}

// Shot is a photograph with what it shows.
type Shot struct {
	Name, Alt, Caption string
}

// Portrait reports whether the picture is taller than it is wide.
func (s Shot) Portrait() bool {
	shape := photoShapes[s.Name]
	return shape[1] > shape[0]
}

// TypeShots are the interiors shown for each type of home, in order.
var TypeShots = map[string][]Shot{
	"A": {
		{"living-bay-a", "A living room with a three-sided glazed bay, an oak window seat and a cobalt throw, with gardens and jacarandas outside", "The living room and its bay, which looks three ways."},
		{"bathroom-ensuite", "A bathroom with a walk-in shower behind a glass panel, terrazzo floors and an oak vanity", "The bathroom: terrazzo, oak and a walk-in shower."},
	},
	"B": {
		{"living-bay-b", "A woman reading on an oak window seat in a corner bay, with a cobalt cushion and treetops outside", "The corner bay of the living room, on a February morning."},
		{"kitchen-b", "A couple cooking at a pale stone island in an open kitchen with oak cabinets and tall windows", "The kitchen, open to the living room."},
	},
	"C": {
		{"dining-river", "A family at an oak dining table in a long living room with tall windows onto trees", "The long living and dining room, on the river side."},
		{"bedroom-principal", "A principal bedroom with a low oak bed, linen curtains and a door to the balcony", "The principal bedroom, with a tall door to the outside."},
	},
	"D": {
		{"study-d", "A study behind a graphite-framed glass partition, with oak shelves and a desk at the window", "The study, behind a glass partition off the living room."},
		{"balcony-morning", "Hands around a coffee cup on a balcony with a glass balustrade, above green treetops", "Morning on the balcony, above the treetops."},
	},
	"P": {
		{"penthouse-living", "A penthouse living room with glass on two sides, a corner bay, a low sofa and a cobalt chair", "The penthouse living room, glazed on two sides."},
		{"penthouse-terrace", "A couple sitting on a timber terrace with planters and a glass balustrade at sunset", "The terrace that wraps the top floor."},
	},
}

// ShotsFor lists the type's pictures that are in the build.
func ShotsFor(code string) []Shot {
	var out []Shot
	for _, s := range TypeShots[code] {
		if HasPhoto(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

func gallerySizes(i int) string {
	if i == 0 {
		return "(min-width: 1000px) 60vw, 100vw"
	}
	return "(min-width: 1000px) 38vw, 100vw"
}
