# Frontmatter Parsing Implementation Plan

**Status:** In progress (Phases 1-3 complete, Phase 4 next)

## Prerequisites / Blockers

- ~~**ORM tooling review**: sqlboiler is unmaintained. Consider replacement (see TODO.md) before major new features.~~ Decided to proceed; storage interfaces provide abstraction.
- **Migration approach**: SQL scripts only (no migration tool added)
- ~~**Page/Article abstraction separation**~~: ✅ Complete. Added `Page` interface with `DisplayTitle()` method. Article implements Page. Non-article pages use `StaticPage`. Layout template uses `{{.Page.DisplayTitle}}`. See `wiki/page.go`.
- ~~**URL-inferred titles**~~: ✅ Complete. Added `InferTitle(url)` function. `DisplayTitle()` falls back to inference when Title is empty. See `wiki/page.go`, `wiki/article.go`.

## Overview

Add NestedText frontmatter parsing to articles. Frontmatter provides article metadata (display title, redirects, visibility, tags) in a format safe for wikilinks.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Format** | **NestedText** | YAML silently corrupts `[[wikilinks]]` as nested arrays; NestedText treats all values as strings |
| **Struct visibility** | **Exported** | Self-documenting via struct tags; read-only so no mutation risk |
| **Function signature** | **Multiple returns** | `(Frontmatter, string)` is idiomatic Go |
| **Zero value** | **Meaningful** | Empty fields fall back to defaults naturally |
| **Known fields** | **Incremental** | Start with `DisplayTitle`, add others as features land |
| **Custom fields** | **Deferred** | `map[string]any` added when user-defined templates need it |
| Package location | `wiki/` | Frontmatter belongs to Article semantically |
| Storage | Original markdown WITH frontmatter | Preserves edit history |
| Title field name | `display_title` | Clarifies it doesn't affect URL |

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

**NestedText** was designed as a reaction to YAML's implicit typing. Its core rule: all scalar values are strings. No type coercion, ever.

```nestedtext
display_title: My Article
see_also: [[Related Article]]
redirect: [[Other Page]]
```

**Go library:** `github.com/danielledeleo/nestedtext` v1.0.0
- Struct tag support (`nt:"field_name"`) for idiomatic Go unmarshaling
- Automatic type coercion: NestedText strings → int, float, bool

---

## Phase 1: Page Interface & Title Inference

> ✅ **COMPLETE**

**What was done:**
- Added `Page` interface with `DisplayTitle()` method (`wiki/page.go`)
- Added `StaticPage` type for non-article pages (login, error, sitemap)
- Article implements `Page` via `DisplayTitle()` method
- Added `InferTitle(url)` function for URL → display title conversion
- Updated layout template: `{{.Article.Title}}` → `{{.Page.DisplayTitle}}`
- Updated all handlers to pass `"Page"` key to templates

**Commits:**
- `092592e` refactor: Add Page interface for render context separation
- `1fe2398` feat: Add URL-inferred title logic for articles
- `c951960` fix: Use InferTitle for 404 page titles

---

## Phase 2: Frontmatter Parser

> ✅ **COMPLETE**

### API Design

```go
// Frontmatter holds parsed article metadata.
type Frontmatter struct {
    DisplayTitle string `nt:"display_title"`
}

// ParseFrontmatter extracts NestedText frontmatter from markdown.
// Returns parsed metadata and content with frontmatter stripped.
// On parse error, returns zero Frontmatter and original markdown.
func ParseFrontmatter(markdown string) (Frontmatter, string)
```

**Design rationale:**
- Exported struct with public fields: self-documenting, struct tags show field names
- Multiple return values: idiomatic Go, no wrapper struct needed
- Zero value meaningful: empty `DisplayTitle` triggers fallback to `InferTitle()`
- Graceful degradation: parse errors return original content unchanged

### New Files

| File | Purpose |
|------|---------|
| `wiki/frontmatter.go` | Parser implementation |
| `wiki/frontmatter_test.go` | Unit tests |

### Implementation

**`wiki/frontmatter.go`:**

```go
package wiki

import (
    "regexp"

    "github.com/danielledeleo/nestedtext"
)

// frontmatterRegex matches YAML-style fences at document start.
var frontmatterRegex = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---(?:\r?\n)?`)

