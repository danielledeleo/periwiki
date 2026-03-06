# Sitemap: Special Pages & Embedded Help Articles

## Summary
Extend the sitemap to include three sources:
1. **Database articles** (already works via `GetAllArticles()`)
2. **Special pages** that opt-in via a `SitemapEntry` interface (e.g. Sitemap itself)
3. **Embedded help articles** (`Periwiki:*` namespace, currently excluded)

## Design

### 1. Special Page Opt-in Interface
**File:** `special/special.go`
```go
// SitemapEntry allows special pages to opt-in to sitemap inclusion.
type SitemapEntry interface {
    // SitemapInfo returns whether this page should be included in the sitemap,
    // along with its display title. LastMod is omitted for special pages.
    SitemapInfo() (title string, include bool)
}
```

### 2. Registry `SitemapEntries()` Method
**File:** `special/special.go`

The same handler can be registered under multiple names (e.g. "Sitemap" and "Sitemap.xml").
Deduplicate by tracking seen handler pointers to avoid duplicate entries.

```go
// SitemapEntries returns all registered special pages that implement SitemapEntry
// and have opted in to sitemap inclusion. Handlers registered under multiple
// names are only returned once.
func (r *Registry) SitemapEntries() []struct{ Name, Title string } {
    r.mu.RLock()
    defer r.mu.RUnlock()

    seen := make(map[Handler]bool)
    var entries []struct{ Name, Title string }
    for name, handler := range r.handlers {
        if seen[handler] {
            continue
        }
        seen[handler] = true
        if se, ok := handler.(SitemapEntry); ok {
            if title, include := se.SitemapInfo(); include {
                entries = append(entries, struct{ Name, Title string }{name, title})
            }
        }
    }
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].Title < entries[j].Title
    })
    return entries
}
```

### 3. SitemapPage Implements Interface
**File:** `special/sitemap.go`

```go
func (p *SitemapPage) SitemapInfo() (string, bool) {
    return "Sitemap", true
}
```

Pages that do NOT implement the interface are automatically excluded:
- `Random` (redirect, no content)
- `RerenderAll` (admin action, no content)
- `WhatLinksHere` (requires `?page=` parameter, not standalone)

### 4. Embedded Help Articles in Sitemap
**File:** `internal/embedded/embedded.go`

Add a method to return sitemap-friendly summaries:
```go
// Summaries returns all embedded articles as ArticleSummary values,
// suitable for sitemap inclusion.
func (ea *EmbeddedArticles) Summaries() []*wiki.ArticleSummary {
    summaries := make([]*wiki.ArticleSummary, 0, len(ea.articles))
    for _, a := range ea.articles {
        summaries = append(summaries, &wiki.ArticleSummary{
            URL:          a.URL,
            DisplayTitle: displayTitle(a.URL),
        })
    }
    sort.Slice(summaries, func(i, j int) bool {
        return summaries[i].DisplayTitle < summaries[j].DisplayTitle
    })
    return summaries
}

// displayTitle derives a human-readable title from an embedded article URL.
// "Periwiki:Writing_articles" -> "Writing articles"
func displayTitle(url string) string {
    name := strings.TrimPrefix(url, embeddedPrefix)
    return strings.ReplaceAll(name, "_", " ")
}
```

Note: `LastMod` is omitted for embedded articles since they ship with the binary
and have no meaningful modification time.

### 5. Update SitemapPage
**File:** `special/sitemap.go`

Add new dependencies to the struct and constructor:
```go
type EmbeddedLister interface {
    Summaries() []*wiki.ArticleSummary
}

type SitemapPage struct {
    lister    ArticleLister
    templater SitemapTemplater
    baseURL   string
    registry  *Registry
    embedded  EmbeddedLister
}

func NewSitemapPage(lister ArticleLister, templater SitemapTemplater, baseURL string, registry *Registry, embedded EmbeddedLister) *SitemapPage
```

Update `handleXML` and `handleHTML` to merge all three sources:
- DB articles (from `lister.GetAllArticles()`)
- Embedded help articles (from `embedded.Summaries()`) as `Periwiki:*` URLs
- Special pages (from `registry.SitemapEntries()`) as `Special:*` URLs

### 6. Update Registration
**File:** `internal/server/setup.go`

```go
sitemapHandler := special.NewSitemapPage(articleService, t, modelConf.BaseURL, specialPages, embeddedArticles)
```

### 7. Update HTML Template
**File:** `templates/special/sitemap.html`

Group the sitemap into sections:

```html
{{define "content"}}
<div id="article-area">
    <article>
        <h1>Sitemap</h1>
        <div class="pw-article-content">
            {{if .Articles}}
            <h2>Articles</h2>
            <ul>
                {{range .Articles}}
                <li><a href="{{articleURL .URL}}">{{.DisplayTitle}}</a></li>
                {{end}}
            </ul>
            {{end}}
            {{if .HelpPages}}
            <h2>Help</h2>
            <ul>
                {{range .HelpPages}}
                <li><a href="{{articleURL .URL}}">{{.DisplayTitle}}</a></li>
                {{end}}
            </ul>
            {{end}}
            {{if .SpecialPages}}
            <h2>Special Pages</h2>
            <ul>
                {{range .SpecialPages}}
                <li><a href="/wiki/Special:{{.Name}}">{{.Title}}</a></li>
                {{end}}
            </ul>
            {{end}}
        </div>
    </article>
</div>
{{end}}
```

## Files to Modify

1. `special/special.go` - Add `SitemapEntry` interface and `SitemapEntries()` method
2. `special/sitemap.go` - Implement `SitemapEntry`, add `EmbeddedLister` interface, accept registry + embedded, merge all sources
3. `internal/embedded/embedded.go` - Add `Summaries()` and `displayTitle()` helper
4. `internal/server/setup.go` - Pass registry and embedded articles to sitemap constructor
5. `templates/special/sitemap.html` - Sectioned layout for articles, help, and special pages
6. `special/sitemap_test.go` - Tests for all three sources and deduplication
7. `internal/embedded/embedded_test.go` - Test `Summaries()` output

## Verification
1. Run `go test ./...`
2. Start server and verify:
   - `/wiki/Special:Sitemap` shows three sections: Articles, Help, Special Pages
   - Help section lists all `Periwiki:*` pages with readable titles
   - Special Pages section shows "Sitemap" but not "Random", "RerenderAll", or "WhatLinksHere"
   - `/wiki/Special:Sitemap.xml` includes all three sources as URLs
   - No duplicate entries (e.g. "Sitemap" doesn't appear twice despite dual registration)
