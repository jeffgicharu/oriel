package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jeffgicharu/oriel/internal/pricing"
	"github.com/jeffgicharu/oriel/internal/viewings"
)

// ErrManagersOnly means only the sales manager may make the change.
var ErrManagersOnly = errors.New("only the sales manager can do that")

// UnitEvent is a line in a home's history.
type UnitEvent struct {
	At        time.Time
	Kind      string
	From, To  string
	Reason    string
	StaffName string
}

// UnitHistory is a home's history, newest first.
func (db *DB) UnitHistory(ctx context.Context, unitID string) ([]UnitEvent, error) {
	rows, err := db.QueryContext(ctx, `SELECT e.at, e.kind, e.from_value, e.to_value, e.reason, COALESCE(s.name, '')
		FROM unit_events e LEFT JOIN staff s ON s.id = e.staff_id WHERE e.unit_id = ? ORDER BY e.at DESC, e.id DESC LIMIT 50`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnitEvent
	for rows.Next() {
		var e UnitEvent
		var at int64
		if err := rows.Scan(&at, &e.Kind, &e.From, &e.To, &e.Reason, &e.StaffName); err != nil {
			return nil, err
		}
		e.At = time.Unix(at, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetUnitStatus changes a home's status by hand, with a reason. Homes tied to
// a reservation change through the reservation instead.
func (db *DB) SetUnitStatus(ctx context.Context, unitID, to, reason string, by Staff, now time.Time) error {
	if !by.Manager() {
		return ErrManagersOnly
	}
	if to != Available && to != Withheld {
		return errors.New("a home can be made available or withheld here; holds, reservations and sales go through a reservation")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("please give a reason")
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var from string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM units WHERE id = ?`, unitID).Scan(&from); err != nil {
			return err
		}
		if from == to {
			return nil
		}
		if from != Available && from != Withheld {
			return fmt.Errorf("home %s is %s through a reservation; decide the reservation instead", unitID, strings.ToLower(StatusName(from)))
		}
		if err := setUnitStatus(ctx, tx, unitID, to, now, by.ID, reason); err != nil {
			return err
		}
		action := "released a home"
		if to == Withheld {
			action = "withheld a home"
		}
		return Activity(ctx, tx, now, by.ID, "staff", action, unitID, reason, "/desk/stack?home="+unitID)
	})
}

// SetPriceOverride sets a home's own price, or clears it with zero.
func (db *DB) SetPriceOverride(ctx context.Context, unitID string, price int64, reason string, by Staff, now time.Time) error {
	if !by.Manager() {
		return ErrManagersOnly
	}
	if price != 0 && (price < 1_000_000 || price > 500_000_000) {
		return errors.New("the price must be between KES 1,000,000 and KES 500,000,000")
	}
	if price%pricing.Rounding != 0 {
		return errors.New("prices are set to the nearest KES 10,000")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("please give a reason")
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var old sql.NullInt64
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT price_override, status FROM units WHERE id = ?`, unitID).Scan(&old, &status); err != nil {
			return err
		}
		if status == Reserved || status == Sold {
			return fmt.Errorf("home %s is %s; its price is fixed by its reservation", unitID, strings.ToLower(StatusName(status)))
		}
		var v any
		to, from := "list price", "list price"
		if price > 0 {
			v = price
			to = "KES " + thousands(price)
		}
		if old.Valid {
			from = "KES " + thousands(old.Int64)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE units SET price_override = ? WHERE id = ?`, v, unitID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO unit_events (unit_id, at, staff_id, kind, from_value, to_value, reason) VALUES (?, ?, ?, 'price', ?, ?, ?)`,
			unitID, now.Unix(), by.ID, from, to, reason); err != nil {
			return err
		}
		return Activity(ctx, tx, now, by.ID, "staff", "set a home’s price", unitID, from+" → "+to+" · "+reason, "/desk/stack?home="+unitID)
	})
}

// PriceListEntry is a published price list with who published it.
type PriceListEntry struct {
	ID          int64
	PublishedAt time.Time
	By          string
	List        pricing.List
	Note        string
}

// PriceLists lists every published price list, newest first.
func (db *DB) PriceLists(ctx context.Context) ([]PriceListEntry, error) {
	rows, err := db.QueryContext(ctx, `SELECT p.id, p.published_at, COALESCE(s.name, ''), p.floor_bp, p.river_bp, p.base_a, p.base_b, p.base_c, p.base_d, p.base_p, p.note
		FROM price_lists p LEFT JOIN staff s ON s.id = p.published_by ORDER BY p.published_at DESC, p.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceListEntry
	for rows.Next() {
		var e PriceListEntry
		var at int64
		var a, b, c, d, p int64
		if err := rows.Scan(&e.ID, &at, &e.By, &e.List.FloorBP, &e.List.RiverBP, &a, &b, &c, &d, &p, &e.Note); err != nil {
			return nil, err
		}
		e.PublishedAt = time.Unix(at, 0)
		e.List.Base = map[string]int64{"A": a, "B": b, "C": c, "D": d, "P": p}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PublishPriceList puts a new price list in force.
func (db *DB) PublishPriceList(ctx context.Context, l pricing.List, note string, by Staff, now time.Time) (int64, error) {
	if !by.Manager() {
		return 0, ErrManagersOnly
	}
	if err := l.Validate(); err != nil {
		return 0, err
	}
	for code, p := range l.Base {
		if p%pricing.Rounding != 0 {
			return 0, fmt.Errorf("type %s must be priced to the nearest KES 10,000", code)
		}
	}
	var id int64
	err := db.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO price_lists (published_at, published_by, floor_bp, river_bp, base_a, base_b, base_c, base_d, base_p, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, now.Unix(), by.ID, l.FloorBP, l.RiverBP, l.Base["A"], l.Base["B"], l.Base["C"], l.Base["D"], l.Base["P"], strings.TrimSpace(note))
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		detail := strings.TrimSpace(note)
		if detail == "" {
			detail = "New prices"
		}
		return Activity(ctx, tx, now, by.ID, "staff", "published a price list", fmt.Sprintf("Price list %d", id), detail, "/desk/pricing")
	})
	return id, err
}

// SetViewingOutcome marks a viewing attended or a no-show, with notes.
func (db *DB) SetViewingOutcome(ctx context.Context, ref, outcome, notes string, by Staff, now time.Time) error {
	if outcome != "attended" && outcome != "no_show" {
		return errors.New("unknown outcome")
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var id, leadID, starts int64
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT id, lead_id, starts_at, status FROM viewings WHERE ref = ?`, ref).Scan(&id, &leadID, &starts, &status); err != nil {
			return err
		}
		name, err := checkOwner(ctx, tx, leadID, by)
		if err != nil {
			return err
		}
		if status == "cancelled" {
			return errors.New("this viewing was cancelled")
		}
		if time.Unix(starts, 0).After(now) {
			return errors.New("this viewing has not happened yet")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE viewings SET status = ?, outcome_notes = ?, updated_at = ? WHERE id = ?`, outcome, strings.TrimSpace(notes), now.Unix(), id); err != nil {
			return err
		}
		label := "Attended the viewing"
		action := "marked a viewing attended"
		if outcome == "no_show" {
			label, action = "Did not come to the viewing", "marked a no-show"
		}
		if n := strings.TrimSpace(notes); n != "" {
			label += ": " + n
		}
		if err := LeadEvent(ctx, tx, leadID, now, by.ID, "viewing_"+outcome, label, ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, by.ID, "staff", action, name, ref, fmt.Sprintf("/desk/leads/%d", leadID))
	})
}

// SaveViewingConfig changes the suite's opening days, times and capacity.
func (db *DB) SaveViewingConfig(ctx context.Context, cfg viewings.Config, by Staff, now time.Time) error {
	if !by.Manager() {
		return ErrManagersOnly
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	days, times, capacity := cfg.Encode()
	return db.Tx(ctx, func(tx *sql.Tx) error {
		for k, v := range map[string]string{"viewing.days": days, "viewing.times": times, "viewing.capacity": capacity} {
			if err := SetSetting(ctx, tx, k, v); err != nil {
				return err
			}
		}
		return Activity(ctx, tx, now, by.ID, "staff", "changed the viewing hours", "Viewings", fmt.Sprintf("%d slots a day, %s per slot", len(cfg.Times), capacity), "/desk/viewings")
	})
}

// BlockDay closes the suite for viewings on a day.
func (db *DB) BlockDay(ctx context.Context, day, reason string, by Staff, now time.Time) error {
	if !by.Manager() {
		return ErrManagersOnly
	}
	t, err := time.ParseInLocation("2006-01-02", day, nairobi)
	if err != nil {
		return errors.New("that date doesn’t look right")
	}
	if t.Before(time.Date(now.In(nairobi).Year(), now.In(nairobi).Month(), now.In(nairobi).Day(), 0, 0, 0, 0, nairobi)) {
		return errors.New("that day has passed")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("please give a reason")
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO viewing_blocks (day, reason) VALUES (?, ?) ON CONFLICT (day) DO UPDATE SET reason = excluded.reason`, day, reason); err != nil {
			return err
		}
		var booked int
		tx.QueryRowContext(ctx, `SELECT count(*) FROM viewings WHERE status = 'booked' AND starts_at >= ? AND starts_at < ?`, t.Unix(), t.AddDate(0, 0, 1).Unix()).Scan(&booked)
		detail := reason
		if booked > 0 {
			detail += fmt.Sprintf(" · %d booked %s to rearrange", booked, map[bool]string{true: "viewing", false: "viewings"}[booked == 1])
		}
		return Activity(ctx, tx, now, by.ID, "staff", "closed a day for viewings", t.Format("Mon 2 Jan"), detail, "/desk/viewings")
	})
}

