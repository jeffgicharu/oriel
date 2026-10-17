// Package sample writes the shared sample: Oriel's sales story from launch to
// the moment it is built. The story is the same every night, moved forward so
// that it always ends now: about seven months of sales, a pipeline of leads,
// viewings booked for the next two weeks, and a few reservations waiting for
// the sales manager.
package sample

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/jeffgicharu/oriel/internal/building"
	"github.com/jeffgicharu/oriel/internal/plans"
	"github.com/jeffgicharu/oriel/internal/pricing"
	"github.com/jeffgicharu/oriel/internal/viewings"
)

// How the story is shaped.
const (
	SoldCount       = 40
	ReservedCount   = 6
	HeldCount       = 3
	DaysSinceLaunch = 212
	HoldLength      = 72 * time.Hour
)

// Withheld are the homes kept back for release at completion.
var Withheld = []string{"1602", "1506"}

// Password is the sales team's password in the sample.
const Password = "Riverside2028"

// Staff is a member of the sales team.
type Staff struct {
	ID       int64
	Username string
	Name     string
	Role     string
	Title    string
	Colour   string
}

// TeamMembers are Oriel's sales team.
var TeamMembers = []Staff{
	{1, "grace.wambui", "Grace Wambui", "manager", "Sales manager", "#1f3bd1"},
	{2, "daniel.otieno", "Daniel Otieno", "agent", "Sales consultant", "#2e6a43"},
	{3, "faith.njeri", "Faith Njeri", "agent", "Sales consultant", "#a26a10"},
	{4, "kevin.mutua", "Kevin Mutua", "agent", "Sales consultant", "#7a3e8f"},
}

// PriceList is a published price list.
type PriceList struct {
	ID          int64
	PublishedAt time.Time
	By          int64
	List        pricing.List
	Note        string
}

// Unit is a home's state at the end of the story.
type Unit struct {
	ID              string
	Status          string
	StatusChangedAt time.Time
}

// UnitEvent is a change to a home.
type UnitEvent struct {
	UnitID string
	At     time.Time
	By     int64
	Kind   string
	From   string
	To     string
	Reason string
}

// Lead is someone interested in buying.
type Lead struct {
	ID             int64
	Name           string
	Email          string
	Phone          string
	Source         string
	Stage          string
	AgentID        int64
	Interest       string
	Budget         int64
	BuyingFor      string
	FollowUpOn     string
	LostReason     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StageChangedAt time.Time
	wantType       string
}

// LeadEvent is something that happened with a lead.
type LeadEvent struct {
	LeadID int64
	At     time.Time
	By     int64
	Kind   string
	Body   string
	UnitID string
}

// Viewing is a visit to the sales suite.
type Viewing struct {
	ID        int64
	Ref       string
	Token     string
	LeadID    int64
	UnitID    string
	StartsAt  time.Time
	PartySize int
	Status    string
	Notes     string
	Outcome   string
	CreatedAt time.Time
}

// Reservation is a request to reserve a home.
type Reservation struct {
	ID            int64
	Ref           string
	Token         string
	LeadID        int64
	UnitID        string
	Plan          string
	Price         int64
	Status        string
	RequestedAt   time.Time
	HoldExpiresAt time.Time
	DecidedAt     time.Time
	DecidedBy     int64
	OfferNumber   string
	DepositAt     time.Time
	DepositAmount int64
}

// Activity is a line in the sales desk's log.
type Activity struct {
	At      time.Time
	StaffID int64
	Actor   string
	Action  string
	Subject string
	Detail  string
	Link    string
}

// Block is a day the sales suite is closed for viewings.
type Block struct {
	Day    string
	Reason string
}

// Story is the whole sample, held in memory before it is written.
type Story struct {
	Now          time.Time
	Launch       time.Time
	Viewing      viewings.Config
	Staff        []Staff
	PriceLists   []PriceList
	Units        map[string]*Unit
	UnitEvents   []UnitEvent
	Leads        []*Lead
	LeadEvents   []LeadEvent
	Viewings     []*Viewing
	Reservations []*Reservation
	Activity     []Activity
	Blocks       []Block

	rng       *rand.Rand
	today     time.Time
	slots     map[string]int
	usedNames map[string]bool
	nextOffer map[int]int // by year: letters of offer are numbered from one each year
	notes     []int       // the order the viewing notes are used in, so they rarely repeat
	refs      map[string]bool
}

