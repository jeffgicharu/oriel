package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jeffgicharu/oriel/internal/viewings"
)

// ViewingConfig reads the sales suite's opening pattern.
func (db *DB) ViewingConfig(ctx context.Context) viewings.Config {
	return viewings.Decode(
		db.Setting(ctx, "viewing.days", ""),
		db.Setting(ctx, "viewing.times", ""),
		db.Setting(ctx, "viewing.capacity", ""),
	)
}

// Block is a day the suite is closed.
type Block struct {
	Day    string
	Reason string
}

// Blocks lists closed days from a date on.
func (db *DB) Blocks(ctx context.Context, from string) ([]Block, error) {
	rows, err := db.QueryContext(ctx, `SELECT day, reason FROM viewing_blocks WHERE day >= ? ORDER BY day`, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Block
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.Day, &b.Reason); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Slot is a viewing time with the room left in it.
type Slot struct {
	Start time.Time
	Left  int
}

// Day is one day of the booking calendar.
type Day struct {
	Date   time.Time
	Open   bool
	Reason string
	Slots  []Slot
}

// Free counts the places left on a day.
func (d Day) Free() int {
	n := 0
	for _, s := range d.Slots {
		n += s.Left
	}
	return n
}

// Calendar lists the days a visitor can book, from tomorrow if it is late,
// with each slot's room left.
func (db *DB) Calendar(ctx context.Context, now time.Time) ([]Day, error) {
	cfg := db.ViewingConfig(ctx)
	local := now.In(nairobi)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, nairobi)
	blocks, err := db.Blocks(ctx, today.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	blocked := map[string]string{}
	for _, b := range blocks {
		blocked[b.Day] = b.Reason
	}
	until := today.AddDate(0, 0, viewings.BookAhead+1)
	counts := map[string]int{}
	rows, err := db.QueryContext(ctx, `SELECT starts_at FROM viewings WHERE status = 'booked' AND starts_at >= ? AND starts_at < ?`, today.Unix(), until.Unix())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var at int64
		if err := rows.Scan(&at); err != nil {
			rows.Close()
			return nil, err
		}
		counts[viewings.SlotKey(time.Unix(at, 0))]++
	}
	rows.Close()

	var days []Day
	for i := 0; i <= viewings.BookAhead; i++ {
		date := today.AddDate(0, 0, i)
		d := Day{Date: date, Open: cfg.Opens(date)}
		if reason, ok := blocked[date.Format("2006-01-02")]; ok {
			d.Open, d.Reason = false, reason
		}
		if d.Open {
			for _, start := range cfg.SlotsOn(date) {
				if start.Before(now.Add(viewings.MinNotice)) {
					continue
				}
				left := cfg.Capacity - counts[viewings.SlotKey(start)]
				if left < 0 {
					left = 0
				}
				d.Slots = append(d.Slots, Slot{Start: start, Left: left})
			}
			if len(d.Slots) == 0 && i == 0 {
				d.Open = false
				d.Reason = "Today's viewings have all started"
			}
		} else if d.Reason == "" {
			d.Reason = "The sales suite is closed on " + date.Weekday().String() + "s"
		}
		days = append(days, d)
	}
	return days, nil
}

// Errors a booking form can show.
var (
	ErrSlotGone   = errors.New("that time has just been taken")
	ErrSlotClosed = errors.New("the sales suite is not open then")
)

// Viewing is a booked visit.
type Viewing struct {
	ID        int64
	Ref       string
	Token     string
	LeadID    int64
	Name      string
	Email     string
	Phone     string
	UnitID    string
	StartsAt  time.Time
	PartySize int
	Status    string
	Notes     string
	Outcome   string
	AgentName string
	CreatedAt time.Time
}

// ViewingRequest is what the booking form sends.
type ViewingRequest struct {
	Contact
	Start     time.Time
	PartySize int
	UnitID    string
	Notes     string
}

