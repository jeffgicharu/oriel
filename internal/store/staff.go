package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Staff is a member of the sales team.
type Staff struct {
	ID       int64
	Username string
	Name     string
	Role     string
	Title    string
	Colour   string
}

// Manager reports whether the member may do everything: change prices and
// statuses, decide reservations and see every lead.
func (s Staff) Manager() bool { return s.Role == "manager" }

// Initials are shown on a lead card: "GW".
func (s Staff) Initials() string {
	var out []rune
	for _, part := range strings.Fields(s.Name) {
		out = append(out, []rune(part)[0])
	}
	return string(out)
}

// SessionLength is how long a desk sign-in lasts.
const SessionLength = 12 * time.Hour

// ErrBadSignIn means the username or password is wrong.
var ErrBadSignIn = errors.New("wrong username or password")

func scanStaff(row interface{ Scan(...any) error }) (Staff, error) {
	var s Staff
	err := row.Scan(&s.ID, &s.Username, &s.Name, &s.Role, &s.Title, &s.Colour)
	return s, err
}

const staffColumns = `id, username, name, role, title, colour`

// Team lists the sales team, the manager first.
func (db *DB) Team(ctx context.Context) ([]Staff, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+staffColumns+` FROM staff ORDER BY role = 'agent', id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Staff
	for rows.Next() {
		s, err := scanStaff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// StaffByID finds a member of the team.
func (db *DB) StaffByID(ctx context.Context, id int64) (Staff, error) {
	return scanStaff(db.QueryRowContext(ctx, `SELECT `+staffColumns+` FROM staff WHERE id = ?`, id))
}

// FirstWithRole is the first member of the team with a role.
func (db *DB) FirstWithRole(ctx context.Context, role string) (Staff, error) {
	return scanStaff(db.QueryRowContext(ctx, `SELECT `+staffColumns+` FROM staff WHERE role = ? ORDER BY id LIMIT 1`, role))
}

// CheckSignIn checks a username and password.
func (db *DB) CheckSignIn(ctx context.Context, username, password string) (Staff, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	var hash string
	s, err := scanStaff(db.QueryRowContext(ctx, `SELECT `+staffColumns+` FROM staff WHERE username = ?`, username))
	if err == nil {
		err = db.QueryRowContext(ctx, `SELECT password_hash FROM staff WHERE id = ?`, s.ID).Scan(&hash)
	}
	if err != nil {
		// Spend the same time as a real check, so a wrong username and a
		// wrong password cannot be told apart.
		bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return Staff{}, ErrBadSignIn
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return Staff{}, ErrBadSignIn
	}
	return s, nil
}

var (
	dummyOnce sync.Once
	dummy     []byte
)

func dummyHash() []byte {
	dummyOnce.Do(func() { dummy, _ = bcrypt.GenerateFromPassword([]byte("not a password"), 10) })
	return dummy
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// StartSession signs a member in and returns the token for their cookie.
func (db *DB) StartSession(ctx context.Context, staffID int64, now time.Time) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	err := db.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM staff_sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO staff_sessions (token_hash, staff_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			hashToken(token), staffID, now.Unix(), now.Add(SessionLength).Unix())
		return err
	})
	return token, err
}

// SessionStaff is the member a session token belongs to, if it is current.
func (db *DB) SessionStaff(ctx context.Context, token string, now time.Time) (Staff, error) {
	if token == "" {
		return Staff{}, sql.ErrNoRows
	}
	return scanStaff(db.QueryRowContext(ctx, `SELECT s.id, s.username, s.name, s.role, s.title, s.colour
		FROM staff_sessions x JOIN staff s ON s.id = x.staff_id WHERE x.token_hash = ? AND x.expires_at > ?`, hashToken(token), now.Unix()))
}

// EndSession signs a member out.
func (db *DB) EndSession(ctx context.Context, token string) error {
	return db.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM staff_sessions WHERE token_hash = ?`, hashToken(token))
		return err
	})
}
