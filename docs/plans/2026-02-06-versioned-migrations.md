# Refactor to Versioned Migration System

**Status:** Done
**Phase:** 2 of 3 (Database Layer Refactoring)
**Prerequisites:** Phase 1 — Remove SQLBoiler
**Next:** Phase 3 — sqlc Migration

## Goals

This phase stabilizes the migration system so that feature development (backlinks, FTS, talk pages) can add schema changes confidently.

1. **Know the database state** — a single `schema_version` value tells you exactly which migrations have run
2. **Dialect-aware from day one** — migration functions receive a dialect parameter, enabling PostgreSQL support in Phase 3 without reworking the runner
3. **Eliminate detection logic** — no more `pragma_table_info` checks to guess what state a database is in
4. **Safe table recreation** — a helper function for SQLite's `ALTER TABLE` limitations that handles foreign key pragmas correctly

## Non-Goals

- **Not changing the query layer** — sqlx and manual SQL stay as-is; that's Phase 3
- **Not adding PostgreSQL migrations** — the runner supports dialects, but only SQLite migrations exist after this phase
- **Not redesigning schema.sql** — it continues to define the canonical schema for new databases

## Context

The current migration system in `internal/storage/migrations.go` is a single `RunMigrations` function containing inline, condition-based migrations. Each migration checks schema state (`pragma_table_info`) to decide whether to run. This is brittle:

- No way to know what version a database is at
- Buggy migrations that partially ran can't be detected or repaired cleanly
- Table recreations (needed for SQLite schema changes) require careful FK handling that's easy to get wrong
- Each new migration adds more fragile detection logic

This plan replaces it with numbered, sequential migrations tracked by a version in the Setting table.

## Design

### Dialect type

```go
type Dialect string

const (
    DialectSQLite   Dialect = "sqlite"
    DialectPostgres Dialect = "postgres"
)
```

This type is defined once and shared across the migration system and (later) the repository layer in Phase 3.

### Version tracking

Store `schema_version` in the existing Setting table. The value is an integer string representing the highest migration that has been applied. `schema.sql` sets it to the current latest version for new databases. On startup, `RunMigrations` reads the current version and runs any migrations with a higher number.

### Migration registry

```go
type migration struct {
    version     int
    description string
    fn          func(db *sqlx.DB, dialect Dialect) error
}
```

Migrations are a `[]migration` slice in `migrations.go`. Each migration has a version number, a description, and a function that receives both the database handle and the dialect. The runner iterates through them, skipping any with version <= current, and executes the rest in order, updating `schema_version` after each.

Most migrations ignore the dialect parameter and run portable SQL. The few that need dialect-specific behavior (like SQLite's table recreation pattern vs PostgreSQL's native `ALTER COLUMN`) branch internally.

### Table recreation helper (SQLite-specific)

```go
func recreateTable(db *sqlx.DB, createSQL string, insertSQL string) error
```

- Executes `PRAGMA foreign_keys = OFF`
- Runs the create + insert + drop + rename SQL
- Re-enables `PRAGMA foreign_keys = ON`
- On error, still re-enables FK checks before returning

PostgreSQL doesn't need this — it supports `ALTER COLUMN` natively. Migrations that use `recreateTable` guard it behind a `dialect == DialectSQLite` check.

### Handling existing databases

Existing databases have no `schema_version` key. The runner detects this and determines the starting version by inspecting schema state — specifically, if the `created_at` column exists on User (the most recent migration), we know all prior migrations have run. Set `schema_version` to the latest legacy version and skip all legacy migrations.

If `created_at` doesn't exist, check in reverse order (role, frontmatter, html nullable, etc.) to find the right starting point. This runs once on first upgrade and never again.

### schema.sql stays as-is

`schema.sql` continues to define the canonical schema for new databases with `CREATE TABLE IF NOT EXISTS`. It also sets `schema_version` to the latest version via `INSERT OR IGNORE INTO Setting(key, value) VALUES ('schema_version', 'N')`. This way new databases skip all migrations.

## Implementation

### Step 1: Add Dialect type and `recreateTable` helper

Define the `Dialect` type (shared with Phase 3). Implement the `recreateTable` helper for SQLite's table recreation pattern with proper FK pragma handling.

### Step 2: Define migration registry

```go
var migrations = []migration{
    {1, "add render_status to Revision", migrateRenderStatus},
    {2, "drop title from Revision", migrateDropTitle},
    {3, "add frontmatter to Article", migrateFrontmatter},
    {4, "make html nullable in Revision", migrateHTMLNullable},
    {5, "backfill frontmatter", migrateBackfillFrontmatter},
    {6, "add role to User", migrateAddRole},
    {7, "add created_at to User", migrateAddCreatedAt},
}
```

Each `migrate*` function contains exactly the logic currently inline in `RunMigrations`, but without the condition checks — the version system handles "should this run?". Each function takes `(db *sqlx.DB, dialect Dialect)`.

### Step 3: Rewrite `RunMigrations`

```go
func RunMigrations(db *sqlx.DB, dialect Dialect) error {
    // 1. Execute schema.sql (idempotent CREATE IF NOT EXISTS)
    // 2. Read schema_version from Setting (default to 0 if missing)
    // 3. For existing unversioned databases, detect current state and set version
    // 4. Run each migration with version > current, update schema_version after each
}
```

The `dialect` parameter flows through from the application's database configuration.

### Step 4: Bootstrap detection for existing databases

If `schema_version` key doesn't exist in Setting but the Setting table exists, this is a pre-versioning database. Detect state by checking the most recent migration marker — if `created_at` column exists on User, all 7 legacy migrations have run, set version = 7. Otherwise check in reverse order to find the right starting point.

This detection logic uses `pragma_table_info` one last time — after bootstrapping, it's never used again.

### Step 5: Update schema.sql

Add at the end (after the anonymous user INSERT):
```sql
INSERT OR IGNORE INTO Setting(key, value) VALUES ('schema_version', '7');
```

### Step 6: Fix the created_at migration (migration 7)

Uses `recreateTable` helper guarded by `dialect == DialectSQLite`. Produces the correct `NOT NULL DEFAULT CURRENT_TIMESTAMP` schema.

### Step 7: Always-run fixups

The anonymous user role fixup (`UPDATE User SET role = '' WHERE id = 0`) runs unconditionally after all migrations, same as today.

## Files

| File | Change |
|---|---|
| `internal/storage/migrations.go` | Full rewrite: dialect type, version tracking, migration registry, helper functions |
| `internal/storage/schema.sql` | Add `INSERT OR IGNORE` for `schema_version` setting |
| `internal/storage/sqlite.go` | Update `Init()` / `RunMigrations` call site to pass dialect |

## Verification

1. `go test -count=1 ./...` — all tests pass
2. Run against existing `periwiki.db` — bootstraps version, runs any needed migrations
3. `sqlite3 periwiki.db "SELECT sql FROM sqlite_master WHERE type='table' AND name='User';"` — shows correct `NOT NULL DEFAULT CURRENT_TIMESTAMP`
4. `sqlite3 periwiki.db "SELECT value FROM Setting WHERE key='schema_version';"` — shows `7`
5. Run again — no migrations execute, clean startup
