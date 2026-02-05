# Embedded Help Articles Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement read-only help articles in a `Periwiki:` namespace, embedded in the binary, and add a syntax help link to the edit page.

**Architecture:** Help content lives as markdown files in `internal/embedded/help/`. At startup, these are rendered through the existing rendering service and cached as `Article` structs with `ReadOnly: true`. A decorator wraps `ArticleService` to intercept requests for `Periwiki:*` URLs. The edit page gets a simple link to `/wiki/Periwiki:Syntax`.

**Tech Stack:** Go embed, existing RenderingService, decorator pattern for ArticleService.

---

### Task 1: Add ReadOnly field to Article struct

**Files:**
- Modify: `wiki/article.go`

**Step 1: Add the ReadOnly field**

In `wiki/article.go`, add `ReadOnly bool` to the Article struct:

```go
type Article struct {
	URL      string
	ReadOnly bool // True for embedded/system articles
	*Revision
}
```

**Step 2: Run existing tests to verify no breakage**

Run: `go test ./wiki/...`
Expected: PASS (no behavior change, just new field)

**Step 3: Commit**

```bash
git add wiki/article.go
git commit -m "feat: add ReadOnly field to Article struct"
```

---

### Task 2: Add ErrReadOnlyArticle error

**Files:**
- Modify: `wiki/errors.go`

**Step 1: Add the error sentinel**

In `wiki/errors.go`, add to the var block:

```go
ErrReadOnlyArticle = errors.New("article is read-only")
```

**Step 2: Commit**

```bash
git add wiki/errors.go
git commit -m "feat: add ErrReadOnlyArticle error"
```

---

### Task 3: Create embedded article infrastructure

**Files:**
- Create: `internal/embedded/embedded.go`
- Create: `internal/embedded/embedded_test.go`

**Step 1: Write the test**

Create `internal/embedded/embedded_test.go`:

```go
package embedded_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/internal/embedded"
)

func TestIsEmbeddedURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"Periwiki:Syntax", true},
		{"Periwiki:Help", true},
		{"periwiki:syntax", false}, // case-sensitive
		{"Regular-Article", false},
		{"Periwiki", false}, // no colon
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			if got := embedded.IsEmbeddedURL(tc.url); got != tc.expected {
				t.Errorf("IsEmbeddedURL(%q) = %v, want %v", tc.url, got, tc.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/embedded/...`
