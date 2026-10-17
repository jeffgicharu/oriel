package pdf

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffgicharu/oriel/internal/floorplan"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/sample"
	"github.com/jeffgicharu/oriel/internal/store"
)

func sampleDB(t *testing.T) *store.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.db")
	if err := sample.Build(context.Background(), path, time.Date(2026, 10, 17, 9, 0, 0, 0, store.Nairobi())); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTheDocumentsDraw(t *testing.T) {
	ctx := context.Background()
	db := sampleDB(t)
	c, err := db.LoadCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	list, err := PriceList(PriceListDoc{Catalog: c, ListID: 2, AsAt: time.Now(), Nairobi: store.Nairobi(), Site: "oriel.test"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(list, []byte("%PDF-")) || len(list) < 20_000 {
		t.Fatalf("price list is %d bytes", len(list))
	}

	all, err := db.Reservations(ctx, "confirmed", 1)
	if err != nil || len(all) == 0 {
		t.Fatalf("no confirmed reservation in the sample: %v", err)
	}
	res := all[0]
	h, _ := c.Home(res.UnitID)
	plan, _ := plans.ByCode(res.Plan)
	s, err := plans.Build(plan, res.Price, plans.Options{From: res.DecidedAt})
	if err != nil {
		t.Fatal(err)
	}
	fp, _ := floorplan.For(h.Type.Code)
	offer, err := OfferLetter(Offer{Reservation: res, Home: h, HomeTitle: "Two bedrooms on level 7", Schedule: s, Plan: fp,
		Manager: "Grace Wambui", Parking: 1, Issued: res.DecidedAt, Nairobi: store.Nairobi(), Site: "oriel.test"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(offer, []byte("%PDF-")) || len(offer) < 20_000 {
		t.Fatalf("letter of offer is %d bytes", len(offer))
	}
}
