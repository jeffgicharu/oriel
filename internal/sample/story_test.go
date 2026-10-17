package sample

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/viewings"
)

// checkStory asserts that the story is true: the counts are as intended,
// nothing that should have happened is dated after now, every home's state
// matches its reservations, and viewings fit the suite's slots.
func checkStory(s *Story) error {
	counts := map[string]int{}
	for _, u := range s.Units {
		counts[u.Status]++
	}
	want := map[string]int{"sold": SoldCount, "reserved": ReservedCount, "held": HeldCount, "withheld": len(Withheld), "available": 86 - SoldCount - ReservedCount - HeldCount - len(Withheld)}
	for status, n := range want {
		if counts[status] != n {
			return fmt.Errorf("%d homes %s, want %d", counts[status], status, n)
		}
	}

	late := func(what string, t time.Time) error {
		if t.After(s.Now) {
			return fmt.Errorf("%s at %s is after now (%s)", what, t.In(nairobi).Format(time.DateTime), s.Now.In(nairobi).Format(time.DateTime))
		}
		return nil
	}
	for _, e := range s.LeadEvents {
		if err := late("lead event "+e.Kind, e.At); err != nil {
			return err
		}
	}
	for _, e := range s.UnitEvents {
		if err := late("home event", e.At); err != nil {
			return err
		}
	}
	for _, a := range s.Activity {
		if err := late("activity "+a.Action, a.At); err != nil {
			return err
		}
	}
	for _, pl := range s.PriceLists {
		if err := late("price list", pl.PublishedAt); err != nil {
			return err
		}
	}

	// Every lead's history runs forward from its enquiry.
	created := map[int64]time.Time{}
	for _, l := range s.Leads {
		created[l.ID] = l.CreatedAt
		if err := late("lead "+l.Name+" created", l.CreatedAt); err != nil {
			return err
		}
		if l.StageChangedAt.Before(l.CreatedAt) || l.UpdatedAt.Before(l.CreatedAt) {
			return fmt.Errorf("lead %d changed before it was created", l.ID)
		}
	}
	for _, e := range s.LeadEvents {
		if e.At.Before(created[e.LeadID]) {
			return fmt.Errorf("lead %d has a %s before its enquiry", e.LeadID, e.Kind)
		}
	}

	// Homes and reservations agree.
	active := map[string][]*Reservation{}
	for _, r := range s.Reservations {
		if err := late("reservation request", r.RequestedAt); err != nil {
			return err
		}
		if !r.DecidedAt.IsZero() {
			if err := late("reservation decision", r.DecidedAt); err != nil {
				return err
			}
			if r.DecidedAt.Before(r.RequestedAt) {
				return fmt.Errorf("reservation %s decided before it was requested", r.Ref)
			}
		}
		if !r.DepositAt.IsZero() {
			if err := late("deposit", r.DepositAt); err != nil {
				return err
			}
			if r.DepositAt.Before(r.DecidedAt) {
				return fmt.Errorf("deposit on %s before its confirmation", r.Ref)
			}
		}
		if r.Price != s.priceAt(r.UnitID, r.RequestedAt) {
			return fmt.Errorf("reservation %s at KES %d, the price list then said KES %d", r.Ref, r.Price, s.priceAt(r.UnitID, r.RequestedAt))
		}
		active[r.UnitID] = append(active[r.UnitID], r)
	}
	for id, u := range s.Units {
		rs := active[id]
		switch u.Status {
		case "sold":
			if len(rs) != 1 || rs[0].Status != "completed" || rs[0].DepositAmount <= 0 {
				return fmt.Errorf("sold home %s has reservations %v", id, rs)
			}
		case "reserved":
			if len(rs) != 1 || rs[0].Status != "confirmed" {
				return fmt.Errorf("reserved home %s has reservations %v", id, rs)
			}
		case "held":
			if len(rs) != 1 || rs[0].Status != "requested" || !rs[0].HoldExpiresAt.After(s.Now) {
				return fmt.Errorf("held home %s has reservations %v", id, rs)
			}
		default:
			if len(rs) != 0 {
				return fmt.Errorf("%s home %s has a reservation", u.Status, id)
			}
		}
	}

	// Viewings fit the suite: open days, real slot times, room in the slot.
	slots := map[string]int{}
	for _, v := range s.Viewings {
		if !s.Viewing.IsSlot(v.StartsAt) {
			return fmt.Errorf("viewing %s at %s is not a slot", v.Ref, v.StartsAt.In(nairobi))
		}
		if s.blocked(v.StartsAt) {
			return fmt.Errorf("viewing %s on a closed day", v.Ref)
		}
		slots[viewings.SlotKey(v.StartsAt)]++
		if v.CreatedAt.After(s.Now) || v.CreatedAt.After(v.StartsAt) {
			return fmt.Errorf("viewing %s booked at %s for %s", v.Ref, v.CreatedAt, v.StartsAt)
		}
		switch v.Status {
		case "booked":
			if !v.StartsAt.After(s.Now) {
				return fmt.Errorf("viewing %s still booked but its time has passed", v.Ref)
			}
		default:
			if v.StartsAt.After(s.Now) {
				return fmt.Errorf("viewing %s is %s but has not happened", v.Ref, v.Status)
			}
		}
		if v.StartsAt.Before(created[v.LeadID]) {
			return fmt.Errorf("viewing %s before the lead enquired", v.Ref)
		}
	}
	for key, n := range slots {
		if n > s.Viewing.Capacity {
			return fmt.Errorf("%d parties in the %s slot", n, key)
		}
	}

	// A lead's stage matches its history.
	stage := map[int64]string{}
	for _, l := range s.Leads {
		stage[l.ID] = l.Stage
	}
	for _, r := range s.Reservations {
		want := map[string]string{"completed": "won", "confirmed": "reservation", "requested": "reservation"}[r.Status]
		if stage[r.LeadID] != want {
			return fmt.Errorf("lead %d with a %s reservation is at %s", r.LeadID, r.Status, stage[r.LeadID])
		}
	}
	if len(s.Leads) < 200 || len(s.Leads) > 260 {
		return fmt.Errorf("%d leads", len(s.Leads))
	}
	upcoming := 0
	for _, v := range s.Viewings {
		if v.Status == "booked" {
			upcoming++
		}
	}
	if upcoming < 10 {
		return fmt.Errorf("only %d viewings booked ahead", upcoming)
	}
	return nil
}

