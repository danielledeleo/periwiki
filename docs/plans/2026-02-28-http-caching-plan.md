# HTTP Caching Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add HTTP caching headers to all routes using a three-tier strategy (stable, conditional, uncacheable).

**Architecture:** Cache helpers in a new `internal/server/cache.go` set headers and handle conditional requests (304). Each handler calls the appropriate helper before writing the response. A `noStore` wrapper handles uncacheable routes at registration time.

**Tech Stack:** Go stdlib `net/http` (conditional request checking, time formatting)

**Design doc:** `docs/plans/2026-02-28-http-caching-design.md`

---

### Task 1: Cache Helper Functions

**Files:**
- Create: `internal/server/cache.go`
- Create: `internal/server/cache_test.go`

**Step 1: Write tests for cache helpers**

```go
// internal/server/cache_test.go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestSetCache|TestCheckNotModified|TestNoStore' -v`
Expected: FAIL — functions not defined

**Step 3: Implement cache helpers**

```go
// internal/server/cache.go
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
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run 'TestSetCache|TestCheckNotModified|TestNoStore' -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/server/cache.go internal/server/cache_test.go
git commit -m "feat: Add HTTP cache helper functions"
```

---

### Task 2: Static File Caching (serveFile + FileServer)

**Files:**
- Modify: `internal/server/app.go:92-98,206-216`

**Step 1: Write tests for static file caching**

Add to `internal/server/handlers_integration_test.go`:

```go
func TestStaticFileCaching(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	tests := []struct {
		path         string
		cacheControl string
	}{
		{"/favicon.ico", "public, max-age=86400"},
		{"/robots.txt", "public, max-age=86400"},
		{"/llms.txt", "public, max-age=86400"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if got := rr.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
			if got := rr.Header().Get("Last-Modified"); got == "" {
				t.Error("expected Last-Modified header to be set")
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestStaticFileCaching -v`
Expected: FAIL — no Cache-Control or Last-Modified headers

**Step 3: Update serveFile to add cache headers**

In `internal/server/app.go`, replace the `serveFile` function:

```go
func serveFile(fsys fs.FS, path string, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := fsys.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		data, err := io.ReadAll(f)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		setCacheStable(w, info.ModTime())
		if checkNotModified(w, r, "", info.ModTime()) {
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}
```

Add `"io"` to the import block in app.go.

**Step 4: Add Cache-Control middleware for static FileServer**

In `RegisterRoutes`, wrap the static file server:

```go
staticSub, _ := fs.Sub(contentFS, "static")
staticFS := http.FileServer(http.FS(staticSub))
router.PathPrefix("/static/").Handler(
	http.StripPrefix("/static/", cacheControlHandler(staticFS, "public, max-age=86400")),
)
```

Add this helper to `cache.go`:

```go
// cacheControlHandler wraps an http.Handler to add a Cache-Control header.
func cacheControlHandler(h http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestStaticFileCaching -v`
Expected: PASS

**Step 6: Commit**

```
git add internal/server/app.go internal/server/cache.go internal/server/handlers_integration_test.go
git commit -m "feat: Add caching headers to static file routes"
```

---

### Task 3: Article View Caching

**Files:**
- Modify: `internal/server/handlers.go:301-370` (handleView)

**Step 1: Write tests for article caching headers**

Add to `internal/server/handlers_integration_test.go`:

```go
func TestArticleCaching_CurrentRevision(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Cache_Test", "Content here.", user)

	req := httptest.NewRequest("GET", "/wiki/Cache_Test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("ETag"); got == "" {
		t.Error("expected ETag header to be set")
	}
	if got := rr.Header().Get("Last-Modified"); got == "" {
		t.Error("expected Last-Modified header to be set")
	}
}

func TestArticleCaching_OldRevision(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Old_Rev", "Version 1.", user)

	// Create a second revision
	article, _ := testApp.Articles.GetArticle("Old_Rev")
	article.Markdown = "Version 2."
	article.PreviousID = article.ID
	article.Creator = user
	testApp.Articles.PostArticle(article)

	// Request old revision (ID 1)
	req := httptest.NewRequest("GET", "/wiki/Old_Rev?revision=1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
}

func TestArticleCaching_304_ETag(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "ETag_Test", "Content.", user)

	// First request to get ETag
	req1 := httptest.NewRequest("GET", "/wiki/ETag_Test", nil)
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	etag := rr1.Header().Get("ETag")

	// Second request with If-None-Match
	req2 := httptest.NewRequest("GET", "/wiki/ETag_Test", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d", rr2.Code, http.StatusNotModified)
	}
	if rr2.Body.Len() != 0 {
		t.Error("expected empty body for 304 response")
	}
}

func TestArticleCaching_NotFound_NoCache(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Does_Not_Exist", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, want empty for 404", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestArticleCaching' -v`
Expected: FAIL

