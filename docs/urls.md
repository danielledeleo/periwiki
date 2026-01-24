# URL Reference

## Articles

| URL | Description |
|-----|-------------|
| `/wiki/{article}` | View article |
| `/wiki/{article}/history` | Revision history |
| `/wiki/{article}/r/{id}` | View specific revision |
| `/wiki/{article}/r/{id}/edit` | Edit from specific revision |
| `/wiki/{article}/diff/{old}/{new}` | Compare two revisions |

Article URLs use underscores for spaces (`Main_Page`). URLs are case-sensitive.

## Special pages

| URL | Description |
|-----|-------------|
| `/wiki/Special:Random` | Redirect to random article |

See [Extending](extending.md) for adding custom special pages.

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
- `server.go` — route definitions (lines 82-104)
- `extensions/wikilink_underscore.go` — URL resolution logic
