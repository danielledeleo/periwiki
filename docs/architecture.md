# Architecture

Periwiki is a layered Go application using interface-based dependency injection.

## Request flow

```
HTTP Request
     │
     ▼
┌──────────────────┐
│  Router          │  gorilla/mux
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Middleware      │  internal/server/middleware.go — injects User into context
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Handler         │  internal/server/handlers.go — access user via req.Context().Value(wiki.UserKey)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Service         │  wiki/service/ — interfaces + implementations
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Repository      │  wiki/repository/ — interfaces only
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Storage         │  internal/storage/ — SQLite implementations
└──────────────────┘
```

## Dependency injection

All services are interfaces. The `App` struct in `internal/server/app.go` holds service references:

```go
type App struct {
    *templater.Templater          // embedded for RenderTemplate()
    Articles      service.ArticleService
    Users         service.UserService
    Sessions      service.SessionService
    Rendering     service.RenderingService
    Preferences   service.PreferenceService
    SpecialPages  *special.Registry
    Config        *wiki.Config
    RuntimeConfig *wiki.RuntimeConfig
}
```

Services are initialized in `internal/server/setup.go` via constructor functions:

```go
renderingService := service.NewRenderingService(renderer, sanitizer)
articleService := service.NewArticleService(database, renderingService, renderQueue)
```

Handlers access services through the receiver: `a.Articles.GetArticle(url)`

## Directory structure

| Directory | Purpose |
|-----------|---------|
| `cmd/periwiki/` | Entry point (`main.go` — route definitions) |
| `internal/server/` | HTTP handlers, middleware, app setup |
| `internal/storage/` | SQLite repository implementations |
| `internal/renderqueue/` | Async render queue (worker pool, priority heap) |
| `internal/config/` | File-based configuration (`config.yaml`) |
| `internal/embedded/` | Embedded article loader and build metadata generator |
| `help/` | Help article markdown sources (embedded into binary) |
| `internal/logger/` | Structured logging setup (pretty/JSON/text) |
| `wiki/` | Domain types (`Article`, `User`, `Revision`, `Frontmatter`) |
| `wiki/service/` | Service interfaces and implementations |
| `wiki/repository/` | Repository interfaces (no implementations) |
| `render/` | Goldmark markdown rendering |
| `extensions/` | Markdown extensions (WikiLinks, footnotes) |
| `special/` | Special page handlers |
| `templater/` | HTML template engine |
| `templates/` | HTML templates |
| `static/` | CSS and static assets |

## Key files

| File | Purpose |
|------|---------|
| `cmd/periwiki/main.go` | `main()`, route definitions |
| `internal/server/app.go` | `App` struct (holds all service references) |
| `internal/server/handlers.go` | `ArticleDispatcher`, `NamespaceHandler`, all HTTP handlers |
| `internal/server/setup.go` | `Setup()` — creates all services and wires dependencies |
| `internal/server/middleware.go` | Session middleware — injects user into request context |
| `internal/config/config.go` | `SetupConfig()` — loads `config.yaml` |
| `wiki/context.go` | Defines `UserKey` for context access |
| `wiki/errors.go` | Sentinel errors (`ErrGenericNotFound`, etc.) |
| `wiki/frontmatter.go` | NestedText frontmatter parsing |
| `wiki/runtime_config.go` | Database-backed runtime settings (secrets, toggles) |
| `internal/storage/migrations.go` | Schema migrations (run at startup) |
| `render/templatehash.go` | SHA-256 hash of render templates for staleness detection |

## Error handling

Custom errors in `wiki/errors.go` control HTTP responses:

```go
if err == wiki.ErrGenericNotFound {
    // return 404
}
if err == wiki.ErrRevisionAlreadyExists {
    // return 409 Conflict
}
```

## Accessing the current user

Middleware injects the user into request context:

```go
user := req.Context().Value(wiki.UserKey).(*wiki.User)
```

Anonymous users have `ID: 0`.

## Render queue

Article edits are rendered asynchronously through a priority queue (`internal/renderqueue/`). The queue has two priority tiers:

- **Interactive** — user-initiated edits and views. Highest priority.
- **Background** — bulk operations like `Special:RerenderAll` and stale content re-renders.

If an article already has a queued job, the existing job is updated in-place (same-article deduplication) rather than adding a duplicate. Worker count is controlled by the `render_workers` runtime setting (`0` = one per CPU core). Each worker has panic recovery so a bad render cannot crash the server.

On shutdown (SIGINT/SIGTERM), the queue drains in-flight jobs with a 30-second timeout before exiting.

**Key files:** `internal/renderqueue/queue.go` (queue + workers), `internal/renderqueue/heap.go` (priority ordering).

## Stale content detection

Render-time templates (`templates/_render/`) are hashed at startup. If the hash differs from the stored `render_template_hash` in the database, Periwiki assumes the rendering pipeline has changed and:

1. Nullifies cached HTML for all non-head revisions (reclaims storage).
2. Queues all head revisions for background re-render.
3. Old revisions with NULL HTML are lazily re-rendered when accessed.

This means changes to TOC, footnote, or wikilink templates are automatically propagated to all articles without manual intervention.

**Key files:** `render/templatehash.go`, `internal/server/setup.go` (`checkRenderTemplateStaleness`).

## Database migrations

Schema changes are applied automatically at startup via `internal/storage/migrations.go`. The migration runner:

1. Executes the base schema (`internal/storage/schema.sql`, all `CREATE TABLE IF NOT EXISTS`).
2. Runs incremental migrations that check column existence before acting (idempotent).

Some migrations recreate tables because SQLite lacks `ALTER TABLE DROP COLUMN`. This is safe but may briefly increase startup time on large databases.

## Embedded content and overlay FS

Templates, static assets, and help articles are compiled into the binary via `//go:embed`. An overlay filesystem (`content.go`) layers on-disk files over the embedded defaults: if a file exists on disk at the same path, it takes precedence. Directory listings always come from the embedded FS.

```
templates/   → page templates, render templates
static/      → CSS, favicon, logo
help/        → help article markdown sources
```

All consumers (templater, renderer, template hasher, embedded articles) receive the overlay FS via the `fs.FS` interface. The database schema (`internal/storage/schema.sql`) is embedded separately within the storage package and is not part of the overlay FS.

An admin page at `/manage/content` shows the full tree of content files and which ones have disk overrides.

**Key files:** `content.go` (overlay FS, file listing), `cmd/periwiki/main.go` (wiring).

## Embedded help articles

Read-only help articles are compiled into the binary from `help/*.md`. They are served under the `Periwiki:` namespace (e.g., `/wiki/Periwiki:Syntax`) and cannot be edited through the wiki interface. Because they are part of the overlay content FS, they can be overridden by placing a file at the same path on disk (e.g., `help/Syntax.md`).

The embedded system also captures the git commit hash at build time (`go generate ./internal/embedded`) and uses it to generate source links pointing to the correct commit on GitHub.

## Configuration layers

Periwiki has a two-tier configuration system:

- **File config** (`config.yaml` → `wiki.Config`): Bootstrap settings needed before the database is available — host, database path, base URL, logging. Loaded by `internal/config/config.go`.
- **Runtime config** (SQLite `Setting` table → `wiki.RuntimeConfig`): Settings stored in the database — cookie secret, session expiry, password policy, anonymous edit toggle, render worker count. Loaded by `wiki/runtime_config.go`. No management UI yet.

See [Installation and Configuration](installation-and-configuration.md) for the full list of settings.
