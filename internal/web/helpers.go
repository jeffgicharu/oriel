package web

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

func jsonLDID(i int) string {
	return "ld-" + strconv.Itoa(i)
}

// current marks the link to the page the visitor is on.
func current(isCurrent bool) templ.Attributes {
	if isCurrent {
		return templ.Attributes{"aria-current": "page"}
	}
	return templ.Attributes{}
}

// KES formats a whole number of shillings as "KES 17,600,000".
func KES(amount int64) string {
	return "KES " + Thousands(amount)
}

// Thousands writes a whole number with commas between thousands.
func Thousands(n int64) string {
	negative := n < 0
	if negative {
		n = -n
	}
	digits := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	if negative {
		return "-" + b.String()
	}
	return b.String()
}

// KESShort writes a price as "KES 17.6M", for headings and labels.
func KESShort(amount int64) string {
	switch {
	case amount >= 1_000_000:
		// Always one decimal, so figures line up in a column: 19.0M beside 18.7M.
		m := float64(amount) / 1_000_000
		return "KES " + strconv.FormatFloat(m, 'f', 1, 64) + "M"
	case amount >= 1_000:
		return fmt.Sprintf("KES %dK", amount/1_000)
	default:
		return KES(amount)
	}
}
