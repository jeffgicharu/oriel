package app

import (
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jeffgicharu/oriel/internal/store"
	"github.com/jeffgicharu/oriel/internal/web"
)

// formErrors maps a field to what is wrong with it.
type formErrors = map[string]string

var phoneDigits = regexp.MustCompile(`^\+?[0-9]{9,15}$`)

// readContact reads and checks the name, email and phone fields.
func readContact(r *http.Request, errs formErrors) store.Contact {
	c := store.Contact{
		Name:  strings.Join(strings.Fields(r.PostFormValue("name")), " "),
		Email: strings.TrimSpace(r.PostFormValue("email")),
		Phone: strings.TrimSpace(r.PostFormValue("phone")),
	}
	switch n := utf8.RuneCountInString(c.Name); {
	case n == 0:
		errs["name"] = "Please tell us your name."
	case n < 2:
		errs["name"] = "Please give your full name."
	case n > 80:
		errs["name"] = "That name is too long."
	}
	if c.Email == "" {
		errs["email"] = "Please give an email address."
	} else if a, err := mail.ParseAddress(c.Email); err != nil || a.Address != c.Email || !strings.Contains(c.Email[strings.LastIndex(c.Email, "@")+1:], ".") || len(c.Email) > 120 {
		errs["email"] = "That email address doesn’t look right."
	}
	phone, ok := normalisePhone(c.Phone)
	switch {
	case c.Phone == "":
		errs["phone"] = "Please give a phone number."
	case !ok:
		errs["phone"] = "Please give a phone number, such as 0712 345 678 or +254 712 345 678."
	default:
		c.Phone = phone
	}
	return c
}

// normalisePhone writes a Kenyan number as +254 7XX XXX XXX and leaves
// international numbers in their own form.
func normalisePhone(raw string) (string, bool) {
	s := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(raw)
	if !phoneDigits.MatchString(s) {
		return "", false
	}
	switch {
	case strings.HasPrefix(s, "0") && len(s) == 10:
		s = "+254" + s[1:]
	case strings.HasPrefix(s, "254") && len(s) == 12:
		s = "+" + s
	case strings.HasPrefix(s, "7") && len(s) == 9, strings.HasPrefix(s, "1") && len(s) == 9:
		s = "+254" + s
	}
	if strings.HasPrefix(s, "+254") && len(s) == 13 {
		return s[:4] + " " + s[4:7] + " " + s[7:10] + " " + s[10:], true
	}
	if !strings.HasPrefix(s, "+") {
		return "", false
	}
	return s, true
}

// isBot reports a form filled in by a robot: the field people never see was
// filled in.
func isBot(r *http.Request) bool {
	return r.PostFormValue("website") != ""
}

// formAllowed applies the per-address limit on forms.
func (a *App) formAllowed(r *http.Request) bool {
	return a.isReleaseCheck(r) || a.formLimiter.allow(clientIP(r), a.now())
}

func intField(r *http.Request, name string, min, max, fallback int) int {
	n, err := strconv.Atoi(r.PostFormValue(name))
	if err != nil || n < min || n > max {
		return fallback
	}
	return n
}

// busyPage is shown when no private copy can be made just now.
func (a *App) busy(w http.ResponseWriter, r *http.Request) {
	p := a.page(web.Meta{Title: "One moment", Description: "Oriel is very busy just now. Please try again in a minute.", Path: r.URL.Path, Quiet: true})
	w.Header().Set("Retry-After", "60")
	a.render(w, r, http.StatusServiceUnavailable, web.Busy(p))
}
