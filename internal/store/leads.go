package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Contact is what a visitor tells us about themselves on a form.
type Contact struct {
	Name     string
	Email    string
	Phone    string
	Interest string
	Budget   int64
	Buying   string
	Source   string
}

// Activity writes a line to the sales desk's log.
func Activity(ctx context.Context, tx *sql.Tx, at time.Time, staffID int64, actor, action, subject, detail, link string) error {
	var staff any
	if staffID > 0 {
		staff = staffID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO activity (at, staff_id, actor, action, subject, detail, link) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		at.Unix(), staff, actor, action, subject, detail, link)
	return err
}

// LeadEvent writes a line to a lead's history.
func LeadEvent(ctx context.Context, tx *sql.Tx, leadID int64, at time.Time, staffID int64, kind, body, unitID string) error {
	var staff, unit any
	if staffID > 0 {
		staff = staffID
	}
	if unitID != "" {
		unit = unitID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO lead_events (lead_id, at, staff_id, kind, body, unit_id) VALUES (?, ?, ?, ?, ?, ?)`,
		leadID, at.Unix(), staff, kind, body, unit); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE leads SET updated_at = MAX(updated_at, ?) WHERE id = ?`, at.Unix(), leadID)
	return err
}

// SetLeadStage moves a lead to a stage and records it.
func SetLeadStage(ctx context.Context, tx *sql.Tx, leadID int64, at time.Time, staffID int64, stage, reason string) error {
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT stage FROM leads WHERE id = ?`, leadID).Scan(&current); err != nil {
		return err
	}
	if current == stage {
		return nil
	}
	lost := ""
	if stage == "lost" {
		lost = reason
	}
	if _, err := tx.ExecContext(ctx, `UPDATE leads SET stage = ?, stage_changed_at = ?, updated_at = ?, lost_reason = ? WHERE id = ?`,
		stage, at.Unix(), at.Unix(), lost, leadID); err != nil {
		return err
	}
	body := "Moved to " + stageTitles[stage]
	if reason != "" {
		body += ": " + reason
	}
	return LeadEvent(ctx, tx, leadID, at, staffID, "stage", body, "")
}

var stageTitles = map[string]string{
	"new": "New", "contacted": "Contacted", "viewing": "Viewing", "reservation": "Reservation", "won": "Won", "lost": "Lost",
}

// StageTitle is a stage as the desk shows it.
func StageTitle(stage string) string { return stageTitles[stage] }

// Stages lists the pipeline in order.
var Stages = []string{"new", "contacted", "viewing", "reservation", "won", "lost"}

// FindOrCreateLead finds a lead by email address, or creates one assigned to
// the agent with the fewest open leads. A returning person's details are
// brought up to date.
func FindOrCreateLead(ctx context.Context, tx *sql.Tx, c Contact, at time.Time) (int64, bool, error) {
	email := strings.ToLower(strings.TrimSpace(c.Email))
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM leads WHERE lower(email) = ? ORDER BY created_at DESC LIMIT 1`, email).Scan(&id)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE leads SET name = ?, phone = ?, updated_at = ?,
			stage = CASE WHEN stage = 'lost' THEN 'new' ELSE stage END,
			stage_changed_at = CASE WHEN stage = 'lost' THEN ? ELSE stage_changed_at END,
			lost_reason = CASE WHEN stage = 'lost' THEN '' ELSE lost_reason END
			WHERE id = ?`, c.Name, c.Phone, at.Unix(), at.Unix(), id)
		return id, false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var agent int64
	err = tx.QueryRowContext(ctx, `SELECT s.id FROM staff s
		LEFT JOIN leads l ON l.agent_id = s.id AND l.stage NOT IN ('won', 'lost')
		WHERE s.role = 'agent'
		GROUP BY s.id ORDER BY count(l.id), s.id LIMIT 1`).Scan(&agent)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var agentArg, budget any
	if agent > 0 {
		agentArg = agent
	}
	if c.Budget > 0 {
		budget = c.Budget
	}
	source := c.Source
	if source == "" {
		source = "Website"
	}
	var maxOrder float64
	tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) FROM leads`).Scan(&maxOrder)
	res, err := tx.ExecContext(ctx, `INSERT INTO leads (name, email, phone, source, stage, agent_id, interest, budget, buying_for, follow_up_on, sort_order, created_at, updated_at, stage_changed_at)
		VALUES (?, ?, ?, ?, 'new', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, email, c.Phone, source, agentArg, c.Interest, budget, c.Buying, at.In(nairobi).AddDate(0, 0, 1).Format("2006-01-02"), maxOrder+1, at.Unix(), at.Unix(), at.Unix())
	if err != nil {
		return 0, false, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	if agent > 0 {
		var agentName string
		tx.QueryRowContext(ctx, `SELECT name FROM staff WHERE id = ?`, agent).Scan(&agentName)
		if err := LeadEvent(ctx, tx, id, at, 0, "assigned", "Assigned to "+agentName, ""); err != nil {
			return 0, false, err
		}
	}
	return id, true, nil
}

var nairobi = func() *time.Location {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}()

// Nairobi is the time zone everything is shown in.
func Nairobi() *time.Location { return nairobi }

// LeadName is a lead's name.
func (db *DB) LeadName(ctx context.Context, id int64) (string, error) {
	var name string
	err := db.QueryRowContext(ctx, `SELECT name FROM leads WHERE id = ?`, id).Scan(&name)
	return name, err
}

// Enquire records a question from the website.
func (db *DB) Enquire(ctx context.Context, c Contact, message string, at time.Time) (int64, error) {
	var leadID int64
	err := db.Tx(ctx, func(tx *sql.Tx) error {
		id, _, err := FindOrCreateLead(ctx, tx, c, at)
		if err != nil {
			return err
		}
		leadID = id
		if err := LeadEvent(ctx, tx, id, at, 0, "enquiry", message, ""); err != nil {
			return err
		}
		return Activity(ctx, tx, at, 0, "visitor", "took an enquiry from", c.Name, c.Interest, fmt.Sprintf("/desk/leads/%d", id))
	})
	return leadID, err
}

const refAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// newRef makes a short reference a person can read out: "V-7K3Q2".
func newRef(prefix string) string {
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = refAlphabet[int(b[i])%len(refAlphabet)]
	}
	return prefix + "-" + string(b)
}

// newToken is the secret part of a link.
func newToken() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 24)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