var nairobi = viewings.Nairobi

// Tell works out the story as of now.
func Tell(now time.Time) *Story {
	now = now.Truncate(time.Minute)
	local := now.In(nairobi)
	s := &Story{
		Now:       now,
		Viewing:   viewings.Default(),
		Staff:     TeamMembers,
		Units:     map[string]*Unit{},
		rng:       rand.New(rand.NewPCG(20261017, 86)),
		today:     time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, nairobi),
		slots:     map[string]int{},
		usedNames: map[string]bool{},
		nextOffer: map[int]int{},
		refs:      map[string]bool{},
	}
	s.Launch = s.today.AddDate(0, 0, -DaysSinceLaunch).Add(10 * time.Hour)

	s.blockHolidays()
	s.publishPriceLists()
	for _, u := range building.Units() {
		s.Units[u.ID] = &Unit{ID: u.ID, Status: "available", StatusChangedAt: s.Launch.Add(-24 * time.Hour)}
	}
	for _, id := range Withheld {
		at := s.Launch.Add(-20 * time.Hour)
		s.setStatus(id, at, 1, "withheld", "Kept back for release at completion")
	}

	sold, reserved, held := s.pickHomes()
	s.sellHomes(sold)
	for _, id := range reserved {
		s.reserveHome(id)
	}
	for _, id := range held {
		s.holdHome(id)
	}
	s.addPipeline()

	sort.SliceStable(s.Activity, func(i, j int) bool { return s.Activity[i].At.Before(s.Activity[j].At) })
	sort.SliceStable(s.LeadEvents, func(i, j int) bool { return s.LeadEvents[i].At.Before(s.LeadEvents[j].At) })
	sort.SliceStable(s.UnitEvents, func(i, j int) bool { return s.UnitEvents[i].At.Before(s.UnitEvents[j].At) })
	return s
}

// ---------- Time ----------

// day is midnight in Nairobi, offset days from today.
func (s *Story) day(offset int) time.Time {
	return s.today.AddDate(0, 0, offset)
}

// past keeps a time that should already have happened at least a few
// minutes before now.
func (s *Story) past(t time.Time) time.Time {
	latest := s.Now.Add(-7 * time.Minute)
	if t.After(latest) {
		return latest
	}
	return t
}

// within picks a time in office hours on a day.
func (s *Story) within(day time.Time, fromHour, toHour int) time.Time {
	minutes := s.rng.IntN((toHour - fromHour) * 60)
	return day.Add(time.Duration(fromHour)*time.Hour + time.Duration(minutes)*time.Minute)
}

func (s *Story) between(lo, hi time.Time) time.Time {
	if !hi.After(lo) {
		return lo
	}
	return lo.Add(time.Duration(s.rng.Int64N(int64(hi.Sub(lo)))))
}

// ---------- Setup ----------

// Kenya's public holidays with fixed dates. The suite closes on those in the
// next month.
var holidays = []struct {
	month time.Month
	day   int
	name  string
}{
	{time.January, 1, "New Year's Day"},
	{time.May, 1, "Labour Day"},
	{time.June, 1, "Madaraka Day"},
	{time.October, 10, "Utamaduni Day"},
	{time.October, 20, "Mashujaa Day"},
	{time.December, 12, "Jamhuri Day"},
	{time.December, 25, "Christmas Day"},
	{time.December, 26, "Boxing Day"},
}

func (s *Story) blockHolidays() {
	for offset := 0; offset <= 31; offset++ {
		d := s.day(offset)
		for _, h := range holidays {
			if d.Month() == h.month && d.Day() == h.day {
				s.Blocks = append(s.Blocks, Block{Day: d.Format("2006-01-02"), Reason: "Public holiday: " + h.name})
			}
		}
	}
}

// blocked reports whether the suite is closed on the day of t: a public
// holiday in any year, or a day blocked in the desk.
func (s *Story) blocked(t time.Time) bool {
	local := t.In(nairobi)
	for _, h := range holidays {
		if local.Month() == h.month && local.Day() == h.day {
			return true
		}
	}
	day := local.Format("2006-01-02")
	for _, b := range s.Blocks {
		if b.Day == day {
			return true
		}
	}
	return false
}

