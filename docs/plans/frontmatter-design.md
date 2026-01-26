# Frontmatter Parsing Implementation Plan

**Status:** Saved for later implementation

## Prerequisites / Blockers

- **ORM tooling review**: sqlboiler is unmaintained. Consider replacement (see TODO.md) before major new features.
- **Migration approach**: SQL scripts only (no migration tool added)

## Overview

Add YAML frontmatter parsing to articles. Phase 1 migrates title to frontmatter (`display_title`). Phase 2 adds `redirect` field for the redirect system.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Parser approach | Simple standalone | Need metadata at save-time, not just render-time; decoupled from Goldmark |
| Package structure | New `frontmatter` package | Separation of concerns, reusable |
| Storage | Original markdown WITH frontmatter | Preserves edit history |
| Title field name | `display_title` | Clarifies it doesn't affect URL |
| Edit UX | Frontmatter only | Remove separate Title input from form |
| Migration | Auto-generate frontmatter | Prepend frontmatter to existing articles |

---

## Phase 1: Title Migration to Frontmatter

### Step 1.1: Create frontmatter package

**New file: `frontmatter/frontmatter.go`**

```go
package frontmatter

import (
    "regexp"
    "gopkg.in/yaml.v3"
)

type Metadata struct {
    DisplayTitle string `yaml:"display_title,omitempty"`

    // All fields for template access
    Fields map[string]interface{} `yaml:",inline"`
}

type Result struct {
    Metadata *Metadata
    Content  string  // Markdown with frontmatter stripped
}

// Parse extracts YAML frontmatter from markdown.
// Returns nil Metadata if no frontmatter present.
// Malformed YAML logs warning, returns nil Metadata with stripped content.
func Parse(markdown string) (*Result, error)
```

Implementation:
- Regex: `^---\n([\s\S]*?)\n---\n?` to detect and extract frontmatter
- `yaml.Unmarshal` with `",inline"` to capture all fields
- Return stripped content for rendering

**New file: `frontmatter/frontmatter_test.go`**
- No frontmatter → nil metadata, original content
- Valid frontmatter → parsed metadata, stripped content
- Empty frontmatter → nil metadata, stripped content
- Malformed YAML → error logged, nil metadata, stripped content
- `display_title` extraction
- Unknown fields captured in `Fields` map

### Step 1.2: Modify ArticleService

**Modify: `wiki/service/article.go`**

```go
func (s *articleService) PostArticle(article *wiki.Article) error {
    // 1. Parse frontmatter
    result, err := frontmatter.Parse(article.Markdown)
    if err != nil {
        slog.Warn("frontmatter parse error", "url", article.URL, "error", err)
    }

    // 2. Extract display_title from frontmatter
    if result.Metadata != nil && result.Metadata.DisplayTitle != "" {
        article.Title = result.Metadata.DisplayTitle
    } else {
        // Fallback: use URL as title if no display_title
        article.Title = article.URL
    }

    // 3. Hash uses ORIGINAL markdown (includes frontmatter)
    x := sha512.Sum384([]byte(article.Title + article.Markdown))
    article.Hash = base64.URLEncoding.EncodeToString(x[:])

    // ... existing change detection ...

    // 4. Sanitize title (still needed for XSS protection)
    strip := bluemonday.StrictPolicy()
    article.Title = strip.Sanitize(article.Title)
    article.Comment = strip.Sanitize(article.Comment)

    // 5. Render STRIPPED markdown
    html, err := s.rendering.Render(result.Content)
    // ...

    return s.repo.InsertArticle(article)
}
```

### Step 1.3: Update edit form

**Modify: `templates/edit.html`** (or equivalent)
- Remove separate Title input field
- Users edit `display_title` in frontmatter within markdown textarea

**Modify: `server.go` handleArticlePost**
- Remove title extraction from form
- Article.Title will be extracted from frontmatter in PostArticle

### Step 1.4: SQL Migration

**New file: `internal/storage/migrations/001_add_frontmatter.sql`**

```sql
-- Migration: Prepend frontmatter with display_title to existing articles
-- Run with: sqlite3 periwiki.db < internal/storage/migrations/001_add_frontmatter.sql
-- BACKUP FIRST: cp periwiki.db periwiki.db.backup

-- Update markdown to prepend frontmatter (only for rows without existing frontmatter)
UPDATE Revision
SET markdown = '---
display_title: ' || REPLACE(REPLACE(title, ':', '\:'), '''', '''''') || '
---

' || markdown
WHERE markdown NOT LIKE '---%';
```

Run once to migrate existing articles:
```bash
cp periwiki.db periwiki.db.backup
sqlite3 periwiki.db < internal/storage/migrations/001_add_frontmatter.sql
```

Notes:
- YAML-escapes colons and single quotes in titles
- Skips articles that already have frontmatter (`---` prefix)
- Updates all revisions (preserves history with frontmatter added)

### Step 1.5: Display title fallback

When displaying articles, if `display_title` is missing:
- Use URL as display title (convert underscores to spaces)
- This handles pre-migration articles during transition

---

## Phase 2: Add Redirect Field

### Step 2.1: Extend Metadata struct

**Modify: `frontmatter/frontmatter.go`**

```go
type Metadata struct {
    DisplayTitle string `yaml:"display_title,omitempty"`
    Redirect     string `yaml:"redirect,omitempty"`

    Fields map[string]interface{} `yaml:",inline"`
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

Add to `PostArticle` after saving article:
```go
// Update redirect table
if result.Metadata != nil && result.Metadata.Redirect != "" {
    if err := s.redirects.UpdateRedirect(article.URL, result.Metadata.Redirect); err != nil {
        slog.Error("redirect update failed", "error", err)
    }
} else {
    s.redirects.DeleteRedirect(article.URL)
}
```

### Step 2.5: Wire up in setup.go

```go
redirectService := service.NewRedirectService(database)
articleService := service.NewArticleService(database, renderingService, redirectService)
```

---

## Files Summary

### Phase 1 - New Files
| File | Purpose |
|------|---------|
| `frontmatter/frontmatter.go` | Parser with `display_title` |
| `frontmatter/frontmatter_test.go` | Unit tests |
| `internal/storage/migrations/001_add_frontmatter.sql` | SQL migration script |

### Phase 1 - Modified Files
| File | Changes |
|------|---------|
| `wiki/service/article.go` | Extract title from frontmatter |
| `server.go` | Remove title from form handling |
| `templates/edit.html` | Remove Title input field |
| `go.mod` | Add `gopkg.in/yaml.v3` |

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
| `frontmatter/frontmatter.go` | Add `Redirect` field |
| `internal/storage/schema.sql` | Add Redirect table |
| `wiki/service/article.go` | Add redirect updates |
| `setup.go` | Wire RedirectService |

---

## Verification

### Phase 1
1. `go test ./frontmatter/...` - unit tests pass
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
