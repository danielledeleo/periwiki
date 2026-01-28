# Frontmatter Parsing Implementation Plan

**Status:** In progress (Phases 0-1 complete, Phase 2 next)

## Prerequisites / Blockers

- ~~**ORM tooling review**: sqlboiler is unmaintained. Consider replacement (see TODO.md) before major new features.~~ Decided to proceed; storage interfaces provide abstraction.
- **Migration approach**: SQL scripts only (no migration tool added)
- ~~**Page/Article abstraction separation**~~: ✅ Complete. Added `Page` interface with `DisplayTitle()` method. Article implements Page. Non-article pages use `StaticPage`. Layout template uses `{{.Page.DisplayTitle}}`. See `wiki/page.go`.
- ~~**URL-inferred titles**~~: ✅ Complete. Added `InferTitle(url)` function. `DisplayTitle()` falls back to inference when Title is empty. See `wiki/page.go`, `wiki/article.go`.

## Overview

Add NestedText frontmatter parsing to articles. Phase 0 refactors the Article API to encapsulate source handling. Phase 1 migrates title to frontmatter (`display_title`). Phase 2 adds `redirect` field for the redirect system.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Format** | **NestedText** | YAML silently corrupts `[[wikilinks]]` as nested arrays; NestedText treats all values as strings |
| Parser approach | Simple standalone | Need metadata at save-time, not just render-time; decoupled from Goldmark |
| Package location | `wiki/` (part of Article) | Frontmatter belongs to Article semantically; parsing lives in wiki package, not a separate internal package |
| Storage | Original markdown WITH frontmatter | Preserves edit history |
| Title field name | `display_title` | Clarifies it doesn't affect URL |
| Edit UX | Frontmatter only | Remove separate Title input from form |
| Migration | Auto-generate frontmatter | Prepend frontmatter to existing articles |
| Article API | Methods over direct field access | Encapsulates parsing, enables caching, enforces invariants |
| Title storage | Stored in DB, derived from frontmatter | Query efficiency; computed once at SetSource() time |
| Parsing timing | Lazy, cached on first access | Content() parses once, caches result |

### Why not a Goldmark extension?

Frontmatter could be extracted during markdown rendering via a Goldmark extension, but this has a fundamental timing problem: we need frontmatter data (title, redirect target) at save-time to update the database, but rendering happens later (possibly async via the render queue). The metadata must be available before rendering begins.

### Why NestedText over YAML?

YAML has a dangerous footgun with wikilinks. This valid YAML:

```yaml
see_also: [[Related Article]]
```

Silently parses as a nested array `[][]string{{"Related Article"}}`, not the string `"[[Related Article]]"`. The `[[...]]` syntax is YAML's flow sequence notation. Users would need to remember to quote every wikilink—they won't.

**Alternatives evaluated:**

| Language | `[[wikilink]]` handling | Verdict |
|----------|------------------------|---------|
| YAML | Silent corruption → nested array | ❌ Footgun |
| TOML | Parse error (`[[` = array-of-tables) | ❌ Syntax clash |
| JSON | Must quote all strings | ⚠️ Too verbose |
| KDL | Must quote strings with brackets | ⚠️ Alien syntax |
| NestedText | Just works (all values are strings) | ✅ |

**NestedText** was designed as a reaction to YAML's implicit typing. Its core rule: all scalar values are strings. No type coercion, ever. Lists use `- item` syntax on separate lines, so `[[...]]` in a value position is unambiguous.

```nestedtext
display_title: My Article
see_also: [[Related Article]]
redirect: [[Other Page]]
```

**Go library:**
- Fork: `github.com/danielledeleo/nestedtext` (forked from npillmayer/nestext)
- Adds struct tag support (`nt:"field_name"`) for idiomatic Go unmarshaling
- Automatic type coercion: NestedText strings → int, float, bool
- Case-insensitive field matching when no tag specified

**Setup (until fork is published):**
```bash
go get github.com/danielledeleo/nestedtext@v1.0.0-beta.1
```

Add to `go.mod`:
```
replace github.com/danielledeleo/nestedtext => ../nestext
```

---

## Phase 0: Page Interface & Title Inference

> ✅ **COMPLETE** — Branch: `refactor/article-revision-separation`

**What was done:**
- Added `Page` interface with `DisplayTitle()` method (`wiki/page.go`)
- Added `StaticPage` type for non-article pages (login, error, sitemap)
- Article implements `Page` via `DisplayTitle()` method
- Added `InferTitle(url)` function for URL → display title conversion
- Updated layout template: `{{.Article.Title}}` → `{{.Page.DisplayTitle}}`
- Updated all handlers to pass `"Page"` key to templates
- Fixed 404 pages to use `InferTitle` instead of `cases.Title()`