func (s *Story) publishPriceLists() {
	launch := pricing.Launch()
	s.PriceLists = append(s.PriceLists, PriceList{ID: 1, PublishedAt: s.Launch.Add(-18 * time.Hour), By: 1, List: launch, Note: "Launch price list"})
	s.Activity = append(s.Activity, Activity{At: s.Launch.Add(-18 * time.Hour), StaffID: 1, Actor: "staff", Action: "published a price list", Subject: "Price list 1", Detail: "Launch prices", Link: "/desk/pricing"})

	revised := pricing.Launch()
	revised.FloorBP = 100
	at := s.Launch.AddDate(0, 0, 119).Add(-1 * time.Hour)
	if at.Before(s.Now) {
		s.PriceLists = append(s.PriceLists, PriceList{ID: 2, PublishedAt: at, By: 1, List: revised, Note: "Floor premium raised from 0.8% to 1% after strong sales on the upper floors"})
		s.Activity = append(s.Activity, Activity{At: at, StaffID: 1, Actor: "staff", Action: "published a price list", Subject: "Price list 2", Detail: "Floor premium 0.8% → 1%", Link: "/desk/pricing"})
	}
}

// priceAt is a home's price on the list in force at t.
func (s *Story) priceAt(id string, t time.Time) int64 {
	u, _ := building.UnitByID(id)
	list := s.PriceLists[0].List
	for _, pl := range s.PriceLists {
		if !pl.PublishedAt.After(t) {
			list = pl.List
		}
	}
	return list.Price(u)
}

func (s *Story) setStatus(id string, at time.Time, by int64, to, reason string) {
	u := s.Units[id]
	s.UnitEvents = append(s.UnitEvents, UnitEvent{UnitID: id, At: at, By: by, Kind: "status", From: u.Status, To: to, Reason: reason})
	u.Status = to
	u.StatusChangedAt = at
}

// pickHomes chooses which homes have sold, are reserved and are on hold.
// Homes on the river side, higher up, and the one-bedrooms (popular with
// investors) go first, as they do in Riverside.
func (s *Story) pickHomes() (sold, reserved, held []string) {
	weight := map[string]float64{}
	var pool []string
	for _, u := range building.Units() {
		if contains(Withheld, u.ID) {
			continue
		}
		w := map[string]float64{"A": 1.35, "B": 1.0, "C": 0.95, "D": 0.85, "P": 1.2}[u.Type.Code]
		w *= 1 + 0.6*float64(u.Level-2)/14
		if u.Position.RiverView {
			w *= 1.5
		}
		weight[u.ID] = w
		pool = append(pool, u.ID)
	}
	take := func(n int) []string {
		var out []string
		for i := 0; i < n; i++ {
			total := 0.0
			for _, id := range pool {
				total += weight[id]
			}
			r := s.rng.Float64() * total
			for j, id := range pool {
				r -= weight[id]
				if r <= 0 || j == len(pool)-1 {
					out = append(out, id)
					pool = append(pool[:j], pool[j+1:]...)
					break
				}
			}
		}
		return out
	}
	return take(SoldCount), take(ReservedCount), take(HeldCount)
}

// ---------- Leads ----------

func (s *Story) newLead(wantType string, created time.Time) *Lead {
	var name string
	for {
		name = firstNames[s.rng.IntN(len(firstNames))] + " " + lastNames[s.rng.IntN(len(lastNames))]
		if !s.usedNames[name] {
			s.usedNames[name] = true
			break
		}
	}
	parts := strings.Fields(strings.ToLower(name))
	email := parts[0] + "." + parts[1]
	if s.rng.IntN(3) == 0 {
		email = parts[0] + parts[1][:1] + fmt.Sprint(10+s.rng.IntN(89))
	}
	email += "@" + emailDomains[s.rng.IntN(len(emailDomains))]
	// Every sample number sits in one made-up block, so none rings a real person.
	phone := fmt.Sprintf("+254 700 555 %03d", s.rng.IntN(1000))

	source := sources[0].name
	total := 0
	for _, src := range sources {
		total += src.weight
	}
	r := s.rng.IntN(total)
	for _, src := range sources {
		if r < src.weight {
			source = src.name
			break
		}
		r -= src.weight
	}

	t := building.Types[wantType]
	interest := t.Short
	if s.rng.IntN(2) == 0 {
		interest += ", river side"
	} else if s.rng.IntN(2) == 0 {
		interest += ", high floor"
	}
	buyingFor := "home"
	if wantType == "A" && s.rng.IntN(3) > 0 || s.rng.IntN(5) == 0 {
		buyingFor = "investment"
	}
	budget := t.ListPrice + int64(s.rng.IntN(8))*500_000
	budget = budget / 500_000 * 500_000

	agent := int64(2 + s.rng.IntN(3))
	if s.rng.IntN(10) == 0 {
		agent = 1
	}
	l := &Lead{
		ID:             int64(len(s.Leads) + 1),
		Name:           name,
		Email:          email,
		Phone:          phone,
		Source:         source,
		Stage:          "new",
		AgentID:        agent,
		Interest:       interest,
		Budget:         budget,
		BuyingFor:      buyingFor,
		CreatedAt:      created,
		UpdatedAt:      created,
		StageChangedAt: created,
		wantType:       wantType,
	}
	s.Leads = append(s.Leads, l)
	messages := enquiryMessages[wantType]
	s.event(l, created, 0, "enquiry", messages[s.rng.IntN(len(messages))], "")
	s.event(l, created.Add(4*time.Minute), 1, "assigned", "Assigned to "+s.staffName(agent), "")
	if source == "Website" {
		s.Activity = append(s.Activity, Activity{At: created, Actor: "visitor", Action: "sent an enquiry", Subject: name, Detail: interest, Link: fmt.Sprintf("/desk/leads/%d", l.ID)})
	}
	return l
}

