package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jeffgicharu/oriel/internal/plans"
)

// HoldLength is how long a home is held for a reservation request.
const HoldLength = 72 * time.Hour

// Reservation is a request to reserve a home.
type Reservation struct {
	ID            int64
	Ref           string
	Token         string
	LeadID        int64
	Name          string
	Email         string
	Phone         string
	UnitID        string
	Plan          string
	Price         int64
	Status        string
	RequestedAt   time.Time
	HoldExpiresAt time.Time
	DecidedAt     time.Time
	DecidedBy     string
	DeclineReason string
	OfferNumber   string
	DepositAt     time.Time
	DepositAmount int64
	BuyerNote     string
	AgentName     string
}

// ErrNotAvailable means the home was taken before the request arrived.
var ErrNotAvailable = errors.New("this home is no longer available")

// ReservationStatusName is a reservation's status as the buyer reads it.
func ReservationStatusName(s string) string {
	switch s {
	case "requested":
		return "Waiting for confirmation"
	case "confirmed":
		return "Confirmed"
	case "declined":
		return "Declined"
	case "lapsed":
		return "Lapsed"
	case "completed":
		return "Deposit received"
	case "cancelled":
		return "Cancelled"
	}
	return s
}

// ReservationRequest is what the reservation form sends.
type ReservationRequest struct {
	Contact
	UnitID string
	Plan   string
	Note   string
}

// RequestReservation holds a home for a buyer while the sales team confirms.
func (db *DB) RequestReservation(ctx context.Context, req ReservationRequest, price int64, now time.Time) (Reservation, error) {
	var r Reservation
	err := db.Tx(ctx, func(tx *sql.Tx) error {
		if err := lapseHolds(ctx, tx, now); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM units WHERE id = ?`, req.UnitID).Scan(&status); err != nil {
			return err
		}
		if status != Available {
			return ErrNotAvailable
		}
		plan, ok := plans.ByCode(req.Plan)
		if !ok {
			return fmt.Errorf("unknown plan")
		}
		leadID, _, err := FindOrCreateLead(ctx, tx, req.Contact, now)
		if err != nil {
			return err
		}
		r = Reservation{
			Ref: newRef("R"), Token: newToken(), LeadID: leadID, Name: req.Name, Email: req.Email, Phone: req.Phone,
			UnitID: req.UnitID, Plan: plan.Code, Price: price, Status: "requested", RequestedAt: now,
			HoldExpiresAt: now.Add(HoldLength), BuyerNote: req.Note,
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO reservations (ref, token, lead_id, unit_id, plan, price, status, requested_at, hold_expires_at, buyer_note)
			VALUES (?, ?, ?, ?, ?, ?, 'requested', ?, ?, ?)`, r.Ref, r.Token, leadID, r.UnitID, r.Plan, price, now.Unix(), r.HoldExpiresAt.Unix(), req.Note)
		if err != nil {
			return err
		}
		r.ID, _ = res.LastInsertId()
		if err := setUnitStatus(ctx, tx, r.UnitID, Held, now, 0, "Reservation "+r.Ref+" requested"); err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, leadID, now, 0, "reservation_requested", fmt.Sprintf("Requested %s on %s", r.UnitID, plan.Name), r.UnitID); err != nil {
			return err
		}
		if err := SetLeadStage(ctx, tx, leadID, now, 0, "reservation", ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, 0, "visitor", "requested a reservation", r.UnitID, req.Name+" · "+plan.Name, "/desk/reservations")
	})
	return r, err
}

