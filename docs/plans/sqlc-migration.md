# Database Layer Migration: sqlc

**Status:** Proposal
**Phase:** 3 of 3 (Database Layer Refactoring)
**Prerequisites:** Phase 1 — Remove SQLBoiler, Phase 2 — Versioned Migrations
**Next:** Feature development (backlinks, FTS, talk pages) on the new foundation

## Summary

Replace direct sqlx usage with sqlc for type-safe, SQL-first database access. This migration enables multi-database support (SQLite, PostgreSQL) while preserving the existing repository pattern.

By this phase, SQLBoiler is already removed (Phase 1) and the migration system is versioned and dialect-aware (Phase 2).

## Goals

1. **Type-safe query layer** — compile-time verification that queries match schema, no runtime SQL surprises
2. **Multi-DB portability** — SQLite and PostgreSQL both work, chosen at runtime via config
3. **Eliminate manual boilerplate** — sqlc generates scanning structs and query functions, replacing `articleResult` and manual `Scan()` calls
4. **Easier feature development** — adding a new query means writing SQL in a `.sql` file and running `sqlc generate`

## Non-Goals

1. **ORM or query builder** — SQL stays explicit. No fluent APIs, no magic
2. **Schema abstraction** — dialect differences are handled with concrete SQL, not hidden behind a generic layer
3. **Multi-tenant support** — no tenant isolation, shared schemas, row-level security
4. **MySQL support** — only SQLite and PostgreSQL
5. **Horizontal scaling** — no sharding, read replicas, or distributed transactions

## Background

### Current State (after Phase 1 and 2)

- **sqlx** handles all data access via prepared statements and manual SQL
- **Manual result structs** (`articleResult`, etc.) with manual mapping to domain types
- **Manual `PreparedStatements` struct** initialized at startup
- **Versioned migration system** with dialect support (from Phase 2)
- **SQLite-only** with dialect-specific functions (`strftime()`, `json_extract()`)

### Why sqlc

1. **SQL is explicit** — the query file IS the specification, no hidden behaviors
2. **Agent-friendly** — SQL is the most represented database language in training data
3. **Low migration lift** — existing SQL queries move to annotated `.sql` files with minimal changes
4. **Generated boilerplate** — sqlc generates scanning structs and query functions
5. **Compile-time safety** — generated Go code is type-checked
6. **Multi-DB via config** — separate engine blocks in `sqlc.yaml` generate dialect-specific code

### Alternatives Evaluated

| Option | Assessment |
|--------|------------|
| **Bob** | Strong type safety via generated column names. Dialect-specific packages by design. Less training data for agent-driven development. |
| **Bun** | Good multi-DB support with `db.Dialect()` detection. Fluent API has large surface area, implicit behaviors. |
| **sqlc** | **Selected.** SQL-first aligns with current approach. Excellent agent comprehension. Clear patterns. |

## Query Dialect Audit

Investigation of the current codebase shows ~80% of queries are portable:

| Category | Count | Examples |
|----------|-------|---------|
| Portable | 16 | Most SELECTs, JOINs, UPDATEs, standard INSERTs |
| Trivially adaptable | 2 | `INSERT OR REPLACE` → `ON CONFLICT`, `ABS(RANDOM())` |
| SQLite-specific | 10 | `strftime()`, `json_extract()`, `jsonb()`, `last_insert_rowid()` |
| DDL differences | 4 | `AUTOINCREMENT` → `SERIAL`, `INSERT OR IGNORE` → `ON CONFLICT DO NOTHING` |

Of the 10 SQLite-specific queries, 5 are `strftime()` → `CURRENT_TIMESTAMP` (trivial), 2 are `jsonb()` cast changes (trivial), 1 is `json_extract()` → `->>'key'` (straightforward), and 1 is `last_insert_rowid()` → `RETURNING` (moderate).

## Architecture

### Directory Structure

Common queries live once. Only genuinely incompatible queries get dialect-specific versions.

