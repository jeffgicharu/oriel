package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Lead is a person the sales team is talking to.
type Lead struct {
	ID             int64
	Name           string
	Email          string
	Phone          string
	Source         string
	Stage          string
	AgentID        int64
	AgentName      string
	AgentColour    string
	Interest       string
	Budget         int64
	BuyingFor      string
	FollowUpOn     string // YYYY-MM-DD, or empty
	LostReason     string
	SortOrder      float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StageChangedAt time.Time
}

// Overdue reports whether the lead's follow-up date has passed.
func (l Lead) Overdue(now time.Time) bool {
	return l.FollowUpOn != "" && l.Stage != "won" && l.Stage != "lost" && l.FollowUpOn <= now.In(nairobi).Format("2006-01-02")
}

const leadColumns = `l.id, l.name, l.email, l.phone, l.source, l.stage, COALESCE(l.agent_id, 0), COALESCE(s.name, ''), COALESCE(s.colour, ''),
	l.interest, COALESCE(l.budget, 0), l.buying_for, COALESCE(l.follow_up_on, ''), l.lost_reason, l.sort_order, l.created_at, l.updated_at, l.stage_changed_at`

func scanLead(row interface{ Scan(...any) error }) (Lead, error) {
	var l Lead
	var created, updated, changed int64
	err := row.Scan(&l.ID, &l.Name, &l.Email, &l.Phone, &l.Source, &l.Stage, &l.AgentID, &l.AgentName, &l.AgentColour,
		&l.Interest, &l.Budget, &l.BuyingFor, &l.FollowUpOn, &l.LostReason, &l.SortOrder, &created, &updated, &changed)
	l.CreatedAt, l.UpdatedAt, l.StageChangedAt = time.Unix(created, 0), time.Unix(updated, 0), time.Unix(changed, 0)
	return l, err
}

// LeadFilter narrows the list of leads.
type LeadFilter struct {
	AgentID int64  // only this agent's leads
	Stage   string // only this stage
	Query   string // name, email, phone or interest containing this
	Sort    string // "recent" (default), "name", "follow"
	Limit   int
}