Expected: FAIL (package doesn't exist)

**Step 3: Create the embedded package**

Create `internal/embedded/embedded.go`:

```go
package embedded

import (
	"embed"
	"io/fs"
	"strings"

	"github.com/danielledeleo/periwiki/wiki"
)

//go:embed help/*.md
var helpFS embed.FS

const embeddedPrefix = "Periwiki:"

// IsEmbeddedURL returns true if the URL is for an embedded article.
func IsEmbeddedURL(url string) bool {
	return strings.HasPrefix(url, embeddedPrefix)
}

// EmbeddedArticles holds pre-rendered embedded help articles.
type EmbeddedArticles struct {
	articles map[string]*wiki.Article
}

// RenderFunc is a function that renders markdown to HTML.
type RenderFunc func(markdown string) (string, error)

// New creates a new EmbeddedArticles instance by loading and rendering
// all markdown files from the embedded filesystem.
func New(render RenderFunc) (*EmbeddedArticles, error) {
	ea := &EmbeddedArticles{
		articles: make(map[string]*wiki.Article),
	}

	entries, err := fs.ReadDir(helpFS, "help")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := fs.ReadFile(helpFS, "help/"+entry.Name())
		if err != nil {
			return nil, err
		}

		// Derive URL from filename: "Syntax.md" -> "Periwiki:Syntax"
		name := strings.TrimSuffix(entry.Name(), ".md")
		url := embeddedPrefix + name

		html, err := render(string(content))
		if err != nil {
			return nil, err
		}

		ea.articles[url] = &wiki.Article{
			URL:      url,
			ReadOnly: true,
			Revision: &wiki.Revision{
				Markdown: string(content),
				HTML:     html,
			},
		}
	}

	return ea, nil
}

// Get returns an embedded article by URL, or nil if not found.
func (ea *EmbeddedArticles) Get(url string) *wiki.Article {
	return ea.articles[url]
}

// List returns all embedded article URLs.
func (ea *EmbeddedArticles) List() []string {
	urls := make([]string, 0, len(ea.articles))
	for url := range ea.articles {
		urls = append(urls, url)
	}
	return urls
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/embedded/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/embedded/
git commit -m "feat: add embedded article infrastructure"
```

---

### Task 4: Create initial Syntax help content

**Files:**
- Create: `internal/embedded/help/Syntax.md`

**Step 1: Create the help file**

Create `internal/embedded/help/Syntax.md` with content adapted from `docs/writing-articles.md`:

```markdown
---
display_title: Syntax Help
---

## WikiLinks

Link between articles using double brackets:

| Syntax | Result |
|--------|--------|
| `[[Target]]` | Link to Target |
| `[[Target\|display text]]` | Link showing "display text" |

Spaces around brackets and pipes are trimmed.

**Dead links** (to non-existent pages) appear in red.

## Frontmatter

Optional metadata at the start of an article:

```
---
display_title: Custom Title
---
```

| Field | Purpose |
|-------|---------|
| `display_title` | Override the displayed title |

## Footnotes

Add references with `[^label]` in text and `[^label]: content` at the end:

```
Fact[^1].

[^1]: Source citation.
```

## Markdown Quick Reference

| Syntax | Result |
|--------|--------|
| `**bold**` | **bold** |
| `*italic*` | *italic* |
| `[text](url)` | Link |
| `# Heading` | Heading (1-6) |
| `` `code` `` | Inline code |
| `> quote` | Block quote |

For internal links, prefer WikiLinks over Markdown links.
```

**Step 2: Add test for embedded content loading**

Add to `internal/embedded/embedded_test.go`:

```go
func TestNew(t *testing.T) {
	// Simple render function for testing
	render := func(md string) (string, error) {
		return "<p>rendered</p>", nil
	}

	ea, err := embedded.New(render)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Syntax.md should be loaded
	article := ea.Get("Periwiki:Syntax")
	if article == nil {
		t.Fatal("expected Periwiki:Syntax to exist")
	}

	if !article.ReadOnly {
		t.Error("expected ReadOnly to be true")
	}

	if article.URL != "Periwiki:Syntax" {
		t.Errorf("expected URL 'Periwiki:Syntax', got %q", article.URL)
	}
}
```

**Step 3: Run test**

Run: `go test ./internal/embedded/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/embedded/help/Syntax.md internal/embedded/embedded_test.go
git commit -m "feat: add Syntax help content"
```

---

### Task 5: Create embedded article service decorator

**Files:**
- Create: `wiki/service/embedded.go`
- Create: `wiki/service/embedded_test.go`

**Step 1: Write the test**

Create `wiki/service/embedded_test.go`:

```go
package service_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
)

func TestEmbeddedArticleService_GetArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create embedded articles
	ea, err := embedded.New(app.Rendering.Render)
	if err != nil {
		t.Fatalf("failed to create embedded articles: %v", err)
	}

	// Wrap the article service
	wrappedService := service.NewEmbeddedArticleService(app.Articles, ea)

	t.Run("returns embedded article", func(t *testing.T) {
		article, err := wrappedService.GetArticle("Periwiki:Syntax")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}
		if !article.ReadOnly {
			t.Error("expected ReadOnly to be true")
		}
	})

	t.Run("falls through to base service", func(t *testing.T) {
		user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")
		testutil.CreateTestArticle(t, app, "Regular-Article", "# Hello", user)

		article, err := wrappedService.GetArticle("Regular-Article")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}
		if article.ReadOnly {
			t.Error("expected ReadOnly to be false for regular article")
		}
	})
}

