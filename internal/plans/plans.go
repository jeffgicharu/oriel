// Package plans works out a buyer's payment schedule on each payment plan.
//
// The same schedule is shown on a home's page, printed on the letter of
// offer and listed in the sales desk, so it is computed here once. Amounts
// are whole shillings and always add up exactly to the price.
package plans

import (
	"fmt"
	"math"
	"time"
)

var nairobi = func() *time.Location {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}()

// Plan is a way of paying for a home.
type Plan struct {
	Code    string
	Name    string
	Summary string
	// DiscountBP is taken off the price, in basis points.
	DiscountBP int64
}

// Plans are the three ways to pay, in the order they are offered.
var Plans = []Plan{
	{
		Code:    "staged",
		Name:    "Pay as it rises",
		Summary: "10% to reserve, 20% when you sign, 40% in quarterly instalments while the building goes up, and 30% when you get the keys.",
	},
	{
		Code:       "cash",
		Name:       "Cash",
		Summary:    "10% to reserve and the rest within 90 days of signing, with 7% off the price.",
		DiscountBP: 700,
	},
	{
		Code:    "mortgage",
		Name:    "Mortgage",
		Summary: "20% from you before completion, and 80% from your bank when you get the keys.",
	},
}

// ByCode finds a plan.
func ByCode(code string) (Plan, bool) {
	for _, p := range Plans {
		if p.Code == code {
			return p, true
		}
	}
	return Plan{}, false
}

// Instalment is one payment.
type Instalment struct {
	Label  string
	Due    time.Time
	Amount int64
	// Percent is the share of the price, in basis points, for display.
	PercentBP int64
	Note      string
}

// Schedule is everything a buyer pays on one plan.
type Schedule struct {
	Plan        Plan
	ListPrice   int64
	Discount    int64
	Price       int64
	Completion  time.Time
	Instalments []Instalment
	// Mortgage is set on the mortgage plan.
	Mortgage *MortgageEstimate
}

// MortgageEstimate is the bank's share and a monthly repayment estimate.
type MortgageEstimate struct {
	Loan       int64
	RateBP     int64
	Years      int
	Monthly    int64
	TotalPaid  int64
	FirstMonth time.Time
}

// Options adjust a schedule.
type Options struct {
	// From is when the reservation is made. Dates follow from it.
	From time.Time
	// Completion is when the building is due to be handed over.
	Completion time.Time
	// RateBP and Years are for the mortgage estimate.
	RateBP int64
	Years  int
}

// Mortgage defaults and limits.
const (
	DefaultRateBP = 1350
	DefaultYears  = 20
	MinRateBP     = 500
	MaxRateBP     = 2500
	MinYears      = 5
	MaxYears      = 25
)

// Build works out the schedule for a price on a plan.
func Build(plan Plan, listPrice int64, opt Options) (Schedule, error) {
	if listPrice <= 0 {
		return Schedule{}, fmt.Errorf("plans: price must be positive")
	}
	if opt.Completion.IsZero() {
		return Schedule{}, fmt.Errorf("plans: no completion date")
	}
	from := opt.From.In(nairobi)
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, nairobi)
	completion := opt.Completion
	if !day.Before(completion.AddDate(0, -1, 0)) {
		return Schedule{}, fmt.Errorf("plans: too close to completion to build a schedule")
	}

	s := Schedule{Plan: plan, ListPrice: listPrice, Completion: completion}
	s.Discount = share(listPrice, plan.DiscountBP)
	s.Price = listPrice - s.Discount

	reserve := day.AddDate(0, 0, 7)
	signing := day.AddDate(0, 0, 30)

	switch plan.Code {
	case "staged":
		deposit := share(s.Price, 1000)
		onSigning := share(s.Price, 2000)
		duringBuild := share(s.Price, 4000)
		onCompletion := s.Price - deposit - onSigning - duringBuild
		s.add("Reservation deposit", reserve, deposit, 1000, "Within 7 days of your letter of offer")
		s.add("On signing the sale agreement", signing, onSigning, 2000, "")
		quarters := quarterDates(signing, completion)
		if len(quarters) == 0 {
			onSigning += duringBuild
			s.Instalments[len(s.Instalments)-1].Amount = onSigning
			s.Instalments[len(s.Instalments)-1].PercentBP = 6000
		} else {
			each := duringBuild / int64(len(quarters))
			for i, due := range quarters {
				amount := each
				if i == len(quarters)-1 {
					amount = duringBuild - each*int64(len(quarters)-1)
				}
				s.add(fmt.Sprintf("Construction instalment %d of %d", i+1, len(quarters)), due, amount, 4000/int64(len(quarters)), "")
			}
		}
		s.add("On completion", completion, onCompletion, 3000, "When you receive the keys")
	case "cash":
		deposit := share(s.Price, 1000)
		s.add("Reservation deposit", reserve, deposit, 1000, "Within 7 days of your letter of offer")
		s.add("Balance", signing.AddDate(0, 0, 90), s.Price-deposit, 9000, "Within 90 days of signing")
	case "mortgage":
		deposit := share(s.Price, 1000)
		second := share(s.Price, 1000)
		loan := s.Price - deposit - second
		s.add("Reservation deposit", reserve, deposit, 1000, "Within 7 days of your letter of offer")
		s.add("On signing the sale agreement", signing, second, 1000, "")
		s.add("From your bank", completion, loan, 8000, "Paid by your lender on completion")
		rate, years := opt.RateBP, opt.Years
		if rate == 0 {
			rate = DefaultRateBP
		}
		if years == 0 {
			years = DefaultYears
		}
		if rate < MinRateBP || rate > MaxRateBP || years < MinYears || years > MaxYears {
			return Schedule{}, fmt.Errorf("plans: rate or term out of range")
		}
		monthly := annuity(loan, rate, years)
		s.Mortgage = &MortgageEstimate{
			Loan:       loan,
			RateBP:     rate,
			Years:      years,
			Monthly:    monthly,
			TotalPaid:  monthly * int64(years*12),
			FirstMonth: completion.AddDate(0, 1, 0),
		}
	default:
		return Schedule{}, fmt.Errorf("plans: unknown plan %q", plan.Code)
	}
	return s, nil
}

func (s *Schedule) add(label string, due time.Time, amount, bp int64, note string) {
	s.Instalments = append(s.Instalments, Instalment{Label: label, Due: due, Amount: amount, PercentBP: bp, Note: note})
}

// Total adds up the instalments.
func (s Schedule) Total() int64 {
	var t int64
	for _, i := range s.Instalments {
		t += i.Amount
	}
	return t
}

// share is bp basis points of amount, rounded to the nearest shilling.
func share(amount, bp int64) int64 {
	return (amount*bp + 5_000) / 10_000
}

// quarterDates are the instalment dates every three months after signing,
// stopping a month before completion.
func quarterDates(signing, completion time.Time) []time.Time {
	var out []time.Time
	last := completion.AddDate(0, -1, 0)
	for d := signing.AddDate(0, 3, 0); !d.After(last); d = d.AddDate(0, 3, 0) {
		out = append(out, d)
	}
	return out
}

// annuity is the monthly repayment on a loan, rounded up to the shilling.
// It is an estimate, so floating point is fine here.
func annuity(loan, rateBP int64, years int) int64 {
	r := float64(rateBP) / 10_000 / 12
	n := float64(years * 12)
	if r == 0 {
		return int64(math.Ceil(float64(loan) / n))
	}
	m := float64(loan) * r / (1 - math.Pow(1+r, -n))
	return int64(math.Ceil(m))
}