**Key design decision:** Deferred full Revision de-embedding (~100 call sites). The `Page` interface provides sufficient abstraction for frontmatter without that churn.

**Commits:**
- `092592e` refactor: Add Page interface for render context separation
- `1fe2398` feat: Add URL-inferred title logic for articles
- `c951960` fix: Use InferTitle for 404 page titles

---

## Phase 0 (Original Plan): Article API Refactor

> **DEFERRED** — The full encapsulation below is optional. Page interface approach unblocks frontmatter without this refactor.

**Goal:** Encapsulate Article source handling to support frontmatter without leaking implementation details.

### Problem: Current API Leakiness

The current `Article` and `Revision` structs expose fields directly:

```go
// Current: leaky API
article := wiki.NewArticle(url, title, markdownBody)  // title separate from markdown
article.Title = req.PostFormValue("title")            // direct field mutation
article.Markdown = "new content"                       // bypasses any parsing
```

This treats title and markdown as independent inputs, but with frontmatter they're coupled—title is derived from markdown.

### New Article API

**Constructor:**
```go
// NewArticle creates an article with URL. Title defaults to URL-derived value.
func NewArticle(url string) *Article

// SetSource sets the raw markdown (with frontmatter). Parses and caches metadata.
func (a *Article) SetSource(raw string)
```

**Getters (computed/cached):**
```go
func (a *Article) Source() string      // raw markdown for storage/hashing
func (a *Article) Content() string     // stripped markdown for rendering (lazy, cached)
func (a *Article) Title() string       // from frontmatter, or URL fallback
func (a *Article) Frontmatter() map[string]string  // all parsed fields
```

**Revision struct:**
```go
type Revision struct {
    ID         int       `db:"id"`
    title      string    `db:"title"`     // private, but has db tag for persistence
    markdown   string    `db:"markdown"`  // private
    HTML       string    `db:"html"`
    Hash       string    `db:"hashval"`
    Creator    *User
    Created    time.Time `db:"created"`
    PreviousID int       `db:"previous_id"`
    Comment    string    `db:"comment"`

    // Cached parsed state (not persisted)
    content     string                 // stripped markdown, computed lazily
    frontmatter map[string]string         // parsed frontmatter fields
    parsed      bool                   // whether content/frontmatter have been computed
}
```

### Title Derivation Rules

Title is computed once when `SetSource()` is called:

1. If frontmatter contains `display_title`, use it
2. Otherwise, derive from URL (title-case, underscores to spaces)

For new/nonexistent articles (before any source is set), title defaults to URL-derived value.

### Migration Strategy: Big Bang

All call sites updated in a single change. The codebase is small enough (~30 call sites) that this is feasible and avoids maintaining compatibility shims.

**Call sites to update:**
- `internal/server/handlers.go` - handleArticlePost, handleEditGet, handleDiff
- `internal/storage/article_repo.go` - SelectArticle, SelectRevisionHistory
- `wiki/service/article.go` - PostArticleWithContext
- `testutil/testutil.go` - CreateTestArticle
- Various test files

### Step 0.1: Update Revision struct

Make `title` and `markdown` private, add cached fields:

```go
// wiki/revision.go
type Revision struct {
    ID         int       `db:"id"`
    title      string    `db:"title"`
    markdown   string    `db:"markdown"`
    HTML       string    `db:"html"`
    // ... other fields ...

    // Cached (not persisted)
    content     string
    frontmatter map[string]string
    parsed      bool
}
```

### Step 0.2: Add Article methods

```go
// wiki/article.go
func NewArticle(url string) *Article {
    a := &Article{URL: url, Revision: &Revision{}}
    a.title = inferTitleFromURL(url)
    return a
}

func (a *Article) SetSource(raw string) {
    a.markdown = raw
    a.parsed = false  // invalidate cache

    // Parse frontmatter immediately for title
    parsed := frontmatter.Parse(raw)
    if parsed.Metadata != nil && parsed.Metadata.DisplayTitle != "" {
        a.title = parsed.Metadata.DisplayTitle
    }
    // else: keep URL-derived default

    a.content = parsed.Content
    a.frontmatter = parsed.Metadata.Fields
    a.parsed = true
}

func (a *Article) Source() string   { return a.markdown }
func (a *Article) Title() string    { return a.title }
func (a *Article) Content() string  { return a.content }
func (a *Article) Frontmatter() map[string]string { return a.frontmatter }
```

