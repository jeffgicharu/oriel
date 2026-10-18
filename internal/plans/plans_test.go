package plans

import (
	"testing"
	"time"
)

// completion is the date the tests build schedules against.
var completion = at("2028-12-15 00:00")

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, nairobi)
	if err != nil {
		panic(err)
	}
	return t
}

// Every schedule adds up to the price exactly, whatever the price and
// whatever day it is built on.
func TestSchedulesAddUpOnEveryDay(t *testing.T) {
	prices := []int64{11_400_000, 17_610_001, 24_910_000, 28_600_000, 57_999_999, 13_090_000}
	for day := at("2026-10-17 02:50"); day.Before(at("2028-10-01 00:00")); day = day.Add(37*time.Hour + 11*time.Minute) {
		for _, plan := range Plans {
			for _, price := range prices {
				s, err := Build(plan, price, Options{From: day, Completion: completion})
				if err != nil {
					t.Fatalf("%s on %s: %v", plan.Code, day, err)
				}
				if s.Total() != s.Price {
					t.Fatalf("%s on %s at %d: instalments add to %d, price %d", plan.Code, day, price, s.Total(), s.Price)
				}
				if s.ListPrice-s.Discount != s.Price {
					t.Fatalf("discount does not reconcile")
				}
				for i, inst := range s.Instalments {
					if inst.Amount <= 0 {
						t.Fatalf("%s on %s: instalment %d is %d", plan.Code, day, i, inst.Amount)
					}
					if inst.Due.Before(day) {
						t.Fatalf("%s on %s: instalment %q falls due in the past (%s)", plan.Code, day, inst.Label, inst.Due)
					}
					if i > 0 && inst.Due.Before(s.Instalments[i-1].Due) {
						t.Fatalf("%s on %s: instalments out of order", plan.Code, day)
					}
				}
			}
		}
	}
}

func TestStagedPlanSplits(t *testing.T) {
	s, err := Build(Plans[0], 17_600_000, Options{From: at("2026-10-17 10:00"), Completion: completion})
	if err != nil {
		t.Fatal(err)
	}
	if s.Instalments[0].Amount != 1_760_000 {
		t.Errorf("deposit %d, want 1,760,000", s.Instalments[0].Amount)
	}
	if s.Instalments[1].Amount != 3_520_000 {
		t.Errorf("signing %d, want 3,520,000", s.Instalments[1].Amount)
	}
	last := s.Instalments[len(s.Instalments)-1]
	if last.Amount != 5_280_000 || !last.Due.Equal(completion) {
		t.Errorf("completion %d on %s", last.Amount, last.Due)
	}
	// Signing is 30 days after 17 Oct 2026: 16 Nov 2026. Quarters run from
	// 16 Feb 2027 to 16 Aug 2028; 16 Nov 2028 falls inside the last month
	// before completion, so it is not one of them.
	quarters := s.Instalments[2 : len(s.Instalments)-1]
	if len(quarters) != 7 {
		t.Fatalf("%d construction instalments, want 7", len(quarters))
	}
	if first := quarters[0].Due.Format("2006-01-02"); first != "2027-02-16" {
		t.Errorf("first construction instalment due %s", first)
	}
	if last := quarters[len(quarters)-1].Due.Format("2006-01-02"); last != "2028-08-16" {
		t.Errorf("last construction instalment due %s", last)
	}
	var during int64
	for _, q := range quarters {
		during += q.Amount
	}
	if during != 7_040_000 {
		t.Errorf("construction instalments add to %d, want 40%% = 7,040,000", during)
	}
}

func TestCashDiscount(t *testing.T) {
	s, _ := Build(Plans[1], 20_000_000, Options{From: at("2026-10-17 10:00"), Completion: completion})
	if s.Discount != 1_400_000 || s.Price != 18_600_000 {
		t.Errorf("discount %d, price %d", s.Discount, s.Price)
	}
}

func TestMortgageEstimate(t *testing.T) {
	s, err := Build(Plans[2], 17_600_000, Options{From: at("2026-10-17 10:00"), Completion: completion})
	if err != nil {
		t.Fatal(err)
	}
	m := s.Mortgage
	if m == nil || m.Loan != 14_080_000 {
		t.Fatalf("loan %+v", m)
	}
	// 14.08M at 13.5% over 20 years is about KES 170,000 a month.
	if m.Monthly < 169_000 || m.Monthly > 171_000 {
		t.Errorf("monthly %d", m.Monthly)
	}
	if _, err := Build(Plans[2], 17_600_000, Options{From: at("2026-10-17 10:00"), Completion: completion, RateBP: 9_999}); err == nil {
		t.Error("a 99.99% rate was accepted")
	}
}

func TestAScheduleNeedsACompletionDate(t *testing.T) {
	if _, err := Build(Plans[0], 17_600_000, Options{From: at("2026-10-17 10:00")}); err == nil {
		t.Error("a schedule was built without a completion date")
	}
}