**Step 3: Add caching to handleView**

In `internal/server/handlers.go`, modify `handleView`:

For the **old revision** path (after line 313, before line 316):
```go
		// Tier 1: old revisions are stable per-run
		setCacheStable(rw, article.Created)
		if checkNotModified(rw, req, "", article.Created) {
			return
		}
```

For the **current revision** path (after line 347 `found := article != nil`, before line 354):
```go
	if found {
		etag := `W/"` + article.Hash + `"`
		setCacheConditional(rw, article.Hash, article.Created)
		if checkNotModified(rw, req, etag, article.Created) {
			return
		}
	}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run 'TestArticleCaching' -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./...`
Expected: PASS

**Step 6: Commit**

```
git add internal/server/handlers.go internal/server/handlers_integration_test.go
git commit -m "feat: Add conditional caching to article view handler"
```

---

### Task 4: Markdown, History, and Diff Caching

**Files:**
- Modify: `internal/server/handlers.go:179-187` (serveArticleMarkdown)
- Modify: `internal/server/handlers.go:405-438` (handleHistory)
- Modify: `internal/server/handlers.go:520-614` (handleDiff)

**Step 1: Write tests**

Add to `internal/server/handlers_integration_test.go`:

```go
func TestMarkdownCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Md_Cache", "# Hello", user)

	req := httptest.NewRequest("GET", "/wiki/Md_Cache.md", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("ETag"); got == "" {
		t.Error("expected ETag header")
	}
}

func TestHistoryCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Hist_Cache", "Content.", user)

	req := httptest.NewRequest("GET", "/wiki/Hist_Cache?history", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("Last-Modified"); got == "" {
		t.Error("expected Last-Modified header")
	}
}

func TestDiffCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Diff_Cache", "Version 1.", user)

	article, _ := testApp.Articles.GetArticle("Diff_Cache")
	oldID := article.ID
	article.Markdown = "Version 2."
	article.PreviousID = article.ID
	article.Creator = user
	testApp.Articles.PostArticle(article)

	req := httptest.NewRequest("GET", fmt.Sprintf("/wiki/Diff_Cache?diff&old=%d", oldID), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
}
```

Add `"fmt"` to the test file import block if not present.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestMarkdownCaching|TestHistoryCaching|TestDiffCaching' -v`
Expected: FAIL

**Step 3: Add caching to serveArticleMarkdown**

In `serveArticleMarkdown`, after fetching the article and before writing:

```go
func (a *App) serveArticleMarkdown(rw http.ResponseWriter, req *http.Request, articleURL string) {
	article, err := a.Articles.GetArticle(articleURL)
	if err != nil {
		http.NotFound(rw, req)
		return
	}
	etag := `W/"` + article.Hash + `"`
	setCacheConditional(rw, article.Hash, article.Created)
	if checkNotModified(rw, req, etag, article.Created) {
		return
	}
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.Write([]byte(article.Markdown))
}
```

**Step 4: Add caching to handleHistory**

After fetching revisions (line 417) and before building template data:

```go
	// Use newest revision's timestamp for caching
	if len(revisions) > 0 {
		setCacheConditional(rw, "", revisions[0].Created)
		if checkNotModified(rw, req, "", revisions[0].Created) {
			return
		}
	}
```

**Step 5: Add caching to handleDiff**

After both revisions are fetched and diff is computed, before `RenderTemplate` (before line 605):

