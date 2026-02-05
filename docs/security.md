# Security

## Passwords

Passwords are hashed with [bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) at minimum cost before storage. The raw password is never saved.

| Setting | Default | Description |
|---------|---------|-------------|
| `min_password_length` | `8` | Minimum characters required |

Usernames must match `^[\p{L}0-9-_]+$` (letters, numbers, hyphens, underscores).

**Key files:**
- `wiki/user.go` — `User` struct, `SetPasswordHash()` method
- `wiki/service/user.go` — validation logic, bcrypt comparison

## Sessions

Sessions use [gorilla/sessions](https://github.com/gorilla/sessions) with an SQLite-backed store.

| Setting | Default | Description |
|---------|---------|-------------|
| `cookie_expiry` | `604800` | Session lifetime in seconds (7 days) |

The session secret is a 64-byte random key auto-generated on first run and stored in the SQLite `Setting` table (key `cookie_secret`). Keep your database secure — anyone with the secret can forge session cookies.

**Key files:**
- `internal/server/middleware.go` — validates session, injects user into request context
- `wiki/service/session.go` — cookie CRUD operations
- `wiki/runtime_config.go` — secret generation and loading

## HTML sanitization

User-submitted markdown passes through a rendering pipeline before display:

```
┌──────────────────────────────────────────────────────────────────┐
│                         User Markdown                            │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Goldmark (CommonMark parser)                                    │
│  + Extensions (extensions/{wikilink,footnote}.go)                │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  TOC Injection (goquery)                                         │
│  - Finds h2/h3/h4 headings, builds table of contents            │
│  - Injects TOC before the first h2 (render/renderer.go)         │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Unsafe HTML (may contain scripts, iframes, etc.)                │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Bluemonday UGC Policy                                           │
│  - Strips: <script>, <iframe>, <object>, <embed>, event attrs    │
│  - Allows: <a>, <img>, <p>, <h1-6>, <ul>, <ol>, <table>, etc.    │
│  - Allows href/src only with safe protocols (http, https, mailto)│
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Custom allowlist additions (internal/server/setup.go)           │
│  - class attribute globally (wiki styling)                       │
│  - data-line-number on <a> (footnote navigation)                 │
│  - style on <ins>, <del> (diff rendering)                        │
│  - text-align on <td>, <th> (table alignment)                    │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                          Safe HTML                               │
└──────────────────────────────────────────────────────────────────┘
```

Sanitization happens at render time via `RenderingService.Render()`. The sanitized HTML is stored in the revision, so articles are not re-sanitized on every view.

**Key files:**
- `internal/server/setup.go` — bluemonday policy configuration
- `wiki/service/rendering.go` — `RenderingService` interface and implementation
- `render/renderer.go` — Goldmark markdown rendering