func setUnitStatus(ctx context.Context, tx *sql.Tx, unitID, to string, at time.Time, staffID int64, reason string) error {
	var from string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM units WHERE id = ?`, unitID).Scan(&from); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE units SET status = ?, status_changed_at = ? WHERE id = ?`, to, at.Unix(), unitID); err != nil {
		return err
	}
	var staff any
	if staffID > 0 {
		staff = staffID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO unit_events (unit_id, at, staff_id, kind, from_value, to_value, reason) VALUES (?, ?, ?, 'status', ?, ?, ?)`,
		unitID, at.Unix(), staff, from, to, reason)
	return err
}

// lapseHolds releases homes whose reservation request was not confirmed in time.
func lapseHolds(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT r.id, r.ref, r.unit_id, r.lead_id, l.name FROM reservations r JOIN leads l ON l.id = r.lead_id
		WHERE r.status = 'requested' AND r.hold_expires_at <= ?`, now.Unix())
	if err != nil {
		return err
	}
	type lapsed struct {
		id, lead        int64
		ref, unit, name string
	}
	var list []lapsed
	for rows.Next() {
		var x lapsed
		if err := rows.Scan(&x.id, &x.ref, &x.unit, &x.lead, &x.name); err != nil {
			rows.Close()
			return err
		}
		list = append(list, x)
	}
	rows.Close()
	for _, x := range list {
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET status = 'lapsed', decided_at = ? WHERE id = ?`, now.Unix(), x.id); err != nil {
			return err
		}
		if err := setUnitStatus(ctx, tx, x.unit, Available, now, 0, "Hold on "+x.ref+" lapsed after 72 hours"); err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, x.lead, now, 0, "reservation_lapsed", "Hold on "+x.unit+" lapsed", x.unit); err != nil {
			return err
		}
		if err := SetLeadStage(ctx, tx, x.lead, now, 0, "viewing", "hold lapsed"); err != nil {
			return err
		}
		if err := Activity(ctx, tx, now, 0, "system", "released a lapsed hold", x.unit, x.name+" · "+x.ref, "/desk/reservations"); err != nil {
			return err
		}
	}
	return nil
}

// LapseHolds releases expired holds in a visitor's copy.
func (db *DB) LapseHolds(ctx context.Context, now time.Time) error {
	if db.readOnly {
		return nil
	}
	return db.Tx(ctx, func(tx *sql.Tx) error { return lapseHolds(ctx, tx, now) })
}

const reservationColumns = `r.id, r.ref, r.token, r.lead_id, l.name, l.email, l.phone, r.unit_id, r.plan, r.price, r.status, r.requested_at, r.hold_expires_at,
	COALESCE(r.decided_at, 0), COALESCE(d.name, ''), r.decline_reason, COALESCE(r.offer_number, ''), COALESCE(r.deposit_received_at, 0), COALESCE(r.deposit_amount, 0), r.buyer_note, COALESCE(a.name, '')`

const reservationJoins = ` FROM reservations r JOIN leads l ON l.id = r.lead_id LEFT JOIN staff d ON d.id = r.decided_by LEFT JOIN staff a ON a.id = l.agent_id`

func scanReservation(row interface{ Scan(...any) error }) (Reservation, error) {
	var r Reservation
	var req, hold, decided, deposit int64
	err := row.Scan(&r.ID, &r.Ref, &r.Token, &r.LeadID, &r.Name, &r.Email, &r.Phone, &r.UnitID, &r.Plan, &r.Price, &r.Status, &req, &hold,
		&decided, &r.DecidedBy, &r.DeclineReason, &r.OfferNumber, &deposit, &r.DepositAmount, &r.BuyerNote, &r.AgentName)
	r.RequestedAt = time.Unix(req, 0)
	r.HoldExpiresAt = time.Unix(hold, 0)
	if decided > 0 {
		r.DecidedAt = time.Unix(decided, 0)
	}
	if deposit > 0 {
		r.DepositAt = time.Unix(deposit, 0)
	}
	return r, err
}

// ReservationByRef finds a reservation by its reference.
func (db *DB) ReservationByRef(ctx context.Context, ref string) (Reservation, error) {
	return scanReservation(db.QueryRowContext(ctx, `SELECT `+reservationColumns+reservationJoins+` WHERE r.ref = ?`, ref))
}

// Reservations lists reservations, newest first, optionally of one status.
func (db *DB) Reservations(ctx context.Context, status string, limit int) ([]Reservation, error) {
	q := `SELECT ` + reservationColumns + reservationJoins
	var args []any
	if status != "" {
		q += ` WHERE r.status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY r.requested_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reservation
	for rows.Next() {
		r, err := scanReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ConfirmReservation issues the letter of offer and reserves the home.
func (db *DB) ConfirmReservation(ctx context.Context, ref string, staffID int64, now time.Time) error {
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var id, leadID int64
		var unit, status, name string
		if err := tx.QueryRowContext(ctx, `SELECT r.id, r.lead_id, r.unit_id, r.status, l.name FROM reservations r JOIN leads l ON l.id = r.lead_id WHERE r.ref = ?`, ref).Scan(&id, &leadID, &unit, &status, &name); err != nil {
			return err
		}
		if status != "requested" {
			return fmt.Errorf("only a waiting request can be confirmed")
		}
		// Letters of offer are numbered from one each year.
		year := now.In(nairobi).Year()
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reservations WHERE offer_number LIKE ?`, fmt.Sprintf("OR-%d-%%", year)).Scan(&n); err != nil {
			return err
		}
		offer := fmt.Sprintf("OR-%d-%04d", year, n+1)
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET status = 'confirmed', decided_at = ?, decided_by = ?, offer_number = ? WHERE id = ?`, now.Unix(), staffID, offer, id); err != nil {
			return err
		}
		if err := setUnitStatus(ctx, tx, unit, Reserved, now, staffID, "Reservation "+ref+" confirmed"); err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, leadID, now, staffID, "reservation_confirmed", "Letter of offer "+offer+" issued", unit); err != nil {
			return err
		}
		return Activity(ctx, tx, now, staffID, "staff", "confirmed a reservation", unit, name+" · "+offer, "/desk/reservations")
	})
}