// Frontmatter holds parsed article metadata.
type Frontmatter struct {
    DisplayTitle string `nt:"display_title"`
}

// ParseFrontmatter extracts NestedText frontmatter from markdown.
// Returns parsed metadata and content with frontmatter stripped.
// On parse error, returns zero Frontmatter and original markdown.
func ParseFrontmatter(markdown string) (Frontmatter, string) {
    match := frontmatterRegex.FindStringSubmatch(markdown)
    if match == nil {
        return Frontmatter{}, markdown
    }

    var fm Frontmatter
    if err := nestedtext.Unmarshal([]byte(match[1]), &fm); err != nil {
        return Frontmatter{}, markdown
    }

    return fm, markdown[len(match[0]):]
}
```

### Test Cases

| Test Case | Input | Expected |
|-----------|-------|----------|
| No frontmatter | `# Hello` | `Frontmatter{}`, original content |
| Valid frontmatter | `---\ndisplay_title: Custom\n---\n# Hello` | `{DisplayTitle: "Custom"}`, `# Hello` |
| Empty block | `---\n---\n# Hello` | `Frontmatter{}`, `# Hello` |
| Invalid NestedText | `---\n: bad\n---\n# Hello` | `Frontmatter{}`, original content |
| CRLF line endings | `---\r\ndisplay_title: Win\r\n---\r\n# Hello` | `{DisplayTitle: "Win"}`, `# Hello` |
| Frontmatter not at start | `# Hello\n---\ndisplay_title: X\n---` | `Frontmatter{}`, original content |

### Verification

1. `go build ./...` - compiles successfully
2. `go test ./wiki/...` - frontmatter tests pass
3. `go test ./...` - full test suite passes

---

## Phase 3: Integration into DisplayTitle

> ✅ **COMPLETE**

Modify `Article.DisplayTitle()` to use parsed frontmatter:

```go
func (a *Article) DisplayTitle() string {
    fm, _ := ParseFrontmatter(a.Markdown)
    if fm.DisplayTitle != "" {
        return fm.DisplayTitle
    }
    return InferTitle(a.URL)
}
```

**Note:** This parses on every call. Caching can be added later when `SetSource()` API refactor lands (see deferred Phase 0 below).

### Modified Files

| File | Changes |
|------|---------|
| `wiki/article.go` | `DisplayTitle()` calls `ParseFrontmatter()` |

### Verification

1. Create article with `display_title` frontmatter → title displays correctly
2. Create article without frontmatter → falls back to URL-inferred title
3. Edit article, change `display_title` → title updates
4. `go test ./...` - full test suite passes

---

## Future Phases

### Phase 4: Edit Form Migration

- Remove separate Title input field from `templates/article_edit.html`
- Users edit `display_title` in frontmatter within markdown textarea
- SQL migration to prepend frontmatter to existing articles

### Phase 5: Redirect Field

Add `redirect` field to Frontmatter struct, create Redirect table and service.
See `docs/plans/alias-redirect-design.md` for full design.

### Phase 6: Additional Fields

As features are implemented, add fields to Frontmatter:

| Field | Feature | Type |
|-------|---------|------|
| `redirect` | Redirect system | `string` |
| `visibility` | Article visibility | `string` (draft/private/internal) |
| `tags` | Tagging system | `[]string` |
| `locked` | Page protection | `bool` |
| `featured` | Featured articles | `bool` |

### Phase 7: Custom Fields for Templates

When user-defined templates need arbitrary frontmatter:

```go
type Frontmatter struct {
    DisplayTitle string         `nt:"display_title"`
    Redirect     string         `nt:"redirect"`
    // ... other known fields
    Custom       map[string]any `nt:"-"` // populated with remaining fields
}
```

---

## Deferred: Article API Refactor

> **OPTIONAL** — The `Page` interface approach unblocks frontmatter without this refactor.

The full encapsulation (private fields, `SetSource()`, caching) can be added later if needed for performance or to enforce invariants. Current approach parses on each `DisplayTitle()` call, which is acceptable for now.

See git history for the original detailed plan if needed.
