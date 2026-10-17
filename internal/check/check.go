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
	"regexp"
	"strings"
	"time"
)

// PublicPaths are the pages checked again through nginx once a release is live.
func PublicPaths() []string {
	return []string{
		"/",
		"/explore",
		"/residences",
		"/residences/0704",
		"/residences/types/B",
		"/building",
		"/location",
		"/progress",
		"/buying",
		"/privacy",
		"/terms",
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
	{"/?story", "homes free today."},
	{"/explore", "Explore the building"},
	{"/explore?home=0704&beds=2", "data-explorer"},
	{"/residences", "Find your home."},
	{"/residences?beds=2&floors=upper&show=all", "homes"},
	{"/residences/0704", "Home 0704"},
	{"/residences/1601", "Penthouse on the top floor"},
	{"/residences/types/B", "Two bedrooms."},
	{"/residences/types/P", "Penthouse."},
	{"/residences/types/A", "One bedroom."},
	{"/residences/types/C", "Three bedrooms."},
	{"/residences/types/D", "Three bedrooms, study and DSQ."},
	{"/building", "Built around the light."},
	{"/location", "Between the forest and the city."},
	{"/progress", "The programme"},
	{"/buying", "Three ways to pay."},
	{"/privacy", "Your privacy."},
	{"/terms", "Terms of use."},
	{"/viewings/new", "Book a viewing."},
	{"/enquire", "Ask us anything."},
}

// Checker walks a release at Base.
type Checker struct {
	Base    string
	Release string
	// Token marks the checker's requests as the release gate's, so the copy
	// it writes to is thrown away and its forms skip the per-address limits.
	Token  string
	client *http.Client
	out    io.Writer
	// reservation is the one the checker made, for the steps after.
	reservation string
}

// New makes a checker with its own cookie jar, as a fresh visitor would have.
func New(base, release, token string, out io.Writer) *Checker {
	return &Checker{
		Base:    strings.TrimRight(base, "/"),
		Release: release,
		Token:   token,
		out:     out,
		client: &http.Client{
			Jar:     newJar(),
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
		{"every link on those pages leads to a page", c.linksLead},
		{"robots.txt, the sitemap and the web manifest load", c.metadataFiles},
		{"an unknown address gets Oriel's own not-found page", c.notFound},
		{"the live-update stream opens", c.stream},
		{"a form with mistakes is shown again with each one explained", c.formMistakes},
		{"a visitor books a viewing, adds it to a calendar and cancels it", c.bookViewing},
		{"a visitor requests a reservation, and the held home can't be reserved twice", func(ctx context.Context) (err error) {
			c.reservation, err = c.reserve(ctx)
			return err
		}},
		{"a visitor's question reaches their open pages live", c.liveQuestion},
		{"a reservation is seen by the visitor who made it and nobody else", func(ctx context.Context) error {
			return c.privateCopy(ctx, c.reservation)
		}},
		{"the checker's own copy is deleted", c.forget},
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
	return c.do(c.client, req)
}

// mark labels a request as the release gate's.
func (c *Checker) mark(req *http.Request) {
	req.Header.Set("User-Agent", "oriel-release-check")
	if c.Token != "" {
		req.Header.Set("X-Oriel-Check", c.Token)
	}
}

func (c *Checker) do(client *http.Client, req *http.Request) (*http.Response, string, error) {
	c.mark(req)
	res, err := client.Do(req)
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
	srcsetRe      = regexp.MustCompile(`srcset="([^"]+)"`)
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
		// Every size of every picture.
		for _, m := range srcsetRe.FindAllStringSubmatch(body, -1) {
			for _, candidate := range strings.Split(m[1], ",") {
				if fields := strings.Fields(candidate); len(fields) > 0 && strings.HasPrefix(fields[0], "/assets/") {
					seen[fields[0]] = true
				}
			}
		}
	}
	if len(seen) == 0 {
		return errors.New("no page asks for any asset")
	}
	// Scripts load further pieces of themselves later (the 3D building, for
	// one); those must be there too.
	checked := map[string]bool{}
	queue := make([]string, 0, len(seen))
	for path := range seen {
		queue = append(queue, path)
	}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if checked[path] {
			continue
		}
		checked[path] = true
		res, body, err := c.get(ctx, path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: status %d", path, res.StatusCode)
		}
		if !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
			return fmt.Errorf("%s is not cached as immutable", path)
		}
		for ext, want := range map[string]string{".avif": "image/avif", ".webp": "image/webp", ".woff2": "font/woff2", ".css": "text/css", ".js": "text/javascript"} {
			if strings.HasSuffix(path, ext) && !strings.HasPrefix(res.Header.Get("Content-Type"), want) {
				return fmt.Errorf("%s is served as %q, want %s", path, res.Header.Get("Content-Type"), want)
			}
		}
		if strings.HasSuffix(path, ".js") {
			dir := path[:strings.LastIndex(path, "/")+1]
			for _, m := range chunkRe.FindAllStringSubmatch(body, -1) {
				if strings.HasPrefix(m[1], "/") {
					queue = append(queue, m[1])
				} else {
					queue = append(queue, cleanPath(dir+m[1]))
				}
			}
		}
	}
	return nil
}

var linkRe = regexp.MustCompile(`href="(/[^"#]*)(?:#[^"]*)?"`)

// linksLead follows every link to another page of the site from the pages
// checked, so a link to a page that does not exist never reaches a visitor.
func (c *Checker) linksLead(ctx context.Context) error {
	links := map[string]string{}
	for _, page := range pages {
		_, body, err := c.get(ctx, page.path)
		if err != nil {
			return err
		}
		for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
			link := strings.ReplaceAll(m[1], "&amp;", "&")
			if strings.HasPrefix(link, "/assets/") || link == "/events" {
				continue
			}
			if _, seen := links[link]; !seen {
				links[link] = page.path
			}
		}
	}
	if len(links) > 600 {
		return fmt.Errorf("%d links is more than the check expects", len(links))
	}
	for link, from := range links {
		res, _, err := c.get(ctx, link)
		if err != nil {
			return fmt.Errorf("%s (linked from %s): %w", link, from, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("%s (linked from %s): status %d", link, from, res.StatusCode)
		}
	}
	return nil
}

var chunkRe = regexp.MustCompile(`["']((?:/assets/js/|\.{1,2}/)(?:chunks/)?[A-Za-z0-9_-]+\.js)["']`)

// cleanPath resolves ./ and ../ in an asset path.
func cleanPath(p string) string {
	var out []string
	for _, part := range strings.Split(p, "/") {
		switch part {
		case ".":
		case "..":
			if len(out) > 1 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/")
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
	if !strings.Contains(body, "isn’t on the plans") {
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
	c.mark(req)
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
