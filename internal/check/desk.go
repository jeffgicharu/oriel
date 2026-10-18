package check

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// The sales desk, walked as the manager and as an agent, each in a throwaway
// copy of their own that is deleted afterwards.

var (
	deskCellRe  = regexp.MustCompile(`data-unit="[0-9]{4}"`)
	confirmRe   = regexp.MustCompile(`action="/desk/reservations/(R-[A-Z0-9]+)/confirm"`)
	offerLinkRe = regexp.MustCompile(`href="(/desk/reservations/R-[A-Z0-9]+/offer\.pdf)"`)
)

func (c *Checker) deskClient() *http.Client {
	return &http.Client{Jar: newJar(), Timeout: c.client.Timeout}
}

func (c *Checker) deskGet(ctx context.Context, client *http.Client, path string) (*http.Response, string, error) {
	return c.send(ctx, client, http.MethodGet, path, nil)
}

// deskManager signs in as the manager by link and works through the desk.
func (c *Checker) deskManager(ctx context.Context) error {
	if c.EnterManager == "" {
		return errors.New("no desk key to sign in with")
	}
	client := c.deskClient()
	res, body, err := c.deskGet(ctx, client, "/desk")
	if err != nil {
		return err
	}
	if res.Request.URL.Path != "/desk/login" || !strings.Contains(body, "Sign in") {
		return fmt.Errorf("the desk opened without a sign-in (%s)", res.Request.URL.Path)
	}
	res, body, err = c.deskGet(ctx, client, "/desk/enter/manager/"+c.EnterManager)
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusOK, "Every home by status", "Grace Wambui"); err != nil {
		return fmt.Errorf("signing in by link: %w", err)
	}
	defer c.send(ctx, client, http.MethodPost, "/privacy/delete", url.Values{})

	res, body, err = c.deskGet(ctx, client, "/desk/stack")
	if err != nil {
		return err
	}
	if n := len(deskCellRe.FindAllString(body, -1)); res.StatusCode != http.StatusOK || n != 86 {
		return fmt.Errorf("the stacking plan shows %d homes (status %d)", n, res.StatusCode)
	}
	for path, phrase := range map[string]string{
		"/desk/leads":            "Drag a lead",
		"/desk/leads?view=table": "Last activity",
		"/desk/viewings":         "Opening hours",
		"/desk/pricing":          "A new price list",
		"/desk/activity":         "Who changed what",
		"/desk/leads/new":        "Add a lead",
	} {
		res, body, err := c.deskGet(ctx, client, path)
		if err != nil {
			return err
		}
		if err := want(res, body, http.StatusOK, phrase); err != nil {
			return err
		}
	}

	res, body, err = c.deskGet(ctx, client, "/desk/leads.csv")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") || !strings.HasPrefix(body, "Name,Email,Phone,Stage") {
		return fmt.Errorf("the leads export is %q", res.Header.Get("Content-Type"))
	}

	// Preview a price list without publishing it.
	_, body, _ = c.deskGet(ctx, client, "/desk/pricing")
	form := url.Values{"note": {"Release check"}}
	for _, code := range []string{"A", "B", "C", "D", "P"} {
		m := regexp.MustCompile(`name="` + code + `" type="text" inputmode="numeric" value="([0-9,]+)"`).FindStringSubmatch(body)
		if m == nil {
			return fmt.Errorf("the pricing form has no base price for type %s", code)
		}
		form.Set(code, m[1])
	}
	form.Set("floor", "1")
	form.Set("river", "4")
	res, body, err = c.send(ctx, client, http.MethodPost, "/desk/pricing/preview", form)
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusOK, "What this list would do", "Publish the new prices"); err != nil {
		return err
	}

	// Confirm the first waiting reservation and fetch its letter of offer.
	_, body, err = c.deskGet(ctx, client, "/desk/reservations")
	if err != nil {
		return err
	}
	m := confirmRe.FindStringSubmatch(body)
	if m == nil {
		return errors.New("no reservation is waiting in the sample to confirm")
	}
	res, body, err = c.send(ctx, client, http.MethodPost, "/desk/reservations/"+m[1]+"/confirm", url.Values{})
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusOK, "Letter of offer OR-"); err != nil {
		return fmt.Errorf("confirming %s: %w", m[1], err)
	}
	link := offerLinkRe.FindStringSubmatch(body)
	if link == nil {
		return errors.New("the confirmed reservation has no letter of offer")
	}
	res, body, err = c.deskGet(ctx, client, link[1])
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(body, "%PDF-") || len(body) < 10_000 {
		return fmt.Errorf("the letter of offer is %q, %d bytes", res.Header.Get("Content-Type"), len(body))
	}
	return nil
}

// deskAgent signs in as an agent and checks the manager's pages stay shut.
func (c *Checker) deskAgent(ctx context.Context) error {
	if c.EnterAgent == "" {
		return errors.New("no desk key to sign in with")
	}
	client := c.deskClient()
	res, body, err := c.deskGet(ctx, client, "/desk/enter/agent/"+c.EnterAgent)
	if err != nil {
		return err
	}
	if err := want(res, body, http.StatusOK, "Every home by status"); err != nil {
		return fmt.Errorf("signing in as an agent: %w", err)
	}
	defer c.send(ctx, client, http.MethodPost, "/privacy/delete", url.Values{})
	if strings.Contains(body, `href="/desk/pricing"`) {
		return errors.New("an agent's menu offers pricing")
	}
	res, _, err = c.deskGet(ctx, client, "/desk/pricing")
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusForbidden {
		return fmt.Errorf("an agent opened pricing (status %d)", res.StatusCode)
	}
	res, _, err = c.send(ctx, client, http.MethodPost, "/desk/pricing", url.Values{"A": {"1"}})
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusForbidden {
		return fmt.Errorf("an agent could post prices (status %d)", res.StatusCode)
	}
	return nil
}