func TestEmbeddedArticleService_PostArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ea, _ := embedded.New(app.Rendering.Render)
	wrappedService := service.NewEmbeddedArticleService(app.Articles, ea)

	t.Run("rejects post to embedded URL", func(t *testing.T) {
		article := wiki.NewArticle("Periwiki:Syntax", "modified content")
		err := wrappedService.PostArticle(article)
		if err != wiki.ErrReadOnlyArticle {
			t.Errorf("expected ErrReadOnlyArticle, got: %v", err)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./wiki/service/... -run TestEmbedded`
Expected: FAIL (NewEmbeddedArticleService doesn't exist)

**Step 3: Implement the decorator**

Create `wiki/service/embedded.go`:

```go
package service

import (
	"context"

	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/wiki"
)

// embeddedArticleService wraps an ArticleService to intercept embedded articles.
type embeddedArticleService struct {
	base     ArticleService
	embedded *embedded.EmbeddedArticles
}

// NewEmbeddedArticleService creates a decorator that serves embedded articles
// for Periwiki:* URLs while delegating other requests to the base service.
func NewEmbeddedArticleService(base ArticleService, ea *embedded.EmbeddedArticles) ArticleService {
	return &embeddedArticleService{
		base:     base,
		embedded: ea,
	}
}

func (s *embeddedArticleService) GetArticle(url string) (*wiki.Article, error) {
	if article := s.embedded.Get(url); article != nil {
		return article, nil
	}
	return s.base.GetArticle(url)
}

func (s *embeddedArticleService) PostArticle(article *wiki.Article) error {
	if embedded.IsEmbeddedURL(article.URL) {
		return wiki.ErrReadOnlyArticle
	}
	return s.base.PostArticle(article)
}

func (s *embeddedArticleService) PostArticleWithContext(ctx context.Context, article *wiki.Article) error {
	if embedded.IsEmbeddedURL(article.URL) {
		return wiki.ErrReadOnlyArticle
	}
	return s.base.PostArticleWithContext(ctx, article)
}

func (s *embeddedArticleService) Preview(markdown string) (string, error) {
	return s.base.Preview(markdown)
}

func (s *embeddedArticleService) GetArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, wiki.ErrRevisionNotFound
	}
	return s.base.GetArticleByRevisionID(url, id)
}

func (s *embeddedArticleService) GetArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, wiki.ErrRevisionNotFound
	}
	return s.base.GetArticleByRevisionHash(url, hash)
}

func (s *embeddedArticleService) GetRevisionHistory(url string) ([]*wiki.Revision, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, nil // Empty history for embedded articles
	}
	return s.base.GetRevisionHistory(url)
}

func (s *embeddedArticleService) GetRandomArticleURL() (string, error) {
	return s.base.GetRandomArticleURL()
}

func (s *embeddedArticleService) GetAllArticles() ([]*wiki.ArticleSummary, error) {
	return s.base.GetAllArticles()
}

func (s *embeddedArticleService) RerenderRevision(ctx context.Context, url string, revisionID int) error {
	if embedded.IsEmbeddedURL(url) {
		return wiki.ErrReadOnlyArticle
	}
	return s.base.RerenderRevision(ctx, url, revisionID)
}

