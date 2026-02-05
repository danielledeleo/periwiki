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

After schema changes in `internal/storage/schema.sql`, run `make model` to regenerate SQLBoiler models.

## Architecture

Layered with interface-based DI: Handlers (`internal/server/`) → Services (`wiki/service/`) → Repositories (`wiki/repository/` interfaces, `internal/storage/` implementations) → SQLite.

User injected into request context by middleware; access via `req.Context().Value(wiki.UserKey).(*wiki.User)`. Anonymous users have `ID: 0`.

Sentinel errors in `wiki/errors.go` control HTTP responses (`ErrGenericNotFound` → 404, etc.).

### Rendering Pipeline

Markdown → Goldmark (with wikilink, footnote extensions) → HTML sanitization (bluemonday) → TOC injection (goquery). Entry point: `render/renderer.go`. Service layer: `wiki/service/rendering.go`.

Templates in `templates/_render/` are render-time templates (TOC, footnotes, wikilinks) hashed for stale content detection. Templates in `templates/` are page-level templates. Both use paths relative to project root.

## Testing

When adding new features or fixing bugs, add new tests or extend existing ones.

`testutil.SetupTestApp()` creates an in-memory SQLite with full schema. Use `testutil.CreateTestUser()` and `testutil.CreateTestArticle()` for fixtures.

All tests require the working directory to be the project root.

## Git Workflow

Conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`, `style:`, `ui:`. Optional scope: `refactor(css):`. Breaking changes: `feat!:`.

## Agent & Tool Use

Use Serena's semantic tools (`find_symbol`, `find_referencing_symbols`, `replace_symbol_body`) for navigating interfaces to implementations and for whole-symbol refactoring. Use built-in tools (Glob, Grep, Read, Edit) for known-path file reads, simple edits, and text search.

For multi-step work, use the Task tool with subagents to parallelize independent tasks. Explore agent for codebase questions; Plan agent before non-trivial implementations.

When unsure about standard library or dependency APIs, check with `go doc` (e.g. `go doc html/template`, `go doc github.com/PuerkitoBio/goquery`) rather than guessing signatures or behavior.
