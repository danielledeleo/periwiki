package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetCacheStable(t *testing.T) {
	rr := httptest.NewRecorder()
	lastMod := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	setCacheStable(rr, lastMod)

	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
	if got := rr.Header().Get("Last-Modified"); got != "Sat, 28 Feb 2026 12:00:00 GMT" {
		t.Errorf("Last-Modified = %q, want %q", got, "Sat, 28 Feb 2026 12:00:00 GMT")
	}
}

func TestSetCacheConditional(t *testing.T) {
	rr := httptest.NewRecorder()
	lastMod := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	setCacheConditional(rr, "abc123", lastMod)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("ETag"); got != `W/"abc123"` {
		t.Errorf("ETag = %q, want %q", got, `W/"abc123"`)
	}
	if got := rr.Header().Get("Last-Modified"); got != "Sat, 28 Feb 2026 12:00:00 GMT" {
		t.Errorf("Last-Modified = %q, want %q", got, "Sat, 28 Feb 2026 12:00:00 GMT")
	}
}

func TestCheckNotModified_ETagMatch(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", `W/"abc123"`)

	if !checkNotModified(rr, req, `W/"abc123"`, time.Time{}) {
		t.Error("expected checkNotModified to return true for matching ETag")
	}
	if rr.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotModified)
	}
}

func TestCheckNotModified_ETagMismatch(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", `W/"old"`)

	if checkNotModified(rr, req, `W/"abc123"`, time.Time{}) {
		t.Error("expected checkNotModified to return false for mismatched ETag")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCheckNotModified_LastModifiedMatch(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	lastMod := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	req.Header.Set("If-Modified-Since", lastMod.UTC().Format(http.TimeFormat))

	if !checkNotModified(rr, req, "", lastMod) {
		t.Error("expected checkNotModified to return true when not modified since")
	}
	if rr.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotModified)
	}
}

func TestCheckNotModified_ModifiedAfter(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	clientTime := time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC)
	serverTime := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	req.Header.Set("If-Modified-Since", clientTime.UTC().Format(http.TimeFormat))

	if checkNotModified(rr, req, "", serverTime) {
		t.Error("expected checkNotModified to return false when modified after client time")
	}
}

func TestCheckNotModified_NoConditionalHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	if checkNotModified(rr, req, `W/"abc"`, time.Now()) {
		t.Error("expected checkNotModified to return false with no conditional headers")
	}
}

func TestNoStore(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	handler := noStore(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if rr.Body.String() != "ok" {
		t.Error("inner handler was not called")
	}
}