```
internal/storage/
├── sqlc.yaml                    # sqlc configuration
├── schema/
│   ├── common.sql               # Shared schema (portable DDL)
│   ├── postgres.sql             # PG-specific (SERIAL, JSONB columns, GIN indexes)
│   └── sqlite.sql               # SQLite-specific (AUTOINCREMENT, JSON1, FTS5)
├── queries/
│   ├── common/                  # ~80% of queries — portable SQL
│   │   ├── article.sql
│   │   ├── user.sql
│   │   └── preference.sql
│   ├── postgres/                # Dialect-specific overrides only
│   │   └── article.sql          # JSONB ops, FTS, RETURNING, timestamps
│   └── sqlite/                  # Dialect-specific overrides only
│       └── article.sql          # json_extract(), FTS5, strftime(), last_insert_rowid()
├── postgres/                    # Generated code (by sqlc)
│   ├── db.go
│   ├── models.go
│   └── queries.sql.go
├── sqlitedb/                    # Generated code (by sqlc)
│   ├── db.go
│   ├── models.go
│   └── queries.sql.go
└── repository.go                # Factory: returns correct implementation by dialect
```

### Configuration

```yaml
# internal/storage/sqlc.yaml
version: "2"
sql:
  # SQLite configuration
  - engine: "sqlite"
    schema:
      - "schema/common.sql"
      - "schema/sqlite.sql"
    queries:
      - "queries/common/"
      - "queries/sqlite/"
    gen:
      go:
        package: "sqlitedb"
        out: "sqlitedb"
        emit_interface: true
        emit_exact_table_names: false

  # PostgreSQL configuration
  - engine: "postgresql"
    schema:
      - "schema/common.sql"
      - "schema/postgres.sql"
    queries:
      - "queries/common/"
      - "queries/postgres/"
    gen:
      go:
        package: "postgres"
        out: "postgres"
        emit_interface: true
        emit_exact_table_names: false
```

Both engines reference `queries/common/`. Only the handful of queries that genuinely differ live in `queries/sqlite/` or `queries/postgres/`. When a dialect-specific file defines a query with the same name as one in common, it overrides it for that engine.

### Query File Convention

```sql
-- queries/common/article.sql

-- name: SelectArticle :one
SELECT
    a.url,
    r.id, r.title, r.markdown, r.html, r.hashval, r.created,
    r.previous_id, r.comment,
    u.id AS user_id, u.screenname
FROM article a
JOIN revision r ON a.id = r.article_id
JOIN "user" u ON r.user_id = u.id
WHERE a.url = sqlc.arg('url')
ORDER BY r.created DESC
LIMIT 1;

-- name: InsertArticle :exec
INSERT INTO article (url) VALUES (sqlc.arg('url'));

-- name: SelectRandomArticleURL :one
SELECT url FROM article ORDER BY RANDOM() LIMIT 1;
```

Cross-engine parameter syntax uses `sqlc.arg('name')`, which compiles to `?` for SQLite and `$1` for PostgreSQL.

### Dialect-Specific Overrides

```sql
-- queries/sqlite/article.sql

-- name: InsertArticleWithTimestamp :execresult
INSERT INTO article (url, created)
VALUES (sqlc.arg('url'), strftime("%Y-%m-%d %H:%M:%f", "now"));

-- name: SelectArticlesByTag :many
SELECT a.url, a.id, r.title
FROM article a
JOIN revision r ON a.id = r.article_id
WHERE EXISTS (
    SELECT 1 FROM json_each(a.frontmatter, '$.tags')
    WHERE value = sqlc.arg('tag')
)
ORDER BY r.title;
```

```sql
-- queries/postgres/article.sql

-- name: InsertArticleWithTimestamp :one
INSERT INTO article (url, created)
VALUES (sqlc.arg('url'), CURRENT_TIMESTAMP)
RETURNING id;

-- name: SelectArticlesByTag :many
SELECT a.url, a.id, r.title
FROM article a
JOIN revision r ON a.id = r.article_id
WHERE a.frontmatter @> jsonb_build_object('tags', jsonb_build_array(sqlc.arg('tag')::text))
ORDER BY r.title;
```

### Repository Implementation

```go
// internal/storage/repository.go

// NewArticleRepository returns the appropriate implementation for the dialect.
func NewArticleRepository(db *sql.DB, dialect Dialect) repository.ArticleRepository {
    switch dialect {
    case DialectPostgres:
        return &postgresArticleRepo{q: postgres.New(db)}
    default:
        return &sqliteArticleRepo{q: sqlitedb.New(db)}
    }
}
```

Each dialect-specific repo struct wraps the generated `Queries` and maps results to domain types. sqlc manages prepared statements internally — the manual `PreparedStatements` struct is removed.

### Domain Mapping

sqlc generates separate row structs per engine (`sqlitedb.SelectArticleRow`, `postgres.SelectArticleRow`). Since the common queries produce identical column sets, mapping functions can be shared via generics or kept as simple per-dialect functions (decide during implementation — generics add complexity for marginal benefit).

