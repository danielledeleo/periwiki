# Remove SQLBoiler Dead Code

**Status:** Proposal
**Phase:** 1 of 3 (Database Layer Refactoring)
**Prerequisites:** None
**Next:** Phase 2 — Versioned Migrations (`2026-02-06-versioned-migrations.md`)

## Goals

Remove all SQLBoiler-related dead code to clean the deck before restructuring the database layer. SQLBoiler's generated models are never imported anywhere in application code — all data access uses sqlx with manual SQL.

## Non-goals

This plan does NOT change any application behavior, query logic, or migration system. It only removes unused code and build dependencies.

## What to remove

1. **`model/` directory** — 15 generated files, zero imports in application code
2. **`sqlboiler.toml`** — SQLBoiler configuration file
3. **`generator.go`** — contains `//go:generate .bin/sqlboiler sqlite3 --wipe`
4. **`internal/storage/skeleton.db`** — SQLite database used by SQLBoiler for code generation
5. **`cmd/mkskeleton/`** — tool that creates `skeleton.db` from `schema.sql`
6. **`.bin/sqlboiler`, `.bin/sqlboiler-sqlite3`** — SQLBoiler binaries (if present in local checkout)

## Makefile changes

The current Makefile has deep SQLBoiler integration:

- `model` target generates SQLBoiler models
- `periwiki` target depends on `model`
- All `test*` targets depend on `model`
- `clean` target removes `model/`, `skeleton.db`, and `.bin/`
- `find_go` explicitly excludes `./model/*`
- `help` target documents the `model` target

Changes:

1. Remove the `model` target and its prerequisites (`.bin/sqlboiler`, `.bin/sqlboiler-sqlite3`, `internal/storage/skeleton.db`)
2. Remove `model` from the `periwiki` and `test*` target dependencies
3. Remove `model/` and `skeleton.db` from `clean` target
4. Remove the `find_go` exclusion of `./model/*`
5. Remove the `.bin` directory target (only used for SQLBoiler binaries)
6. Remove `.bin` from `clean` target
7. Update `help` target to remove the `model` entry

## Dependency removal

After deleting the files above, run `go mod tidy`. The following should drop from `go.mod`:

- `github.com/aarondl/sqlboiler/v4`
- `github.com/aarondl/null/v8`
- `github.com/aarondl/strmangle`
- `github.com/friendsofgo/errors`

These are only referenced transitively through the generated `model/` package and `generator.go`.

## Verification

1. `go build ./cmd/periwiki` — binary builds
2. `go test ./...` — all tests pass
3. `go vet ./...` — no issues
4. `go mod tidy` — no unused dependencies remain
5. `make` — builds successfully with updated Makefile
6. `make test` — tests pass via Makefile
7. `make clean` — cleans without errors

## Risks

Essentially none. This is removing code that is never imported or referenced by application code. The generated `model/` package has zero consumers. The `skeleton.db` and `mkskeleton` tool exist solely for the SQLBoiler generation pipeline.
