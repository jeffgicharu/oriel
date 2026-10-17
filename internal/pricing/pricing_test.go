package pricing

import (
	"testing"

	"github.com/jeffgicharu/oriel/internal/building"
)

func price(t *testing.T, id string) int64 {
	t.Helper()
	u, ok := building.UnitByID(id)
	if !ok {
		t.Fatalf("no home %s", id)
	}
	return Launch().Price(u)
}

func TestLaunchPrices(t *testing.T) {
	cases := map[string]int64{
		"0205": 11_400_000, // A, level 2, south: the list price itself
		"0204": 17_600_000, // B, level 2, south
		"0201": 24_910_000, // C, level 2, river: 23.95M × 1.04 = 24.908M
		"0203": 28_600_000, // D, level 2, river: 27.5M × 1.04
		"1601": 58_000_000, // P, level 16, river: 50.15M × 1.112 × 1.04 = 57.997M
		"1502": 13_090_000, // A, level 15, river: 11.4M × 1.104 × 1.04 = 13.089M
	}
	for id, want := range cases {
		if got := price(t, id); got != want {
			t.Errorf("%s: KES %d, want KES %d", id, got, want)
		}
	}
}

func TestPricesRiseWithEveryFloor(t *testing.T) {
	for _, pos := range []string{"01", "02", "03", "04", "05", "06"} {
		previous := int64(0)
		for level := 2; level <= 15; level++ {
			id := twoDigits(level) + pos
			p := price(t, id)
			if p <= previous {
				t.Errorf("%s at KES %d is not dearer than the floor below (KES %d)", id, p, previous)
			}
			previous = p
		}
	}
}

func TestEveryPriceIsRounded(t *testing.T) {
	for _, u := range building.Units() {
		if p := Launch().Price(u); p%Rounding != 0 {
			t.Errorf("%s: KES %d is not a multiple of %d", u.ID, p, Rounding)
		}
	}
}

func TestValidate(t *testing.T) {
	l := Launch()
	if err := l.Validate(); err != nil {
		t.Fatalf("launch list rejected: %v", err)
	}
	l.FloorBP = 900
	if l.Validate() == nil {
		t.Error("a 9% floor premium was accepted")
	}
	l = Launch()
	l.Base["A"] = 5_000
	if l.Validate() == nil {
		t.Error("a KES 5,000 home was accepted")
	}
}

func TestPercent(t *testing.T) {
	for bp, want := range map[int64]string{80: "0.8%", 400: "4%", 125: "1.25%", 0: "0%"} {
		if got := Percent(bp); got != want {
			t.Errorf("Percent(%d) = %q, want %q", bp, got, want)
		}
	}
}

func twoDigits(n int) string {
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
