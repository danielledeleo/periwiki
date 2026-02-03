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
articleService := service.NewArticleService(database, renderingService)
```

Handlers access services through the receiver: `a.Articles.GetArticle(url)`

## Directory structure

| Directory | Purpose |
|-----------|---------|
| `cmd/periwiki/` | Entry point (`main.go` — route definitions) |
| `internal/server/` | HTTP handlers, middleware, app setup |
| `internal/storage/` | SQLite repository implementations |
| `internal/renderqueue/` | Async render queue (worker pool, priority heap) |
| `internal/config/` | File-based configuration |
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
