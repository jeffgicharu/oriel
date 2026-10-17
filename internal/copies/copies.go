// Package copies gives each visitor who changes something their own copy of
// Oriel's data.
//
// Everyone starts by reading the shared sample, which no request may change.
// A visitor's first change copies the sample to a file of their own, and from
// then on that browser reads and writes only its copy. A copy is deleted a day
// after it was last used, and the oldest idle copies make way when the folder
// reaches its limits. The sample is rebuilt every night with its dates moved on.
package copies

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jeffgicharu/oriel/internal/store"
)

// Errors a caller can act on.
var (
	// ErrNoCopy means the visitor's copy has gone (it expired).
	ErrNoCopy = errors.New("copies: no such copy")
	// ErrFull means every copy is in use and none can be made just now.
	ErrFull = errors.New("copies: no room for another copy")
)

// Builder writes a fresh sample database at path, as of now.
type Builder func(ctx context.Context, path string, now time.Time) error

// Options set the limits.
type Options struct {
	MaxCopies int
	MaxBytes  int64
	// IdleLife is how long a copy lives after it was last used.
	IdleLife time.Duration
	// Protect keeps a copy used this recently from being evicted to make room.
	Protect time.Duration
	// MaxOpen is how many copies may be open at once; others are reopened on use.
	MaxOpen int
	// RebuildAt is the time of day (UTC) the sample is rebuilt.
	RebuildHour, RebuildMinute int
	// SampleVersion is the sample story this release expects; a sample made
	// from another story is rebuilt when the store opens.
	SampleVersion string
}

// DefaultOptions are the limits agreed for the server.
func DefaultOptions() Options {
	return Options{
		MaxCopies:     900,
		MaxBytes:      1 << 30,
		IdleLife:      24 * time.Hour,
		Protect:       2 * time.Minute,
		MaxOpen:       24,
		RebuildHour:   2,
		RebuildMinute: 50,
	}
}

// Store manages the sample and the copies in one data folder.
type Store struct {
	dir   string
	opt   Options
	build Builder
	log   *slog.Logger
	now   func() time.Time

	mu          sync.Mutex
	sample      *store.DB
	samplePath  string
	sampleCheck time.Time
	open        map[string]*entry
}

type entry struct {
	db       *store.DB
	refs     int
	lastUsed time.Time
	touched  time.Time
}

// Handle is a copy in use by one request. Release it when done.
type Handle struct {
	s  *Store
	id string
	e  *entry
}

// ID is the copy's identifier, kept in the visitor's cookie.
func (h *Handle) ID() string { return h.id }

// DB is the copy's database.
func (h *Handle) DB() *store.DB { return h.e.db }

// Release lets the copy be closed when it has been idle for a while.
func (h *Handle) Release() {
	h.s.mu.Lock()
	h.e.refs--
	h.s.mu.Unlock()
}

// Open prepares the data folder, building the sample if there is none or if
// it is older than this release's schema.
func Open(ctx context.Context, dir string, opt Options, build Builder, log *slog.Logger) (*Store, error) {
	s := &Store{dir: dir, opt: opt, build: build, log: log, now: time.Now, open: map[string]*entry{}}
	for _, sub := range []string{"sample", "copies"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			return nil, err
		}
	}
	if err := s.loadSample(); err != nil {
		log.Info("building the sample", "reason", err.Error())
		if err := s.RebuildSample(ctx); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) link() string { return filepath.Join(s.dir, "sample.db") }

// loadSample opens the sample the link points at.
func (s *Store) loadSample() error {
	target, err := filepath.EvalSymlinks(s.link())
	if err != nil {
		return err
	}
	db, err := store.OpenReadOnly(target)
	if err != nil {
		return err
	}
	if want := s.opt.SampleVersion; want != "" && s.sample == nil {
		if got := db.Setting(context.Background(), "sample.version", ""); got != want {
			db.Close()
			return fmt.Errorf("the sample tells story %q, this release expects %q", got, want)
		}
	}
	s.mu.Lock()
	old := s.sample
	s.sample, s.samplePath, s.sampleCheck = db, target, s.now()
	s.mu.Unlock()
	if old != nil {
		// Requests already reading the old sample finish first.
		time.AfterFunc(time.Minute, func() { old.Close() })
	}
	return nil
}

// Sample is the shared, read-only database. A sample rebuilt by another
// process (the other release slot) is picked up within half a minute.
func (s *Store) Sample() *store.DB {
	s.mu.Lock()
	stale := s.now().Sub(s.sampleCheck) > 30*time.Second
	if stale {
		s.sampleCheck = s.now()
	}
	db, path := s.sample, s.samplePath
	s.mu.Unlock()
	if stale {
		if target, err := filepath.EvalSymlinks(s.link()); err == nil && target != path {
			if err := s.loadSample(); err != nil {
				s.log.Warn("could not open the new sample", "err", err)
			} else {
				s.mu.Lock()
				db = s.sample
				s.mu.Unlock()
			}
		}
	}
	return db
}

