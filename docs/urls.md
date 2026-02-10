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

### Help pages (Periwiki: namespace)

| URL | Description |
|-----|-------------|
| `/wiki/Periwiki:Syntax` | Built-in Markdown and WikiLink syntax reference |

Help articles are read-only and compiled into the binary from `help/`. They can be overridden by placing a file at the same path on disk. They cannot be edited through the wiki interface.

### Special pages

| URL | Description |
|-----|-------------|
| `/wiki/Special:Random` | Redirect to random article |
| `/wiki/Special:RerenderAll` | Rerender all articles (admin only) |
| `/wiki/Special:Sitemap` | HTML sitemap |
| `/wiki/Special:Sitemap.xml` | XML sitemap |

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

## Other

| URL | Description |
|-----|-------------|
| `/` | Redirects to `/wiki/Main_Page` (302) |
| `/static/*` | Static assets |

## WikiLinks

WikiLinks resolve to `/wiki/` URLs:

| Markup | Resolves to |
|--------|-------------|
| `[[Page Name]]` | `/wiki/Page_Name` |
| `[[Page Name\|Display Text]]` | `/wiki/Page_Name` (displays "Display Text") |

Spaces become underscores. Leading/trailing whitespace is trimmed.

**Key files:**
- `cmd/periwiki/main.go` — route definitions
- `internal/server/handlers.go` — `ArticleDispatcher`, `NamespaceHandler`, and all article handlers
- `templater/urlhelper.go` — URL generation helpers for templates
- `extensions/wikilink_underscore.go` — WikiLink URL resolution
