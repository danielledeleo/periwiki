package storage

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
)

//go:embed schema.sql
var schemaSQL string

// Dialect represents the database backend type.
type Dialect int

const (
	DialectSQLite   Dialect = iota
	DialectPostgres         // reserved for Phase 3
)

// migration is a numbered, named schema migration.
type migration struct {
	version     int
	description string
	fn          func(*sqlx.DB, Dialect) error
}

// migrations is the ordered registry of all schema migrations.
// Each version number must be unique and monotonically increasing.
var migrations = []migration{
	{1, "add render_status to Revision", migrateAddRenderStatus},
	{2, "drop title from Revision", migrateDropTitle},
	{3, "add frontmatter to Article", migrateAddFrontmatter},
	{4, "make html nullable in Revision", migrateHTMLNullable},
	{5, "backfill frontmatter", migrateBackfillFrontmatter},
	{6, "add role to User", migrateAddRole},
	{7, "add created_at to User", migrateAddCreatedAt},
}

// latestVersion is the highest migration version in the registry.
var latestVersion = migrations[len(migrations)-1].version

// recreateTable replaces a table using the rename-and-swap pattern.
// It pins the connection to ensure PRAGMA foreign_keys state is consistent
// across the PRAGMA disable, transaction, and PRAGMA restore.
func recreateTable(db *sqlx.DB, table, createSQL, insertSQL string) error {
	ctx := context.Background()
	conn, err := db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("recreateTable %s: pin connection: %w", table, err)
	}
	defer conn.Close()

	// Disable FK checks on this connection (required for DROP+RENAME).
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("recreateTable %s: disable FKs: %w", table, err)
	}
	// Always restore FK checks on this connection before releasing it.
	defer func() {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			slog.Error("recreateTable: failed to restore foreign_keys", "table", table, "error", err)
		}
	}()

	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recreateTable %s: begin tx: %w", table, err)
	}

	newTable := table + "_new"
	stmts := fmt.Sprintf("%s;\n%s;\nDROP TABLE %s;\nALTER TABLE %s RENAME TO %s;",
		createSQL, insertSQL, table, newTable, table)

	if _, err := tx.ExecContext(ctx, stmts); err != nil {
		tx.Rollback()
		return fmt.Errorf("recreateTable %s: exec: %w", table, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recreateTable %s: commit: %w", table, err)
	}
	return nil
}

// detectLegacyVersion inspects the schema to determine which migrations have
// already been applied to a database that predates the version tracking system.
// Returns 0 for a completely fresh database (no tables yet).
func detectLegacyVersion(db *sqlx.DB) (int, error) {
	// Check in reverse order so the first match gives us the highest version.

	// Version 7: User has created_at column
	var hasCreatedAt int
	if err := db.Get(&hasCreatedAt, `SELECT COUNT(*) FROM pragma_table_info('User') WHERE name = 'created_at'`); err != nil {
		return 0, err
	}
	if hasCreatedAt > 0 {
		return 7, nil
	}

	// Version 6: User has role column
	var hasRole int
	if err := db.Get(&hasRole, `SELECT COUNT(*) FROM pragma_table_info('User') WHERE name = 'role'`); err != nil {
		return 0, err
	}
	if hasRole > 0 {
		return 6, nil
	}

	// Version 4/5: Article has frontmatter AND Revision.html is nullable
	// Migration 5 (backfill) has no schema marker; it always co-ran with 4.
	// If html is nullable, migrations 4 and 5 have both been applied.
	var hasFrontmatter int
	if err := db.Get(&hasFrontmatter, `SELECT COUNT(*) FROM pragma_table_info('Article') WHERE name = 'frontmatter'`); err != nil {
		return 0, err
	}
	var htmlNotNull int
	if err := db.Get(&htmlNotNull, `SELECT "notnull" FROM pragma_table_info('Revision') WHERE name = 'html'`); err != nil {
		return 0, err
	}
	if hasFrontmatter > 0 && htmlNotNull == 0 {
		return 5, nil
	}

	// Version 4: html nullable but no frontmatter (shouldn't happen in practice
	// since 3 always ran before 4, but handle defensively)
	if htmlNotNull == 0 {
		return 4, nil
	}

	// Version 3: frontmatter exists but html is still NOT NULL
	if hasFrontmatter > 0 {
		return 3, nil
	}

	// Version 2: Revision lacks title column (title was dropped)
	var hasTitle int
	if err := db.Get(&hasTitle, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'title'`); err != nil {
		return 0, err
	}
	if hasTitle == 0 {
		// Title is absent — but we need to distinguish "never had title" (new DB)
		// from "title was dropped" (migration 2). Check for render_status (migration 1).
		var hasRenderStatus int
		if err := db.Get(&hasRenderStatus, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'render_status'`); err != nil {
			return 0, err
		}
		if hasRenderStatus > 0 {
			return 2, nil
		}
		// No render_status and no title — this is a fresh schema.sql database
		// that was never migrated. All columns match latest schema.
		// But we're in detectLegacyVersion, meaning Setting exists but has no
		// schema_version. This is a pre-migration database at version 0 OR
		// a database created from current schema.sql. Check if Revision table
		// exists at all to distinguish.
		var revisionExists int
		if err := db.Get(&revisionExists, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='Revision'`); err != nil {
			return 0, err
		}
		if revisionExists == 0 {
			return 0, nil
		}
		// Revision exists with render_status=0 and no title — could be current
		// schema.sql. Return latestVersion since all migrations are effectively applied.
		return latestVersion, nil
	}

	// Version 1: Revision has render_status but still has title
	var hasRenderStatus int
	if err := db.Get(&hasRenderStatus, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'render_status'`); err != nil {
		return 0, err
	}
	if hasRenderStatus > 0 {
		return 1, nil
	}

	// Version 0: pristine pre-migration database
	return 0, nil
}

