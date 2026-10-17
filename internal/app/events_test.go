package app

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jeffgicharu/oriel/internal/config"
)

func testApp(t *testing.T) (*httptest.Server, *App) {
	t.Helper()
	cfg := config.Config{
		Addr:    "127.0.0.1:0",
		DataDir: t.TempDir(),
		SiteURL: "http://oriel.test",
		Secret:  []byte(strings.Repeat("s", 40)),
		DeskKey: []byte(strings.Repeat("d", 20)),
	}
	a, err := New(cfg, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(func() {
		srv.Close()
		a.Close()
		a.Shutdown()
	})
	return srv, a
}

// A change made the instant a browser sees its stream open must still reach
// it: the stream subscribes before it sends its first line.
func TestAChangeReachesAStreamTheMomentItOpens(t *testing.T) {
	srv, a := testApp(t)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	check := ReleaseCheckToken(a.cfg.Secret)
	ask := func(message string) {
		form := url.Values{"name": {"Stream Test"}, "email": {"stream@example.com"}, "phone": {"0712 345 678"}, "message": {message}}
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/enquire", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Oriel-Check", check)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("question answered %d", res.StatusCode)
		}
	}
	ask("The first question makes the copy.")

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
		stream, err := client.Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		lines := bufio.NewReader(stream.Body)
		if first, err := lines.ReadString('\n'); err != nil || !strings.HasPrefix(first, "retry:") {
			t.Fatalf("stream opened with %q (%v)", first, err)
		}
		ask("Another question, sent the moment the stream opened.")
		got := false
		for !got {
			line, err := lines.ReadString('\n')
			if err != nil {
				t.Fatalf("round %d: the change never arrived: %v", i+1, err)
			}
			got = strings.HasPrefix(line, "data:") && strings.Contains(line, `"type":"lead"`)
		}
		stream.Body.Close()
		cancel()
	}
}
