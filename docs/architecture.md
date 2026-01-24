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
│  Middleware      │  session_middleware.go — injects User into context
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Handler         │  server.go — access user via req.Context().Value(wiki.UserKey)
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

All services are interfaces. The `app` struct in `server.go` holds service references:

```go
type app struct {
    *templater.Templater          // embedded for RenderTemplate()
    articles     service.ArticleService
    users        service.UserService
    sessions     service.SessionService
    // ...
}
```

Services are initialized in `setup.go` via constructor functions:

```go
renderingService := service.NewRenderingService(renderer, sanitizer)
articleService := service.NewArticleService(database, renderingService)
```

Handlers access services through the receiver: `a.articles.GetArticle(url)`

## Directory structure

| Directory | Purpose |
|-----------|---------|
| `/` (root) | Entry point, handlers, middleware, config |
| `wiki/` | Domain types (`Article`, `User`, `Revision`) |
| `wiki/service/` | Service interfaces and implementations |
| `wiki/repository/` | Repository interfaces (no implementations) |
| `internal/storage/` | SQLite repository implementations |
| `render/` | Goldmark markdown rendering |
| `extensions/` | Markdown extensions (WikiLinks, footnotes) |
| `special/` | Special page handlers |
| `templater/` | HTML template engine |
| `templates/` | HTML templates |
| `static/` | CSS and static assets |

## Key files

| File | Purpose |
|------|---------|
| `server.go` | `main()`, routes, handlers, `app` struct |
| `setup.go` | `Setup()` — creates all services and wires dependencies |
| `config.go` | `SetupConfig()` — loads `config.yaml` |
| `session_middleware.go` | Injects user into request context |
| `wiki/context.go` | Defines `UserKey` for context access |
| `wiki/errors.go` | Sentinel errors (`ErrGenericNotFound`, etc.) |

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