func (s *embeddedArticleService) QueueRerenderRevision(ctx context.Context, url string, revisionID int) (<-chan RerenderResult, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, wiki.ErrReadOnlyArticle
	}
	return s.base.QueueRerenderRevision(ctx, url, revisionID)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./wiki/service/... -run TestEmbedded`
Expected: PASS

**Step 5: Commit**

```bash
git add wiki/service/embedded.go wiki/service/embedded_test.go
git commit -m "feat: add embedded article service decorator"
```

---

### Task 6: Wire up embedded articles in setup

**Files:**
- Modify: `internal/server/setup.go`

**Step 1: Import the embedded package and wire it up**

In `internal/server/setup.go`, add import:

```go
"github.com/danielledeleo/periwiki/internal/embedded"
```

Then, after `articleService := service.NewArticleService(...)`, add:

```go
// Create embedded articles and wrap the article service
embeddedArticles, err := embedded.New(renderingService.Render)
if err != nil {
	slog.Error("failed to load embedded articles", "error", err)
	os.Exit(1)
}
articleService = service.NewEmbeddedArticleService(articleService, embeddedArticles)
```

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS

**Step 3: Manual verification**

Run: `go run ./cmd/periwiki` and visit `http://localhost:8080/wiki/Periwiki:Syntax`
Expected: Rendered syntax help page

**Step 4: Commit**

```bash
git add internal/server/setup.go
git commit -m "feat: wire up embedded articles in server setup"
```

---

### Task 7: Update article template for read-only articles

**Files:**
- Modify: `templates/article.html`

**Step 1: Add conditional for ReadOnly**

Update `templates/article.html` to hide edit tab and show info banner for read-only articles:

```html
{{define "content"}}
<div id="article-area">
    {{with .Article }}
    {{if $.IsOldRevision}}
    <div class="pw-callout pw-info">
        You are viewing an old revision of this article from {{.Created.Format "January 2, 2006 at 3:04 pm"}}.
        <a href="{{articleURL .URL}}">View current version</a>
    </div>
    {{end}}
    {{if .ReadOnly}}
    <div class="pw-callout pw-neutral">
        This is a built-in help article.
    </div>
    {{end}}
    <ul class="pw-tabs">
        <li class="pw-active"><a href="{{articleURL .URL}}">Article</a></li>
        {{if not .ReadOnly}}
        <li><a href="{{editURL .URL .ID}}">Edit</a></li>
        <li><a href="{{historyURL .URL}}">History</a></li>
        {{end}}
    </ul>
    <article>
        <h1>{{.DisplayTitle}}</h1>
        <div class="pw-article-content">
            {{.HTML}}
        </div>
    </article>
    {{if not .ReadOnly}}
    <span class="pw-last-edited">Last edited on {{.Created.Format "January 2, 2006 at 3:04 pm"}}</span>
    {{end}}
    {{end}}
</div>
{{end}}
```

**Step 2: Manual verification**

Visit `http://localhost:8080/wiki/Periwiki:Syntax`
Expected: No Edit/History tabs, "built-in help article" banner shown

**Step 3: Commit**

```bash
git add templates/article.html
git commit -m "feat: hide edit controls for read-only articles"
```

---

### Task 8: Block edit handler for embedded articles

**Files:**
- Modify: `internal/server/handlers.go`

**Step 1: Add redirect in handleEdit**

At the start of `handleEdit` (after the anonymous edit check), add:

```go
// Block editing of embedded articles
if embedded.IsEmbeddedURL(articleURL) {
	http.Redirect(rw, req, "/wiki/"+articleURL, http.StatusSeeOther)
	return
}
```

Add the import at the top of the file:

```go
"github.com/danielledeleo/periwiki/internal/embedded"
```

**Step 2: Add integration test**

Add to `internal/server/article_edit_integration_test.go`:

```go
func TestEditEmbeddedArticle_Redirects(t *testing.T) {
	// Setup test app with embedded articles
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Periwiki:Syntax?edit", nil)
	req = testutil.RequestWithUser(req, testutil.AnonymousUser())
	rr := httptest.NewRecorder()

	app.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status %d, got %d", http.StatusSeeOther, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/wiki/Periwiki:Syntax" {
		t.Errorf("expected redirect to /wiki/Periwiki:Syntax, got %q", location)
	}
}
```

**Step 3: Run test**

