# Alias and Redirect System Design

**Status:** Blocked on frontmatter parsing (query param routing complete ✓)

## Overview

Three mechanisms for URL indirection:

| Type | Example | Behavior | Source |
|------|---------|----------|--------|
| **Rewrite** | `/sitemap.xml` → `Special:Sitemap.xml` | Silent rewrite (200) | Config |
| **Config Redirect** | `/old-wiki/(.*)` → `/wiki/$1` | HTTP 301/302 | Config |
| **Article Redirect** | `/wiki/Prince` → `Prince_(Disambiguation)` | HTTP 301 | Article frontmatter |

## Config-Based URL Rules

Admin-defined rules in `config.yaml`. Both rewrites and redirects support exact matches and regex patterns.

### Example config

```yaml
rewrites:
  # Exact match
  - pattern: "/sitemap.xml"
    target: "/wiki/Special:Sitemap.xml"
  # Regex match
  - pattern: "^/static/legacy/(.*)$"
    target: "/static/$1"
    regex: true

redirects:
  # Case-insensitive article access:
  - pattern: "^/wiki/([A-Z][a-z]+)$"
    target: "/wiki/$1"
    code: 301
    regex: true
  # Legacy URL migration:
  - pattern: "^/old-wiki/(.*)$"
    target: "/wiki/$1"
    code: 301
    regex: true
  # Exact match redirect:
  - pattern: "/about"
    target: "/wiki/About"
    code: 302
```

### Implementation

1. **Compile on startup** — regex patterns compiled once, stored in memory
2. **Result caching** — cache `url → result` for recent requests (LRU, configurable size)
3. **Middleware evaluation** — check rules before routing
4. **Rewrites** — modify `req.URL.Path` internally, continue to router
5. **Redirects** — return HTTP 301/302 immediately

### Regex safety

Admin-defined patterns are trusted. To prevent ReDoS:
- Warn on patterns with nested quantifiers at config load
- Consider optional timeout on regex evaluation

## Article Redirects

HTTP 301 redirects defined via article frontmatter. Changes browser URL.

### Frontmatter syntax

```yaml
---
redirect: Prince_(Disambiguation)
---
```

**Note:** Use plain article name, not wikilink syntax. `redirect: [[Prince]]` would be parsed by YAML as a nested array.

### Database schema

```sql
CREATE TABLE Redirect (
    source_url      TEXT PRIMARY KEY,  -- article URL with redirect frontmatter
    immediate_target TEXT NOT NULL,    -- direct target from frontmatter
    final_target    TEXT NOT NULL,     -- resolved chain endpoint
    is_loop         BOOLEAN DEFAULT FALSE,
    chain_warning   TEXT,              -- e.g., "A → B → C"
    updated_at      TIMESTAMP
);

CREATE INDEX idx_redirect_final ON Redirect(final_target);
CREATE INDEX idx_redirect_immediate ON Redirect(immediate_target);
```

**Why both `immediate_target` and `final_target`?**
- `immediate_target`: What the frontmatter says (for display, debugging)
- `final_target`: Pre-resolved endpoint (for fast lookups)
- Index on `immediate_target`: Enables reverse lookup for chain recomputation

### In-memory cache

Redirect table is **lazily loaded** from database:
- On first request for article X, check cache → miss → query DB → cache result
- On article save with redirect, update DB → invalidate cache entry
- Cache: `map[string]*RedirectEntry` with LRU eviction

### Chain resolution (on article save)

```
Article saved: A with redirect → B
  1. Query DB: SELECT final_target FROM Redirect WHERE source_url = 'B'
  2. If B has no redirect: store A → B (immediate=B, final=B)
  3. If B → C (final): store A → B (immediate=B, final=C), log chain warning
  4. Query DB: SELECT source_url FROM Redirect WHERE immediate_target = 'A'
  5. For each article pointing to A, recompute their chains
  6. Detect loops: if final_target = source_url, set is_loop = TRUE
```

### Which routes redirect?

| Route | Redirects? |
|-------|------------|
| `/wiki/Prince` | Yes |
| `/wiki/Prince?redirect=no` | No |
| `/wiki/Prince?history` | No |
| `/wiki/Prince?edit` | No |
| `/wiki/Prince?revision={id}` | No |
| `/wiki/Prince?diff` | No |

Only the default "view" action triggers the redirect.

### Passing redirect info to render context

