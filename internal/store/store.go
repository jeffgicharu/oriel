// Package store reads and writes Oriel's SQLite databases.
//
// There is one shared sample database, opened read-only, and one copy per
// visitor who has changed something. Both have the same schema, kept up to
// date by the numbered files in migrations/. A migration may only add: a
// release and the one before it run side by side for a minute, against the
// same files.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB is one Oriel database.
type DB struct {
	*sql.DB
	readOnly bool
}

// ReadOnly reports whether this is the shared sample, which no request may change.
func (db *DB) ReadOnly() bool { return db.readOnly }

// Open opens a database for reading and writing and brings its schema up to date.
func Open(path string) (*DB, error) {
	q := url.Values{}
	for _, p := range []string{"busy_timeout(5000)", "journal_mode(WAL)", "foreign_keys(1)", "synchronous(NORMAL)", "cache_size(-1024)"} {
		q.Add("_pragma", p)
	}
	q.Set("_txlock", "immediate")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	// One writer at a time is SQLite's rule anyway; a second connection lets
	// a read go ahead while a write is open.
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)
	db := &DB{DB: sqlDB}
	if err := db.Migrate(context.Background()); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("store: migrating %s: %w", path, err)
	}
	return db, nil
}

// OpenForBuild opens a new file for writing the sample. It skips the safety
// of a journal for speed: if the build fails the file is thrown away.
func OpenForBuild(path string) (*DB, error) {
	q := url.Values{}
	for _, p := range []string{"journal_mode(OFF)", "synchronous(OFF)", "foreign_keys(1)"} {
		q.Add("_pragma", p)
	}
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	db := &DB{DB: sqlDB}
	if err := db.Migrate(context.Background()); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Seal compacts a freshly built sample into one self-contained file and
// closes it, ready to be opened read-only and copied.
func (db *DB) Seal(ctx context.Context) error {
	for _, stmt := range []string{"PRAGMA foreign_key_check", "PRAGMA journal_mode = DELETE", "VACUUM", "PRAGMA optimize"} {
		rows, err := db.QueryContext(ctx, stmt)
		if err != nil {
			db.Close()
			return fmt.Errorf("%s: %w", stmt, err)
		}
		if stmt == "PRAGMA foreign_key_check" && rows.Next() {
			var table string
			var rowid sql.NullInt64
			var parent string
			var fkid int
			rows.Scan(&table, &rowid, &parent, &fkid)
			rows.Close()
			db.Close()
			return fmt.Errorf("store: %s row %d points at a missing %s", table, rowid.Int64, parent)
		}
		rows.Close()
	}
	return db.Close()
}

// OpenReadOnly opens the shared sample. The file is never changed once
// written, so it is opened as immutable and needs no locks.
func OpenReadOnly(path string) (*DB, error) {
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("immutable", "1")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "foreign_keys(1)")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	db := &DB{DB: sqlDB, readOnly: true}
	// A newer sample is fine (migrations only add); an older one is not.
	if v, err := db.SchemaVersion(context.Background()); err != nil {
		sqlDB.Close()
		return nil, err
	} else if v < LatestVersion() {
		sqlDB.Close()
		return nil, fmt.Errorf("store: sample %s is at schema %d, this release needs %d", path, v, LatestVersion())
	}
	return db, nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() []migration {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	var out []migration
	for _, e := range entries {
		number, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			panic("store: migration without a number: " + e.Name())
		}
		v, err := strconv.Atoi(number)
		if err != nil {
			panic("store: migration without a number: " + e.Name())
		}
		body, err := migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			panic(err)
		}
		out = append(out, migration{version: v, name: e.Name(), sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i, m := range out {
		if m.version != i+1 {
			panic(fmt.Sprintf("store: migrations must be numbered 1, 2, 3...; found %s", m.name))
		}
	}
	return out
}

// LatestVersion is the schema version this release expects.
func LatestVersion() int {
	return len(loadMigrations())
}

// SchemaVersion is the schema version a database is at.
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v)
	return v, err
}

// Migrate applies every migration the database has not had, each in its own
// transaction. A database newer than this release is left alone: the
// release before reads it, which is safe because migrations only add.
func (db *DB) Migrate(ctx context.Context) error {
	current, err := db.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	for _, m := range loadMigrations() {
		if m.version <= current {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Tx runs fn in a transaction, committing if it returns nil.
func (db *DB) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	if db.readOnly {
		return ErrReadOnly
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ErrReadOnly is returned when a change is attempted on the shared sample.
var ErrReadOnly = fmt.Errorf("store: the shared sample cannot be changed")

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = sql.ErrNoRows

// Unix converts a time to the stored form.
func Unix(t time.Time) int64 { return t.Unix() }

// FromUnix converts the stored form to a time.
func FromUnix(s int64) time.Time { return time.Unix(s, 0) }

// Settings

// Setting reads a setting, with a default when it is not set.
func (db *DB) Setting(ctx context.Context, key, fallback string) string {
	var v string
	if err := db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&v); err != nil {
		return fallback
	}
	return v
}

// SetSetting writes a setting.
func SetSetting(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	return err
}
