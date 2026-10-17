package sample

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jeffgicharu/oriel/internal/store"
)

// Build writes the shared sample, as of now, to a new file at path.
func Build(ctx context.Context, path string, now time.Time) error {
	story := Tell(now)
	hash, err := bcrypt.GenerateFromPassword([]byte(Password), 10)
	if err != nil {
		return err
	}
	db, err := store.OpenForBuild(path)
	if err != nil {
		return err
	}
	if err := db.Tx(ctx, func(tx *sql.Tx) error { return story.write(ctx, tx, string(hash)) }); err != nil {
		db.Close()
		return err
	}
	return db.Seal(ctx)
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Unix()
}

func (s *Story) write(ctx context.Context, tx *sql.Tx, passwordHash string) error {
	exec := func(query string, args ...any) error {
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	}

	days, times, capacity := s.Viewing.Encode()
	for key, value := range map[string]string{
		"viewing.days":     days,
		"viewing.times":    times,
		"viewing.capacity": capacity,
		"launched_on":      s.Launch.In(nairobi).Format("2006-01-02"),
		"built_at":         s.Now.UTC().Format(time.RFC3339),
	} {
		if err := store.SetSetting(ctx, tx, key, value); err != nil {
			return err
		}
	}

	for _, m := range s.Staff {
		if err := exec(`INSERT INTO staff (id, username, name, role, title, password_hash, colour) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.Username, m.Name, m.Role, m.Title, passwordHash, m.Colour); err != nil {
			return err
		}
	}

	for _, pl := range s.PriceLists {
		b := pl.List.Base
		if err := exec(`INSERT INTO price_lists (id, published_at, published_by, floor_bp, river_bp, base_a, base_b, base_c, base_d, base_p, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pl.ID, pl.PublishedAt.Unix(), nullID(pl.By), pl.List.FloorBP, pl.List.RiverBP, b["A"], b["B"], b["C"], b["D"], b["P"], pl.Note); err != nil {
			return err
		}
	}

	insertUnit, err := tx.PrepareContext(ctx, `INSERT INTO units (id, status, status_changed_at) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertUnit.Close()
	for _, u := range s.Units {
		if _, err := insertUnit.ExecContext(ctx, u.ID, u.Status, u.StatusChangedAt.Unix()); err != nil {
			return err
		}
	}

	insertUnitEvent, err := tx.PrepareContext(ctx, `INSERT INTO unit_events (unit_id, at, staff_id, kind, from_value, to_value, reason) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertUnitEvent.Close()
	for _, e := range s.UnitEvents {
		if _, err := insertUnitEvent.ExecContext(ctx, e.UnitID, e.At.Unix(), nullID(e.By), e.Kind, e.From, e.To, e.Reason); err != nil {
			return err
		}
	}

	insertLead, err := tx.PrepareContext(ctx, `INSERT INTO leads (id, name, email, phone, source, stage, agent_id, interest, budget, buying_for, follow_up_on, lost_reason, sort_order, created_at, updated_at, stage_changed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertLead.Close()
	for i, l := range s.Leads {
		follow := l.FollowUpOn
		if l.Stage == "won" || l.Stage == "lost" || l.Stage == "reservation" {
			follow = ""
		}
		if _, err := insertLead.ExecContext(ctx, l.ID, l.Name, l.Email, l.Phone, l.Source, l.Stage, nullID(l.AgentID), l.Interest, l.Budget, l.BuyingFor,
			nullText(follow), l.LostReason, float64(len(s.Leads)-i), l.CreatedAt.Unix(), l.UpdatedAt.Unix(), l.StageChangedAt.Unix()); err != nil {
			return err
		}
	}

	insertLeadEvent, err := tx.PrepareContext(ctx, `INSERT INTO lead_events (lead_id, at, staff_id, kind, body, unit_id) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertLeadEvent.Close()
	for _, e := range s.LeadEvents {
		if _, err := insertLeadEvent.ExecContext(ctx, e.LeadID, e.At.Unix(), nullID(e.By), e.Kind, e.Body, nullText(e.UnitID)); err != nil {
			return err
		}
	}

	for _, v := range s.Viewings {
		if err := exec(`INSERT INTO viewings (id, ref, token, lead_id, unit_id, starts_at, party_size, status, notes, outcome_notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			v.ID, v.Ref, v.Token, v.LeadID, nullText(v.UnitID), v.StartsAt.Unix(), v.PartySize, v.Status, v.Notes, v.Outcome, v.CreatedAt.Unix(), v.CreatedAt.Unix()); err != nil {
			return err
		}
	}

	for _, b := range s.Blocks {
		if err := exec(`INSERT INTO viewing_blocks (day, reason) VALUES (?, ?)`, b.Day, b.Reason); err != nil {
			return err
		}
	}

	for _, r := range s.Reservations {
		var deposit any
		if r.DepositAmount > 0 {
			deposit = r.DepositAmount
		}
		if err := exec(`INSERT INTO reservations (id, ref, token, lead_id, unit_id, plan, price, status, requested_at, hold_expires_at, decided_at, decided_by, offer_number, deposit_received_at, deposit_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.Ref, r.Token, r.LeadID, r.UnitID, r.Plan, r.Price, r.Status, r.RequestedAt.Unix(), r.HoldExpiresAt.Unix(),
			nullTime(r.DecidedAt), nullID(r.DecidedBy), nullText(r.OfferNumber), nullTime(r.DepositAt), deposit); err != nil {
			return err
		}
	}

	insertActivity, err := tx.PrepareContext(ctx, `INSERT INTO activity (at, staff_id, actor, action, subject, detail, link) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertActivity.Close()
	for _, a := range s.Activity {
		if _, err := insertActivity.ExecContext(ctx, a.At.Unix(), nullID(a.StaffID), a.Actor, a.Action, a.Subject, a.Detail, a.Link); err != nil {
			return err
		}
	}
	return nil
}
