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
	var avif, webp []string
	for _, w := range photoWidths {
		a := assets.URL("img/photos/" + name + "-" + strconv.Itoa(w) + ".avif")
		b := assets.URL("img/photos/" + name + "-" + strconv.Itoa(w) + ".webp")
		if a == "" || b == "" {
			return PhotoSource{}, false
		}
		avif = append(avif, a+" "+strconv.Itoa(w)+"w")
		webp = append(webp, b+" "+strconv.Itoa(w)+"w")
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