// Leads lists leads.
func (db *DB) Leads(ctx context.Context, f LeadFilter) ([]Lead, error) {
	q := `SELECT ` + leadColumns + ` FROM leads l LEFT JOIN staff s ON s.id = l.agent_id WHERE 1 = 1`
	var args []any
	if f.AgentID > 0 {
		q += ` AND l.agent_id = ?`
		args = append(args, f.AgentID)
	}
	if f.Stage != "" {
		q += ` AND l.stage = ?`
		args = append(args, f.Stage)
	}
	if term := strings.TrimSpace(f.Query); term != "" {
		like := "%" + strings.ToLower(term) + "%"
		q += ` AND (lower(l.name) LIKE ? OR lower(l.email) LIKE ? OR replace(l.phone, ' ', '') LIKE ? OR lower(l.interest) LIKE ?)`
		args = append(args, like, like, "%"+strings.ReplaceAll(term, " ", "")+"%", like)
	}
	switch f.Sort {
	case "name":
		q += ` ORDER BY l.name COLLATE NOCASE`
	case "follow":
		q += ` ORDER BY l.follow_up_on IS NULL, l.follow_up_on, l.updated_at DESC`
	case "board":
		q += ` ORDER BY l.sort_order, l.id`
	default:
		q += ` ORDER BY l.updated_at DESC, l.id DESC`
	}
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, f.Limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// LeadByID finds a lead.
func (db *DB) LeadByID(ctx context.Context, id int64) (Lead, error) {
	return scanLead(db.QueryRowContext(ctx, `SELECT `+leadColumns+` FROM leads l LEFT JOIN staff s ON s.id = l.agent_id WHERE l.id = ?`, id))
}

// TimelineEntry is a line in a lead's history.
type TimelineEntry struct {
	At        time.Time
	Kind      string
	Body      string
	UnitID    string
	StaffName string
}

// Timeline is a lead's history, newest first.
func (db *DB) Timeline(ctx context.Context, leadID int64) ([]TimelineEntry, error) {
	rows, err := db.QueryContext(ctx, `SELECT e.at, e.kind, e.body, COALESCE(e.unit_id, ''), COALESCE(s.name, '')
		FROM lead_events e LEFT JOIN staff s ON s.id = e.staff_id WHERE e.lead_id = ? ORDER BY e.at DESC, e.id DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimelineEntry
	for rows.Next() {
		var e TimelineEntry
		var at int64
		if err := rows.Scan(&at, &e.Kind, &e.Body, &e.UnitID, &e.StaffName); err != nil {
			return nil, err
		}
		e.At = time.Unix(at, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

// LeadViewings and LeadReservations are a lead's bookings.
func (db *DB) LeadViewings(ctx context.Context, leadID int64) ([]Viewing, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+viewingColumns+` FROM viewings v JOIN leads l ON l.id = v.lead_id LEFT JOIN staff s ON s.id = l.agent_id
		WHERE v.lead_id = ? ORDER BY v.starts_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Viewing
	for rows.Next() {
		v, err := scanViewing(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LeadReservations lists a lead's reservations, newest first.
func (db *DB) LeadReservations(ctx context.Context, leadID int64) ([]Reservation, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+reservationColumns+reservationJoins+` WHERE r.lead_id = ? ORDER BY r.requested_at DESC`, leadID)
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

// ErrNotYours means an agent tried to change another agent's lead.
var ErrNotYours = errors.New("this lead belongs to another agent")

func checkOwner(ctx context.Context, tx *sql.Tx, leadID int64, by Staff) (string, error) {
	var agent sql.NullInt64
	var name string
	if err := tx.QueryRowContext(ctx, `SELECT agent_id, name FROM leads WHERE id = ?`, leadID).Scan(&agent, &name); err != nil {
		return "", err
	}
	if !by.Manager() && agent.Int64 != by.ID {
		return "", ErrNotYours
	}
	return name, nil
}

// MoveLead moves a lead to a stage, and to a place in that column of the
// board between two other leads (either may be zero).
func (db *DB) MoveLead(ctx context.Context, leadID int64, stage, reason string, after, before int64, by Staff, now time.Time) error {
	if StageTitle(stage) == "" {
		return fmt.Errorf("unknown stage %q", stage)
	}
	if stage == "lost" && strings.TrimSpace(reason) == "" {
		return errors.New("please say why the lead was lost")
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		name, err := checkOwner(ctx, tx, leadID, by)
		if err != nil {
			return err
		}
		order, err := orderBetween(ctx, tx, after, before)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE leads SET sort_order = ? WHERE id = ?`, order, leadID); err != nil {
			return err
		}
		var current string
		tx.QueryRowContext(ctx, `SELECT stage FROM leads WHERE id = ?`, leadID).Scan(&current)
		if current == stage {
			return nil
		}
		if err := SetLeadStage(ctx, tx, leadID, now, by.ID, stage, strings.TrimSpace(reason)); err != nil {
			return err
		}
		return Activity(ctx, tx, now, by.ID, "staff", "moved {} to "+StageTitle(stage), name, strings.TrimSpace(reason), fmt.Sprintf("/desk/leads/%d", leadID))
	})
}

// orderBetween is a sort order between two leads on the board.
func orderBetween(ctx context.Context, tx *sql.Tx, after, before int64) (float64, error) {
	get := func(id int64) (float64, bool) {
		if id == 0 {
			return 0, false
		}
		var o float64
		if err := tx.QueryRowContext(ctx, `SELECT sort_order FROM leads WHERE id = ?`, id).Scan(&o); err != nil {
			return 0, false
		}
		return o, true
	}
	a, hasA := get(after)
	b, hasB := get(before)
	switch {
	case hasA && hasB:
		return (a + b) / 2, nil
	case hasA:
		return a + 1, nil
	case hasB:
		return b - 1, nil
	}
	var max float64
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) FROM leads`).Scan(&max)
	return max + 1, err
}

// AddNote writes a note on a lead.
func (db *DB) AddNote(ctx context.Context, leadID int64, body string, by Staff, now time.Time) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return errors.New("the note is empty")
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		name, err := checkOwner(ctx, tx, leadID, by)
		if err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, leadID, now, by.ID, "note", body, ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, by.ID, "staff", "added a note on", name, clip(body, 80), fmt.Sprintf("/desk/leads/%d", leadID))
	})
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// SetFollowUp sets or clears a lead's follow-up date.
func (db *DB) SetFollowUp(ctx context.Context, leadID int64, day string, by Staff, now time.Time) error {
	if day != "" {
		if _, err := time.ParseInLocation("2006-01-02", day, nairobi); err != nil {
			return errors.New("that date doesn’t look right")
		}
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := checkOwner(ctx, tx, leadID, by); err != nil {
			return err
		}
		var v any
		body := "Follow-up cleared"
		if day != "" {
			v = day
			t, _ := time.ParseInLocation("2006-01-02", day, nairobi)
			body = "Follow up on " + t.Format("Mon 2 Jan")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE leads SET follow_up_on = ?, updated_at = ? WHERE id = ?`, v, now.Unix(), leadID); err != nil {
			return err
		}
		return LeadEvent(ctx, tx, leadID, now, by.ID, "follow_up", body, "")
	})
}

// AssignLead gives a lead to an agent. Only a manager may.
func (db *DB) AssignLead(ctx context.Context, leadID, agentID int64, by Staff, now time.Time) error {
	if !by.Manager() {
		return ErrNotYours
	}
	return db.Tx(ctx, func(tx *sql.Tx) error {
		var agentName, leadName string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM staff WHERE id = ?`, agentID).Scan(&agentName); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT name FROM leads WHERE id = ?`, leadID).Scan(&leadName); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE leads SET agent_id = ?, updated_at = ? WHERE id = ?`, agentID, now.Unix(), leadID); err != nil {
			return err
		}
		if err := LeadEvent(ctx, tx, leadID, now, by.ID, "assigned", "Assigned to "+agentName, ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, by.ID, "staff", "assigned {} to "+agentName, leadName, "", fmt.Sprintf("/desk/leads/%d", leadID))
	})
}

