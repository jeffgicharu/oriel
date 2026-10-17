package store

import (
	"context"
	"time"
)

// Count is a label with a number, for the desk's charts.
type Count struct {
	Key   string
	Label string
	N     int
	Value int64
}

// Dashboard is the sales desk's front page.
type Dashboard struct {
	Months      []Count // sales confirmed in each of the last months, oldest first
	Sources     []Count // leads by where they came from
	Funnel      []Count // leads by stage, in pipeline order
	Waiting     int     // reservation requests waiting for a decision
	FollowUps   int     // open leads due a follow-up today or earlier
	SoldValue   int64   // the price of every confirmed or completed reservation
	Week        []Viewing
	NewLeads    int // leads in the last seven days
	NewLeadsWas int // leads in the seven days before
}

// DashboardOn gathers the dashboard as it stands at now. An agent's figures
// for follow-ups count only their own leads.
func (db *DB) DashboardOn(ctx context.Context, now time.Time, agentID int64) (Dashboard, error) {
	var d Dashboard
	local := now.In(nairobi)
	monthStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, nairobi)
	for i := 7; i >= 0; i-- {
		from := monthStart.AddDate(0, -i, 0)
		to := from.AddDate(0, 1, 0)
		c := Count{Key: from.Format("2006-01"), Label: from.Format("Jan")}
		if err := db.QueryRowContext(ctx, `SELECT count(*), COALESCE(SUM(price), 0) FROM reservations
			WHERE status IN ('confirmed', 'completed') AND decided_at >= ? AND decided_at < ?`, from.Unix(), to.Unix()).Scan(&c.N, &c.Value); err != nil {
			return d, err
		}
		d.Months = append(d.Months, c)
	}

	rows, err := db.QueryContext(ctx, `SELECT source, count(*) FROM leads GROUP BY source ORDER BY count(*) DESC, source`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Key, &c.N); err != nil {
			rows.Close()
			return d, err
		}
		c.Label = c.Key
		d.Sources = append(d.Sources, c)
	}
	rows.Close()

	byStage := map[string]int{}
	rows, err = db.QueryContext(ctx, `SELECT stage, count(*) FROM leads GROUP BY stage`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var stage string
		var n int
		if err := rows.Scan(&stage, &n); err != nil {
			rows.Close()
			return d, err
		}
		byStage[stage] = n
	}
	rows.Close()
	for _, s := range Stages {
		d.Funnel = append(d.Funnel, Count{Key: s, Label: StageTitle(s), N: byStage[s]})
	}

	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reservations WHERE status = 'requested' AND hold_expires_at > ?`, now.Unix()).Scan(&d.Waiting); err != nil {
		return d, err
	}
	today := local.Format("2006-01-02")
	q := `SELECT count(*) FROM leads WHERE stage NOT IN ('won', 'lost') AND follow_up_on IS NOT NULL AND follow_up_on <= ?`
	args := []any{today}
	if agentID > 0 {
		q += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	if err := db.QueryRowContext(ctx, q, args...).Scan(&d.FollowUps); err != nil {
		return d, err
	}
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(price), 0) FROM reservations WHERE status IN ('confirmed', 'completed')`).Scan(&d.SoldValue); err != nil {
		return d, err
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM leads WHERE created_at >= ?`, now.AddDate(0, 0, -7).Unix()).Scan(&d.NewLeads); err != nil {
		return d, err
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM leads WHERE created_at >= ? AND created_at < ?`, now.AddDate(0, 0, -14).Unix(), now.AddDate(0, 0, -7).Unix()).Scan(&d.NewLeadsWas); err != nil {
		return d, err
	}
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, nairobi)
	d.Week, err = db.ViewingsBetween(ctx, dayStart, dayStart.AddDate(0, 0, 7), 0)
	return d, err
}

// ViewingsBetween lists viewings from one time to another, earliest first;
// with an agent, only those of the agent's leads.
func (db *DB) ViewingsBetween(ctx context.Context, from, to time.Time, agentID int64) ([]Viewing, error) {
	q := `SELECT ` + viewingColumns + ` FROM viewings v JOIN leads l ON l.id = v.lead_id LEFT JOIN staff s ON s.id = l.agent_id
		WHERE v.starts_at >= ? AND v.starts_at < ?`
	args := []any{from.Unix(), to.Unix()}
	if agentID > 0 {
		q += ` AND l.agent_id = ?`
		args = append(args, agentID)
	}
	q += ` ORDER BY v.starts_at, v.id`
	rows, err := db.QueryContext(ctx, q, args...)
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