func (s *Story) staffName(id int64) string {
	for _, m := range s.Staff {
		if m.ID == id {
			return m.Name
		}
	}
	return ""
}

func (s *Story) event(l *Lead, at time.Time, by int64, kind, body, unit string) {
	s.LeadEvents = append(s.LeadEvents, LeadEvent{LeadID: l.ID, At: at, By: by, Kind: kind, Body: body, UnitID: unit})
	if at.After(l.UpdatedAt) && !at.After(s.Now) {
		l.UpdatedAt = at
	}
}

func (s *Story) stage(l *Lead, at time.Time, by int64, stage, reason string) {
	l.Stage = stage
	l.StageChangedAt = at
	body := "Moved to " + stageNames[stage]
	if reason != "" {
		body += ": " + reason
	}
	s.event(l, at, by, "stage", body, "")
}

var stageNames = map[string]string{
	"new": "New", "contacted": "Contacted", "viewing": "Viewing", "reservation": "Reservation", "won": "Won", "lost": "Lost",
}

func (s *Story) contact(l *Lead, at time.Time) {
	s.event(l, at, l.AgentID, "call", callNotes[s.rng.IntN(len(callNotes))], "")
	s.stage(l, at.Add(time.Minute), l.AgentID, "contacted", "")
}

// ---------- Viewings ----------

// slotNear finds an open slot with room, searching from day towards the
// given direction (-1 backwards, +1 forwards) for up to three weeks.
func (s *Story) slotNear(day time.Time, direction int, notBefore, notAfter time.Time) (time.Time, bool) {
	for i := 0; i < 21; i++ {
		d := day.AddDate(0, 0, i*direction)
		if s.blocked(d) {
			continue
		}
		options := s.Viewing.SlotsOn(d)
		s.rng.Shuffle(len(options), func(a, b int) { options[a], options[b] = options[b], options[a] })
		for _, slot := range options {
			if slot.Before(notBefore) || slot.After(notAfter) {
				continue
			}
			if s.slots[viewings.SlotKey(slot)] < s.Viewing.Capacity {
				return slot, true
			}
		}
	}
	return time.Time{}, false
}

func (s *Story) book(l *Lead, slot, bookedAt time.Time, unit, status string) *Viewing {
	s.slots[viewings.SlotKey(slot)]++
	v := &Viewing{
		ID:        int64(len(s.Viewings) + 1),
		Ref:       s.ref("V"),
		Token:     s.token(),
		LeadID:    l.ID,
		UnitID:    unit,
		StartsAt:  slot,
		PartySize: 1 + s.rng.IntN(3),
		Status:    status,
		CreatedAt: bookedAt,
	}
	s.Viewings = append(s.Viewings, v)
	when := slot.In(nairobi).Format("Mon 2 Jan, 15:04")
	s.event(l, bookedAt, l.AgentID, "viewing_booked", "Viewing booked for "+when, unit)
	switch status {
	case "attended":
		if len(s.notes) == 0 {
			s.notes = s.rng.Perm(len(viewingNotes))
		}
		v.Outcome, s.notes = viewingNotes[s.notes[0]], s.notes[1:]
		s.event(l, slot.Add(Length()), l.AgentID, "viewing_attended", v.Outcome, unit)
	case "no_show":
		v.Outcome = "Did not arrive. Sent a message to rebook."
		s.event(l, slot.Add(30*time.Minute), l.AgentID, "viewing_no_show", v.Outcome, unit)
	}
	return v
}

