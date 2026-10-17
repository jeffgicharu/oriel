package check

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// The journeys a visitor takes through the forms. Each one writes to the
// checker's own throwaway copy, never to the shared sample.

// plainJar keeps cookies marked Secure even though the gate talks to the
// release over plain HTTP on the loopback address.
type plainJar struct{ http.CookieJar }

func (j plainJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _, c := range cookies {
		c.Secure = false
	}
	j.CookieJar.SetCookies(u, cookies)
}

func newJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return plainJar{jar}
}

// visitor is a made-up buyer the journeys fill the forms in as.
var visitor = url.Values{
	"name":  {"Release Check"},
	"email": {"release-check@example.com"},
	"phone": {"0712 345 678"},
}

func (c *Checker) post(ctx context.Context, path string, form url.Values) (*http.Response, string, error) {
	return c.send(ctx, c.client, http.MethodPost, path, form)
}

func (c *Checker) send(ctx context.Context, client *http.Client, method, path string, form url.Values) (*http.Response, string, error) {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return nil, "", err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return c.do(client, req)
}

func with(base url.Values, more ...string) url.Values {
	out := url.Values{}
	for k, v := range base {
		out[k] = append([]string(nil), v...)
	}
	for i := 0; i+1 < len(more); i += 2 {
		out.Set(more[i], more[i+1])
	}
	return out
}

func want(res *http.Response, body string, status int, phrases ...string) error {
	if res.StatusCode != status {
		return fmt.Errorf("%s: status %d, want %d", res.Request.URL.Path, res.StatusCode, status)
	}
	for _, p := range phrases {
		if !strings.Contains(body, p) {
			return fmt.Errorf("%s does not say %q", res.Request.URL.Path, p)
		}
	}
	return nil
}

// formMistakes sends each form empty and expects every mistake explained,
// with nothing saved.
func (c *Checker) formMistakes(ctx context.Context) error {
	res, body, err := c.post(ctx, "/viewings", url.Values{})
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusUnprocessableEntity, "Please tell us your name.", "Please give an email address.", "Please give a phone number.", "Please choose a time."); err != nil {
		return err
	}
	res, body, err = c.post(ctx, "/enquire", with(visitor, "email", "not-an-address", "phone", "12"))
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusUnprocessableEntity, "That email address doesn", "Please give a phone number, such as", "Please write your question.", "Release Check"); err != nil {
		return err
	}
	for _, ck := range c.client.Jar.Cookies(c.baseURL()) {
		if ck.Name == "oriel_copy" {
			return errors.New("a form with mistakes made a private copy")
		}
	}
	return nil
}

func (c *Checker) baseURL() *url.URL {
	u, _ := url.Parse(c.Base + "/")
	return u
}

var (
	slotRe    = regexp.MustCompile(`<input type="radio" name="slot" value="([0-9T:-]+)"([^>]*)>`)
	viewRefRe = regexp.MustCompile(`^/viewings/([A-Z0-9-]+)$`)
)

// bookViewing books the first free time, downloads it as a calendar file and
// cancels it.
func (c *Checker) bookViewing(ctx context.Context) error {
	_, page, err := c.get(ctx, "/viewings/new")
	if err != nil {
		return err
	}
	slot := ""
	for _, m := range slotRe.FindAllStringSubmatch(page, -1) {
		if !strings.Contains(m[2], "disabled") {
			slot = m[1]
			break
		}
	}
	if slot == "" {
		return errors.New("the booking page offers no free time")
	}
	res, body, err := c.post(ctx, "/viewings", with(visitor, "slot", slot, "party", "2"))
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusOK, "booked, Release.", "Add to calendar", "Cancel this viewing"); err != nil {
		return err
	}
	m := viewRefRe.FindStringSubmatch(res.Request.URL.Path)
	if m == nil || res.Request.URL.Query().Get("booked") != "1" {
		return fmt.Errorf("booking ended at %s", res.Request.URL)
	}
	ref := m[1]

	res, body, err = c.get(ctx, "/viewings/"+ref+"/calendar.ics")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/calendar") || !strings.Contains(body, "BEGIN:VEVENT") || !strings.Contains(body, "DTSTART:") {
		return fmt.Errorf("the calendar file is %q", res.Header.Get("Content-Type"))
	}

	res, body, err = c.post(ctx, "/viewings/"+ref+"/cancel", url.Values{})
	if err != nil {
		return err
	}
	return want(res, body, http.StatusOK, "Viewing cancelled.", "Book another time")
}