// BookViewing books a slot, checking again inside the transaction that it is
// open and has room, so two people cannot take the last place at once.
func (db *DB) BookViewing(ctx context.Context, req ViewingRequest, now time.Time) (Viewing, error) {
	var v Viewing
	err := db.Tx(ctx, func(tx *sql.Tx) error {
		cfg := db.ViewingConfig(ctx)
		if !cfg.IsSlot(req.Start) || req.Start.Before(now.Add(viewings.MinNotice)) || req.Start.After(now.AddDate(0, 0, viewings.BookAhead+1)) {
			return ErrSlotClosed
		}
		var blocked int
		tx.QueryRowContext(ctx, `SELECT count(*) FROM viewing_blocks WHERE day = ?`, req.Start.In(nairobi).Format("2006-01-02")).Scan(&blocked)
		if blocked > 0 {
			return ErrSlotClosed
		}
		var taken int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM viewings WHERE status = 'booked' AND starts_at = ?`, req.Start.Unix()).Scan(&taken); err != nil {
			return err
		}
		if taken >= cfg.Capacity {
			return ErrSlotGone
		}
		leadID, _, err := FindOrCreateLead(ctx, tx, req.Contact, now)
		if err != nil {
			return err
		}
		v = Viewing{
			Ref: newRef("V"), Token: newToken(), LeadID: leadID, Name: req.Name, Email: req.Email, Phone: req.Phone,
			UnitID: req.UnitID, StartsAt: req.Start, PartySize: req.PartySize, Status: "booked", Notes: req.Notes, CreatedAt: now,
		}
		var unit any
		if req.UnitID != "" {
			unit = req.UnitID
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO viewings (ref, token, lead_id, unit_id, starts_at, party_size, status, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'booked', ?, ?, ?)`, v.Ref, v.Token, leadID, unit, req.Start.Unix(), req.PartySize, req.Notes, now.Unix(), now.Unix())
		if err != nil {
			return err
		}
		v.ID, _ = res.LastInsertId()
		when := req.Start.In(nairobi).Format("Mon 2 Jan, 15:04")
		if err := LeadEvent(ctx, tx, leadID, now, 0, "viewing_booked", "Viewing booked for "+when, req.UnitID); err != nil {
			return err
		}
		var stage string
		tx.QueryRowContext(ctx, `SELECT stage FROM leads WHERE id = ?`, leadID).Scan(&stage)
		if stage == "new" || stage == "contacted" {
			if err := SetLeadStage(ctx, tx, leadID, now, 0, "viewing", ""); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE leads SET follow_up_on = ? WHERE id = ?`, req.Start.In(nairobi).Format("2006-01-02"), leadID)
		if err != nil {
			return err
		}
		detail := when
		if req.UnitID != "" {
			detail += " · home " + req.UnitID
		}
		return Activity(ctx, tx, now, 0, "visitor", "booked a viewing", req.Name, detail, fmt.Sprintf("/desk/leads/%d", leadID))
	})
	return v, err
}

const viewingColumns = `v.id, v.ref, v.token, v.lead_id, l.name, l.email, l.phone, COALESCE(v.unit_id, ''), v.starts_at, v.party_size, v.status, v.notes, v.outcome_notes, COALESCE(s.name, ''), v.created_at`

func scanViewing(row interface{ Scan(...any) error }) (Viewing, error) {
	var v Viewing
	var starts, created int64
	err := row.Scan(&v.ID, &v.Ref, &v.Token, &v.LeadID, &v.Name, &v.Email, &v.Phone, &v.UnitID, &starts, &v.PartySize, &v.Status, &v.Notes, &v.Outcome, &v.AgentName, &created)
	v.StartsAt = time.Unix(starts, 0)
	v.CreatedAt = time.Unix(created, 0)
	return v, err
}

// ViewingByRef finds a viewing by its reference.
func (db *DB) ViewingByRef(ctx context.Context, ref string) (Viewing, error) {
	return scanViewing(db.QueryRowContext(ctx, `SELECT `+viewingColumns+` FROM viewings v JOIN leads l ON l.id = v.lead_id LEFT JOIN staff s ON s.id = l.agent_id WHERE v.ref = ?`, ref))
}

// CancelViewing cancels a booked viewing.
func (db *DB) CancelViewing(ctx context.Context, ref string, now time.Time, staffID int64) error {
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var id, leadID, starts int64
		var status, name string
		err := tx.QueryRowContext(ctx, `SELECT v.id, v.lead_id, v.starts_at, v.status, l.name FROM viewings v JOIN leads l ON l.id = v.lead_id WHERE v.ref = ?`, ref).Scan(&id, &leadID, &starts, &status, &name)
		if err != nil {
			return err
		}
		if status != "booked" {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE viewings SET status = 'cancelled', updated_at = ? WHERE id = ?`, now.Unix(), id); err != nil {
			return err
		}
		when := time.Unix(starts, 0).In(nairobi).Format("Mon 2 Jan, 15:04")
		if err := LeadEvent(ctx, tx, leadID, now, staffID, "viewing_cancelled", "Viewing for "+when+" cancelled", ""); err != nil {
			return err
		}
		actor := "visitor"
		if staffID > 0 {
			actor = "staff"
		}
		return Activity(ctx, tx, now, staffID, actor, "cancelled a viewing", name, when, fmt.Sprintf("/desk/leads/%d", leadID))
	})
}
