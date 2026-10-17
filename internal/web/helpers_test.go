package web

import "testing"

func TestKES(t *testing.T) {
	cases := map[int64]string{
		0:          "KES 0",
		999:        "KES 999",
		1000:       "KES 1,000",
		17_600_000: "KES 17,600,000",
		-2_500:     "KES -2,500",
	}
	for in, want := range cases {
		if got := KES(in); got != want {
			t.Errorf("KES(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestKESShort(t *testing.T) {
	cases := map[int64]string{
		11_400_000: "KES 11.4M",
		58_000_000: "KES 58.0M",
		750_000:    "KES 750K",
	}
	for in, want := range cases {
		if got := KESShort(in); got != want {
			t.Errorf("KESShort(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestElevationHasEveryBay(t *testing.T) {
	// Three south-facing bays on each of levels 2 to 15, and the two
	// penthouse bays.
	if got := elevationBays(); got != 14*3+2 {
		t.Fatalf("drew %d bays, want %d", got, 14*3+2)
	}
}
