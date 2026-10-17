package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffgicharu/oriel/internal/sample"
	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/viewings"
)

// A Saturday morning in Nairobi.
var now = time.Date(2026, 10, 17, 9, 0, 0, 0, viewings.Nairobi)

// writableSample builds the sample and opens it for writing, as a visitor's
// copy would be.
func writableSample(t *testing.T) *store.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "copy.db")
	if err := sample.Build(context.Background(), path, now); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var buyer = store.Contact{Name: "Amani Test", Email: "amani@example.com", Phone: "+254 712 345 678"}

func slotLeft(t *testing.T, db *store.DB, start time.Time) int {
	t.Helper()
	days, err := db.Calendar(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range days {
		for _, s := range d.Slots {
			if s.Start.Equal(start) {
				return s.Left
			}
		}
	}
	t.Fatalf("no slot at %s", start)
	return 0
}

func TestASlotTakesOnlyItsCapacity(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	days, err := db.Calendar(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != viewings.BookAhead+1 {
		t.Fatalf("calendar has %d days", len(days))
	}
	var start time.Time
	for _, d := range days {
		for _, s := range d.Slots {
			if s.Left > 0 && start.IsZero() {
				start = s.Start
			}
		}
	}
	if start.IsZero() {
		t.Fatal("no free slot in the fortnight")
	}
	left := slotLeft(t, db, start)
	for i := 0; i < left; i++ {
		c := buyer
		c.Email = strings.Replace(c.Email, "@", string(rune('a'+i))+"@", 1)
		if _, err := db.BookViewing(ctx, store.ViewingRequest{Contact: c, Start: start, PartySize: 2}, now); err != nil {
			t.Fatalf("booking place %d: %v", i+1, err)
		}
	}
	if got := slotLeft(t, db, start); got != 0 {
		t.Fatalf("slot shows %d left once full", got)
	}
	if _, err := db.BookViewing(ctx, store.ViewingRequest{Contact: buyer, Start: start, PartySize: 2}, now); !errors.Is(err, store.ErrSlotGone) {
		t.Fatalf("booking a full slot gave %v", err)
	}
}

func TestClosedTimesCannotBeBooked(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	at := func(day, h, m int) time.Time { return time.Date(2026, 10, day, h, m, 0, 0, viewings.Nairobi) }
	for name, start := range map[string]time.Time{
		"a Monday":                at(19, 10, 0),
		"inside two hours":        at(17, 10, 45),
		"between slots":           at(21, 10, 20),
		"after the suite closes":  at(21, 18, 0),
		"more than 14 days ahead": at(17, 10, 0).AddDate(0, 0, 21),
	} {
		if _, err := db.BookViewing(ctx, store.ViewingRequest{Contact: buyer, Start: start, PartySize: 1}, now); !errors.Is(err, store.ErrSlotClosed) {
			t.Errorf("%s: got %v, want ErrSlotClosed", name, err)
		}
	}
	days, _ := db.Calendar(ctx, now)
	for _, d := range days {
		if d.Date.Weekday() == time.Monday && (d.Open || len(d.Slots) > 0) {
			t.Errorf("%s is open", d.Date.Format("Mon 2 Jan"))
		}
		if d.Date.Weekday() != time.Monday && !d.Open && d.Reason == "" {
			t.Errorf("%s is closed with no reason given", d.Date.Format("Mon 2 Jan"))
		}
		for _, s := range d.Slots {
			if s.Start.Before(now.Add(viewings.MinNotice)) {
				t.Errorf("slot %s is offered with under two hours' notice", s.Start)
			}
		}
	}
}

func TestCancellingFreesThePlace(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	start := time.Date(2026, 10, 22, 14, 45, 0, 0, viewings.Nairobi)
	before := slotLeft(t, db, start)
	if before == 0 {
		t.Skip("the sample filled this slot")
	}
	v, err := db.BookViewing(ctx, store.ViewingRequest{Contact: buyer, Start: start, PartySize: 3, UnitID: "0704", Notes: "Coming with my parents."}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Ref, "V-") || len(v.Token) < 20 {
		t.Fatalf("viewing ref %q, token %q", v.Ref, v.Token)
	}
	got, err := db.ViewingByRef(ctx, v.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != buyer.Name || got.UnitID != "0704" || got.PartySize != 3 || !got.StartsAt.Equal(start) || got.Status != "booked" {
		t.Fatalf("read back %+v", got)
	}
	if slotLeft(t, db, start) != before-1 {
		t.Fatal("booking did not take a place")
	}
	for i := 0; i < 2; i++ {
		if err := db.CancelViewing(ctx, v.Ref, now, 0); err != nil {
			t.Fatalf("cancel %d: %v", i+1, err)
		}
	}
	if slotLeft(t, db, start) != before {
		t.Fatal("cancelling did not free the place")
	}
	if got, _ := db.ViewingByRef(ctx, v.Ref); got.Status != "cancelled" {
		t.Fatalf("status after cancelling is %q", got.Status)
	}
}

func availableHome(t *testing.T, db *store.DB) store.Home {
	t.Helper()
	c, err := db.LoadCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range c.Homes {
		if h.OnSale() {
			return h
		}
	}
	t.Fatal("no home for sale")
	return store.Home{}
}

func homeStatus(t *testing.T, db *store.DB, id string) string {
	t.Helper()
	c, err := db.LoadCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := c.Home(id)
	if !ok {
		t.Fatalf("no home %s", id)
	}
	return h.Status
}

func TestAReservationHoldsTheHomeAndLapses(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	h := availableHome(t, db)
	r, err := db.RequestReservation(ctx, store.ReservationRequest{Contact: buyer, UnitID: h.ID, Plan: "staged"}, h.Price, now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "requested" || !r.HoldExpiresAt.Equal(now.Add(store.HoldLength)) || r.Price != h.Price {
		t.Fatalf("reservation %+v", r)
	}
	if got := homeStatus(t, db, h.ID); got != store.Held {
		t.Fatalf("home is %s, want held", got)
	}
	if _, err := db.RequestReservation(ctx, store.ReservationRequest{Contact: buyer, UnitID: h.ID, Plan: "cash"}, h.Price, now.Add(time.Hour)); !errors.Is(err, store.ErrNotAvailable) {
		t.Fatalf("reserving a held home gave %v", err)
	}

	// A minute before the hold ends nothing changes; a minute after, the
	// home is released.
	if err := db.LapseHolds(ctx, r.HoldExpiresAt.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := homeStatus(t, db, h.ID); got != store.Held {
		t.Fatalf("hold lapsed early: home is %s", got)
	}
	if err := db.LapseHolds(ctx, r.HoldExpiresAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := homeStatus(t, db, h.ID); got != store.Available {
		t.Fatalf("after the hold the home is %s", got)
	}
	if got, _ := db.ReservationByRef(ctx, r.Ref); got.Status != "lapsed" {
		t.Fatalf("reservation is %s after the hold", got.Status)
	}
}

func TestConfirmingIssuesTheNextOfferNumber(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	existing := map[string]bool{}
	all, err := db.Reservations(ctx, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all {
		if r.OfferNumber != "" {
			if existing[r.OfferNumber] {
				t.Fatalf("the sample has offer %s twice", r.OfferNumber)
			}
			existing[r.OfferNumber] = true
		}
	}

	h := availableHome(t, db)
	r, err := db.RequestReservation(ctx, store.ReservationRequest{Contact: buyer, UnitID: h.ID, Plan: "mortgage"}, h.Price, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ConfirmReservation(ctx, r.Ref, 1, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ := db.ReservationByRef(ctx, r.Ref)
	if got.Status != "confirmed" || !strings.HasPrefix(got.OfferNumber, "OR-2026-") || existing[got.OfferNumber] {
		t.Fatalf("confirmed as %q with offer %q", got.Status, got.OfferNumber)
	}
	if homeStatus(t, db, h.ID) != store.Reserved {
		t.Fatal("a confirmed home is not reserved")
	}
	if err := db.ConfirmReservation(ctx, r.Ref, 1, now.Add(3*time.Hour)); err == nil {
		t.Fatal("confirmed the same reservation twice")
	}
	if err := db.RecordDeposit(ctx, r.Ref, h.Price/10, 1, now.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if homeStatus(t, db, h.ID) != store.Sold {
		t.Fatal("a home with its deposit paid is not sold")
	}
	if err := db.DeclineReservation(ctx, r.Ref, "changed mind", 1, now.Add(72*time.Hour)); err == nil {
		t.Fatal("declined a completed reservation")
	}
}

func TestDecliningFreesTheHome(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	h := availableHome(t, db)
	r, err := db.RequestReservation(ctx, store.ReservationRequest{Contact: buyer, UnitID: h.ID, Plan: "cash"}, h.Price, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeclineReservation(ctx, r.Ref, "The buyer chose another home.", 1, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if homeStatus(t, db, h.ID) != store.Available {
		t.Fatal("a declined home is not for sale again")
	}
	got, _ := db.ReservationByRef(ctx, r.Ref)
	if got.Status != "declined" || got.DeclineReason != "The buyer chose another home." {
		t.Fatalf("declined as %+v", got)
	}
}

func TestOneBuyerIsOneLead(t *testing.T) {
	ctx := context.Background()
	db := writableSample(t)
	first, err := db.Enquire(ctx, buyer, "Is there parking?", now)
	if err != nil {
		t.Fatal(err)
	}
	again := buyer
	again.Email = "AMANI@example.com"
	second, err := db.Enquire(ctx, again, "And a gym?", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("the same buyer became leads %d and %d", first, second)
	}
}
