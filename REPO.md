# REPO.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Philosophy

Periwiki is a wiki for people who remember the old web. Key values:

- **No JavaScript** - Server-rendered HTML only, cache-friendly
- **Minimal aesthetic is intentional** - Don't modernize it for modernity's sake
- **Simplicity over features** - Think: Hugo, not Joomla
- **Performance is key** - The simple aesthetic isn't just for looks
- **Single binary** - Easy to run, default config works

When making decisions, prefer the simpler solution that preserves these values.

## Build & Test

Use the Makefile (`make`, `make test`) or standard Go commands (`go test ./...`).

After schema changes in `internal/storage/schema.sql`, run `make model` to regenerate SQLBoiler models.

## Architecture

Layered with interface-based DI: Handlers → Services (`wiki/service/`) → Repositories (`wiki/repository/` interfaces, `internal/storage/` implementations) → SQLite.

User injected into request context by middleware; access via `req.Context().Value(wiki.UserKey).(*wiki.User)`. Anonymous users have `ID: 0`.

Sentinel errors in `wiki/errors.go` control HTTP responses (`ErrGenericNotFound` → 404, etc.).

## Testing

`testutil.SetupTestApp()` creates an in-memory SQLite with full schema. Use `testutil.CreateTestUser()` and `testutil.CreateTestArticle()` for fixtures.

## Git Workflow

Conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`, `style:`, `ui:`. Optional scope: `refactor(css):`. Breaking changes: `feat!:`.

## Agent & Tool Use

Prefer Serena's semantic tools for Go code navigation and refactoring—`find_symbol`, `find_referencing_symbols`, `replace_symbol_body`. The interface-based architecture makes symbolic navigation effective: find an interface in `wiki/service/` or `wiki/repository/`, then trace to implementations.

For multi-step work, use the Task tool with subagents to parallelize independent tasks. Explore agent for codebase questions; Plan agent before non-trivial implementations.
