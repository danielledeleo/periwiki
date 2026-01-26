# Query Param Routing Refactor

**Status:** Design needed

## Motivation

Current routing uses path segments for actions:
- `/wiki/{article}/history`
- `/wiki/{article}/r/{id}`
- `/wiki/{article}/r/{id}/edit`
- `/wiki/{article}/diff/{a}/{b}`

This prevents support for **subpages** — articles with `/` in their names like `Template:Navbox/doc`.

## Proposed change

Move actions to query parameters (MediaWiki style):

| Current | Proposed |
|---------|----------|
| `/wiki/Article/history` | `/wiki/Article?action=history` |
| `/wiki/Article/r/5` | `/wiki/Article?oldid=5` |
| `/wiki/Article/r/5/edit` | `/wiki/Article?action=edit&oldid=5` |
| `/wiki/Article/diff/3/5` | `/wiki/Article?diff=5&oldid=3` |

This allows:
- `/wiki/Template:Navbox/doc` — subpage article
- `/wiki/Template:Navbox/doc?action=edit` — edit that subpage

## Benefits

1. **Subpages** — Articles can have `/` in names
2. **Cleaner redirect logic** — Redirect only when no `action` param
3. **MediaWiki familiarity** — Users and tools expect this pattern
4. **Simpler router** — One route handles all article actions

## Implementation considerations

- Update `server.go` route definitions
- Single `articleHandler` dispatches based on query params
- Update all internal links and templates
- Update `docs/urls.md`
- Backward compatibility: 301 redirect old paths to new query param URLs?

## Blockers

- None identified

## Blocks

- Alias and redirect system (`docs/plans/alias-redirect-design.md`)