// UnblockDay opens a closed day again.
func (db *DB) UnblockDay(ctx context.Context, day string, by Staff, now time.Time) error {
	if !by.Manager() {
		return ErrManagersOnly
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM viewing_blocks WHERE day = ?`, day)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		return Activity(ctx, tx, now, by.ID, "staff", "opened a day for viewings", day, "", "/desk/viewings")
	})
}

// ActivityEntry is a line of the desk's log.
type ActivityEntry struct {
	ID        int64
	At        time.Time
	StaffName string
	Colour    string
	Actor     string
	Action    string
	Subject   string
	Detail    string
	Link      string
}

// ActivityLog lists the log, newest first, a page at a time.
func (db *DB) ActivityLog(ctx context.Context, actorKind string, offset, limit int) ([]ActivityEntry, error) {
	q := `SELECT a.id, a.at, COALESCE(s.name, ''), COALESCE(s.colour, ''), a.actor, a.action, a.subject, a.detail, a.link
		FROM activity a LEFT JOIN staff s ON s.id = a.staff_id WHERE 1 = 1`
	var args []any
	if actorKind != "" {
		q += ` AND a.actor = ?`
		args = append(args, actorKind)
	}
	q += fmt.Sprintf(` ORDER BY a.at DESC, a.id DESC LIMIT %d OFFSET %d`, limit, offset)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		var at int64
		if err := rows.Scan(&e.ID, &at, &e.StaffName, &e.Colour, &e.Actor, &e.Action, &e.Subject, &e.Detail, &e.Link); err != nil {
			return nil, err
		}
		e.At = time.Unix(at, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

// UnitReservation is the reservation that holds, reserves or sold a home, if any.
func (db *DB) UnitReservation(ctx context.Context, unitID string) (*Reservation, error) {
	r, err := scanReservation(db.QueryRowContext(ctx, `SELECT `+reservationColumns+reservationJoins+`
		WHERE r.unit_id = ? AND r.status IN ('requested', 'confirmed', 'completed') ORDER BY r.requested_at DESC LIMIT 1`, unitID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ReservationGroups are the tabs of the reservations page.
var ReservationGroups = map[string][]string{
	"waiting":   {"requested"},
	"confirmed": {"confirmed"},
	"completed": {"completed"},
	"closed":    {"declined", "lapsed", "cancelled"},
}

// ReservationsIn lists reservations in a group, for one agent's leads or all.
func (db *DB) ReservationsIn(ctx context.Context, group string, agentID int64) ([]Reservation, error) {
	statuses := ReservationGroups[group]
	if len(statuses) == 0 {
		return nil, fmt.Errorf("unknown group %q", group)
	}
	q := `SELECT ` + reservationColumns + reservationJoins + ` WHERE r.status IN (?` + strings.Repeat(", ?", len(statuses)-1) + `)`
	var args []any
	for _, s := range statuses {
		args = append(args, s)
	}
	if agentID > 0 {
		q += ` AND l.agent_id = ?`
		args = append(args, agentID)
	}
	if group == "waiting" {
		q += ` ORDER BY r.hold_expires_at, r.id`
	} else {
		q += ` ORDER BY COALESCE(r.decided_at, r.requested_at) DESC, r.id DESC LIMIT 200`
	}
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

// ReservationGroupCounts is how many reservations each tab holds.
func (db *DB) ReservationGroupCounts(ctx context.Context, agentID int64) (map[string]int, error) {
	q := `SELECT r.status, count(*) FROM reservations r JOIN leads l ON l.id = r.lead_id`
	var args []any
	if agentID > 0 {
		q += ` WHERE l.agent_id = ?`
		args = append(args, agentID)
	}
	q += ` GROUP BY r.status`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byStatus := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		byStatus[s] = n
	}
	out := map[string]int{}
	for g, statuses := range ReservationGroups {
		for _, s := range statuses {
			out[g] += byStatus[s]
		}
	}
	return out, rows.Err()
}
