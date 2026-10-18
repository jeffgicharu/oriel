package pdf

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jeffgicharu/oriel/internal/floorplan"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/store"
)

// Offer is what a letter of offer says.
type Offer struct {
	Reservation store.Reservation
	Home        store.Home
	HomeTitle   string // "Two bedrooms on level 7"
	Schedule    plans.Schedule
	Plan        floorplan.Plan
	Manager     string // who signs it
	Parking     int
	Issued      time.Time
	Nairobi     *time.Location
	Site        string // the website's address without the scheme
}

// OfferLetter draws a buyer's letter of offer.
func OfferLetter(o Offer) ([]byte, error) {
	r := o.Reservation
	issued := o.Issued.In(o.Nairobi)
	d, err := newDoc("Letter of offer "+r.OfferNumber, "Home "+o.Home.ID+", Oriel, Riverside",
		"Oriel Residences Limited · Riverside, Nairobi · Letter of offer "+r.OfferNumber+" · Subject to contract")
	if err != nil {
		return nil, err
	}
	d.header("Letter of offer", [][2]string{
		{"Offer", r.OfferNumber},
		{"Date", issued.Format("2 January 2006")},
		{"Reservation", r.Ref},
	})

	// To.
	d.label(marginL, "To")
	d.y += 14
	d.text(marginL, d.y, semi, 11, graphite, r.Name)
	d.y += 15
	d.text(marginL, d.y, sans, 9, inkSoft, r.Email+"  ·  "+r.Phone)
	d.y += 34

	// The home's name, on two lines if it is long.
	d.y -= 2
	d.para(marginL, width, display, 19, 24, graphite, "Home "+o.Home.ID+", "+lowerFirst(o.HomeTitle))
	d.y += 12

	first := strings.Fields(r.Name)[0]
	d.para(marginL, width, sans, 10, 15, graphite, "Dear "+first+",")
	d.y += 6
	d.para(marginL, width, sans, 10, 15, graphite, fmt.Sprintf("Thank you for choosing a home at Oriel. Following your reservation on %s, we are pleased to offer you home %s on the terms below. The home is reserved for you while you pay the deposit.", r.RequestedAt.In(o.Nairobi).Format("2 January 2006"), o.Home.ID))
	d.y += 14

	// The home's terms on the left, its plan on the right.
	top := d.y
	colW := 262.0
	d.label(marginL, "The home")
	d.y += 10
	terms := [][2]string{
		{"Home", o.Home.ID + " · type " + o.Home.Type.Code},
		{"Floor", fmt.Sprintf("Level %d", o.Home.Level)},
		{"Faces", upperFirst(o.Home.Facings())},
		{"Inside", fmt.Sprintf("%d m²", o.Home.Type.Internal)},
		{o.Home.Type.OutdoorName, fmt.Sprintf("%d m²", o.Home.Type.Outdoor)},
		{"Parking", fmt.Sprintf("%d %s in the basement", o.Parking, map[bool]string{true: "space", false: "spaces"}[o.Parking == 1])},
		{"Payment plan", o.Schedule.Plan.Name},
	}
	for _, t := range terms {
		d.line(marginL, d.y, marginL+colW, d.y, 0.4, rule)
		d.y += 13
		d.text(marginL, d.y, sans, 8.5, inkSoft, t[0])
		d.text(marginL+86, d.y, sans, 9.5, graphite, t[1])
		d.y += 8
	}
	d.line(marginL, d.y, marginL+colW, d.y, 0.4, rule)
	d.y += 16
	d.text(marginL, d.y, sans, 8.5, inkSoft, "Price")
	d.text(marginL+86, d.y-3, display, 15, cobalt, kes(o.Schedule.Price))
	if o.Schedule.Discount > 0 {
		d.y += 15
		d.text(marginL+86, d.y, sans, 8, inkSoft, fmt.Sprintf("%s off the list price of %s for paying in cash", kes(o.Schedule.Discount), kes(o.Schedule.ListPrice)))
	}
	leftEnd := d.y + 10

	planX, planW := marginL+colW+24, width-colW-24
	planH := drawPlan(d, o.Plan, planX, top, planW, 200)
	d.text(planX, top+planH+12, sans, 7.5, inkSoft, "Plan of home "+o.Home.ID+". Not to scale.")
	d.y = math.Max(leftEnd, top+planH+26) + 16

	// The payments.
	d.ensure(60)
	d.label(marginL, "Payments")
	d.y += 12
	cols := []float64{marginL, marginL + 236, marginL + 330, right}
	d.text(cols[0], d.y, sans, 7.5, inkSoft, "Payment")
	d.text(cols[1], d.y, sans, 7.5, inkSoft, "Due")
	d.text(cols[2], d.y, sans, 7.5, inkSoft, "Share")
	d.rightText(cols[3], d.y, sans, 7.5, inkSoft, "Amount")
	d.y += 6
	d.line(marginL, d.y, right, d.y, 0.8, graphite)
	for _, in := range o.Schedule.Instalments {
		d.ensure(22)
		d.y += 13
		d.text(cols[0], d.y, sans, 9, graphite, in.Label)
		d.text(cols[1], d.y, mono, 8, graphite, in.Due.In(o.Nairobi).Format("2 Jan 2006"))
		d.text(cols[2], d.y, mono, 8, graphite, percent(in.PercentBP))
		d.rightText(cols[3], d.y, sans, 9, graphite, kes(in.Amount))
		d.y += 7
		d.line(marginL, d.y, right, d.y, 0.4, rule)
	}
	d.y += 14
	d.text(cols[0], d.y, semi, 9.5, graphite, "Total")
	d.rightText(cols[3], d.y, semi, 9.5, graphite, kes(o.Schedule.Total()))
	d.y += 26

	// Conditions.
	d.ensure(80)
	d.label(marginL, "Terms of this offer")
	d.y += 14
	deposit := o.Schedule.Instalments[0]
	conditions := []string{
		fmt.Sprintf("The reservation deposit of %s is due by %s, by bank transfer to the project’s escrow account. The account details are on the deposit invoice that comes with this letter.", kes(deposit.Amount), deposit.Due.In(o.Nairobi).Format("2 January 2006")),
		"Once the deposit is received the home is yours and comes off the market. If it is not received by that date, this offer lapses and the home is released.",
		"The agreement for sale follows within 30 days of the deposit, prepared by our advocates; your own advocate reviews it with you before you sign.",
		"Payments are made against the stages of construction certified by the project’s quantity surveyor. Completion is planned for " + plans.Completion.Format("January 2006") + ".",
		"Stamp duty, your advocate’s fees and the other costs of buying are paid by you, as set out at " + o.Site + "/buying.",
		"This letter is an offer and is subject to contract. Nothing is binding until the agreement for sale is signed by both parties.",
	}
	for i, c := range conditions {
		d.ensure(30)
		d.text(marginL, d.y, mono, 8, cobalt, fmt.Sprintf("%d", i+1))
		d.para(marginL+18, width-18, sans, 9, 13.5, graphite, c)
		d.y += 5
	}

	// Signatures.
	d.ensure(110)
	d.y += 18
	sigW := (width - 40) / 2
	d.text(marginL, d.y, sans, 9, inkSoft, "For Oriel Residences Limited")
	d.text(marginL+sigW+40, d.y, sans, 9, inkSoft, "Accepted by the buyer")
	d.y += 46
	d.line(marginL, d.y, marginL+sigW, d.y, 0.6, graphite)
	d.line(marginL+sigW+40, d.y, right, d.y, 0.6, graphite)
	d.y += 13
	d.text(marginL, d.y, semi, 9.5, graphite, o.Manager)
	d.text(marginL+sigW+40, d.y, semi, 9.5, graphite, r.Name)
	d.y += 13
	d.text(marginL, d.y, sans, 8.5, inkSoft, "Sales manager")
	d.text(marginL+sigW+40, d.y, sans, 8.5, inkSoft, "Date")
	return d.bytes()
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToLower(string(r[0])) + string(r[1:])
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// drawPlan draws a home's plan, simplified, in a box: the outdoor space and
// bay, the rooms, the outer walls, the windows and the room names. It returns
// the height used.
func drawPlan(d *doc, p floorplan.Plan, x, y, w, maxH float64) float64 {
	b := p.Bounds()
	scale := math.Min(w/b.W(), maxH/b.H())
	ox := x + (w-b.W()*scale)/2 - b.X0*scale
	oy := y - b.Y0*scale
	px := func(v float64) float64 { return ox + v*scale }
	py := func(v float64) float64 { return oy + v*scale }
	box := func(r floorplan.Rect, fill *colour, stroke *colour, lw float64) {
		if r.W() <= 0 || r.H() <= 0 {
			return
		}
		d.rect(px(r.X0), py(r.Y0), r.W()*scale, r.H()*scale, fill, stroke, lw)
	}
	light := colour{236, 237, 234}
	for _, r := range append([]floorplan.Rect{p.Outdoor}, p.OutdoorMore...) {
		box(r, &light, &ruleDark, 0.4)
	}
	if p.Bay.W() > 0 {
		box(p.Bay, &wash, &cobalt, 0.7)
	}
	for _, room := range p.Rooms {
		for _, r := range room.Rects {
			fill := plaster
			box(r, &fill, &ruleDark, 0.5)
		}
	}
	box(floorplan.Rect{X0: 0, Y0: 0, X1: p.W, Y1: p.D}, nil, &graphite, 1.6)
	for _, win := range p.Windows {
		d.line(px(win.X0), py(win.Y0), px(win.X1), py(win.Y1), 1.8, cobalt)
	}
	for _, room := range p.Rooms {
		if len(room.Rects) == 0 || room.Name == "" {
			continue
		}
		// Label the largest rectangle of the room, if the name fits.
		big := room.Rects[0]
		for _, r := range room.Rects[1:] {
			if r.W()*r.H() > big.W()*big.H() {
				big = r
			}
		}
		// One line if the name fits, else two, else a size smaller.
		cx, cy := px((big.X0+big.X1)/2), py((big.Y0+big.Y1)/2)
		fit := big.W()*scale - 4
		lines, size := []string{room.Name}, 6.0
		if d.measure(sans, size, room.Name) > fit {
			lines = nil
			if first, rest, ok := strings.Cut(room.Name, " "); ok && big.H()*scale > 20 &&
				d.measure(sans, size, first) <= fit && d.measure(sans, size, rest) <= fit {
				lines = []string{first, rest}
			} else if d.measure(sans, 5, room.Name) <= fit {
				lines, size = []string{room.Name}, 5
			}
		}
		for i, line := range lines {
			y := cy - 2 + (float64(i)-float64(len(lines)-1)/2)*7.5
			d.text(cx-d.measure(sans, size, line)/2, y, sans, size, inkSoft, line)
		}
	}
	return b.H() * scale
}
