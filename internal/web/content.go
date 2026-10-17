package web

import (
	"time"

	"github.com/jeffgicharu/oriel/internal/plans"
)

// The words for the building, buying, privacy and terms pages.

// Amenity is something every resident shares.
type Amenity struct {
	Code, Name, Where, Text, Image, Alt string
}

// Amenities are the shared spaces, from the roof down.
var Amenities = []Amenity{
	{Code: "R", Name: "Rooftop pool and terrace", Where: "Roof",
		Text:  "A twenty-metre pool with a pale stone edge, a timber deck and a slatted pergola for shade, with the treetops of Riverside on every side.",
		Image: "rooftop-pool", Alt: "A long pool on the roof with a timber deck, a slatted pergola and treetops beyond"},
	{Code: "G", Name: "Residents’ gym", Where: "Level 1",
		Text:  "Cardio, free weights and a stretching studio behind a glass wall onto the garden. Open from 5 in the morning to 11 at night.",
		Image: "gym", Alt: "A bright gym with oak floors and a glass wall onto the garden"},
	{Code: "L", Name: "Lounge and workroom", Where: "Level 1",
		Text:  "Quiet desks, a long table and a meeting room residents can book, with fibre and a kitchen for coffee.",
		Image: "lounge-workroom", Alt: "A lounge with oak tables, low chairs and a long window"},
	{Code: "E", Name: "Lobby and concierge", Where: "Ground",
		Text:  "A lobby four and a half metres high, a concierge desk staffed day and night, and a cool room for parcels and deliveries.",
		Image: "lobby", Alt: "A tall lobby with a concierge desk in oak and a terrazzo floor"},
	{Code: "W", Name: "Garden and river walk", Where: "Ground",
		Text:  "A planted garden that runs down to a path along the river, kept wild at the water’s edge.",
		Image: "river-walk", Alt: "A shaded path beside a river, with tall trees and ferns"},
}

// SpecGroup is a part of the specification.
type SpecGroup struct {
	Code  string
	Title string
	Items []SpecItem
}

// SpecItem is one line of the specification.
type SpecItem struct{ Name, Text string }

// Specification is what the building is made of and what every home comes with.
var Specification = []SpecGroup{
	{Code: "S1", Title: "Structure and envelope", Items: []SpecItem{
		{"Frame", "Reinforced concrete frame and flat slabs, designed for Nairobi’s seismic zone."},
		{"Façade", "Smooth off-white render, with deep concrete fins shading the east and west faces."},
		{"Bays", "Glazed bays in graphite thermally broken aluminium, under a cobalt powder-coated band."},
		{"Glazing", "Double glazing with a low-emissivity coating throughout; tilt-and-turn windows that open."},
		{"Balconies", "Frameless glass balustrades with a slim graphite rail, and porcelain decking."},
	}},
	{Code: "S2", Title: "Inside every home", Items: []SpecItem{
		{"Floors", "Engineered oak in living rooms and bedrooms; large porcelain tiles in kitchens and bathrooms."},
		{"Ceilings", "2.8 metres clear in living rooms, lowered over each bay to frame the window seat."},
		{"Doors", "Full-height doors in oak veneer, with brushed steel handles."},
		{"Wardrobes", "Built in, in oak veneer, in every bedroom."},
		{"Window seat", "Solid oak, with storage below and a cushion in the bay."},
	}},
	{Code: "S3", Title: "Kitchens and bathrooms", Items: []SpecItem{
		{"Cabinets", "Handleless, in oak veneer or matt graphite, chosen during fit-out."},
		{"Worktops", "20 mm quartz with a matching upstand."},
		{"Appliances", "Induction hob, oven, extractor, integrated fridge-freezer and dishwasher; a washer-dryer in the laundry."},
		{"Bathrooms", "Wall-hung WC, walk-in shower with a glass screen, and a bath in the main bathroom of three-bedroom homes and penthouses."},
		{"Fittings", "Brushed steel mixers, a heated towel rail and a lit mirror cabinet."},
	}},
	{Code: "S4", Title: "Services", Items: []SpecItem{
		{"Water", "Borehole and mains, with a day’s storage for the whole building. Rainwater is kept for the garden."},
		{"Hot water", "Solar water heating on the roof, with an electric top-up."},
		{"Power", "A backup generator for every home, the lifts, the pumps and the common areas."},
		{"Lifts", "Two passenger lifts and a goods lift, reaching both basement levels and the roof."},
		{"Connectivity", "Fibre to every home from two providers."},
		{"Security", "Gatehouse staffed day and night, access control at the lobby and lifts, cameras in the common areas, and a video entryphone in each home."},
		{"Parking", "Two basement levels: one space for each one- and two-bedroom home, two for three-bedroom homes and penthouses, and charging points for electric cars."},
	}},
}

