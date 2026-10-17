// Package pricing works out what each home costs from the published price list.
//
// All money is whole Kenyan shillings in int64. Percentages are basis points
// (1% = 100), so no step ever uses floating point.
package pricing

import (
	"fmt"

	"github.com/jeffgicharu/oriel/internal/building"
)

// List is one published price list.
type List struct {
	// Base is the list price per type: the home on level 2 without a river view.
	Base map[string]int64
	// FloorBP is the premium per floor above level 2, in basis points.
	FloorBP int64
	// RiverBP is the premium for homes on the river side, in basis points.
	RiverBP int64
}

// Launch is the price list Oriel launched with.
func Launch() List {
	base := map[string]int64{}
	for code, t := range building.Types {
		base[code] = t.ListPrice
	}
	return List{Base: base, FloorBP: 80, RiverBP: 400}
}

// Rounding is the step every price is rounded to.
const Rounding = 10_000

// Price is a home's price on this list.
func (l List) Price(u building.Unit) int64 {
	base := l.Base[u.Type.Code]
	floors := int64(u.Level - building.FirstHomeLevel)
	if floors < 0 {
		floors = 0
	}
	floor := 10_000 + l.FloorBP*floors
	river := int64(10_000)
	if u.Position.RiverView {
		river += l.RiverBP
	}
	// base × floor/10000 × river/10000, rounded to the nearest 10,000.
	num := base * floor * river
	const den = 10_000 * 10_000
	return roundTo(num, den*Rounding) * Rounding
}

// roundTo divides num by den and rounds half up.
func roundTo(num, den int64) int64 {
	return (num + den/2) / den
}

// Validate reports a price list a manager should not be able to publish.
func (l List) Validate() error {
	for _, code := range building.TypeOrder {
		p, ok := l.Base[code]
		if !ok {
			return fmt.Errorf("type %s has no price", code)
		}
		if p < 1_000_000 || p > 500_000_000 {
			return fmt.Errorf("type %s at KES %d is outside KES 1M to 500M", code, p)
		}
	}
	if l.FloorBP < 0 || l.FloorBP > 500 {
		return fmt.Errorf("the floor premium must be between 0%% and 5%%")
	}
	if l.RiverBP < 0 || l.RiverBP > 2_000 {
		return fmt.Errorf("the river premium must be between 0%% and 20%%")
	}
	return nil
}

// Percent writes basis points as a percentage: 80 → "0.8%".
func Percent(bp int64) string {
	whole := bp / 100
	frac := bp % 100
	switch {
	case frac == 0:
		return fmt.Sprintf("%d%%", whole)
	case frac%10 == 0:
		return fmt.Sprintf("%d.%d%%", whole, frac/10)
	default:
		return fmt.Sprintf("%d.%02d%%", whole, frac)
	}
}