// RunMigrations executes the database schema and any necessary migrations.
// It is idempotent and safe to run on both new and existing databases.
func RunMigrations(db *sqlx.DB, dialect Dialect) error {
	// Step 1: Detect if this is an existing (legacy) database.
	// We must do this BEFORE running schema.sql, because schema.sql now
	// inserts schema_version = latestVersion for new databases.
	var legacyVersion int
	isLegacy := false

	settingExists := tableExists(db, "Setting")
	if settingExists {
		// Check if schema_version already exists
		var ver string
		err := db.Get(&ver, `SELECT value FROM Setting WHERE key = ?`, wiki.SettingSchemaVersion)
		if err != nil {
			// No schema_version row — this is a legacy database
			isLegacy = true
			legacyVersion, err = detectLegacyVersion(db)
			if err != nil {
				return fmt.Errorf("detect legacy version: %w", err)
			}
			slog.Info("detected legacy database", "version", legacyVersion)
		}
	}

	// Step 2: Execute schema.sql (idempotent CREATE IF NOT EXISTS).
	// For new databases this creates all tables and sets schema_version = latestVersion.
	// For existing databases the CREATEs are no-ops and the INSERT OR IGNORE
	// for schema_version is ignored (row already exists, or we'll overwrite below).
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("execute schema.sql: %w", err)
	}

	// Step 3: If legacy, overwrite schema_version with the detected value.
	if isLegacy {
		if _, err := db.Exec(
			`UPDATE Setting SET value = ? WHERE key = ?`,
			strconv.Itoa(legacyVersion), wiki.SettingSchemaVersion,
		); err != nil {
			return fmt.Errorf("set legacy version: %w", err)
		}
	}

	// Step 4: Read current version and run pending migrations.
	currentVersion, err := getSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		slog.Info("running migration", "version", m.version, "description", m.description)
		if err := m.fn(db, dialect); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.description, err)
		}
		// Update version after each successful migration so partial runs are resumable.
		if _, err := db.Exec(
			`UPDATE Setting SET value = ? WHERE key = ?`,
			strconv.Itoa(m.version), wiki.SettingSchemaVersion,
		); err != nil {
			return fmt.Errorf("update schema version to %d: %w", m.version, err)
		}
	}

	// Always-run fixup: anonymous user (id=0) must have empty role.
	// On new databases the column DEFAULT is 'user', so this corrects it.
	if _, err := db.Exec(`UPDATE User SET role = '' WHERE id = 0`); err != nil {
		return fmt.Errorf("fixup anonymous user role: %w", err)
	}

	return nil
}

// tableExists checks whether a table exists in the database.
func tableExists(db *sqlx.DB, name string) bool {
	var n int
	err := db.Get(&n, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name)
	return err == nil && n > 0
}

// getSchemaVersion reads the current schema_version from the Setting table.
func getSchemaVersion(db *sqlx.DB) (int, error) {
	var ver string
	if err := db.Get(&ver, `SELECT value FROM Setting WHERE key = ?`, wiki.SettingSchemaVersion); err != nil {
		return 0, err
	}
	return strconv.Atoi(ver)
}

// ---------------------------------------------------------------------------
// Individual migration functions
// ---------------------------------------------------------------------------

