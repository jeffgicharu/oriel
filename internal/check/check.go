// Package check is the release gate. A new release runs it against itself,
// in the idle slot beside the live one, before any visitor is sent to it.
// It walks the site the way a visitor and a sales agent would, and fails on
// the first thing that does not work.
package check

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"
)

// PublicPaths are the pages checked again through nginx once a release is live.
func PublicPaths() []string {
	return []string{
		"/",
		"/robots.txt",
		"/sitemap.xml",
		"/site.webmanifest",
		"/favicon.ico",
		"/favicon.svg",
		"/apple-touch-icon.png",
	}
}

// pages are HTML pages, with a phrase each must contain.
var pages = []struct {
	path   string
	phrase string
}{
	{"/", "Homes designed around the light."},
}

// Checker walks a release at Base.
type Checker struct {
	Base    string
	Release string
	client  *http.Client
	out     io.Writer
}

// New makes a checker with its own cookie jar, as a fresh visitor would have.
func New(base, release string, out io.Writer) *Checker {
	jar, _ := cookiejar.New(nil)
	return &Checker{
		Base:    strings.TrimRight(base, "/"),
		Release: release,
		out:     out,
		client: &http.Client{
			Jar:     jar,
			Timeout: 20 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
}

// Run performs every check in order and stops at the first failure.
func (c *Checker) Run(ctx context.Context) error {
	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		{"the release answers as itself", c.health},
		{"every page loads with its title, description and share tags", c.pagesLoad},
		{"the files every page asks for load", c.assetsLoad},
		{"robots.txt, the sitemap and the web manifest load", c.metadataFiles},
		{"an unknown address gets Oriel's own not-found page", c.notFound},
		{"the live-update stream opens", c.stream},
	}
	for _, step := range steps {
		start := time.Now()
		if err := step.run(ctx); err != nil {
			fmt.Fprintf(c.out, "FAIL  %s: %v\n", step.name, err)
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Fprintf(c.out, "ok    %s (%d ms)\n", step.name, time.Since(start).Milliseconds())
	}
	return nil
}

func (c *Checker) get(ctx context.Context, path string) (*http.Response, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+path, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "oriel-release-check")
	res, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, "", err
	}
	return res, string(body), nil
}

func (c *Checker) health(ctx context.Context) error {
	res, _, err := c.get(ctx, "/healthz")
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	if got := res.Header.Get("X-Release"); got != c.Release {
		return fmt.Errorf("X-Release is %q, want %q", got, c.Release)
	}
	return nil
}

var (
	titleRe       = regexp.MustCompile(`<title>([^<]+)</title>`)
	descriptionRe = regexp.MustCompile(`<meta name="description" content="([^"]+)"`)
	canonicalRe   = regexp.MustCompile(`<link rel="canonical" href="https://[^"]+"`)
	ogImageRe     = regexp.MustCompile(`<meta property="og:image" content="https://[^"]+/assets/[^"]+"`)
	assetRe       = regexp.MustCompile(`(?:href|src)="(/assets/[^"]+)"`)
)

func (c *Checker) pagesLoad(ctx context.Context) error {
	for _, page := range pages {
		res, body, err := c.get(ctx, page.path)
		if err != nil {
			return fmt.Errorf("%s: %w", page.path, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: status %d", page.path, res.StatusCode)
		}
		if !strings.Contains(body, page.phrase) {
			return fmt.Errorf("%s does not say %q", page.path, page.phrase)
		}
		if m := titleRe.FindStringSubmatch(body); m == nil || strings.TrimSpace(m[1]) == "" {
			return fmt.Errorf("%s has no title", page.path)
		}
		if m := descriptionRe.FindStringSubmatch(body); m == nil || len(m[1]) < 50 {
			return fmt.Errorf("%s has no proper description", page.path)
		}
		if !canonicalRe.MatchString(body) {
			return fmt.Errorf("%s has no canonical address", page.path)
		}
		if !ogImageRe.MatchString(body) {
			return fmt.Errorf("%s has no share image", page.path)
		}
		if res.Header.Get("X-Robots-Tag") != "noindex, nofollow" {
			return fmt.Errorf("%s does not ask search engines to stay away", page.path)
		}
		if strings.Contains(body, `href=""`) || strings.Contains(body, `src=""`) {
			return fmt.Errorf("%s links to an empty address (a missing asset?)", page.path)
		}
	}
	return nil
}

func (c *Checker) assetsLoad(ctx context.Context) error {
	seen := map[string]bool{}
	for _, page := range pages {
		_, body, err := c.get(ctx, page.path)
		if err != nil {
			return err
		}
		for _, m := range assetRe.FindAllStringSubmatch(body, -1) {
			seen[m[1]] = true
		}
	}
	if len(seen) == 0 {
		return errors.New("no page asks for any asset")
	}
	for path := range seen {
		res, _, err := c.get(ctx, path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: status %d", path, res.StatusCode)
		}
		if !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
			return fmt.Errorf("%s is not cached as immutable", path)
		}
	}
	return nil
}

func (c *Checker) metadataFiles(ctx context.Context) error {
	for path, want := range map[string]string{
		"/robots.txt":       "Sitemap: ",
		"/sitemap.xml":      "<urlset",
		"/site.webmanifest": `"short_name"`,
	} {
		res, body, err := c.get(ctx, path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if res.StatusCode != http.StatusOK || !strings.Contains(body, want) {
			return fmt.Errorf("%s: status %d, missing %q", path, res.StatusCode, want)
		}
	}
	for _, path := range []string{"/favicon.ico", "/favicon.svg", "/apple-touch-icon.png"} {
		res, _, err := c.get(ctx, path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: status %d", path, res.StatusCode)
		}
	}
	return nil
}

func (c *Checker) notFound(ctx context.Context) error {
	res, body, err := c.get(ctx, "/no-such-page-on-the-plans")
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("status %d, want 404", res.StatusCode)
	}
	if !strings.Contains(body, "isn&#39;t on the plans") && !strings.Contains(body, "isn't on the plans") {
		return errors.New("not Oriel's own not-found page")
	}
	return nil
}

func (c *Checker) stream(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/events", nil)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return fmt.Errorf("content type %q", res.Header.Get("Content-Type"))
	}
	line, err := bufio.NewReader(res.Body).ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "retry:") {
		return fmt.Errorf("stream opened with %q", line)
	}
	return nil
}
