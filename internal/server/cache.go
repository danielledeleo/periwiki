package server

import (
	"net/http"
	"time"
)

// setCacheStable sets Tier 1 headers for content that is fixed for the
// lifetime of the server process (static assets, old revisions, embedded articles).
func setCacheStable(w http.ResponseWriter, lastMod time.Time) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if !lastMod.IsZero() {
		w.Header().Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
	}
}

// setCacheConditional sets Tier 2 headers for content that changes when
// articles are edited (current articles, sitemaps, history).
func setCacheConditional(w http.ResponseWriter, etag string, lastMod time.Time) {
	w.Header().Set("Cache-Control", "public, no-cache")
	if etag != "" {
		w.Header().Set("ETag", `W/"`+etag+`"`)
	}
	if !lastMod.IsZero() {
		w.Header().Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
	}
}

// checkNotModified checks If-None-Match and If-Modified-Since request headers.
// Returns true and writes 304 if the client's cached copy is still fresh.
// The etag parameter should be the full ETag value including W/ prefix and quotes.
func checkNotModified(w http.ResponseWriter, r *http.Request, etag string, lastMod time.Time) bool {
	// ETag takes priority per RFC 7232
	if inmatch := r.Header.Get("If-None-Match"); inmatch != "" && etag != "" {
		if inmatch == etag {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
		return false
	}

	if ims := r.Header.Get("If-Modified-Since"); ims != "" && !lastMod.IsZero() {
		t, err := http.ParseTime(ims)
		if err == nil && !lastMod.Truncate(time.Second).After(t.Truncate(time.Second)) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	return false
}

// noStore wraps a handler to set Cache-Control: no-store (Tier 3).
func noStore(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		handler(w, r)
	}
}

// cacheControlHandler wraps an http.Handler to add a Cache-Control header.
func cacheControlHandler(h http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}