// DeclineReservation turns a request down and frees the home.
func (db *DB) DeclineReservation(ctx context.Context, ref, reason string, staffID int64, now time.Time) error {
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var id, leadID int64
		var unit, status, name string
		if err := tx.QueryRowContext(ctx, `SELECT r.id, r.lead_id, r.unit_id, r.status, l.name FROM reservations r JOIN leads l ON l.id = r.lead_id WHERE r.ref = ?`, ref).Scan(&id, &leadID, &unit, &status, &name); err != nil {
			return err
		}
		if status != "requested" && status != "confirmed" {
			return fmt.Errorf("this reservation is already closed")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET status = 'declined', decided_at = ?, decided_by = ?, decline_reason = ? WHERE id = ?`, now.Unix(), staffID, reason, id); err != nil {
			return err
		}
		if err := setUnitStatus(ctx, tx, unit, Available, now, staffID, "Reservation "+ref+" declined: "+reason); err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, leadID, now, staffID, "reservation_declined", "Reservation for "+unit+" declined: "+reason, unit); err != nil {
			return err
		}
		if err := SetLeadStage(ctx, tx, leadID, now, staffID, "viewing", ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, staffID, "staff", "declined a reservation", unit, name+" · "+reason, "/desk/reservations")
	})
}

// RecordDeposit marks the deposit received: the home is sold.
func (db *DB) RecordDeposit(ctx context.Context, ref string, amount int64, staffID int64, now time.Time) error {
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var id, leadID int64
		var unit, status, name string
		if err := tx.QueryRowContext(ctx, `SELECT r.id, r.lead_id, r.unit_id, r.status, l.name FROM reservations r JOIN leads l ON l.id = r.lead_id WHERE r.ref = ?`, ref).Scan(&id, &leadID, &unit, &status, &name); err != nil {
			return err
		}
		if status != "confirmed" {
			return fmt.Errorf("a deposit can only be recorded on a confirmed reservation")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET status = 'completed', deposit_received_at = ?, deposit_amount = ? WHERE id = ?`, now.Unix(), amount, id); err != nil {
			return err
		}
		if err := setUnitStatus(ctx, tx, unit, Sold, now, staffID, "Deposit received on "+ref); err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, leadID, now, staffID, "deposit", fmt.Sprintf("Deposit of KES %s received", thousands(amount)), unit); err != nil {
			return err
		}
		if err := SetLeadStage(ctx, tx, leadID, now, staffID, "won", ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, staffID, "staff", "recorded a deposit", unit, name+" · KES "+thousands(amount), "/desk/reservations")
	})
}

func thousands(n int64) string {
	s := fmt.Sprint(n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