### Step 0.3: Add frontmatter parsing to wiki package

Frontmatter parsing belongs to Article semantically. Add parsing helpers to the wiki package (e.g., `wiki/frontmatter.go` or directly in `wiki/article.go`).

**New file: `wiki/frontmatter.go`**

```go
package wiki

import (
    "regexp"

    "github.com/danielledeleo/nestedtext"
)

// frontmatterRegex matches YAML-style fences with NestedText content
var frontmatterRegex = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---(?:\r?\n)?`)

// frontmatterMetadata holds the parsed frontmatter fields.
type frontmatterMetadata struct {
    DisplayTitle string `nt:"display_title"`
}

// parseFrontmatter extracts NestedText frontmatter from markdown.
// Returns the metadata (nil if none/error) and the content with frontmatter stripped.
func parseFrontmatter(markdown string) (*frontmatterMetadata, string) {
    match := frontmatterRegex.FindStringSubmatch(markdown)
    if match == nil {
        return nil, markdown
    }

    var meta frontmatterMetadata
    if err := nestedtext.Unmarshal([]byte(match[1]), &meta); err != nil {
        // Parse error, return content without metadata
        return nil, markdown
    }

    content := markdown[len(match[0]):]
    return &meta, content
}
```

### Step 0.4: Update all call sites

Replace direct field access with method calls:

```go
// Before
article := wiki.NewArticle(url, title, markdown)
article.Title = req.PostFormValue("title")
article.Markdown = req.PostFormValue("body")
html := render(article.Markdown)

// After
article := wiki.NewArticle(url)
article.SetSource(req.PostFormValue("body"))  // title comes from frontmatter
html := render(article.Content())
```

### Step 0.5: Update storage layer

The repository needs to handle private fields. Options:
1. Add setter methods for DB hydration: `SetTitleFromDB()`, `SetMarkdownFromDB()`
2. Use a separate "row" struct for scanning, then convert
3. Keep fields exported but document they shouldn't be set directly

Recommend option 1 for clarity.

---

## Phase 1: Title Migration to Frontmatter

*Depends on Phase 0 completion.*

### Step 1.1: Update edit form

**Modify: `templates/article_edit.html`**
- Remove separate Title input field
- Users edit `display_title` in frontmatter within markdown textarea

**Modify: `internal/server/handlers.go` handleArticlePost**
- Remove title extraction from form (already handled by SetSource)

### Step 1.2: Update ArticleService

**Modify: `wiki/service/article.go`**

```go
func (s *articleService) PostArticleWithContext(ctx context.Context, article *wiki.Article) error {
    // Hash uses raw source (includes frontmatter)
    x := sha512.Sum384([]byte(article.Title() + article.Source()))
    article.SetHash(base64.URLEncoding.EncodeToString(x[:]))

    // ... existing change detection ...

    // Sanitize title
    strip := bluemonday.StrictPolicy()
    article.SetTitle(strip.Sanitize(article.Title()))
    article.SetComment(strip.Sanitize(article.Comment()))

    // Render stripped content
    html, err := s.rendering.Render(article.Content())
    // ...
}
```

### Step 1.3: SQL Migration

**New file: `internal/storage/migrations/001_add_frontmatter.sql`**

```sql
-- Migration: Prepend NestedText frontmatter with display_title to existing articles
-- Run with: sqlite3 periwiki.db < internal/storage/migrations/001_add_frontmatter.sql
-- BACKUP FIRST: cp periwiki.db periwiki.db.backup

-- NestedText is simpler than YAML: no special escaping needed for most characters.
-- Colons in values are fine (only the first colon is the delimiter).
UPDATE Revision
SET markdown = '---
display_title: ' || title || '
---