// LeadExistsError means a lead with the same email address is already on the desk.
type LeadExistsError struct{ ID int64 }

func (e LeadExistsError) Error() string { return "a lead with that email address already exists" }

// NewLead is a lead added by hand.
type NewLead struct {
	Contact
	Note    string
	AgentID int64
}

// AddLead adds a lead by hand. An agent's new lead is theirs.
func (db *DB) AddLead(ctx context.Context, n NewLead, by Staff, now time.Time) (int64, error) {
	var id int64
	err := db.Tx(ctx, func(tx *sql.Tx) error {
		var existing int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM leads WHERE lower(email) = ?`, strings.ToLower(n.Email)).Scan(&existing); err == nil {
			return LeadExistsError{ID: existing}
		}
		agent := n.AgentID
		if !by.Manager() || agent == 0 {
			agent = by.ID
		}
		if n.Source == "" {
			n.Source = "Walk-in"
		}
		var maxOrder float64
		tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) FROM leads`).Scan(&maxOrder)
		var budget any
		if n.Budget > 0 {
			budget = n.Budget
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO leads (name, email, phone, source, stage, agent_id, interest, budget, buying_for, follow_up_on, sort_order, created_at, updated_at, stage_changed_at)
			VALUES (?, ?, ?, ?, 'new', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.Name, strings.ToLower(n.Email), n.Phone, n.Source, agent, n.Interest, budget, n.Buying, now.In(nairobi).AddDate(0, 0, 1).Format("2006-01-02"), maxOrder+1, now.Unix(), now.Unix(), now.Unix())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		body := "Added by hand"
		if n.Note != "" {
			body += ": " + n.Note
		}
		if err := LeadEvent(ctx, tx, id, now, by.ID, "created", body, ""); err != nil {
			return err
		}
		return Activity(ctx, tx, now, by.ID, "staff", "added {} as a lead", n.Name, n.Source, fmt.Sprintf("/desk/leads/%d", id))
	})
	return id, err
}

// LeadCounts is how many leads are at each stage, for one agent or everyone.
func (db *DB) LeadCounts(ctx context.Context, agentID int64) (map[string]int, error) {
	q := `SELECT stage, count(*) FROM leads`
	var args []any
	if agentID > 0 {
		q += ` WHERE agent_id = ?`
		args = append(args, agentID)
	}
	q += ` GROUP BY stage`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}
