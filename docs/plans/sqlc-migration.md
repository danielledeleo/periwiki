# Database Layer Migration: sqlc

**Status:** Proposal
**Author:** Claude (with Dani)
**Date:** January 2026

## Summary

Replace SQLBoiler and direct sqlx usage with sqlc for type-safe, SQL-first database access. This migration enables multi-database support (SQLite, PostgreSQL, MySQL) while preserving the existing repository pattern.

## Background

### Current State

The project uses a hybrid approach that provides minimal benefit:

- **SQLBoiler** generates 14 model files (~11.7K LOC) that are never used for queries
- **sqlx** handles all actual data access via prepared statements and manual SQL
- **Manual result structs** (`articleResult`, etc.) duplicate effort with manual mapping to domain types
- **SQLite-only** with dialect-specific functions (`strftime()`, `RANDOM()`)

### Problems

1. **Dead code**: SQLBoiler models generated but unused
2. **No multi-DB path**: Queries use SQLite-specific syntax
3. **Maintenance burden**: SQLBoiler in "low-maintenance mode", maintainers recommend alternatives
4. **Duplicate mappings**: `articleResult` → `wiki.Article` done manually

### Alternatives Evaluated

| Option | Assessment |
|--------|------------|
| **Bob** | Strong type safety via generated column names. Dialect-specific packages by design. Less training data for agent-driven development. |
| **Bun** | Good multi-DB support with `db.Dialect()` detection. Fluent API has large surface area, implicit behaviors. |
| **sqlc** | **Selected.** SQL-first aligns with current approach. Excellent agent comprehension. Clear patterns. |

## Why sqlc

1. **SQL is explicit**: The query file IS the specification—no hidden behaviors
2. **Agent-friendly**: SQL is the most represented database language in training data
3. **Low migration lift**: Existing SQL queries move to annotated `.sql` files
4. **Generated boilerplate**: sqlc generates scanning structs and query functions
5. **Compile-time safety**: Generated Go code is type-checked

## Architecture

### Directory Structure

```
internal/storage/
├── sqlc.yaml                    # sqlc configuration
├── schema/
│   ├── common.sql               # Shared schema (portable DDL)
│   ├── postgres.sql             # PG-specific (JSONB columns, GIN indexes)
│   └── sqlite.sql               # SQLite-specific (JSON1, FTS5)
├── queries/
│   ├── common/                  # Queries that work on all dialects
│   │   ├── article.sql
│   │   ├── user.sql
│   │   └── preference.sql
│   ├── postgres/                # PostgreSQL-specific queries
│   │   └── article.sql          # JSONB operations, full-text search
│   └── sqlite/                  # SQLite-specific queries
│       └── article.sql          # json_extract(), FTS5
├── postgres/                    # Generated code
│   ├── db.go
│   ├── models.go
│   └── queries.sql.go
├── sqlite/                      # Generated code
│   ├── db.go
│   ├── models.go
│   └── queries.sql.go
└── repository.go                # Factory: returns correct implementation
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
        package: "sqlite"
        out: "sqlite"
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

### Query File Convention

```sql
-- queries/common/article.sql

-- name: SelectArticle :one
-- SelectArticle retrieves an article with its latest revision and creator.
SELECT
    a.url,
    r.id, r.title, r.markdown, r.html, r.hashval, r.created,
    r.previous_id, r.comment,
    u.id AS user_id, u.screenname
FROM article a
JOIN revision r ON a.id = r.article_id
JOIN "user" u ON r.user_id = u.id
WHERE a.url = ?
ORDER BY r.created DESC
LIMIT 1;

-- name: SelectArticleByRevisionHash :one
SELECT
    a.url,
    r.id, r.title, r.markdown, r.html, r.hashval, r.created,
    r.previous_id, r.comment,
    u.id AS user_id, u.screenname
FROM article a
JOIN revision r ON a.id = r.article_id
JOIN "user" u ON r.user_id = u.id
WHERE a.url = ? AND r.hashval = ?;

-- name: InsertArticle :exec
INSERT INTO article (url) VALUES (?);

-- name: InsertRevision :exec
INSERT INTO revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: SelectRandomArticleURL :one
SELECT url FROM article ORDER BY RANDOM() LIMIT 1;
```

### Dialect-Specific Queries

```sql
-- queries/postgres/article.sql