// Length is a viewing's length.
func Length() time.Duration { return viewings.Length }

// ---------- Reservations ----------

func (s *Story) request(l *Lead, unit string, at time.Time, fromWebsite bool) *Reservation {
	plan := plans.Plans[s.rng.IntN(len(plans.Plans))]
	r := &Reservation{
		ID:            int64(len(s.Reservations) + 1),
		Ref:           s.ref("R"),
		Token:         s.token(),
		LeadID:        l.ID,
		UnitID:        unit,
		Plan:          plan.Code,
		Price:         s.priceAt(unit, at),
		Status:        "requested",
		RequestedAt:   at,
		HoldExpiresAt: at.Add(HoldLength),
	}
	s.Reservations = append(s.Reservations, r)
	by := l.AgentID
	actor := "staff"
	if fromWebsite {
		by, actor = 0, "visitor"
	}
	s.setStatus(unit, at, by, "held", "Reservation "+r.Ref+" requested")
	s.event(l, at, by, "reservation_requested", fmt.Sprintf("Requested %s on %s", unit, plan.Name), unit)
	s.stage(l, at.Add(time.Minute), by, "reservation", "")
	s.Activity = append(s.Activity, Activity{At: at, StaffID: by, Actor: actor, Action: "requested a reservation", Subject: unit, Detail: l.Name + " · " + plan.Name, Link: "/desk/reservations"})
	return r
}

func (s *Story) confirm(l *Lead, r *Reservation, at time.Time) {
	year := at.In(nairobi).Year()
	s.nextOffer[year]++
	r.Status = "confirmed"
	r.DecidedAt = at
	r.DecidedBy = 1
	r.OfferNumber = fmt.Sprintf("OR-%d-%04d", year, s.nextOffer[year])
	s.setStatus(r.UnitID, at, 1, "reserved", "Reservation "+r.Ref+" confirmed")
	s.event(l, at, 1, "reservation_confirmed", "Letter of offer "+r.OfferNumber+" issued", r.UnitID)
	s.Activity = append(s.Activity, Activity{At: at, StaffID: 1, Actor: "staff", Action: "confirmed a reservation", Subject: r.UnitID, Detail: l.Name + " · " + r.OfferNumber, Link: "/desk/reservations"})
}

func (s *Story) deposit(l *Lead, r *Reservation, at time.Time) {
	plan, _ := plans.ByCode(r.Plan)
	price := r.Price - (r.Price*plan.DiscountBP+5_000)/10_000
	r.Status = "completed"
	r.DepositAt = at
	r.DepositAmount = (price*1_000 + 5_000) / 10_000
	s.setStatus(r.UnitID, at, l.AgentID, "sold", "Deposit received on "+r.Ref)
	s.event(l, at, l.AgentID, "deposit", "Deposit of KES "+thousands(r.DepositAmount)+" received", r.UnitID)
	s.stage(l, at.Add(time.Minute), l.AgentID, "won", "")
	s.Activity = append(s.Activity, Activity{At: at, StaffID: l.AgentID, Actor: "staff", Action: "recorded a deposit", Subject: r.UnitID, Detail: l.Name + " · KES " + thousands(r.DepositAmount), Link: "/desk/reservations"})
}

// ---------- The sales ----------

func (s *Story) sellHomes(ids []string) {
	// A dozen sales in the launch month, the rest spread over the months since.
	start := s.Launch.AddDate(0, 0, 8)
	launchEnd := s.Launch.AddDate(0, 0, 35)
	end := s.day(-5)
	var depositDays []time.Time
	for i := range ids {
		var d time.Time
		if i < 12 {
			d = s.between(start, launchEnd)
		} else {
			d = s.between(launchEnd, end)
		}
		depositDays = append(depositDays, d)
	}
	sort.Slice(depositDays, func(i, j int) bool { return depositDays[i].Before(depositDays[j]) })

	for i, id := range ids {
		u, _ := building.UnitByID(id)
		dayOfDeposit := time.Date(depositDays[i].Year(), depositDays[i].Month(), depositDays[i].Day(), 0, 0, 0, 0, nairobi)
		depositAt := s.past(s.within(dayOfDeposit, 9, 16))
		confirmAt := s.within(dayOfDeposit.AddDate(0, 0, -(3+s.rng.IntN(4))), 9, 17)
		requestAt := confirmAt.Add(-time.Duration(16+s.rng.IntN(10)) * time.Hour)
		l, ok := s.leadThroughViewing(u.Type.Code, requestAt, id)
		if !ok {
			continue
		}
		r := s.request(l, id, requestAt, s.rng.IntN(3) == 0)
		s.confirm(l, r, confirmAt)
		s.deposit(l, r, depositAt)
	}
}

