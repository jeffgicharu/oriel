package app

import (
	"net/http"
	"runtime/debug"
)

// contentSecurityPolicy allows only Oriel's own scripts, styles, fonts and
// pictures, plus Cloudflare's page-speed beacon. Styles may be inline because
// the 3D scene and the animations set element styles.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' https://static.cloudflareinsights.com; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self'; " +
	"connect-src 'self' https://cloudflareinsights.com; " +
	"worker-src 'self' blob:; " +
	"object-src 'none'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// headers sets the security headers every response carries, and asks search
// engines not to list the site.
func (a *App) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Strict-Transport-Security", "max-age=31536000")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

// recoverer turns a panic in a handler into Oriel's error page and a log line.
func (a *App) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				a.log.Error("panic", "path", r.URL.Path, "value", v, "stack", string(debug.Stack()))
				a.serverError(w, r)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