// SampleBuiltAt is when the sample in use was built.
func (s *Store) SampleBuiltAt() time.Time {
	s.mu.Lock()
	path := s.samplePath
	s.mu.Unlock()
	return builtAt(path)
}

func builtAt(path string) time.Time {
	name := strings.TrimSuffix(filepath.Base(path), ".db")
	stamp := strings.TrimPrefix(name, "sample-")
	t, err := time.Parse("20060102T150405Z", stamp)
	if err != nil {
		return time.Time{}
	}
	return t
}

// RebuildSample writes a new sample and points the link at it. Only one
// process does this at a time.
func (s *Store) RebuildSample(ctx context.Context) error {
	unlock, err := s.lock("sample")
	if err != nil {
		return err
	}
	defer unlock()

	now := s.now().UTC()
	name := "sample-" + now.Format("20060102T150405Z") + ".db"
	final := filepath.Join(s.dir, "sample", name)
	tmp := final + ".building"
	os.Remove(tmp)
	start := time.Now()
	if err := s.build(ctx, tmp, now); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("copies: building the sample: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	link := s.link()
	os.Remove(link + ".new")
	if err := os.Symlink(filepath.Join("sample", name), link+".new"); err != nil {
		return err
	}
	if err := os.Rename(link+".new", link); err != nil {
		return err
	}
	if err := s.loadSample(); err != nil {
		return err
	}
	s.log.Info("sample built", "file", name, "ms", time.Since(start).Milliseconds())
	s.pruneSamples(final)
	return nil
}

// pruneSamples keeps the newest two samples: the one in use and the one a
// release that has not noticed the change yet may still be reading.
func (s *Store) pruneSamples(current string) {
	files, _ := filepath.Glob(filepath.Join(s.dir, "sample", "sample-*.db"))
	sort.Strings(files)
	for i := 0; i < len(files)-2; i++ {
		if files[i] != current {
			os.Remove(files[i])
		}
	}
}

// lock takes an exclusive lock shared with the other release slot.
func (s *Store) lock(name string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(s.dir, "."+name+".lock"), os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

func (s *Store) copyPath(id string) string {
	return filepath.Join(s.dir, "copies", id+".db")
}

// ValidID reports whether id has the shape of a copy identifier.
func ValidID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '2' && c <= '7') {
			return false
		}
	}
	return true
}

// Acquire opens a visitor's existing copy.
func (s *Store) Acquire(id string) (*Handle, error) {
	if !ValidID(id) {
		return nil, ErrNoCopy
	}
	now := s.now()
	s.mu.Lock()
	if e, ok := s.open[id]; ok {
		e.refs++
		e.lastUsed = now
		touch := now.Sub(e.touched) > time.Minute
		if touch {
			e.touched = now
		}
		s.mu.Unlock()
		if touch {
			os.Chtimes(s.copyPath(id), now, now)
		}
		return &Handle{s: s, id: id, e: e}, nil
	}
	s.mu.Unlock()

	path := s.copyPath(id)
	if _, err := os.Stat(path); err != nil {
		return nil, ErrNoCopy
	}
	db, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	os.Chtimes(path, now, now)

	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.open[id]; ok {
		// Another request opened it meanwhile.
		db.Close()
		e.refs++
		e.lastUsed = now
		return &Handle{s: s, id: id, e: e}, nil
	}
	e := &entry{db: db, refs: 1, lastUsed: now, touched: now}
	s.open[id] = e
	s.closeIdleLocked()
	return &Handle{s: s, id: id, e: e}, nil
}

// Create makes a new copy of the sample for a visitor.
func (s *Store) Create(prefix string) (*Handle, error) {
	if err := s.makeRoom(); err != nil {
		return nil, err
	}
	id, err := newID(prefix)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	source := s.samplePath
	s.mu.Unlock()

	path := s.copyPath(id)
	if err := copyFile(source, path+".tmp"); err != nil {
		os.Remove(path + ".tmp")
		return nil, err
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return nil, err
	}
	return s.Acquire(id)
}

// Delete removes a copy at once.
func (s *Store) Delete(id string) {
	if !ValidID(id) {
		return
	}
	s.mu.Lock()
	if e, ok := s.open[id]; ok {
		delete(s.open, id)
		db := e.db
		time.AfterFunc(5*time.Second, func() { db.Close() })
	}
	s.mu.Unlock()
	s.removeFiles(id)
}

func (s *Store) removeFiles(id string) {
	base := s.copyPath(id)
	for _, suffix := range []string{"", "-wal", "-shm", ".tmp"} {
		os.Remove(base + suffix)
	}
}