Run: `go test ./internal/server/... -run TestEditEmbeddedArticle`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/server/handlers.go internal/server/article_edit_integration_test.go
git commit -m "feat: redirect edit attempts for embedded articles"
```

---

### Task 9: Add syntax help link to edit page

**Files:**
- Modify: `templates/article_edit.html`

**Step 1: Add the help link**

In `templates/article_edit.html`, add a help link after the buttons:

```html
<form action="{{articleURL .URL}}" method="POST">
{{if $.Other.CurrentRevisionID}}
<input type="hidden" name="previous_id" value="{{$.Other.CurrentRevisionID}}" />
{{else}}
<input type="hidden" name="previous_id" value="{{.ID}}" />
{{end}}
<div class="pw-article-content">
    <textarea name="body" id="body-edit">{{.Markdown}}</textarea>
    <input type="text" name="comment" placeholder="Describe your changes..." {{ if $.Other.Preview }}value="{{.Comment}}"{{end}}/>
    <button name="action" value="submit">Submit</button>
    <button name="action" value="preview">Preview</button>
    <a href="/wiki/Periwiki:Syntax" class="pw-syntax-help" target="_blank">Syntax help</a>
</div>
</form>
```

**Step 2: Add CSS styling**

In `static/main.css`, add:

```css
/* Syntax help link on edit page */
a.pw-syntax-help {
    margin-left: 1rem;
    font-size: 0.85rem;
}
```

**Step 3: Manual verification**

Visit any article's edit page (e.g., `http://localhost:8080/wiki/Test?edit`)
Expected: "Syntax help" link appears after the buttons, opens in new tab

**Step 4: Commit**

```bash
git add templates/article_edit.html static/main.css
git commit -m "feat: add syntax help link to edit page"
```

---

### Task 10: Handle history page for embedded articles

**Files:**
- Modify: `templates/article_history.html`

**Step 1: Read current template**

Check current template structure to understand what needs updating.

**Step 2: Add handling for empty/null history**

The template should gracefully handle the case where history is empty (for embedded articles). If accessing history for an embedded article, redirect to the article view.

In `internal/server/handlers.go`, update `handleHistory`:

```go
func (a *App) handleHistory(rw http.ResponseWriter, req *http.Request, articleURL string) {
	// Embedded articles have no history
	if embedded.IsEmbeddedURL(articleURL) {
		http.Redirect(rw, req, "/wiki/"+articleURL, http.StatusSeeOther)
		return
	}
	// ... rest of existing code
}
```

**Step 3: Run tests**

Run: `go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/server/handlers.go
git commit -m "feat: redirect history requests for embedded articles"
```

---

### Task 11: Final verification

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

**Step 2: Manual testing checklist**

- [ ] Visit `/wiki/Periwiki:Syntax` - shows rendered help
- [ ] No Edit/History tabs on help article
- [ ] "Built-in help article" banner shown
- [ ] `/wiki/Periwiki:Syntax?edit` redirects to view
- [ ] `/wiki/Periwiki:Syntax?history` redirects to view
- [ ] Edit page for regular articles shows "Syntax help" link
- [ ] Syntax help link opens in new tab
- [ ] POST to `/wiki/Periwiki:Syntax` returns appropriate error

**Step 3: Commit any fixes needed**

---

## Files Summary

| New Files | Purpose |
|-----------|---------|
| `internal/embedded/embedded.go` | Embedded article loader |
| `internal/embedded/embedded_test.go` | Tests for embedded package |
| `internal/embedded/help/Syntax.md` | Syntax help content |
| `wiki/service/embedded.go` | Decorator service |
| `wiki/service/embedded_test.go` | Tests for decorator |

| Modified Files | Changes |
|----------------|---------|
| `wiki/article.go` | Add `ReadOnly` field |
| `wiki/errors.go` | Add `ErrReadOnlyArticle` |
| `internal/server/setup.go` | Wire up embedded articles |
| `internal/server/handlers.go` | Block edit/history for embedded |
| `templates/article.html` | Conditional edit tab, info callout |
| `templates/article_edit.html` | Add syntax help link |
| `static/main.css` | Style syntax help link |