// leadThroughViewing makes a lead who enquired, was called and viewed the
// suite before the given time.
func (s *Story) leadThroughViewing(wantType string, before time.Time, unit string) (*Lead, bool) {
	viewDay := before.In(nairobi).AddDate(0, 0, -(1 + s.rng.IntN(8)))
	earliest := s.Launch.AddDate(0, 0, -40)
	slot, ok := s.slotNear(viewDay, -1, earliest.Add(72*time.Hour), before.Add(-2*time.Hour))
	if !ok {
		return nil, false
	}
	contacted := slot.Add(-time.Duration(48+s.rng.IntN(5*24)) * time.Hour)
	created := contacted.Add(-time.Duration(2+s.rng.IntN(40)) * time.Hour)
	if created.Before(earliest) {
		created = earliest.Add(time.Duration(s.rng.IntN(48)) * time.Hour)
		if !contacted.After(created) {
			contacted = created.Add(3 * time.Hour)
		}
	}
	l := s.newLead(wantType, created)
	s.contact(l, contacted)
	booked := s.between(contacted.Add(10*time.Minute), slot.Add(-20*time.Hour))
	s.book(l, slot, booked, unit, "attended")
	s.stage(l, slot.Add(Length()+time.Minute), l.AgentID, "viewing", "")
	return l, true
}

func (s *Story) reserveHome(id string) {
	u, _ := building.UnitByID(id)
	confirmAt := s.past(s.within(s.day(-(1 + s.rng.IntN(6))), 9, 17))
	requestAt := confirmAt.Add(-time.Duration(14+s.rng.IntN(12)) * time.Hour)
	l, ok := s.leadThroughViewing(u.Type.Code, requestAt, id)
	if !ok {
		return
	}
	r := s.request(l, id, requestAt, s.rng.IntN(2) == 0)
	s.confirm(l, r, confirmAt)
}

func (s *Story) holdHome(id string) {
	u, _ := building.UnitByID(id)
	requestAt := s.Now.Add(-time.Duration(2*60+s.rng.IntN(58*60)) * time.Minute)
	var l *Lead
	if s.rng.IntN(2) == 0 {
		var ok bool
		l, ok = s.leadThroughViewing(u.Type.Code, requestAt, id)
		if !ok {
			return
		}
	} else {
		// Straight from the website, without a viewing first.
		l = s.newLead(u.Type.Code, requestAt.Add(-3*time.Minute))
	}
	s.request(l, id, requestAt, true)
}