type copyInfo struct {
	id      string
	size    int64
	modTime time.Time
}

func (s *Store) list() []copyInfo {
	entries, err := os.ReadDir(filepath.Join(s.dir, "copies"))
	if err != nil {
		return nil
	}
	byID := map[string]*copyInfo{}
	for _, e := range entries {
		name := e.Name()
		id := name
		for _, suffix := range []string{".db-wal", ".db-shm", ".db.tmp", ".db"} {
			if strings.HasSuffix(name, suffix) {
				id = strings.TrimSuffix(name, suffix)
				break
			}
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		c := byID[id]
		if c == nil {
			c = &copyInfo{id: id}
			byID[id] = c
		}
		c.size += info.Size()
		if strings.HasSuffix(name, ".db") && info.ModTime().After(c.modTime) {
			c.modTime = info.ModTime()
		}
	}
	out := make([]copyInfo, 0, len(byID))
	for _, c := range byID {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].modTime.Before(out[j].modTime) })
	return out
}

// makeRoom evicts the least recently used idle copies until a new one fits.
func (s *Store) makeRoom() error {
	all := s.list()
	var total int64
	for _, c := range all {
		total += c.size
	}
	sampleSize := int64(2 << 20)
	if info, err := os.Stat(s.samplePath); err == nil {
		sampleSize = info.Size()
	}
	now := s.now()
	for i := 0; len(all)-i >= s.opt.MaxCopies || total+sampleSize > s.opt.MaxBytes; i++ {
		if i >= len(all) {
			return ErrFull
		}
		c := all[i]
		if now.Sub(c.modTime) < s.opt.Protect || s.inUse(c.id) {
			return ErrFull
		}
		s.Delete(c.id)
		total -= c.size
		s.log.Info("copy evicted to make room", "idle", now.Sub(c.modTime).Round(time.Second).String())
	}
	return nil
}

func (s *Store) inUse(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.open[id]
	return ok && e.refs > 0
}

// closeIdleLocked closes the least recently used open copies over the limit.
func (s *Store) closeIdleLocked() {
	if len(s.open) <= s.opt.MaxOpen {
		return
	}
	type candidate struct {
		id string
		at time.Time
	}
	var idle []candidate
	for id, e := range s.open {
		if e.refs == 0 {
			idle = append(idle, candidate{id, e.lastUsed})
		}
	}
	sort.Slice(idle, func(i, j int) bool { return idle[i].at.Before(idle[j].at) })
	for _, c := range idle {
		if len(s.open) <= s.opt.MaxOpen {
			break
		}
		e := s.open[c.id]
		delete(s.open, c.id)
		e.db.Close()
	}
}

// Sweep deletes copies unused for longer than their life.
func (s *Store) Sweep() int {
	unlock, err := s.lock("sweep")
	if err != nil {
		return 0
	}
	defer unlock()
	now := s.now()
	removed := 0
	for _, c := range s.list() {
		life := s.opt.IdleLife
		if strings.HasPrefix(c.id, "chk") {
			life = 10 * time.Minute
		}
		if now.Sub(c.modTime) > life && !s.inUse(c.id) {
			s.Delete(c.id)
			removed++
		}
	}
	s.mu.Lock()
	s.closeIdleLocked()
	s.mu.Unlock()
	return removed
}

// Usage is the number of copies and the space they take.
func (s *Store) Usage() (count int, bytes int64) {
	for _, c := range s.list() {
		count++
		bytes += c.size
	}
	return count, bytes
}

// dueRebuild is the latest scheduled rebuild time at or before now.
func (s *Store) dueRebuild(now time.Time) time.Time {
	now = now.UTC()
	due := time.Date(now.Year(), now.Month(), now.Day(), s.opt.RebuildHour, s.opt.RebuildMinute, 0, 0, time.UTC)
	if due.After(now) {
		due = due.AddDate(0, 0, -1)
	}
	return due
}

// Run keeps the sample fresh and sweeps expired copies until ctx ends.
func (s *Store) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	lastSweep := time.Time{}
	for {
		now := s.now()
		if s.SampleBuiltAt().Before(s.dueRebuild(now)) {
			if err := s.RebuildSample(ctx); err != nil {
				s.log.Error("nightly rebuild failed", "err", err)
			}
		}
		if now.Sub(lastSweep) >= 10*time.Minute {
			if n := s.Sweep(); n > 0 {
				s.log.Info("expired copies deleted", "count", n)
			}
			lastSweep = now
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Close closes every open database.
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.open {
		e.db.Close()
		delete(s.open, id)
	}
	if s.sample != nil {
		s.sample.Close()
	}
}

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// newID is 32 characters of base32: a short prefix, then randomness.
func newID(prefix string) (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := strings.ToLower(idEncoding.EncodeToString(b))
	return (prefix + id)[:32], nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
