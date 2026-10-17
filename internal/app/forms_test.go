package app

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPhoneNumbersAreWrittenOneWay(t *testing.T) {
	for raw, want := range map[string]string{
		"0712 345 678":     "+254 712 345 678",
		"0712-345-678":     "+254 712 345 678",
		"712345678":        "+254 712 345 678",
		"254712345678":     "+254 712 345 678",
		"+254 712 345 678": "+254 712 345 678",
		"(0110) 123 456":   "+254 110 123 456",
		"+44 20 7946 0958": "+442079460958",
	} {
		got, ok := normalisePhone(raw)
		if !ok || got != want {
			t.Errorf("normalisePhone(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "12", "0712 345", "phone me", "07123456789012345", "1234567890"} {
		if got, ok := normalisePhone(raw); ok {
			t.Errorf("normalisePhone(%q) accepted as %q", raw, got)
		}
	}
}

func TestContactFieldsAreChecked(t *testing.T) {
	read := func(form url.Values) formErrors {
		r := httptest.NewRequest("POST", "/enquire", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		errs := formErrors{}
		readContact(r, errs)
		return errs
	}
	good := url.Values{"name": {"  Wanjiru   Kamau "}, "email": {"wanjiru@example.co.ke"}, "phone": {"0722 000 111"}}
	if errs := read(good); len(errs) > 0 {
		t.Fatalf("a good contact gave %v", errs)
	}
	for field, value := range map[string]string{
		"name":  "W",
		"email": "wanjiru@localhost",
	} {
		form := url.Values{}
		for k, v := range good {
			form[k] = v
		}
		form.Set(field, value)
		if errs := read(form); errs[field] == "" {
			t.Errorf("%s %q was accepted", field, value)
		}
	}
	for _, email := range []string{"Wanjiru <wanjiru@example.com>", "wanjiru@", "@example.com", "wanjiru example.com"} {
		form := url.Values{"name": good["name"], "phone": good["phone"], "email": {email}}
		if errs := read(form); errs["email"] == "" {
			t.Errorf("email %q was accepted", email)
		}
	}
	if errs := read(url.Values{}); len(errs) != 3 {
		t.Errorf("an empty contact gave %v", errs)
	}
}

func TestLimiterRefills(t *testing.T) {
	l := newLimiter(60, 3)
	at := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if !l.allow("a", at) {
			t.Fatalf("refused within the burst at %d", i)
		}
	}
	if l.allow("a", at) {
		t.Fatal("allowed past the burst")
	}
	if !l.allow("b", at) {
		t.Fatal("one address used up another's allowance")
	}
	if !l.allow("a", at.Add(time.Minute+time.Second)) {
		t.Fatal("did not refill after a minute at 60 an hour")
	}
}

func TestBrokerKeepsCopiesApart(t *testing.T) {
	b := newBroker()
	mine, stop := b.subscribe("copy-a")
	defer stop()
	theirs, stopTheirs := b.subscribe("copy-b")
	defer stopTheirs()
	b.publish("copy-a", event{Type: "unit", ID: "0704", Status: "held"})
	select {
	case body := <-mine:
		if !strings.Contains(string(body), `"id":"0704"`) {
			t.Fatalf("got %s", body)
		}
	default:
		t.Fatal("the change did not reach its own copy")
	}
	select {
	case body := <-theirs:
		t.Fatalf("another copy saw %s", body)
	default:
	}
	b.publish("", event{Type: "unit"})
	stop()
	if len(b.subs["copy-a"]) != 0 {
		t.Fatal("unsubscribing left the channel behind")
	}
}

func TestSignedValuesDependOnTheSecret(t *testing.T) {
	a := sign([]byte(strings.Repeat("a", 32)), "copy", "x")
	b := sign([]byte(strings.Repeat("b", 32)), "copy", "x")
	c := sign([]byte(strings.Repeat("a", 32)), "check", "x")
	if a == b || a == c || len(a) != 24 {
		t.Fatalf("signatures %q %q %q", a, b, c)
	}
	if ReleaseCheckToken([]byte(strings.Repeat("a", 32))) != sign([]byte(strings.Repeat("a", 32)), "check", "release") {
		t.Fatal("the release token is not the signed check value")
	}
}