When a redirect is followed, the target page needs to know where the user came from.

**Approach: Referer header**

When browser follows a 301, it sends `Referer: http://host/wiki/Prince` on the subsequent request.

```go
func (a *app) handleView(...) {
    referer := req.Header.Get("Referer")
    if refererURL, err := url.Parse(referer); err == nil {
        refererArticle := extractArticleURL(refererURL.Path)
        // Check if referer is a redirect source for THIS article
        if a.redirects.IsSourceFor(refererArticle, articleURL) {
            render["RedirectedFrom"] = refererArticle
        }
    }
}
```

**Why this works:**
- No URL pollution (`?from=` param)
- No cookies needed
- Self-validating — we check the redirect table, so arbitrary Referer values can't spoof the note
- Browser handles it automatically

**Failure modes (acceptable):**
- Referer stripped by privacy settings/extensions → no redirect note shown
- User bookmarked/typed the redirect source URL → no Referer → no note
- Both are fine — the note is informational, not critical

### Loop handling

When a user visits an article marked as a loop (`is_loop = TRUE`), serve **200 OK** with the article content plus an error banner:

```html
<div class="pw-error">
  This article is part of a redirect loop: Prince → Prince_(Disambiguation) → Prince
</div>

<!-- normal article content follows -->
```

This lets the user see the content while surfacing the configuration error. Editors can click the edit tab to fix it.

### Redirect note

When Referer header indicates arrival via redirect:
> (Redirected from [Prince](/wiki/Prince?redirect=no))

## Testing Plan

### Unit Tests

**Config rule parsing** (`config_test.go`)
```go
func TestParseRewriteRules(t *testing.T)      // exact + regex
func TestParseRedirectRules(t *testing.T)     // exact + regex + status codes
func TestInvalidRegexWarning(t *testing.T)    // malformed patterns logged
func TestNestedQuantifierWarning(t *testing.T) // ReDoS-prone patterns flagged
```

**Config rule matching** (`url_rules_test.go`)
```go
func TestRewriteExactMatch(t *testing.T)
func TestRewriteRegexMatch(t *testing.T)
func TestRewriteRegexCapture(t *testing.T)    // /old/(.*)$ → /new/$1
func TestRedirectExactMatch(t *testing.T)
func TestRedirectRegexMatch(t *testing.T)
func TestRedirectStatusCodes(t *testing.T)    // 301 vs 302
func TestRulePriority(t *testing.T)           // first match wins
func TestNoMatchPassthrough(t *testing.T)     // unmatched URLs continue to router
```

**Redirect table** (`redirect_repo_test.go`)
```go
func TestInsertRedirect(t *testing.T)
func TestUpdateRedirect(t *testing.T)
func TestDeleteRedirect(t *testing.T)
func TestLookupRedirect(t *testing.T)
func TestLookupNonexistent(t *testing.T)      // returns nil, not error
func TestReverseLoookup(t *testing.T)         // find sources pointing to target
```

**Chain resolution** (`redirect_chain_test.go`)
```go
func TestDirectRedirect(t *testing.T)         // A → B (no chain)
func TestChainResolution(t *testing.T)        // A → B → C resolves to A → C
func TestChainWarningLogged(t *testing.T)
func TestLoopDetection(t *testing.T)          // A → B → A detected
func TestSelfLoopDetection(t *testing.T)      // A → A detected
func TestChainRecomputation(t *testing.T)     // B changes, A's chain updated
func TestDeepChain(t *testing.T)              // A → B → C → D → E
```

**Redirect cache** (`redirect_cache_test.go`)
```go
func TestCacheMiss(t *testing.T)              // loads from DB
func TestCacheHit(t *testing.T)               // returns cached value
func TestCacheInvalidation(t *testing.T)      // save clears entry
func TestCacheEviction(t *testing.T)          // LRU behavior
```

**Frontmatter parsing** (`frontmatter_test.go`) — blocked until implemented
```go
func TestParseFrontmatter(t *testing.T)
func TestParseRedirectField(t *testing.T)
func TestStripFrontmatterFromMarkdown(t *testing.T)
func TestNoFrontmatter(t *testing.T)          // plain markdown unchanged
func TestMalformedFrontmatter(t *testing.T)   // graceful handling
func TestEmptyRedirectField(t *testing.T)     // treated as no redirect
```

### Integration Tests

