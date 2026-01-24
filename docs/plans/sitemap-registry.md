# Special Pages in Sitemap - Opt-in Interface

## Summary
Allow special pages to opt-in to sitemap inclusion via an interface. Pages like `Sitemap` can be indexed while `Random` (a redirect) stays excluded.

## Design

### New Interface
**File:** `special/special.go`
```go
// SitemapEntry allows special pages to opt-in to sitemap inclusion.
type SitemapEntry interface {
    // SitemapInfo returns whether this page should be included in the sitemap,
    // along with its display title. LastMod is omitted for special pages.
    SitemapInfo() (title string, include bool)
}
```

### Registry Enhancement
**File:** `special/special.go`

Add method to collect sitemap entries:
```go
// SitemapEntries returns all registered special pages that implement SitemapEntry
// and have opted in to sitemap inclusion.
func (r *Registry) SitemapEntries() []struct{ Name, Title string } {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var entries []struct{ Name, Title string }
    for name, handler := range r.handlers {
        if se, ok := handler.(SitemapEntry); ok {
            if title, include := se.SitemapInfo(); include {
                entries = append(entries, struct{ Name, Title string }{name, title})
            }
        }
    }
    // Sort alphabetically by title
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].Title < entries[j].Title
    })
    return entries
}
```

### SitemapPage Implements Interface
**File:** `special/sitemap.go`

```go
// SitemapInfo implements SitemapEntry - the sitemap page should be indexed.
func (p *SitemapPage) SitemapInfo() (string, bool) {
    return "Sitemap", true
}
```

### Update SitemapPage to Include Special Pages
**File:** `special/sitemap.go`

- Add `registry *Registry` field to `SitemapPage` struct
- Update constructor to accept registry
- Modify `handleXML` and `handleHTML` to include special page entries

### Update Registration
**File:** `setup.go`

Pass registry to sitemap handler:
```go
sitemapHandler := special.NewSitemapPage(articleService, t, modelConf.BaseURL, specialPages)
```

## Files to Modify

1. `special/special.go` - Add `SitemapEntry` interface and `SitemapEntries()` method
2. `special/sitemap.go` - Implement interface, accept registry, include special pages in output
3. `setup.go` - Pass registry to sitemap constructor
4. `special/sitemap_test.go` - Add tests for special page inclusion
5. `templates/special/sitemap.html` - Add section for special pages

## Verification
1. Run `go test ./...`
2. Start server and verify:
   - `/wiki/Special:Sitemap` shows both articles AND "Sitemap" in special pages section
   - `/wiki/Special:Sitemap.xml` includes `Special:Sitemap` URL
   - `Random` does NOT appear (it doesn't implement the interface)