// addPipeline adds the leads still in progress and the ones lost along the way.
func (s *Story) addPipeline() {
	types := []string{"A", "A", "A", "B", "B", "B", "C", "C", "D", "D", "P"}
	pick := func() string { return types[s.rng.IntN(len(types))] }

	// New: enquired in the last ten days, not called yet.
	for i := 0; i < 22; i++ {
		var created time.Time
		if i < 5 {
			created = s.Now.Add(-time.Duration(20+s.rng.IntN(20*60)) * time.Minute)
		} else {
			created = s.past(s.within(s.day(-(1 + s.rng.IntN(9))), 7, 22))
		}
		l := s.newLead(pick(), created)
		l.FollowUpOn = s.day(s.rng.IntN(3)).Format("2006-01-02")
	}

	// Contacted: called, not yet booked. A few booked a viewing in the last
	// week and did not come.
	for i := 0; i < 44; i++ {
		created := s.within(s.day(-(4 + s.rng.IntN(70))), 7, 21)
		l := s.newLead(pick(), created)
		contacted := s.past(created.Add(time.Duration(3+s.rng.IntN(60)) * time.Hour))
		s.contact(l, contacted)
		follow := s.rng.IntN(12) - 2 // a few are overdue
		l.FollowUpOn = s.day(follow).Format("2006-01-02")
		if i < 5 {
			if slot, ok := s.slotNear(s.day(-(1 + i%7)), -1, contacted.Add(24*time.Hour), s.Now.Add(-2*time.Hour)); ok {
				s.book(l, slot, s.between(contacted.Add(10*time.Minute), slot.Add(-12*time.Hour)), "", "no_show")
				l.FollowUpOn = s.day(s.rng.IntN(2)).Format("2006-01-02")
			}
		}
	}

	// Viewing: some booked for the coming fortnight, some seen over the last
	// ten days, spread so that every recent day the suite was open has visits.
	for i := 0; i < 30; i++ {
		created := s.within(s.day(-(3 + s.rng.IntN(40))), 7, 21)
		l := s.newLead(pick(), created)
		contacted := s.past(created.Add(time.Duration(2+s.rng.IntN(48)) * time.Hour))
		s.contact(l, contacted)
		if i < 15 {
			slot, ok := s.slotNear(s.day(1+s.rng.IntN(12)), 1, s.Now.Add(3*time.Hour), s.day(viewings.BookAhead))
			if !ok {
				continue
			}
			booked := s.past(s.between(contacted.Add(10*time.Minute), s.Now))
			s.book(l, slot, booked, "", "booked")
			s.stage(l, booked.Add(time.Minute), l.AgentID, "viewing", "")
			l.FollowUpOn = slot.In(nairobi).Format("2006-01-02")
		} else {
			slot, ok := s.slotNear(s.day(-(1 + (i-15)%10)), -1, contacted.Add(24*time.Hour), s.Now.Add(-2*time.Hour))
			if !ok {
				continue
			}
			booked := s.between(contacted.Add(10*time.Minute), slot.Add(-12*time.Hour))
			s.book(l, slot, booked, "", "attended")
			s.stage(l, slot.Add(Length()+time.Minute), l.AgentID, "viewing", "")
			l.FollowUpOn = s.day(s.rng.IntN(4)).Format("2006-01-02")
		}
	}

	// Lost, across the whole period.
	for i := 0; i < 96; i++ {
		created := s.within(s.Launch.AddDate(0, 0, -30+s.rng.IntN(DaysSinceLaunch+20)), 7, 21)
		if created.After(s.day(-6)) {
			created = s.within(s.day(-6-s.rng.IntN(20)), 7, 21)
		}
		l := s.newLead(pick(), created)
		contacted := created.Add(time.Duration(2+s.rng.IntN(72)) * time.Hour)
		reason := lostReasons[s.rng.IntN(len(lostReasons))]
		lostAt := contacted.Add(time.Duration(3+s.rng.IntN(30)) * 24 * time.Hour)
		if reason != "No reply after three calls" {
			s.contact(l, contacted)
			if s.rng.IntN(3) == 0 {
				if slot, ok := s.slotNear(contacted.In(nairobi).AddDate(0, 0, 3+s.rng.IntN(7)), 1, contacted.Add(24*time.Hour), s.day(-2)); ok {
					status := "attended"
					if s.rng.IntN(4) == 0 {
						status = "no_show"
					}
					s.book(l, slot, s.between(contacted.Add(10*time.Minute), slot.Add(-12*time.Hour)), "", status)
					if status == "attended" {
						s.stage(l, slot.Add(Length()+time.Minute), l.AgentID, "viewing", "")
					}
					if lostAt.Before(slot.Add(2 * 24 * time.Hour)) {
						lostAt = slot.Add(time.Duration(2+s.rng.IntN(10)) * 24 * time.Hour)
					}
				}
			}
		} else {
			lostAt = created.Add(time.Duration(8+s.rng.IntN(10)) * 24 * time.Hour)
		}
		lostAt = s.past(lostAt)
		l.LostReason = reason
		s.stage(l, lostAt, l.AgentID, "lost", reason)
	}
}

// ---------- Identifiers ----------

const refAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func (s *Story) ref(prefix string) string {
	for {
		b := make([]byte, 5)
		for i := range b {
			b[i] = refAlphabet[s.rng.IntN(len(refAlphabet))]
		}
		r := prefix + "-" + string(b)
		if !s.refs[r] {
			s.refs[r] = true
			return r
		}
	}
}

func (s *Story) token() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 24)
	for i := range b {
		b[i] = alphabet[s.rng.IntN(len(alphabet))]
	}
	return string(b)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func thousands(n int64) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