## Open Decision

**`context.Context` on repository interfaces** — sqlc generates context-aware functions by default. The current repository interfaces (22 methods across 4 interfaces) do not take `context.Context`. Adding it requires ~50-60 mechanical call site changes across ~15 files. Options:
- Add context to all repo interfaces (Go standard, sqlc-native)
- Wrap sqlc functions to strip context at the boundary (less churn, worse practice)

Decision deferred to implementation time.

## Implementation Plan

### Step 1: Setup and Parallel Implementation

**Goal:** Get sqlc working alongside existing code.

1. Install sqlc, add `sqlc.yaml` configuration
2. Create `schema/common.sql` and `schema/sqlite.sql` from existing `schema.sql`
3. Move portable queries to `queries/common/` with sqlc annotations
4. Move SQLite-specific queries to `queries/sqlite/`
5. Run `sqlc generate`, verify compilation
6. Write domain mapping functions
7. Add `sqlc generate` to Makefile

**Deliverable:** sqlc generates code, existing tests still pass (sqlx still active).

### Step 2: Migrate SQLite Implementation

**Goal:** Replace sqlx usage with sqlc-generated code for SQLite.

1. Resolve `context.Context` decision for repository interfaces
2. Implement `sqliteArticleRepo` using generated queries
3. Migrate one repository at a time:
   - `ArticleRepository`
   - `UserRepository`
   - `PreferenceRepository`
4. Remove `PreparedStatements` struct from `sqlite.go`
5. Remove `articleResult` and other manual scanning structs
6. Remove sqlx dependency (if fully replaced)

**Deliverable:** All repositories use sqlc. No manual SQL string building.

### Step 3: Add PostgreSQL Support

**Goal:** Enable PostgreSQL as an alternative database.

1. Create `schema/postgres.sql` with dialect-specific DDL (`SERIAL`, JSONB columns, GIN indexes)
2. Add PostgreSQL-specific query overrides where needed
3. Run `sqlc generate` for PostgreSQL engine
4. Implement PostgreSQL repository wrappers
5. Add database dialect to application configuration (runtime switch)
6. Update `Init()` to select appropriate driver and dialect
7. Add integration tests for PostgreSQL (separate test database)
8. Write PostgreSQL-specific migrations in Phase 2's migration system

**Deliverable:** Application runs on SQLite or PostgreSQL via config.

## Future Features (designed for, not implemented here)

| Feature | Common Query | PostgreSQL | SQLite |
|---------|--------------|------------|--------|
| Backlinks | New table, portable queries | Partial index | Standard index |
| Full-text Search | - | tsvector + GIN | FTS5 virtual table |
| Talk Pages | New table, portable queries | Same | Same |
| Frontmatter/Tags | Already implemented | JSONB + GIN | json_each() |

## Conventions for Agent-Driven Development

Document these in CLAUDE.md after implementation:

### Adding a New Query

1. Determine if query is portable or dialect-specific
2. Add to `queries/common/` or `queries/{dialect}/`
3. Use annotation: `-- name: QueryName :one|:many|:exec|:execresult`
4. Use `sqlc.arg('name')` for parameters (engine-agnostic)
5. Run `sqlc generate`
6. Add domain mapping if new result shape
7. Update repository interface and both implementations

### Keeping Dialects in Sync

When adding dialect-specific queries:
1. Both `postgres/` and `sqlite/` must define the same query name
2. Return types must match (same columns, same order)
3. Test both implementations

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Dialect queries drift out of sync | CI check that query names match across dialect directories |
| `sqlc.arg()` syntax unfamiliar | Document in CLAUDE.md, consistent examples in query files |
| Generated code conflicts with existing | Parallel implementation (Step 1) before cutting over |
| PostgreSQL integration test infra | Docker-based test database, skip in CI if unavailable |

## Success Criteria

- [ ] All repositories use sqlc-generated code
- [ ] Existing tests pass with SQLite
- [ ] New integration tests pass with PostgreSQL
- [ ] `PreparedStatements` struct removed
- [ ] `articleResult` and manual scanning structs removed
- [ ] Query patterns documented in CLAUDE.md
- [ ] No manual SQL string building in repository layer
- [ ] Application selects database at runtime via config

## Dependencies

- sqlc v2.x (latest stable)
- Keep: `modernc.org/sqlite` (driver)
- Add: `jackc/pgx` (PostgreSQL driver)
- Remove (after Step 2): `jmoiron/sqlx` (if fully replaced)