' || markdown
WHERE markdown NOT LIKE '---%';
```

### Step 1.4: Display title fallback

When displaying articles, if `display_title` is missing:
- Use URL as display title (convert underscores to spaces)
- This handles pre-migration articles during transition

---

## Phase 2: Add Redirect Field

### Step 2.1: Extend Metadata struct

**Modify: `wiki/frontmatter.go`**

```go
type Metadata struct {
    DisplayTitle string            `nt:"display_title"`
    Redirect     string            `nt:"redirect"`
    Fields       map[string]string `nt:"-"` // populated separately for unknown fields
}
```

### Step 2.2: Create Redirect table

**Modify: `internal/storage/schema.sql`**

```sql
CREATE TABLE IF NOT EXISTS Redirect (
    source_url      TEXT PRIMARY KEY,
    immediate_target TEXT NOT NULL,
    final_target    TEXT NOT NULL,
    is_loop         BOOLEAN DEFAULT FALSE,
    chain_warning   TEXT,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_redirect_final ON Redirect(final_target);
CREATE INDEX IF NOT EXISTS idx_redirect_immediate ON Redirect(immediate_target);
```

### Step 2.3: Create RedirectService

**New file: `wiki/redirect.go`**
```go
type Redirect struct {
    SourceURL       string
    ImmediateTarget string
    FinalTarget     string
    IsLoop          bool
    ChainWarning    string
    UpdatedAt       time.Time
}
```

**New file: `wiki/repository/redirect.go`**
```go
type RedirectRepository interface {
    UpsertRedirect(sourceURL, immediateTarget, finalTarget string, isLoop bool, chainWarning string) error
    DeleteRedirect(sourceURL string) error
    SelectRedirect(sourceURL string) (*wiki.Redirect, error)
    SelectRedirectByTarget(targetURL string) ([]*wiki.Redirect, error)
}
```

**New file: `wiki/service/redirect.go`**
```go
type RedirectService interface {
    UpdateRedirect(sourceURL, targetURL string) error  // handles chain resolution
    DeleteRedirect(sourceURL string) error
    GetRedirect(sourceURL string) (*wiki.Redirect, error)
}
```

**New file: `internal/storage/redirect_repo.go`**
- SQLite implementation

### Step 2.4: Integrate redirect into ArticleService

**Modify: `wiki/service/article.go`**

Add to `articleService` struct:
```go
redirects RedirectService
```

Add to `PostArticleWithContext` after saving article:
```go
// Update redirect table (NestedText values are always strings, no type assertion needed)
if redirectTarget := article.Frontmatter()["redirect"]; redirectTarget != "" {
    if err := s.redirects.UpdateRedirect(article.URL, redirectTarget); err != nil {
        slog.Error("redirect update failed", "error", err)
    }
} else {
    s.redirects.DeleteRedirect(article.URL)
}
```

### Step 2.5: Wire up in internal/server/setup.go

```go
redirectService := service.NewRedirectService(database)
articleService := service.NewArticleService(database, renderingService, redirectService)
```

---

## Files Summary

### Phase 0 - New Files
| File | Purpose |
|------|---------|
| `wiki/frontmatter.go` | Frontmatter parsing (belongs to Article semantically) |
| `wiki/frontmatter_test.go` | Unit tests |

### Phase 0 - Modified Files
| File | Changes |
|------|---------|
| `wiki/revision.go` | Private fields, cached state |
| `wiki/article.go` | New constructor, SetSource, getters |
| `internal/storage/article_repo.go` | Handle private fields |
| `internal/server/handlers.go` | Use new Article API |
| `wiki/service/article.go` | Use new Article API |
| `testutil/testutil.go` | Use new Article API |
| Various test files | Use new Article API |

### Phase 1 - New Files
| File | Purpose |
|------|---------|
| `internal/storage/migrations/001_add_frontmatter.sql` | SQL migration script |

### Phase 1 - Modified Files
| File | Changes |
|------|---------|
| `internal/server/handlers.go` | Remove title from form handling |
| `templates/article_edit.html` | Remove Title input field |

### Phase 2 - New Files
| File | Purpose |
|------|---------|
| `wiki/redirect.go` | Redirect domain model |
| `wiki/repository/redirect.go` | Repository interface |
| `wiki/service/redirect.go` | Service + chain resolution |
| `internal/storage/redirect_repo.go` | SQLite implementation |

### Phase 2 - Modified Files
| File | Changes |
|------|---------|
| `wiki/frontmatter.go` | Add `Redirect` field |
| `internal/storage/schema.sql` | Add Redirect table |
| `wiki/service/article.go` | Add redirect updates |
| `internal/server/setup.go` | Wire RedirectService |

---

## Verification

### Phase 0
1. `go build ./...` - compiles successfully
2. `go test ./...` - all existing tests pass
3. No direct access to `article.Title` or `article.Markdown` outside Article methods

### Phase 1
1. `go test ./wiki/...` - frontmatter unit tests pass
2. Create new article with frontmatter - title extracted correctly
3. Edit existing article - frontmatter preserved
4. Run migration on test DB - all articles get frontmatter
5. `go test ./...` - full test suite passes

### Phase 2
1. Create article with `redirect: Target` - redirect table updated
2. Remove redirect from article - redirect table entry deleted
3. Test chain: A→B→C - verify `final_target` resolved
4. Test loop: A→B→A - verify `is_loop` flag set
5. `go test ./...` - full test suite passes