-- name: SelectArticlesByTag :many
-- PostgreSQL: Use JSONB containment operator with GIN index
SELECT a.url, a.id, r.title
FROM article a
JOIN revision r ON a.id = r.article_id
WHERE a.frontmatter @> jsonb_build_object('tags', jsonb_build_array($1::text))
ORDER BY r.title;

-- name: SearchArticles :many
-- PostgreSQL: Full-text search with tsvector
SELECT a.url, r.title, ts_rank(r.search_vector, query) AS rank
FROM article a
JOIN revision r ON a.id = r.article_id,
     plainto_tsquery('english', $1) query
WHERE r.search_vector @@ query
ORDER BY rank DESC
LIMIT $2;
```

```sql
-- queries/sqlite/article.sql

-- name: SelectArticlesByTag :many
-- SQLite: Use json_each() table-valued function
SELECT a.url, a.id, r.title
FROM article a
JOIN revision r ON a.id = r.article_id
WHERE EXISTS (
    SELECT 1 FROM json_each(a.frontmatter, '$.tags')
    WHERE value = ?
)
ORDER BY r.title;

-- name: SearchArticles :many
-- SQLite: Use FTS5 virtual table
SELECT a.url, r.title, bm25(article_fts) AS rank
FROM article_fts
JOIN article a ON article_fts.rowid = a.id
JOIN revision r ON a.id = r.article_id
WHERE article_fts MATCH ?
ORDER BY rank
LIMIT ?;
```

### Repository Implementation

```go
// internal/storage/repository.go

package storage

import (
    "context"
    "database/sql"

    "github.com/danielledeleo/periwiki/internal/storage/postgres"
    "github.com/danielledeleo/periwiki/internal/storage/sqlite"
    "github.com/danielledeleo/periwiki/wiki"
    "github.com/danielledeleo/periwiki/wiki/repository"
)

type Dialect string

const (
    DialectSQLite   Dialect = "sqlite"
    DialectPostgres Dialect = "postgres"
)

// NewArticleRepository returns the appropriate implementation for the dialect.
func NewArticleRepository(db *sql.DB, dialect Dialect) repository.ArticleRepository {
    switch dialect {
    case DialectPostgres:
        return &postgresArticleRepo{q: postgres.New(db)}
    default:
        return &sqliteArticleRepo{q: sqlite.New(db)}
    }
}

// SQLite implementation
type sqliteArticleRepo struct {
    q *sqlite.Queries
}

func (r *sqliteArticleRepo) SelectArticle(ctx context.Context, url string) (*wiki.Article, error) {
    row, err := r.q.SelectArticle(ctx, url)
    if err != nil {
        return nil, err
    }
    return mapRowToArticle(row), nil
}

// PostgreSQL implementation
type postgresArticleRepo struct {
    q *postgres.Queries
}

func (r *postgresArticleRepo) SelectArticle(ctx context.Context, url string) (*wiki.Article, error) {
    row, err := r.q.SelectArticle(ctx, url)
    if err != nil {
        return nil, err
    }
    return mapRowToArticle(row), nil
}
```

### Domain Mapping

```go
// internal/storage/mapping.go

package storage

import "github.com/danielledeleo/periwiki/wiki"

