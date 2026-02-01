# Plan: Embedded Read-Only Help Articles

## Overview

Add embedded, read-only help articles using the `Periwiki:` namespace prefix. Content is embedded in the binary via `//go:embed`, rendered through the normal article templates, but not editable.

## Design Decisions

- **Namespace**: `Periwiki:` prefix (e.g., `/wiki/Periwiki:Syntax`)
- **Detection**: `strings.HasPrefix(url, "Periwiki:")`
- **ReadOnly field**: Add to `Article` struct - templates check this to hide edit controls
- **Architecture**: Decorator pattern wrapping `ArticleService`
- **Rendering**: Pre-render markdown to HTML at startup, cache in memory
- **Sitemap**: Automatically excluded (embedded articles aren't in database)

## Implementation

### 1. Add ReadOnly field to Article

**File**: [wiki/article.go](wiki/article.go)

```go
type Article struct {
    URL      string
    ReadOnly bool  // True for embedded articles
    *Revision
}
```

### 2. Add error type

**File**: [wiki/errors.go](wiki/errors.go)

```go
ErrReadOnlyArticle = errors.New("article is read-only")
```

### 3. Create embedded content infrastructure

**New directory**: `internal/embedded/help/`

Sample files:
- `Syntax.md` - WikiLink syntax guide
- `Formatting.md` - Markdown reference

**New file**: `internal/embedded/embedded.go`

```go
//go:embed help/*.md
var helpFS embed.FS

type EmbeddedArticles struct {
    articles map[string]*wiki.Article
}

func NewEmbeddedArticles(renderer service.RenderingService) (*EmbeddedArticles, error)
func (ea *EmbeddedArticles) Get(url string) (*wiki.Article, bool)
func IsEmbeddedURL(url string) bool
```

- Reads all `*.md` files from embedded FS at startup
- Renders markdown using existing `RenderingService`
- Caches as `*wiki.Article` with `ReadOnly: true`
- URL derived from filename: `Syntax.md` → `Periwiki:Syntax`

### 4. Create decorator service

**New file**: `wiki/service/embedded_article.go`

```go
type embeddedArticleService struct {
    ArticleService
    embedded EmbeddedGetter
}

func NewEmbeddedArticleService(base ArticleService, embedded EmbeddedGetter) ArticleService
```

Methods:
- `GetArticle()` - Check embedded first, fall through to database
- `PostArticle()` / `PostArticleWithContext()` - Return `ErrReadOnlyArticle` for embedded URLs
- `GetRevisionHistory()` - Return empty slice for embedded
- `GetArticleByRevisionID()` - Return `ErrRevisionNotFound` for embedded

### 5. Wire up in setup

**File**: [internal/server/setup.go](internal/server/setup.go)

```go
// After creating baseArticleService:
embeddedArticles, err := embedded.NewEmbeddedArticles(renderingService)
articleService := service.NewEmbeddedArticleService(baseArticleService, embeddedArticles)
```

### 6. Add source view handler

**File**: [internal/server/handlers.go](internal/server/handlers.go)

In `ArticleDispatcher`, add before other handlers:
```go
if params.Has("source") {
    a.handleSource(rw, req, vars["article"])
    return
}
```

New `handleSource()` method renders `article_source.html` showing raw markdown.

**New file**: `templates/article_source.html` - Shows markdown in `<pre>` block

### 7. Block edit attempts

**File**: [internal/server/handlers.go](internal/server/handlers.go)

In `handleEdit()`, add at start:
```go
article, err := a.Articles.GetArticle(articleURL)
if err == nil && article.ReadOnly {
    http.Redirect(rw, req, "/wiki/"+articleURL+"?source", http.StatusSeeOther)
    return
}
```

### 8. Update templates

**File**: [templates/article.html](templates/article.html)

```html
{{if not .Article.ReadOnly}}
<li><a href="{{editURL .URL .ID}}">Edit</a></li>
{{end}}

{{if .Article.ReadOnly}}
<div class="pw-callout pw-info">
  This is an embedded help article. <a href="{{articleURL .URL}}?source">View source</a>.
</div>
{{end}}
```

Hide "Last edited" timestamp for embedded articles.

**File**: [templates/article_history.html](templates/article_history.html)

Handle empty revision list with message for embedded articles.

## Files Summary

| New Files | Purpose |
|-----------|---------|
| `internal/embedded/embedded.go` | Embedded article loader |
| `internal/embedded/help/*.md` | Help content |
| `wiki/service/embedded_article.go` | Decorator service |
| `templates/article_source.html` | Source view template |

| Modified Files | Changes |
|----------------|---------|
| `wiki/article.go` | Add `ReadOnly` field |
| `wiki/errors.go` | Add `ErrReadOnlyArticle` |
| `internal/server/setup.go` | Wire up embedded articles |
| `internal/server/handlers.go` | Add `handleSource`, modify `handleEdit` |
| `templates/article.html` | Conditional edit tab, info callout |
| `templates/article_history.html` | Handle embedded case |

## Verification

1. **Build**: `make` should succeed with embedded content
2. **View**: `GET /wiki/Periwiki:Syntax` returns 200 with rendered help
3. **Source**: `GET /wiki/Periwiki:Syntax?source` shows markdown
4. **Edit blocked**: `GET /wiki/Periwiki:Syntax?edit` redirects to `?source`
5. **POST blocked**: `POST /wiki/Periwiki:Syntax` returns error
6. **Sitemap**: `/wiki/Special:Sitemap` does not list `Periwiki:*` articles
7. **Tests**: `go test ./...` passes
