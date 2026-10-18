package web

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"

	"github.com/jeffgicharu/oriel/internal/programme"
	"github.com/jeffgicharu/oriel/internal/store"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var b bytes.Buffer
	if err := c.Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestDrawingsAreWellFormed(t *testing.T) {
	for name, svg := range map[string]string{
		"sun path":    render(t, SunPath()),
		"bay section": render(t, BaySection()),
		"map":         render(t, LocationMap()),
		"progress":    render(t, ProgressDrawing(7.2)),
		"built":       render(t, ProgressDrawing(programme.Storeys)),
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

func TestPlanRowsAddUp(t *testing.T) {
	for _, s := range PlanExamples(time.Date(2026, 10, 17, 0, 0, 0, 0, Nairobi)) {
		var share, amount int64
		for _, r := range PlanRows(s) {
			share += r.ShareBP
			amount += r.Amount
		}
		if share != 10000 || amount != s.Total() {
			t.Errorf("%s: rows add to %d bp and %d, want 10000 bp and %d", s.Plan.Name, share, amount, s.Total())
		}
	}
	rows := PlanRows(PlanExamples(time.Date(2026, 10, 17, 0, 0, 0, 0, Nairobi))[0])
	folded := 0
	for _, r := range rows {
		if r.Each > 0 {
			folded++
		}
	}
	if folded != 1 {
		t.Errorf("the staged plan should fold its construction instalments into one row, got %d", folded)
	}
}

func TestFunnelCountsEveryLeadAsNew(t *testing.T) {
	rows := Funnel([]store.Count{{Key: "new", N: 10}, {Key: "contacted", N: 5}, {Key: "viewing", N: 3}, {Key: "reservation", N: 1}, {Key: "won", N: 1}, {Key: "lost", N: 20}})
	if rows[0].Reached != 40 || rows[0].Percent != 100 {
		t.Fatalf("new: %+v", rows[0])
	}
	if rows[1].Reached != 10 || rows[4].Reached != 1 {
		t.Fatalf("later stages: %+v", rows)
	}
	if last := rows[len(rows)-1]; !last.Lost || last.N != 20 || last.Percent != 50 {
		t.Fatalf("lost: %+v", last)
	}
}

func TestTheElevationDrawsInAFewStrokes(t *testing.T) {
	strokes := elevationStrokes()
	lines := elevationLines()
	if len(strokes) < 10 || len(strokes) > 120 {
		t.Fatalf("%d strokes for %d lines; the drawing should trace itself in a few dozen", len(strokes), len(lines))
	}
	want, got := 0, 0
	for _, l := range lines {
		want += strings.Count(l.D, "M")
	}
	last := 0
	for _, s := range strokes {
		if s.Order < last {
			t.Fatalf("strokes are out of order: storey %d after %d", s.Order, last)
		}
		last = s.Order
		got += strings.Count(s.D, "M")
	}
	if got != want {
		t.Fatalf("the strokes hold %d subpaths, the lines %d", got, want)
	}
	if last < 15 {
		t.Fatalf("the strokes reach storey %d; the tower has seventeen", last)
	}
}
