# Stale Content Detection and Re-rendering

## Problem

When render-time templates or the rendering pipeline change, the HTML stored in
`Revision.html` becomes stale. Periwiki pre-renders article HTML at edit time
and stores it in the database, so template changes don't take effect until
articles are re-rendered.

## Design

### Template folder structure

Render-time templates (baked into `Revision.html` during Goldmark rendering) are
separated from request-time page chrome:

- **`templates/_render/`** — Templates baked into stored HTML. Changes here
  invalidate cached content. Contains TOC, wikilink, and footnote templates.
- **`templates/`** (everything else) — Page chrome applied at request time.
  Changes here take effect immediately with no re-rendering needed.

The `_render` prefix signals "internal — changes affect cached data."

### Boot-time staleness check

On startup, the server:

1. Computes a SHA-256 hash of all files in `templates/_render/` (sorted by path
   for determinism, including file paths in the hash to detect renames).
2. Compares against the `render_template_hash` value in the `Setting` table.
3. If unchanged: no action.
4. If changed (or first run): invalidates old revision HTML, queues head
   revision re-renders, and updates the stored hash.

### Invalidation strategy

- **Head revisions** (latest per article): Eagerly queued for re-render via the
  existing render queue at `TierBackground` priority.
- **Old revisions**: HTML set to `NULL`. Re-rendered lazily on first access.

### Nullable HTML column

The `Revision.html` column is nullable (`TEXT` without `NOT NULL`). This serves
double duty:

- **Storage efficiency**: `NULL` takes essentially zero space in SQLite, so
  invalidating thousands of old revisions reclaims disk space.
- **Staleness signal**: `NULL` HTML means "needs rendering." The serving code
  checks for empty HTML and renders on demand.

SQL queries use `COALESCE(html, '')` to scan into Go's `string` type, keeping
the domain model simple (no `*string` or `sql.NullString`).

### Lazy re-rendering

When `GetArticle`, `GetArticleByRevisionID`, or `GetArticleByRevisionHash`
returns a revision with empty HTML:

1. Render the markdown through the current pipeline.
2. Persist the result back to the database.
3. Serve the freshly rendered HTML.

Subsequent requests for the same revision serve the cached result. Revisions
that are never accessed never waste render cycles.

## Key files

- `render/templatehash.go` — Hash computation for `_render/` directory.
- `internal/server/setup.go` — Boot-time staleness check
  (`checkRenderTemplateStaleness`).
- `wiki/service/article.go` — Lazy re-rendering (`ensureHTML` helper).
- `internal/storage/article_repo.go` — `InvalidateNonHeadRevisionHTML` query.
- `wiki/runtime_config.go` — `SettingRenderTemplateHash` constant and
  `UpdateSetting` helper.

## Future considerations

- If the wiki grows large enough that full head re-renders at boot are slow,
  a per-article generation counter could enable incremental re-rendering.
- The same mechanism could be extended to detect Goldmark extension changes
  (e.g., new parser options) by including a pipeline version in the hash.