// mapRowToArticle converts sqlc-generated row types to domain types.
// Both sqlite.SelectArticleRow and postgres.SelectArticleRow have the same fields.
func mapRowToArticle[T interface {
    GetUrl() string
    GetId() int32
    GetTitle() string
    GetMarkdown() string
    GetHtml() string
    GetHashval() string
    GetCreated() time.Time
    GetPreviousId() int32
    GetComment() string
    GetUserId() int64
    GetScreenname() string
}](row T) *wiki.Article {
    return &wiki.Article{
        URL: row.GetUrl(),
        Revision: &wiki.Revision{
            ID:         int(row.GetId()),
            Title:      row.GetTitle(),
            Markdown:   row.GetMarkdown(),
            HTML:       row.GetHtml(),
            Hash:       row.GetHashval(),
            Created:    row.GetCreated(),
            PreviousID: int(row.GetPreviousId()),
            Comment:    row.GetComment(),
            Creator: &wiki.User{
                ID:         int(row.GetUserId()),
                ScreenName: row.GetScreenname(),
            },
        },
    }
}
```

Note: Generic mapping requires sqlc's `emit_methods_with_db_argument` or manual implementation per dialect. Alternatively, keep separate mapping functions if generics add complexity.

## Migration Plan

### Phase 1: Setup and Parallel Implementation

**Goal:** Get sqlc working alongside existing code.

1. Add sqlc dependency and configuration
2. Create `schema/` files from existing `schema.sql`
3. Create `queries/common/` with portable queries
4. Generate code, verify compilation
5. Write mapping functions

**Deliverable:** sqlc generates code, existing tests still pass.

### Phase 2: Migrate SQLite Implementation

**Goal:** Replace sqlx usage with sqlc for SQLite.

1. Update repository interfaces to include `context.Context`
2. Implement `sqliteArticleRepo` using generated queries
3. Migrate one repository at a time:
   - `ArticleRepository`
   - `UserRepository`
   - `PreferenceRepository`
4. Remove `PreparedStatements` struct
5. Remove `articleResult` and other manual scanning structs
6. Delete SQLBoiler configuration and generated models

**Deliverable:** All repositories use sqlc. SQLBoiler removed.

### Phase 3: Add PostgreSQL Support

**Goal:** Enable PostgreSQL as an alternative database.

1. Create `schema/postgres.sql` with dialect-specific DDL
2. Add PostgreSQL-specific queries where needed
3. Implement PostgreSQL repository wrappers
4. Add database dialect to configuration
5. Update `Init()` to select appropriate driver and dialect
6. Add integration tests for PostgreSQL

**Deliverable:** Application runs on SQLite or PostgreSQL via config.

### Phase 4: Feature-Specific Queries

**Goal:** Implement planned features with proper dialect support.

| Feature | Common Query | PostgreSQL | SQLite |
|---------|--------------|------------|--------|
| Frontmatter/Tags | - | JSONB + GIN | json_each() |
| Full-text Search | - | tsvector | FTS5 |
| Backlinks | Yes | Partial index | Standard index |
| Rate Limiting | Yes (minor syntax diff) | INTERVAL | datetime() |

## Database Feature Matrix

Features from TODO.md mapped to database requirements:

| Feature | Schema Change | Dialect Consideration |
|---------|---------------|----------------------|
| Frontmatter | `frontmatter` column on article | JSONB (PG) vs TEXT (SQLite) |
| Redirect table | New table + indexes | Portable |
| Backlinks | New table | Partial indexes (PG only) |
| Search | FTS infrastructure | tsvector (PG) vs FTS5 (SQLite) |
| Rate limiting | Action log table | INTERVAL vs datetime() |
| User settings | Settings table or JSON | Portable or JSONB |
| File metadata | Asset table | JSONB for EXIF (PG) |
| Plugin tables | Dynamic schema | Multi-schema (PG) vs attached DB (SQLite) |

## Conventions for Agent-Driven Development

Document these in CLAUDE.md:

### Adding a New Query

1. Determine if query is portable or dialect-specific
2. Add to appropriate file in `queries/common/` or `queries/{dialect}/`
3. Use annotation format: `-- name: QueryName :one|:many|:exec`
4. Run `sqlc generate`
5. Add domain mapping if new result shape
6. Update repository interface and implementations

### Query Annotations

```sql
-- name: GetUser :one        -- Returns single row or error
-- name: ListUsers :many     -- Returns slice (empty if none)
-- name: CreateUser :exec    -- No return value
-- name: CreateUserReturningID :execresult  -- Returns last insert ID
```

### Keeping Dialects in Sync

When adding dialect-specific queries:
1. Both `postgres/` and `sqlite/` must have the same query name
2. Return types must match (same columns, same order)
3. Test both implementations

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Dialect queries drift out of sync | CI check that query names match across dialects |
| Complex queries hard to express | sqlc allows raw SQL—no limitations |
| Migration breaks existing functionality | Parallel implementation, migrate one repo at a time |
| Agent forgets annotations | Document pattern in CLAUDE.md, add linter |

## Success Criteria

- [ ] SQLBoiler removed from project
- [ ] All repositories use sqlc-generated code
- [ ] Existing tests pass with SQLite
- [ ] New integration tests pass with PostgreSQL
- [ ] Query patterns documented in CLAUDE.md
- [ ] No manual SQL string building in repository layer

## Dependencies

- sqlc v1.25+ (latest stable)
- Remove: `volatiletech/sqlboiler`, `aarondl/null`, `aarondl/strmangle`, `friendsofgo/errors`
- Keep: `modernc.org/sqlite` (driver)
- Add: `lib/pq` or `jackc/pgx` (PostgreSQL driver)

## References

- [sqlc documentation](https://docs.sqlc.dev/)
- [sqlc multi-database config](https://docs.sqlc.dev/en/stable/reference/config.html)
- [TODO.md database features analysis](./TODO.md)
