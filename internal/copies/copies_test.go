package copies

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffgicharu/oriel/internal/store"
)

type sqlTx = sql.Tx

// tinyBuilder writes a sample with one setting recording when it was built.
func tinyBuilder(ctx context.Context, path string, now time.Time) error {
	db, err := store.OpenForBuild(path)
	if err != nil {
		return err
	}
	err = db.Tx(ctx, func(tx *sqlTx) error {
		return store.SetSetting(ctx, tx, "built", now.Format(time.RFC3339))
	})
	if err != nil {
		return err
	}
	return db.Seal(ctx)
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func openStore(t *testing.T, opt Options) (*Store, *time.Time) {
	t.Helper()
	clock := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	s, err := Open(context.Background(), t.TempDir(), opt, tinyBuilder, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return clock }
	t.Cleanup(s.Close)
	// Start from a sample built on the test's own clock.
	if err := s.RebuildSample(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, &clock
}

func TestSampleIsReadOnly(t *testing.T) {
	s, _ := openStore(t, DefaultOptions())
	db := s.Sample()
	if !db.ReadOnly() {
		t.Fatal("sample opened for writing")
	}
	if err := db.Tx(context.Background(), func(tx *sqlTx) error { return nil }); !errors.Is(err, store.ErrReadOnly) {
		t.Fatalf("a transaction on the sample gave %v", err)
	}
	if _, err := db.Exec("INSERT INTO settings (key, value) VALUES ('x', 'y')"); err == nil {
		t.Fatal("wrote to the sample")
	}
}

func TestCopyIsPrivate(t *testing.T) {
	s, _ := openStore(t, DefaultOptions())
	ctx := context.Background()
	a, err := s.Create("")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Release()
	b, err := s.Create("")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()
	if a.ID() == b.ID() || !ValidID(a.ID()) {
		t.Fatalf("ids %q and %q", a.ID(), b.ID())
	}
	if err := a.DB().Tx(ctx, func(tx *sqlTx) error { return store.SetSetting(ctx, tx, "mine", "a") }); err != nil {
		t.Fatal(err)
	}
	if got := b.DB().Setting(ctx, "mine", "none"); got != "none" {
		t.Fatalf("copy b sees copy a's change: %q", got)
	}
	if got := s.Sample().Setting(ctx, "mine", "none"); got != "none" {
		t.Fatalf("the sample sees a copy's change: %q", got)
	}
	again, err := s.Acquire(a.ID())
	if err != nil {
		t.Fatal(err)
	}
	defer again.Release()
	if got := again.DB().Setting(ctx, "mine", "none"); got != "a" {
		t.Fatalf("reopened copy lost its change: %q", got)
	}
}

func TestUnknownCopy(t *testing.T) {
	s, _ := openStore(t, DefaultOptions())
	for _, id := range []string{"", "nope", "../../etc/passwd", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if _, err := s.Acquire(id); !errors.Is(err, ErrNoCopy) {
			t.Errorf("Acquire(%q) gave %v", id, err)
		}
	}
}

func TestFullFolderEvictsTheOldestIdleCopy(t *testing.T) {
	opt := DefaultOptions()
	opt.MaxCopies = 3
	s, clock := openStore(t, opt)
	var ids []string
	for i := 0; i < 3; i++ {
		h, err := s.Create("")
		if err != nil {
			t.Fatal(err)
		}
		h.Release()
		os.Chtimes(s.copyPath(h.ID()), clock.Add(time.Duration(i-10)*time.Minute), clock.Add(time.Duration(i-10)*time.Minute))
		ids = append(ids, h.ID())
	}
	h, err := s.Create("")
	if err != nil {
		t.Fatalf("a fourth copy was refused: %v", err)
	}
	h.Release()
	if _, err := os.Stat(s.copyPath(ids[0])); !os.IsNotExist(err) {
		t.Error("the oldest copy was kept")
	}
	if _, err := os.Stat(s.copyPath(ids[1])); err != nil {
		t.Error("a newer copy was evicted")
	}
}

func TestBusyCopiesAreNeverEvicted(t *testing.T) {
	opt := DefaultOptions()
	opt.MaxCopies = 2
	s, _ := openStore(t, opt)
	a, _ := s.Create("")
	b, _ := s.Create("")
	defer a.Release()
	defer b.Release()
	if _, err := s.Create(""); !errors.Is(err, ErrFull) {
		t.Fatalf("with every copy in use, Create gave %v", err)
	}
}

func TestSweepDeletesExpiredCopies(t *testing.T) {
	s, clock := openStore(t, DefaultOptions())
	old, _ := s.Create("")
	old.Release()
	fresh, _ := s.Create("")
	fresh.Release()
	check, _ := s.Create("chk")
	check.Release()
	os.Chtimes(s.copyPath(old.ID()), clock.Add(-25*time.Hour), clock.Add(-25*time.Hour))
	os.Chtimes(s.copyPath(check.ID()), clock.Add(-11*time.Minute), clock.Add(-11*time.Minute))
	os.Chtimes(s.copyPath(fresh.ID()), clock.Add(-23*time.Hour), clock.Add(-23*time.Hour))
	if n := s.Sweep(); n != 2 {
		t.Fatalf("swept %d copies, want 2", n)
	}
	if _, err := os.Stat(s.copyPath(fresh.ID())); err != nil {
		t.Error("a copy used 23 hours ago was swept")
	}
}

func TestRebuildPointsAtTheNewSample(t *testing.T) {
	s, clock := openStore(t, DefaultOptions())
	ctx := context.Background()
	first := s.SampleBuiltAt()
	*clock = clock.Add(24 * time.Hour)
	if err := s.RebuildSample(ctx); err != nil {
		t.Fatal(err)
	}
	if !s.SampleBuiltAt().After(first) {
		t.Fatal("the new sample is not in use")
	}
	if got := s.Sample().Setting(ctx, "built", ""); got != clock.Format(time.RFC3339) {
		t.Fatalf("sample says it was built %q", got)
	}
	files, _ := filepath.Glob(filepath.Join(s.dir, "sample", "sample-*.db"))
	if len(files) > 2 {
		t.Errorf("%d samples kept, want at most 2", len(files))
	}
}

func TestDueRebuild(t *testing.T) {
	s, _ := openStore(t, DefaultOptions())
	cases := map[string]string{
		"2026-10-17T02:49:59Z": "2026-10-17T01:50:00Z",
		"2026-10-17T02:50:00Z": "2026-10-17T02:50:00Z",
		"2026-10-17T23:10:00Z": "2026-10-17T22:50:00Z",
		"2026-10-18T00:20:00Z": "2026-10-17T23:50:00Z",
	}
	for now, want := range cases {
		n, _ := time.Parse(time.RFC3339, now)
		if got := s.dueRebuild(n).Format(time.RFC3339); got != want {
			t.Errorf("at %s the due rebuild is %s, want %s", now, got, want)
		}
	}
}