**Config rules middleware** (`url_rules_integration_test.go`)
```go
func TestRewriteMiddleware(t *testing.T) {
    // Request /sitemap.xml → serves /wiki/Special:Sitemap.xml content
    // Response status 200, URL unchanged
}

func TestRedirectMiddleware(t *testing.T) {
    // Request /old-wiki/Foo → 301 → /wiki/Foo
    // Verify Location header
}
```

**Article redirect flow** (`redirect_integration_test.go`)
```go
func TestArticleRedirectOnView(t *testing.T) {
    // Create article A with redirect → B
    // GET /wiki/A → 301 → /wiki/B
}

func TestRedirectBypassWithParam(t *testing.T) {
    // GET /wiki/A?redirect=no → 200, serves A's content
}

func TestNoRedirectOnEdit(t *testing.T) {
    // GET /wiki/A?edit → 200, edit form for A (not B)
}

func TestNoRedirectOnHistory(t *testing.T) {
    // GET /wiki/A?history → 200, history of A
}

func TestRedirectNote(t *testing.T) {
    // GET /wiki/B with Referer: /wiki/A (where A → B)
    // Response contains "(Redirected from A)"
}

func TestRedirectNoteValidation(t *testing.T) {
    // GET /wiki/B with Referer: /wiki/X (X doesn't redirect to B)
    // No redirect note shown
}

func TestRedirectNoteNoReferer(t *testing.T) {
    // GET /wiki/B with no Referer header
    // No redirect note shown (acceptable)
}

func TestRedirectChainIntegration(t *testing.T) {
    // Create A → B, then B → C
    // GET /wiki/A → 301 → /wiki/C (not /wiki/B)
}

func TestRedirectLoopHandling(t *testing.T) {
    // Create A → B, then B → A
    // GET /wiki/A → 200 with article content + error banner
    // Banner shows loop chain
}
```

### Test Fixtures

```go
var testRedirectConfig = `
rewrites:
  - pattern: "/sitemap.xml"
    target: "/wiki/Special:Sitemap.xml"
  - pattern: "^/legacy/(.*)$"
    target: "/static/$1"
    regex: true

redirects:
  - pattern: "^/old-wiki/(.*)$"
    target: "/wiki/$1"
    code: 301
    regex: true
`

var testArticles = []struct {
    URL      string
    Markdown string
}{
    {"Prince", "---\nredirect: Prince_(Disambiguation)\n---\n"},
    {"Prince_(Disambiguation)", "# Prince\n\nThis is a disambiguation page."},
    {"Loop_A", "---\nredirect: Loop_B\n---\n"},
    {"Loop_B", "---\nredirect: Loop_A\n---\n"},
}
```

### Edge Cases to Cover

| Scenario | Expected Behavior |
|----------|-------------------|
| Redirect to nonexistent article | 301 to target, target shows "not found" |
| Delete article that is a redirect source | Remove from redirect table |
| Delete article that is a redirect target | Sources now point to nonexistent article (OK) |
| Edit redirect article to remove frontmatter | Remove from redirect table |
| Circular chain of 3+ articles | All marked as loops, 200 with error banner |
| Unicode in article URLs | Handled correctly |
| Regex with special URL chars | Properly escaped |

## Blocked on

~~Query param routing refactor~~ ✓ Complete (commit 653f8d0)

**Current blocker: Frontmatter parsing**

Redirects are defined via YAML frontmatter. Frontmatter parsing doesn't exist yet. Implementation requires:

1. Parse YAML header at save-time
2. Extract `redirect:` field (and potentially other metadata later)
3. Strip frontmatter before rendering markdown
4. Update redirect table (DB + invalidate cache) with resolved chain

## Decisions

| Question | Decision |
|----------|----------|
| Redirect definition mechanism | Frontmatter with plain article name (not wikilink syntax) |
| Regex support for config rules | Yes for both rewrites and redirects — admin-defined, compiled, cached |
| Config rule types | `rewrites:` (200) and `redirects:` (30x with code) |
| Redirect storage | SQLite table, lazily cached in memory |
| Redirect chains | Resolve at save-time, store final target in DB, detect loops |
| Passing redirect source to template | Referer header — validated against redirect table |
| Case normalization | Handled via config regex rules (no separate feature) |
| Admin UI | Config-only (no UI) |
| Redirect loops | 200 with error banner showing loop chain |
| Redirect versioning | Part of frontmatter → part of revision history |
