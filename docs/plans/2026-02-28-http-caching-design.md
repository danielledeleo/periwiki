# HTTP Caching Design

## Problem

Periwiki sets no caching headers on any response. Every request is a full round-trip, even for content that hasn't changed. This is wasteful for browsers and makes proxy deployment (nginx, Caddy, Cloudflare) less effective.

## Design

Three caching tiers based on how content changes.

### Tier 1: Stable Per-Run

Content that is fixed for the lifetime of a server process but may change between restarts (overlay FS file replaced on disk, render templates changed, binary updated).

**Routes:**
- `/static/*`, `/favicon.ico`, `/robots.txt`, `/llms.txt`
- `/wiki/Periwiki:*` (embedded help articles)
- `/wiki/{article}?revision=N` (old revisions — source is immutable but rendered HTML depends on templates)
- `/wiki/{article}?diff&old=N&new=M`

**Headers:**
```
Cache-Control: public, max-age=86400
Last-Modified: <file mod time or Revision.Created>
```

Supports `If-Modified-Since` → 304 Not Modified.

**Rationale:** A 24-hour max-age means browsers skip revalidation for repeat visits within a day. After restart with changed content, the `Last-Modified` value changes and clients revalidate on next request after TTL expires. File mod time comes from the overlay FS — `os.DirFS` provides real mod times, `embed.FS` provides the embed time.

### Tier 2: Conditional

Content that changes when articles are edited. Must revalidate every time, but can avoid re-downloading unchanged content.

**Routes:**
- `/wiki/{article}` — current article view
- `/wiki/{article}.md` — raw markdown
- `/wiki/{article}?history` — revision history
- `/wiki/Talk:*` — talk pages (same as articles)
- `/sitemap.xml`, `/sitemap.md`, `/wiki/Special:Sitemap`
- `/wiki/Special:WhatLinksHere?page=X`

**Headers:**
```
Cache-Control: public, no-cache
ETag: "<Revision.Hash>"
Last-Modified: <Revision.Created>
```

For sitemaps, `Last-Modified` is the newest `ArticleSummary.LastModified` across all articles.

Supports `If-None-Match` and `If-Modified-Since` → 304 Not Modified.

**Rationale:** `no-cache` means "always revalidate" (not "don't cache"). Browsers and proxies store the response but check freshness on every request. A 304 response is just headers — no body — so unchanged articles are essentially free to serve.

### Tier 3: Uncacheable

Content that is user-specific, random, or mutating.

**Routes:**
- `/user/*` (login, register, logout)
- `/manage/*` (admin pages)
- `/wiki/Special:Random` (random redirect)
- `/wiki/{article}?edit` (edit form)
- All POST endpoints
- `/` (redirect to Main_Page)

**Headers:**
```
Cache-Control: no-store
```

**Rationale:** Auth pages set cookies, admin pages are role-gated, edit forms must be fresh to avoid conflicts. `no-store` tells browsers and proxies not to retain the response at all.

## Implementation Notes

### Where to set headers

Per-handler, not middleware. The caching tier depends on what data the handler fetches (revision hash, file mod time, etc.), which isn't available at middleware time.

Exception: Tier 3 `no-store` for `/user/*` and `/manage/*` could be set in a middleware wrapping those route groups, since it's unconditional.

### Conditional request handling

For Tier 1 and Tier 2 routes, use Go's `http.ServeContent` or manual ETag/Last-Modified checking:

1. Set `Last-Modified` and/or `ETag` headers on the response
2. Call `checkPreconditions` (or let `http.ServeContent` handle it for file-like responses)
3. If match → write 304 with no body
4. If no match → write full response

For `/static/*`, Go's `http.FileServer` already handles `Last-Modified` and `If-Modified-Since` natively from `fs.Stat()`. We just need to add the `Cache-Control` header.

### ETag format

Use weak ETags (`W/"hash"`) for rendered HTML since the same semantic content could be byte-different after template changes. Use the `Revision.Hash` directly — it's already a content hash.

### Template rendering and 304s

For Tier 2 article routes, the handler currently always renders the template. To benefit from 304s, check conditional headers *before* template execution:

1. Fetch article metadata (hash, created time)
2. Check `If-None-Match` / `If-Modified-Since`
3. If 304 → return immediately, skip template render
4. Otherwise → render and serve

### Vary header

If any response depends on cookies or auth state for content differences (not just access control), add `Vary: Cookie`. Currently article HTML is identical for all users, so this isn't needed for article views — but edit forms and admin pages would need it if ever cached.

## Drop X-UA-Compatible

Remove `<meta http-equiv="X-UA-Compatible" content="IE=edge">` from `templates/layouts/index.html`. IE is dead and this adds no value for modern or text-based browsers.