// The story holds whatever the date and whatever the hour, including the
// nightly rebuild at 02:50 UTC and odd minutes in the evening.
func TestStoryHoldsOnEveryDay(t *testing.T) {
	start := time.Date(2026, 10, 17, 2, 50, 0, 0, time.UTC)
	hours := []int{0, 7, 14, 20}
	for d := 0; d < 400; d += 3 {
		for _, h := range hours {
			now := start.AddDate(0, 0, d).Add(time.Duration(h)*time.Hour + time.Duration(d%60)*time.Minute)
			if err := checkStory(Tell(now)); err != nil {
				t.Fatalf("on %s: %v", now.Format(time.RFC3339), err)
			}
		}
	}
}

func TestStoryIsTheSameEveryNight(t *testing.T) {
	a := Tell(time.Date(2026, 10, 17, 2, 50, 0, 0, time.UTC))
	b := Tell(time.Date(2026, 10, 18, 2, 50, 0, 0, time.UTC))
	for id, u := range a.Units {
		if b.Units[id].Status != u.Status {
			t.Errorf("home %s is %s one night and %s the next", id, u.Status, b.Units[id].Status)
		}
	}
}

func TestEveryHomeIsInTheStory(t *testing.T) {
	s := Tell(time.Now())
	for _, u := range building.Units() {
		if _, ok := s.Units[u.ID]; !ok {
			t.Errorf("home %s missing", u.ID)
		}
	}
}

// The database accepts the story: every constraint, check and foreign key.
func TestBuildWritesAValidDatabase(t *testing.T) {
	ctx := context.Background()
	for _, now := range []time.Time{
		time.Date(2026, 10, 17, 2, 50, 0, 0, time.UTC),
		time.Date(2027, 3, 1, 19, 13, 0, 0, time.UTC),
	} {
		path := filepath.Join(t.TempDir(), "sample.db")
		if err := Build(ctx, path, now); err != nil {
			t.Fatalf("build on %s: %v", now, err)
		}
		db, err := store.OpenReadOnly(path)
		if err != nil {
			t.Fatal(err)
		}
		var units, leads, sold int
		db.QueryRow("SELECT count(*) FROM units").Scan(&units)
		db.QueryRow("SELECT count(*) FROM leads").Scan(&leads)
		db.QueryRow("SELECT count(*) FROM units WHERE status = 'sold'").Scan(&sold)
		db.Close()
		if units != 86 || sold != SoldCount || leads < 200 {
			t.Fatalf("database has %d homes, %d sold, %d leads", units, sold, leads)
		}
	}
}
