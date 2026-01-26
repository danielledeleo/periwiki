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

Article URLs use underscores for spaces (`Main_Page`). URLs are case-sensitive.

### Edit form submission

Edit forms POST to `/wiki/{article}` with form data including:
- `title` — Article title
- `body` — Markdown content
- `comment` — Edit summary (optional)
- `previous_id` — Revision ID being edited from
- `action` — "submit" or "preview"

## Special pages

| URL | Description |
|-----|-------------|
| `/wiki/Special:Random` | Redirect to random article |
| `/wiki/Special:Sitemap` | HTML sitemap |
| `/wiki/Special:Sitemap.xml` | XML sitemap |

## Authentication

| URL | Method | Description |
|-----|--------|-------------|
| `/user/register` | GET | Registration form |
| `/user/register` | POST | Submit registration |
| `/user/login` | GET | Login form |
| `/user/login` | POST | Submit login |
| `/user/logout` | POST | End session |

## Other

| URL | Description |
|-----|-------------|
| `/` | Home page |
| `/static/*` | Static assets |

## WikiLinks

WikiLinks resolve to `/wiki/` URLs:

| Markup | Resolves to |
|--------|-------------|
| `[[Page Name]]` | `/wiki/Page_Name` |
| `[[Page Name\|Display Text]]` | `/wiki/Page_Name` (displays "Display Text") |

Spaces become underscores. Leading/trailing whitespace is trimmed.

**Key files:**
- `server.go` — route definitions and `articleDispatcher`
- `templater/urlhelper.go` — URL generation helpers for templates
- `extensions/wikilink_underscore.go` — WikiLink URL resolution
