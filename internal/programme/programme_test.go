package programme

import (
	"testing"
	"time"
)

func TestTheFrameRisesOnProgramme(t *testing.T) {
	if got := frameReached(day(2026, 6, 1)); got != 0 {
		t.Fatalf("frame before the ground slab is %v", got)
	}
	if got := frameReached(toppingOut); got != Storeys {
		t.Fatalf("frame at topping out is %v", got)
	}
	if mid := frameReached(written); mid < 6 || mid > 8 {
		t.Fatalf("frame on the day the dates are written is %v; the photographs show it near level seven", mid)
	}
}

// On the day the dates are written, the site is where the photographs show it.
func TestTheProgrammeAsWritten(t *testing.T) {
	g := On(written.Add(10 * time.Hour))
	if g.Current != "Frame at level 7" || g.Percent != 24 || g.Complete {
		t.Fatalf("progress as written: %+v", g)
	}
	if !g.Completion.Equal(completion) {
		t.Fatalf("completion as written is %s", g.Completion)
	}
	if len(g.Photos) != len(photos) || g.Photos[0].Image != "construction-3" {
		t.Fatalf("photos as written: %+v", g.Photos)
	}
}

// As the calendar moves on, the programme moves with it: the site stands
// where it stood when the dates were written, completion never comes
// closer, and every photograph is already taken.
func TestTheSiteStandsStillAsTheCalendarMoves(t *testing.T) {
	first := On(written.Add(10 * time.Hour))
	for d := 0; d < 4*365; d += 5 {
		for _, h := range []int{0, 5, 13, 23} {
			now := written.AddDate(0, 0, d).Add(time.Duration(h)*time.Hour + 17*time.Minute)
			g := On(now)
			if g.Current != first.Current || g.Percent != first.Percent || int(g.Frame) != int(first.Frame) {
				t.Fatalf("on %s the site has moved: %+v", now.Format(time.DateTime), g)
			}
			if want := completion.AddDate(0, 0, d); !g.Completion.Equal(want) || !Completion(now).Equal(want) {
				t.Fatalf("on %s completion is %s, want %s", now.Format(time.DateTime), g.Completion.Format(time.DateOnly), want.Format(time.DateOnly))
			}
			if g.Completion.Before(now.AddDate(2, 0, 0)) {
				t.Fatalf("on %s completion %s is less than two years away", now.Format(time.DateTime), g.Completion.Format(time.DateOnly))
			}
			if !ShowFlatsOpen(now).After(now) {
				t.Fatalf("on %s the show apartments are already open", now.Format(time.DateTime))
			}
			for i, ph := range g.Photos {
				if ph.Taken.After(now) {
					t.Fatalf("on %s photo %s is dated %s, after today", now.Format(time.DateTime), ph.Image, ph.Taken.Format(time.DateOnly))
				}
				if i > 0 && ph.Taken.After(g.Photos[i-1].Taken) {
					t.Fatalf("on %s photos are not newest first", now.Format(time.DateTime))
				}
			}
			if len(g.Photos) != len(first.Photos) {
				t.Fatalf("on %s there are %d photos, want %d", now.Format(time.DateTime), len(g.Photos), len(first.Photos))
			}
			for i, m := range g.Milestones {
				if got, want := m.Status(now), first.Milestones[i].Status(first.Now); got != want {
					t.Fatalf("on %s %q is %s, want %s", now.Format(time.DateTime), m.Title, got, want)
				}
			}
		}
	}
}

// The day turns at midnight in Nairobi, whatever the clock's zone.
func TestTheDayTurnsAtMidnightInNairobi(t *testing.T) {
	before := time.Date(2027, 3, 1, 20, 59, 0, 0, time.UTC) // 23:59 in Nairobi
	after := before.Add(2 * time.Minute)                    // 00:01 on 2 March
	if d := Completion(after).Sub(Completion(before)); d != 24*time.Hour {
		t.Fatalf("completion moved %s across midnight in Nairobi", d)
	}
	if !Completion(before).Equal(Completion(time.Date(2027, 3, 1, 0, 5, 0, 0, nairobi))) {
		t.Fatal("completion moved within a day in Nairobi")
	}
}

func TestMilestonesKnowWhereTheyStand(t *testing.T) {
	g := On(written.Add(10 * time.Hour))
	seenNow := false
	for i, m := range g.Milestones {
		if i > 0 && m.Start.Before(g.Milestones[i-1].Start) {
			t.Errorf("%q starts before %q", m.Title, g.Milestones[i-1].Title)
		}
		if !m.End.IsZero() && m.End.Before(m.Start) {
			t.Errorf("%q ends before it starts", m.Title)
		}
		if m.Status(g.Now) == "now" {
			seenNow = true
		}
	}
	if !seenNow {
		t.Error("nothing is under way")
	}
	if last := g.Milestones[len(g.Milestones)-1]; last.Title != "Completion" || !last.Start.Equal(g.Completion) {
		t.Errorf("the programme ends with %q on %s", last.Title, last.Start.Format(time.DateOnly))
	}
	single := Milestone{Start: day(2026, 6, 15)}
	for when, want := range map[time.Time]string{day(2026, 6, 14): "next", day(2026, 6, 15).Add(10 * time.Hour): "now", day(2026, 6, 16).Add(time.Hour): "done"} {
		if got := single.Status(when); got != want {
			t.Errorf("a one-day milestone on %s is %q, want %q", when.Format("2 Jan 15:04"), got, want)
		}
	}
	if got := (Milestone{Start: day(2027, 6, 1), End: day(2028, 9, 30)}).When(); got != "Jun 2027 to Sep 2028" {
		t.Errorf("a stage over two years is written %q", got)
	}
}

func TestFrameLevelsAreNamed(t *testing.T) {
	for frame, want := range map[float64]string{0: "the ground floor", 7.3: "level 7", Storeys - 1: "the roof"} {
		if got := FrameLevelName(frame); got != want {
			t.Errorf("frame %v is named %q, want %q", frame, got, want)
		}
	}
}