// BuyingStep is a step in buying off-plan.
type BuyingStep struct {
	Title, Text, When string
}

// BuyingSteps take a buyer from a viewing to the keys.
var BuyingSteps = []BuyingStep{
	{Title: "Choose your home", When: "Any time", Text: "Explore the building, shortlist homes and come to the sales suite to see the show apartments and the model."},
	{Title: "Request a reservation", When: "Day 1", Text: "Choose a payment plan online. The home is held for you for 72 hours while the sales team checks it is free and prepares a letter of offer."},
	{Title: "Pay the deposit", When: "Within 7 days", Text: "Pay 10% by bank transfer to the project’s escrow account. The home is then yours, and it comes off the market."},
	{Title: "Sign the sale agreement", When: "Within 30 days", Text: "Your advocate reviews the agreement for sale; you sign and pay the next instalment as your plan sets out."},
	{Title: "Pay as it rises", When: "Until completion", Text: "Instalments fall due as the building goes up. You can follow every stage on the progress page."},
	{Title: "Collect your keys", When: "December 2028", Text: "We walk through the home with you, fix anything on the snagging list, and hand over the keys and your title."},
}

// Cost is one of the costs of buying beyond the price.
type Cost struct{ Name, Amount, Note string }

// BuyingCosts are paid on top of the price.
var BuyingCosts = []Cost{
	{"Stamp duty", "4% of the price", "Paid to the government when the transfer is registered. It is 4% for homes in a town or city."},
	{"Advocate’s fees", "About 1% to 1.5%, plus VAT", "Set by the Advocates Remuneration Order. You choose your own advocate."},
	{"Service charge deposit", "Three months’ charge", "Held by the management company. The charge is estimated at KES 65 a square metre a month."},
	{"Utility deposits", "About KES 15,000", "For water and electricity meters in your name."},
	{"Mortgage costs", "If you borrow", "Your bank’s arrangement and valuation fees, and stamp duty on the charge, usually 0.1%."},
}

// Question is one of the questions on the buying page.
type Question struct{ Q, A string }

// Questions are the questions buyers ask most.
var Questions = []Question{
	{"Can I buy from outside Kenya?", "Yes. Many of our buyers live abroad. You can view by video call, sign documents with a notary or at a Kenyan embassy, and pay by international bank transfer. Each home is held as a lease, which buyers who are not Kenyan citizens can own."},
	{"What title will I get?", "Each home is registered with its own sectional title under the Sectional Properties Act, 2020. It gives you your home and a share in the land and the common areas, run by a management company the owners control."},
	{"Where does my money go before completion?", "Deposits are paid into an escrow account with the project’s bank and released only as the building reaches each stage, certified by the project’s quantity surveyor."},
	{"What if the building is late?", "The agreement for sale sets the completion date and what you are owed if it slips. You can follow every stage on the progress page."},
	{"Can I choose my finishes?", "Yes, from two kitchen colourways and two floor tones, until the fit-out freeze about a year before completion."},
	{"Is parking included?", "Yes: one space with every one- and two-bedroom home and two with three-bedroom homes and penthouses. Extra spaces can be bought while they last."},
	{"What does the service charge cover?", "Security, cleaning, the gardens and the river walk, the pool and gym, lifts, the backup generator, insurance of the building, and a fund for repairs in years to come."},
	{"Can I let my home?", "Yes. Long lets need nothing more than notice to the management company. Short lets are allowed with its approval, under the building’s house rules."},
}

// ExamplePrice is the price the buying page uses to show each plan.
const ExamplePrice = 18_000_000

// PlanExamples are the payment plans worked out on the example price.
func PlanExamples(now time.Time) []plans.Schedule {
	var out []plans.Schedule
	for _, pl := range plans.Plans {
		if s, err := plans.Build(pl, ExamplePrice, plans.Options{From: now}); err == nil {
			out = append(out, s)
		}
	}
	return out
}
