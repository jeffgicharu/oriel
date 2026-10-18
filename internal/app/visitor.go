package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jeffgicharu/oriel/internal/copies"
	"github.com/jeffgicharu/oriel/internal/store"
)

const copyCookie = "oriel_copy"

// visitor is the database a request reads and writes: the shared sample
// until the visitor first changes something, then their own copy.
type visitor struct {
	db       *store.DB
	copyID   string
	releases []func()
}

type visitorKey struct{}

func visitorFrom(ctx context.Context) *visitor {
	v, _ := ctx.Value(visitorKey{}).(*visitor)
	return v
}

// db is the database for this request.
func (a *App) db(r *http.Request) *store.DB {
	if v := visitorFrom(r.Context()); v != nil {
		return v.db
	}
	return a.copies.Sample()
}

// withVisitor finds the visitor's copy, if they have one, for the length of
// the request.
func (a *App) withVisitor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		v := &visitor{db: a.copies.Sample()}
		if id, ok := a.readCopyCookie(r); ok {
			h, err := a.copies.Acquire(id)
			switch {
			case err == nil:
				v.db, v.copyID = h.DB(), id
				v.releases = append(v.releases, h.Release)
			case errors.Is(err, copies.ErrNoCopy):
				// The copy expired; the visitor carries on with the sample.
				http.SetCookie(w, a.expiredCopyCookie())
			default:
				a.log.Error("opening a visitor's copy", "err", err)
			}
		}
		defer func() {
			for _, release := range v.releases {
				release()
			}
		}()
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), visitorKey{}, v)))
	})
}

// errBusy means no copy could be made just now; the visitor is asked to try
// again in a moment.
var errBusy = errors.New("busy")

// writable returns a database the request may change, making the visitor's
// own copy first if they do not have one yet.
func (a *App) writable(w http.ResponseWriter, r *http.Request) (*store.DB, error) {
	v := visitorFrom(r.Context())
	if v == nil {
		return nil, errors.New("no visitor")
	}
	if v.copyID != "" {
		return v.db, nil
	}
	check := a.isReleaseCheck(r)
	if !check && !a.copyLimiter.allow(clientIP(r), a.now()) {
		return nil, errBusy
	}
	prefix := ""
	if check {
		prefix = "chk"
	}
	h, err := a.copies.Create(prefix)
	if errors.Is(err, copies.ErrFull) {
		return nil, errBusy
	}
	if err != nil {
		return nil, err
	}
	v.db, v.copyID = h.DB(), h.ID()
	v.releases = append(v.releases, h.Release)
	http.SetCookie(w, a.copyCookieFor(h.ID()))
	return v.db, nil
}

func (a *App) sign(purpose, value string) string { return sign(a.cfg.Secret, purpose, value) }

func sign(secret []byte, purpose, value string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose + ":" + value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:18])
}

// ReleaseCheckToken is what the release gate sends in X-Oriel-Check, so its
// copies are marked as throwaway and its forms skip the per-address limits.
func ReleaseCheckToken(secret []byte) string { return sign(secret, "check", "release") }

func (a *App) copyCookieFor(id string) *http.Cookie {
	return &http.Cookie{
		Name:     copyCookie,
		Value:    id + "." + a.sign("copy", id),
		Path:     "/",
		MaxAge:   7 * 24 * 3600,
		HttpOnly: true,
		Secure:   strings.HasPrefix(a.cfg.SiteURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

func (a *App) expiredCopyCookie() *http.Cookie {
	c := a.copyCookieFor("")
	c.Value = ""
	c.MaxAge = -1
	return c
}

func (a *App) readCopyCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(copyCookie)
	if err != nil {
		return "", false
	}
	id, sig, ok := strings.Cut(c.Value, ".")
	if !ok || !copies.ValidID(id) {
		return "", false
	}
	if !hmac.Equal([]byte(sig), []byte(a.sign("copy", id))) {
		return "", false
	}
	return id, true
}

// isReleaseCheck reports whether the request comes from the release gate,
// whose copies are thrown away within minutes.
func (a *App) isReleaseCheck(r *http.Request) bool {
	got := r.Header.Get("X-Oriel-Check")
	return got != "" && hmac.Equal([]byte(got), []byte(a.sign("check", "release")))
}

// forget deletes the release gate's own copy the moment it has finished.
// Only the gate can call it, and only on a copy it made.
// deleteMine deletes the visitor's private copy at once, with everything they
// sent from this browser, instead of waiting for it to lapse.
func (a *App) deleteMine(w http.ResponseWriter, r *http.Request) {
	if v := visitorFrom(r.Context()); v != nil && v.copyID != "" {
		id := v.copyID
		for _, release := range v.releases {
			release()
		}
		v.releases, v.copyID, v.db = nil, "", a.copies.Sample()
		a.copies.Delete(id)
	}
	http.SetCookie(w, a.expiredCopyCookie())
	http.Redirect(w, r, "/privacy?deleted=1#yours", http.StatusSeeOther)
}

// clientIP is the visitor's address, as nginx passes it on from Cloudflare.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limiter allows a burst of events per address and refills slowly, so one
// address clicking in a loop uses up its own allowance and nobody else's.
type limiter struct {
	mu      sync.Mutex
	perHour float64
	burst   float64
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	at     time.Time
}

func newLimiter(perHour, burst float64) *limiter {
	return &limiter{perHour: perHour, burst: burst, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.at).Hours() * l.perHour
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.at = now
	if len(l.buckets) > 20000 {
		for k, old := range l.buckets {
			if now.Sub(old.at) > 2*time.Hour {
				delete(l.buckets, k)
			}
		}
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
