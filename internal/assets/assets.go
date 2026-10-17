// Package assets serves the browser files built by web/build.mjs.
//
// Every file carries a fingerprint in its name, so it can be cached for a year.
// manifest.json maps the name a template asks for to the fingerprinted path.
// In production nginx serves the same files from disk; the handler here is the
// fallback, and what the release check uses before nginx is pointed at it.
package assets

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
)

//go:embed all:dist
var dist embed.FS

//go:embed public
var public embed.FS

var manifest = map[string]string{}

func init() {
	body, err := dist.ReadFile("dist/assets/manifest.json")
	if err != nil {
		// A build without the browser files still compiles; Check reports it.
		return
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		panic(fmt.Sprintf("assets: manifest.json: %v", err))
	}
}

// URL returns the fingerprinted path of a built file, such as
// URL("css/site.css") == "/assets/css/site-HRGIECZF.css". An unknown name
// returns an empty string, which Check catches before a release goes live.
func URL(name string) string {
	return manifest[name]
}

// Has reports whether the build produced a file with this name.
func Has(name string) bool {
	_, ok := manifest[name]
	return ok
}

// Names lists every name in the manifest, sorted.
func Names() []string {
	names := make([]string, 0, len(manifest))
	for name := range manifest {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Check reports the first file a release needs that is missing from the build.
func Check(required ...string) error {
	if len(manifest) == 0 {
		return fmt.Errorf("assets: the browser files were not built")
	}
	for _, name := range required {
		u, ok := manifest[name]
		if !ok {
			return fmt.Errorf("assets: %s is not in the manifest", name)
		}
		if _, err := fs.Stat(dist, "dist"+u); err != nil {
			return fmt.Errorf("assets: %s is in the manifest but %s is missing", name, u)
		}
	}
	return nil
}

// Handler serves /assets/ from the embedded build.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasSuffix(r.URL.Path, "/") || path.Base(r.URL.Path) == "manifest.json" {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	})
}

// PublicFile returns one of the files served at the site's root, such as
// favicon.ico, with its content type.
func PublicFile(name string) ([]byte, string, bool) {
	body, err := public.ReadFile("public/" + name)
	if err != nil {
		return nil, "", false
	}
	types := map[string]string{
		".ico": "image/x-icon",
		".svg": "image/svg+xml",
		".png": "image/png",
		".jpg": "image/jpeg",
	}
	return body, types[path.Ext(name)], true
}