var (
	homeLinkRe = regexp.MustCompile(`href="/residences/([0-9]{4})"`)
	planRe     = regexp.MustCompile(`name="plan" value="([a-z-]+)"`)
	resRefRe   = regexp.MustCompile(`^/reservations/([A-Z0-9-]+)$`)
)

// reserve requests a reservation on a home for sale, and then finds it can't
// be reserved twice. It returns the reservation's reference.
func (c *Checker) reserve(ctx context.Context) (string, error) {
	_, list, err := c.get(ctx, "/residences")
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	for _, m := range homeLinkRe.FindAllStringSubmatch(list, -1) {
		id := m[1]
		if seen[id] || len(seen) >= 12 {
			continue
		}
		seen[id] = true
		_, form, err := c.get(ctx, "/reserve/"+id)
		if err != nil {
			return "", err
		}
		if !strings.Contains(form, `name="agree"`) {
			continue
		}
		plan := planRe.FindStringSubmatch(form)
		if plan == nil {
			return "", fmt.Errorf("/reserve/%s offers no payment plan", id)
		}
		res, body, err := c.post(ctx, "/reserve/"+id, with(visitor, "plan", plan[1], "agree", "yes", "buying", "home"))
		if err != nil {
			return "", err
		}
		if err := want(res, body, http.StatusOK, "Home "+id+" is held for you.", "Your payments on"); err != nil {
			return "", err
		}
		m := resRefRe.FindStringSubmatch(res.Request.URL.Path)
		if m == nil {
			return "", fmt.Errorf("the request ended at %s", res.Request.URL)
		}
		res, body, err = c.post(ctx, "/reserve/"+id, with(visitor, "plan", plan[1], "agree", "yes"))
		if err != nil {
			return "", err
		}
		if err := want(res, body, http.StatusConflict, "Home "+id+" is no longer available."); err != nil {
			return "", fmt.Errorf("reserving a held home again: %w", err)
		}
		return m[1], nil
	}
	return "", errors.New("no home on the residences page can be reserved")
}

// liveQuestion opens the live stream, sends a question, and expects the
// change to arrive on the stream.
func (c *Checker) liveQuestion(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/events", nil)
	if err != nil {
		return err
	}
	c.mark(req)
	stream, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer stream.Body.Close()
	lines := bufio.NewReader(stream.Body)
	if first, err := lines.ReadString('\n'); err != nil || !strings.HasPrefix(first, "retry:") {
		return fmt.Errorf("stream opened with %q (%v)", first, err)
	}

	res, body, err := c.post(ctx, "/enquire", with(visitor, "interest", "Two bedrooms", "message", "Is parking included with a two-bedroom home?"))
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusOK, "Thank you, Release."); err != nil {
		return err
	}
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			return fmt.Errorf("the question never reached the open stream: %w", err)
		}
		if strings.HasPrefix(line, "data:") && strings.Contains(line, `"type":"lead"`) {
			return nil
		}
	}
}

// privateCopy checks a stranger can't see the reservation, then has the
// checker's copy deleted and checks it is gone.
func (c *Checker) privateCopy(ctx context.Context, ref string) error {
	stranger := &http.Client{Jar: newJar(), Timeout: c.client.Timeout}
	res, _, err := c.send(ctx, stranger, http.MethodGet, "/reservations/"+ref, nil)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("another visitor sees reservation %s (status %d)", ref, res.StatusCode)
	}
	res, _, err = c.get(ctx, "/reservations/"+ref)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("the visitor who made reservation %s can't see it (status %d)", ref, res.StatusCode)
	}
	return nil
}

func (c *Checker) forget(ctx context.Context) error {
	res, _, err := c.post(ctx, "/internal/forget", url.Values{})
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	for _, ck := range c.client.Jar.Cookies(c.baseURL()) {
		if ck.Name == "oriel_copy" {
			return errors.New("the copy's cookie is still set")
		}
	}
	return nil
}
