# URL Reference

## Articles

All article actions use query parameters on the base article URL:

| URL | Description |
|-----|-------------|
| `/wiki/{article}` | View article (current revision) |
| `/wiki/{article}?revision={id}` | View specific revision |
| `/wiki/{article}?edit` | Edit current revision |
| `/wiki/{article}?edit&revision={id}` | Edit/restore specific revision |
| `/wiki/{article}?history` | Revision history |
| `/wiki/{article}?diff&old={a}&new={b}` | Compare revisions a and b |
| `/wiki/{article}?diff&old={a}` | Compare revision a to current |
| `/wiki/{article}?diff&new={b}` | Compare previous revision to b |
| `/wiki/{article}?diff` | Compare two most recent revisions |
| `/wiki/{article}?rerender` | Force re-render current revision |
| `/wiki/{article}?rerender&revision={id}` | Force re-render specific revision |

Article URLs use underscores for spaces (`Main_Page`). URLs are case-sensitive.

### Edit form submission

Edit forms POST to `/wiki/{article}` with form data including:
- `title` — Article title
- `body` — Markdown content
- `comment` — Edit summary (optional)
- `previous_id` — Revision ID being edited from
- `action` — "submit" or "preview"

## Namespaced URLs

All URLs containing a colon (`Foo:Bar`) are routed to `NamespaceHandler`. Only recognized namespaces are served; unrecognized namespaces return 404. This reserves the colon as a system-controlled namespace delimiter.

### Talk pages (Talk: namespace)

| URL | Description |
|-----|-------------|
| `/wiki/Talk:{article}` | Discussion page for an article |

Talk pages use the same query parameters as articles (`?edit`, `?history`, `?diff`, etc.). A talk page can only be created if its subject article exists.

### Help pages (Periwiki: namespace)

| URL | Description |
|-----|-------------|
| `/wiki/Periwiki:Help_overview` | Help index — links to all help articles |
| `/wiki/Periwiki:Writing_articles` | Writing articles guide |
| `/wiki/Periwiki:Syntax` | Markdown and WikiLink syntax quick reference |
| `/wiki/Periwiki:Installation` | Installation and configuration |
| `/wiki/Periwiki:Security` | Passwords, sessions, and HTML sanitization |
| `/wiki/Periwiki:Troubleshooting` | Common issues and fixes |

Help articles are read-only and compiled into the binary from `help/`. They can be overridden by placing a file at the same path on disk. They cannot be edited through the wiki interface.

### Special pages

| URL | Description |
|-----|-------------|
| `/wiki/Special:Random` | Redirect to random article |
| `/wiki/Special:RerenderAll` | Rerender all articles (admin only) |
| `/wiki/Special:Sitemap` | HTML sitemap |
| `/wiki/Special:Sitemap.xml` | XML sitemap |
| `/wiki/Special:WhatLinksHere?page={slug}` | Articles that link to the given page |
| `/wiki/Special:SourceCode` | Download source tarball (AGPL compliance) |

The `Special` namespace is case-insensitive (`special:Random` also works).

## Authentication

| URL | Method | Description |
|-----|--------|-------------|
| `/user/register` | GET | Registration form |
| `/user/register` | POST | Submit registration |
| `/user/login` | GET | Login form |
| `/user/login` | POST | Submit login |
| `/user/logout` | POST | End session |

## Admin

| URL | Method | Description |
|-----|--------|-------------|
| `/manage/users` | GET | List all users (admin only) |
| `/manage/users/{id}` | POST | Change user role (admin only) |
| `/manage/settings` | GET | Runtime settings (admin only) |
| `/manage/settings` | POST | Update runtime settings (admin only) |
| `/manage/content` | GET | Content files tree (admin only) |
| `/manage/tools` | GET | Admin tools (admin only) |
| `/manage/tools/reset-main-page` | POST | Reset Main_Page to default (admin only) |
| `/manage/tools/backfill-links` | POST | Rebuild link graph (admin only) |

## Other

| URL | Description |
|-----|-------------|
| `/` | Redirects to `/wiki/Main_Page` (302) |
| `/favicon.ico` | Favicon (served from static/) |
| `/robots.txt` | Crawler rules (served from static/) |
| `/sitemap.xml` | XML sitemap (rewrite to Special:Sitemap.xml) |
| `/static/*` | Static assets |

## WikiLinks

WikiLinks resolve to `/wiki/` URLs:

| Markup | Resolves to |
|--------|-------------|
| `[[Page Name]]` | `/wiki/Page_Name` |
| `[[Page Name\|Display Text]]` | `/wiki/Page_Name` (displays "Display Text") |

Spaces become underscores. Leading/trailing whitespace is trimmed.

**Key files:**
- `internal/server/app.go` — route definitions (`RegisterRoutes`)
- `internal/server/handlers.go` — `ArticleDispatcher`, `NamespaceHandler`, and all article handlers
- `templater/urlhelper.go` — URL generation helpers for templates
- `extensions/wikilink_underscore.go` — WikiLink URL resolution