```go
	// Diffs compare specific revisions — stable per-run
	setCacheStable(rw, newArticle.Created)
	if checkNotModified(rw, req, "", newArticle.Created) {
		return
	}
```

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/server/ -run 'TestMarkdownCaching|TestHistoryCaching|TestDiffCaching' -v`
Expected: PASS

**Step 7: Commit**

```
git add internal/server/handlers.go internal/server/handlers_integration_test.go
git commit -m "feat: Add caching to markdown, history, and diff handlers"
```

---

### Task 5: Sitemap Caching

**Files:**
- Modify: `special/sitemap.go:41-57,72-92,100-132,134-145`

**Step 1: Write tests**

Add to a new file or existing test file. Since sitemap is in the `special` package, add to the integration test that goes through the full router.

Add to `internal/server/handlers_integration_test.go`:

```go
func TestSitemapCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Sitemap_Cache", "Content.", user)

	paths := []string{"/sitemap.xml", "/sitemap.md", "/wiki/Special:Sitemap"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
				t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
			}
			if got := rr.Header().Get("Last-Modified"); got == "" {
				t.Error("expected Last-Modified header")
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestSitemapCaching -v`
Expected: FAIL

**Step 3: Add caching to SitemapPage.Handle**

In `special/sitemap.go`, set cache headers in the `Handle` method after fetching articles, before dispatching to format-specific handlers. Compute the newest `LastModified` across all articles:

```go
func (p *SitemapPage) Handle(rw http.ResponseWriter, req *http.Request) {
	articles, err := p.lister.GetAllArticles()
	if err != nil {
		slog.Error("failed to get articles for sitemap", "category", "special", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Find newest modification time for conditional caching
	var newest time.Time
	for _, a := range articles {
		if a.LastModified.After(newest) {
			newest = a.LastModified
		}
	}

	rw.Header().Set("Cache-Control", "public, no-cache")
	if !newest.IsZero() {
		rw.Header().Set("Last-Modified", newest.UTC().Format(http.TimeFormat))

		// Check conditional request
		if ims := req.Header.Get("If-Modified-Since"); ims != "" {
			t, parseErr := http.ParseTime(ims)
			if parseErr == nil && !newest.Truncate(time.Second).After(t.Truncate(time.Second)) {
				rw.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// Detect format from URL path
	if strings.HasSuffix(req.URL.Path, ".xml") {
		p.handleXML(rw, articles)
	} else if strings.HasSuffix(req.URL.Path, ".md") {
		p.handleMarkdown(rw, articles)
	} else {
		p.handleHTML(rw, req, articles)
	}
}
```

Add `"time"` to the import block if not present.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestSitemapCaching -v`
Expected: PASS

**Step 5: Commit**

```
git add special/sitemap.go internal/server/handlers_integration_test.go
git commit -m "feat: Add conditional caching to sitemap handlers"
```

---

### Task 6: Embedded Article Caching

**Files:**
- Modify: `internal/server/handlers.go:752-772` (Periwiki namespace in NamespaceHandler)

**Step 1: Write test**

Add to `internal/server/handlers_integration_test.go`:

```go
func TestEmbeddedArticleCaching(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	// Periwiki:Main_Page is an embedded help article
	req := httptest.NewRequest("GET", "/wiki/Periwiki:Main_Page", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
}
```

Note: Verify the correct embedded article URL by checking `help/` directory contents. Adjust the URL if needed (e.g. `Periwiki:Writing_articles`).

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestEmbeddedArticleCaching -v`
Expected: FAIL

**Step 3: Add caching to Periwiki namespace handler**

In `handlers.go`, in the Periwiki namespace block, after fetching the article and before `RenderTemplate`:

```go
	// Periwiki namespace: embedded help articles
	if strings.EqualFold(namespace, "periwiki") {
		if mdPage, ok := strings.CutSuffix(page, ".md"); ok {
			a.serveArticleMarkdown(rw, req, "Periwiki:"+mdPage)
			return
		}
		articleURL := "Periwiki:" + page
		article, err := a.Articles.GetArticle(articleURL)
		if err != nil {
			a.ErrorHandler(http.StatusNotFound, rw, req, err)
			return
		}
		// Tier 1: embedded articles are stable per-run
		setCacheStable(rw, article.Created)
		if checkNotModified(rw, req, "", article.Created) {
			return
		}
		err = a.RenderTemplate(rw, "article.html", "index.html", map[string]interface{}{
			...
		})
		check(err)
		return
	}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestEmbeddedArticleCaching -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/server/handlers.go internal/server/handlers_integration_test.go
git commit -m "feat: Add stable caching to embedded help articles"
```

---

### Task 7: Uncacheable Routes (noStore wrapper)

**Files:**
- Modify: `internal/server/app.go:121-134` (route registration)

**Step 1: Write tests**

Add to `internal/server/handlers_integration_test.go`:

```go
func TestUncacheableRoutes(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	paths := []string{
		"/user/login",
		"/user/register",
		"/manage/users",
		"/manage/settings",
		"/manage/tools",
		"/manage/content",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if got := rr.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
		})
	}
}
```

Note: Some manage routes will redirect (302) to login. That's fine — the `no-store` wrapper runs before the handler, so the redirect response will still have the header.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestUncacheableRoutes -v`
Expected: FAIL

**Step 3: Wrap uncacheable routes with noStore**

In `internal/server/app.go`, wrap all `/user/*` and `/manage/*` routes:

```go
	router.HandleFunc("/user/register", noStore(a.RegisterHandler)).Methods("GET")
	router.HandleFunc("/user/register", noStore(a.RegisterPostHandler)).Methods("POST")
	router.HandleFunc("/user/login", noStore(a.LoginHandler)).Methods("GET")
	router.HandleFunc("/user/login", noStore(a.LoginPostHandler)).Methods("POST")
	router.HandleFunc("/user/logout", noStore(a.LogoutPostHandler)).Methods("POST")

	router.HandleFunc("/manage/users", noStore(a.ManageUsersHandler)).Methods("GET")
	router.HandleFunc("/manage/users/{id:[0-9]+}", noStore(a.ManageUserRoleHandler)).Methods("POST")
	router.HandleFunc("/manage/settings", noStore(a.ManageSettingsHandler)).Methods("GET")
	router.HandleFunc("/manage/settings", noStore(a.ManageSettingsPostHandler)).Methods("POST")
	router.HandleFunc("/manage/tools", noStore(a.ManageToolsHandler)).Methods("GET")
	router.HandleFunc("/manage/tools/reset-main-page", noStore(a.ResetMainPageHandler)).Methods("POST")
	router.HandleFunc("/manage/tools/backfill-links", noStore(a.BackfillLinksHandler)).Methods("POST")
	router.HandleFunc("/manage/content", noStore(a.ManageContentHandler)).Methods("GET")
```

Also add `no-store` to the edit handler. In `handleEdit` (handlers.go), add at the top after the embedded/talk page redirect checks:

```go
	rw.Header().Set("Cache-Control", "no-store")
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestUncacheableRoutes -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/server/app.go internal/server/handlers.go internal/server/handlers_integration_test.go
git commit -m "feat: Add no-store headers to auth, manage, and edit routes"
```

---

### Task 8: Drop X-UA-Compatible

**Files:**
- Modify: `templates/layouts/index.html:5`

**Step 1: Remove the meta tag**

Delete line 5 from `templates/layouts/index.html`:

```html
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
```

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS

**Step 3: Commit**

```
git add templates/layouts/index.html
git commit -m "style: Drop X-UA-Compatible meta tag"
```

---

### Task 9: Final Verification

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

**Step 2: Build binary**

Run: `make`
Expected: Clean build

**Step 3: Manual smoke test (optional)**

Start server and verify headers with curl:

```bash
# Tier 1: static
curl -sI http://localhost:8080/favicon.ico | grep -E 'Cache-Control|Last-Modified'

# Tier 2: article
curl -sI http://localhost:8080/wiki/Main_Page | grep -E 'Cache-Control|ETag|Last-Modified'

# Tier 2: 304 response
ETAG=$(curl -sI http://localhost:8080/wiki/Main_Page | grep ETag | tr -d '\r' | cut -d' ' -f2)
curl -sI -H "If-None-Match: $ETAG" http://localhost:8080/wiki/Main_Page | head -1

# Tier 3: uncacheable
curl -sI http://localhost:8080/user/login | grep Cache-Control
```
