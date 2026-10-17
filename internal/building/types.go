package building

// Type is a kind of home: every home of a type shares its plan and size.
type Type struct {
	Code string
	// Name is written out in full: "Two bedrooms".
	Name string
	// Short is the compact form for cards and tables: "2 bed".
	Short     string
	Bedrooms  int
	Bathrooms int
	Study     bool
	// DSQ is a staff room with its own shower room, as Nairobi buyers expect
	// in larger homes.
	DSQ bool
	// Internal is the floor area inside the home, in square metres.
	Internal int
	// Outdoor is the balcony or terrace, in square metres.
	Outdoor     int
	OutdoorName string
	Parking     int
	// ListPrice is the starting list price: the home on level 2 without a
	// river view. Floor and river premiums are added to it.
	ListPrice int64
	Summary   string
	Features  []string
}

// Types are Oriel's five kinds of home.
var Types = map[string]Type{
	"A": {
		Code: "A", Name: "One bedroom", Short: "1 bed",
		Bedrooms: 1, Bathrooms: 1,
		Internal: 72, Outdoor: 8, OutdoorName: "Balcony", Parking: 1,
		ListPrice: 11_400_000,
		Summary:   "A one-bedroom home with an open living room built around its bay, a window seat deep enough to read in, and a bedroom that opens to the balcony.",
		Features: []string{
			"Living room bay with oak window seat",
			"Galley kitchen with stone worktops",
			"Bedroom opening to the balcony",
			"Walk-in shower and storage hall",
		},
	},
	"B": {
		Code: "B", Name: "Two bedrooms", Short: "2 bed",
		Bedrooms: 2, Bathrooms: 2,
		Internal: 112, Outdoor: 12, OutdoorName: "Balcony", Parking: 1,
		ListPrice: 17_600_000,
		Summary:   "A corner home with two bedrooms and two bathrooms. The bay wraps the corner of the living room, so it looks two ways at once.",
		Features: []string{
			"Corner bay looking two ways",
			"Principal bedroom with en suite and dressing area",
			"Second bedroom beside its own bathroom",
			"Open kitchen with island",
		},
	},
	"C": {
		Code: "C", Name: "Three bedrooms", Short: "3 bed",
		Bedrooms: 3, Bathrooms: 3,
		Internal: 158, Outdoor: 16, OutdoorName: "Balcony", Parking: 2,
		ListPrice: 23_950_000,
		Summary:   "Three bedrooms on the river side of the building, with the corner bay at the end of a long living and dining room.",
		Features: []string{
			"River-side corner with bay",
			"Living and dining room 11 metres long",
			"Two en suite bedrooms",
			"Separate kitchen and laundry",
		},
	},
	"D": {
		Code: "D", Name: "Three bedrooms, study and DSQ", Short: "3 bed + study",
		Bedrooms: 3, Bathrooms: 3, Study: true, DSQ: true,
		Internal: 176, Outdoor: 18, OutdoorName: "Balcony", Parking: 2,
		ListPrice: 27_500_000,
		Summary:   "The largest typical home: three bedrooms, a study, and staff quarters with their own entrance, on the north-east corner where the morning sun arrives first.",
		Features: []string{
			"North-east corner bay, first light of the day",
			"Study off the living room",
			"DSQ with its own shower room and service door",
			"Kitchen with pantry and laundry",
		},
	},
	"P": {
		Code: "P", Name: "Penthouse", Short: "Penthouse",
		Bedrooms: 4, Bathrooms: 4, Study: true, DSQ: true,
		Internal: 310, Outdoor: 120, OutdoorName: "Terrace", Parking: 3,
		ListPrice: 50_150_000,
		Summary:   "One of two homes on the top floor, set back behind a terrace on three sides, with the pool and garden directly above.",
		Features: []string{
			"Terrace on three sides, 120 m²",
			"Four en suite bedrooms",
			"Two bays, at the corners",
			"Private lift lobby",
		},
	},
}

// TypeOrder is the order types are listed in.
var TypeOrder = []string{"A", "B", "C", "D", "P"}

// OrderedTypes lists the types in order.
func OrderedTypes() []Type {
	out := make([]Type, 0, len(TypeOrder))
	for _, code := range TypeOrder {
		out = append(out, Types[code])
	}
	return out
}

// CountOfType is how many homes of a type the building has.
func CountOfType(code string) int {
	n := 0
	for _, u := range units {
		if u.Type.Code == code {
			n++
		}
	}
	return n
}
