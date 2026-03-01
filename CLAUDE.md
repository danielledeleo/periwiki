# CLAUDE.md

## Project Philosophy

Periwiki is a wiki for people who remember the old web. Key values:

- **No JavaScript** - Server-rendered HTML only, cache-friendly
- **Minimal aesthetic is intentional** - Don't modernize it for modernity's sake
- **Simplicity over features** - Think: Hugo, not Joomla
- **Performance is key** - The simple aesthetic isn't just for looks
- **Single binary** - Easy to run, default config works

When making decisions, prefer the simpler solution that preserves these values.

## Build & Test

`make` builds the binary. `make test` or `go test ./...` runs tests.

## Architecture

Layered with interface-based DI: Handlers (`internal/server/`) → Services (`wiki/service/`) → Repositories (`wiki/repository/` interfaces, `internal/storage/` implementations) → SQLite.

User injected into request context by middleware; access via `req.Context().Value(wiki.UserKey).(*wiki.User)`. Anonymous users have `ID: 0`.

Sentinel errors in `wiki/errors.go` control HTTP responses (`ErrGenericNotFound` → 404, etc.).

### Rendering Pipeline

Markdown → Goldmark (with wikilink, footnote extensions) → TOC injection (goquery) → HTML sanitization (bluemonday). Entry point: `render/renderer.go`. Service layer: `wiki/service/rendering.go`.

Templates in `templates/_render/` are render-time templates (TOC, footnotes, wikilinks) hashed for stale content detection. Templates in `templates/` are page-level templates. Both use paths relative to project root.

## Testing

When adding new features or fixing bugs, add new tests or extend existing ones.

`testutil.SetupTestApp()` creates an in-memory SQLite with full schema. Use `testutil.CreateTestUser()` and `testutil.CreateTestArticle()` for fixtures.

All tests require the working directory to be the project root.

## Feature Development Checklist

When building or modifying a feature, walk this list and ask "does this change touch or affect this subsystem?" For each yes, verify the interaction is handled. Keep this list current — when a subsystem is added, removed, or restructured, update the relevant item.

**Rendering pipeline** — Markdown → Goldmark (extensions) → TOC injection → HTML sanitization. If your feature produces or alters rendered content, consider the stale content hash, the sanitizer policy, and render-time templates.

**Overlay FS & embedded content** — All templates, static files, and help articles are embedded and served through an overlay FS (disk overrides embedded per-file). New files in embedded directories are included automatically, but template loading globs are explicit.

**Template loading (dual-registration)** — Page templates are loaded by glob in two places that must stay in sync: the production setup and the test setup. If you add a new template subdirectory, add its glob to both. Extension templates have their own load calls — also in both places.

**Database schema & migrations** — schema.sql is the source of truth for fresh databases. Existing databases are brought forward by migrations. If you add or change a table/column: update the schema, write a migration, and remember SQLite's ALTER TABLE limitations (no non-constant defaults, no column drops without table recreation).

**Prepared statements** — User and article queries use pre-prepared statements initialized at startup. If you add columns, update the SELECT lists in both the production and test statement initialization.

**Repository interfaces** — Adding a data access method means implementing it in three places: the interface, the SQLite implementation, and the test implementation. The compiler catches the first two; the third fails at runtime if missed.

**Service layer** — Services compose repositories and contain business logic. New services must be instantiated in setup with correct dependencies (order matters) and wired into the App struct.

**Route registration** — All routes live in RegisterRoutes(). If your feature needs HTTP endpoints, add them there. Check method restrictions and consider auth requirements.

**Auth & permissions** — Middleware injects the user into request context (anonymous = ID 0). Protected routes use RequireAuth/RequireAdmin helpers. If your feature has access control, wire it through these and test both allowed and denied paths.

**Special pages & namespaces** — Special pages are registered in a central registry. Namespaces (Talk:, Periwiki:, Special:) are handled by NamespaceHandler. If your feature adds a special page or namespace, register it and ensure the existence checker knows about it (affects wikilink rendering).

**Sentinel errors** — Service-layer errors map to HTTP status codes in handlers. If you introduce a new failure mode, define a sentinel error and map it — otherwise it surfaces as a 500.

**HTTP caching** — Responses carry Cache-Control, ETag, and Last-Modified headers. If your feature serves content that changes (or doesn't), set appropriate caching. Mutable content uses validation caching; immutable content uses long max-age.

**Session & cookies** — Session state is cookie-based. If your feature reads or writes session data, handle decode failures gracefully (treat as anonymous, don't 500).

## Git Workflow

Conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`, `style:`, `ui:`. Optional scope: `refactor(css):`. Breaking changes: `feat!:`.

## Agent & Tool Use

Use Serena's semantic tools (`find_symbol`, `find_referencing_symbols`, `replace_symbol_body`) for navigating interfaces to implementations and for whole-symbol refactoring. Use built-in tools (Glob, Grep, Read, Edit) for known-path file reads, simple edits, and text search.

For multi-step work, use the Task tool with subagents to parallelize independent tasks. Explore agent for codebase questions; Plan agent before non-trivial implementations.

When unsure about standard library or dependency APIs, check with `go doc` (e.g. `go doc html/template`, `go doc github.com/PuerkitoBio/goquery`) rather than guessing signatures or behavior.
