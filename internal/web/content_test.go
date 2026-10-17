package web

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var b bytes.Buffer
	if err := c.Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestTheFrameRisesOnProgramme(t *testing.T) {
	if got := FrameReached(day(2026, 6, 1)); got != 0 {
		t.Fatalf("frame before the ground slab is %v", got)
	}
	if got := FrameReached(toppingOut); got != float64(storeys) {
		t.Fatalf("frame at topping out is %v", got)
	}
	mid := FrameReached(day(2026, 10, 17))
	if mid < 6 || mid > 8 {
		t.Fatalf("frame in mid October 2026 is %v; the photographs show it near level seven", mid)
	}
	if g := ProgressOn(day(2026, 10, 17)); !strings.HasPrefix(g.Current, "Frame at level ") || g.Percent < 15 || g.Percent > 35 {
		t.Fatalf("progress in October 2026: %+v", g)
	}
	if g := ProgressOn(day(2029, 1, 10)); !g.Complete || g.Percent != 100 {
		t.Fatalf("progress after completion: %+v", g)
	}
	if g := ProgressOn(day(2025, 12, 1)); g.Percent != 0 || g.Frame != 0 {
		t.Fatalf("progress before ground broken: %+v", g)
	}
}

func TestMilestonesKnowWhereTheyStand(t *testing.T) {
	now := day(2026, 10, 17)
	seenNow := false
	for i, m := range Milestones {
		if i > 0 && m.Start.Before(Milestones[i-1].Start) {
			t.Errorf("%q starts before %q", m.Title, Milestones[i-1].Title)
		}
		if !m.End.IsZero() && m.End.Before(m.Start) {
			t.Errorf("%q ends before it starts", m.Title)
		}
		if m.Status(now) == "now" {
			seenNow = true
		}
	}
	if !seenNow {
		t.Error("nothing is under way in October 2026")
	}
	single := Milestone{Start: day(2026, 6, 15)}
	for when, want := range map[time.Time]string{day(2026, 6, 14): "next", day(2026, 6, 15).Add(10 * time.Hour): "now", day(2026, 6, 16).Add(time.Hour): "done"} {
		if got := single.Status(when); got != want {
			t.Errorf("a one-day milestone on %s is %q, want %q", when.Format("2 Jan 15:04"), got, want)
		}
	}
	if got := PhotosBy(day(2026, 9, 1)); len(got) != 2 || got[0].Image != "construction-2" {
		t.Errorf("photos by September 2026: %+v", got)
	}
}

func TestDrawingsAreWellFormed(t *testing.T) {
	for name, svg := range map[string]string{
		"sun path":    render(t, SunPath()),
		"bay section": render(t, BaySection()),
		"map":         render(t, LocationMap()),
		"progress":    render(t, ProgressDrawing(7.2)),
		"built":       render(t, ProgressDrawing(float64(storeys))),
	} {
		if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
			t.Errorf("%s is not one svg element", name)
		}
		if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
			t.Errorf("%s has a coordinate that is not a number", name)
		}
		if strings.Count(svg, "<title") != 1 {
			t.Errorf("%s needs exactly one title", name)
		}
	}
	if strings.Count(render(t, SunPath()), "<polyline") != 3 {
		t.Error("the sun path should draw three days")
	}
}

func TestPlacesSitOnTheMap(t *testing.T) {
	const lonMin, lonMax, latMax, latMin = 36.755, 36.865, -1.222, -1.312
	for _, p := range Places {
		inside := p.Lon > lonMin && p.Lon < lonMax && p.Lat < latMax && p.Lat > latMin
		if inside == p.Edge {
			t.Errorf("%s: inside the map is %v but marked as off it %v", p.Name, inside, p.Edge)
		}
		if p.Minutes == "" || p.How == "" {
			t.Errorf("%s has no travel time", p.Name)
		}
	}
}