func migrateAddRenderStatus(db *sqlx.DB, _ Dialect) error {
	_, err := db.Exec(`ALTER TABLE Revision ADD COLUMN render_status TEXT NOT NULL DEFAULT 'rendered'`)
	return err
}

func migrateDropTitle(db *sqlx.DB, _ Dialect) error {
	return recreateTable(db, "Revision",
		`CREATE TABLE Revision_new (
			id INTEGER NOT NULL,
			article_id INT NOT NULL,
			hashval TEXT NOT NULL,
			markdown TEXT NOT NULL,
			html TEXT,
			user_id INTEGER NOT NULL,
			created TIMESTAMP NOT NULL,
			previous_id INT NOT NULL,
			comment TEXT,
			render_status TEXT NOT NULL DEFAULT 'rendered',
			PRIMARY KEY (id, article_id),
			FOREIGN KEY(article_id) REFERENCES Article(id),
			FOREIGN KEY(user_id) REFERENCES User(id)
		)`,
		`INSERT INTO Revision_new (id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status)
		SELECT id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status FROM Revision`,
	)
}

func migrateAddFrontmatter(db *sqlx.DB, _ Dialect) error {
	_, err := db.Exec(`ALTER TABLE Article ADD COLUMN frontmatter BLOB`)
	return err
}

func migrateHTMLNullable(db *sqlx.DB, _ Dialect) error {
	return recreateTable(db, "Revision",
		`CREATE TABLE Revision_new (
			id INTEGER NOT NULL,
			article_id INT NOT NULL,
			hashval TEXT NOT NULL,
			markdown TEXT NOT NULL,
			html TEXT,
			user_id INTEGER NOT NULL,
			created TIMESTAMP NOT NULL,
			previous_id INT NOT NULL,
			comment TEXT,
			render_status TEXT NOT NULL DEFAULT 'rendered',
			PRIMARY KEY (id, article_id),
			FOREIGN KEY(article_id) REFERENCES Article(id),
			FOREIGN KEY(user_id) REFERENCES User(id)
		)`,
		`INSERT INTO Revision_new (id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status)
		SELECT id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status FROM Revision`,
	)
}

func migrateBackfillFrontmatter(db *sqlx.DB, _ Dialect) error {
	return backfillFrontmatter(db)
}

func migrateAddRole(db *sqlx.DB, _ Dialect) error {
	if _, err := db.Exec(`ALTER TABLE User ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`); err != nil {
		return err
	}
	// Promote first registered user to admin
	_, err := db.Exec(`UPDATE User SET role = 'admin' WHERE id = 1`)
	return err
}

func migrateAddCreatedAt(db *sqlx.DB, _ Dialect) error {
	// The column may not exist at all (upgrade from v6) or may be nullable
	// (buggy earlier migration). Use different INSERT SQL for each case.
	var colExists int
	if err := db.Get(&colExists, `SELECT COUNT(*) FROM pragma_table_info('User') WHERE name = 'created_at'`); err != nil {
		return err
	}

	insertSQL := `INSERT INTO User_new (id, email, screenname, role, created_at)
		SELECT id, email, screenname, role, CURRENT_TIMESTAMP FROM User`
	if colExists > 0 {
		insertSQL = `INSERT INTO User_new (id, email, screenname, role, created_at)
		SELECT id, email, screenname, role, COALESCE(created_at, CURRENT_TIMESTAMP) FROM User`
	}

	return recreateTable(db, "User",
		`CREATE TABLE User_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			screenname TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		insertSQL,
	)
}

// backfillFrontmatter populates the frontmatter column for articles where it's NULL.
func backfillFrontmatter(db *sqlx.DB) error {
	type articleData struct {
		URL      string
		Markdown string
	}

	// Collect all articles needing backfill
	rows, err := db.Queryx(`
		SELECT a.url, r.markdown
		FROM Article a
		JOIN Revision r ON a.id = r.article_id
		WHERE a.frontmatter IS NULL
		  AND r.id = (SELECT MAX(r2.id) FROM Revision r2 WHERE r2.article_id = a.id)
	`)
	if err != nil {
		return err
	}

	var articles []articleData
	for rows.Next() {
		var a articleData
		if err := rows.Scan(&a.URL, &a.Markdown); err != nil {
			rows.Close()
			return err
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, a := range articles {
		fm, _ := wiki.ParseFrontmatter(a.Markdown)
		fmJSON, err := json.Marshal(fm)
		if err != nil {
			return err
		}
		_, err = db.Exec(`UPDATE Article SET frontmatter = jsonb(?) WHERE url = ?`, string(fmJSON), a.URL)
		if err != nil {
			return err
		}
	}

	return nil
}
