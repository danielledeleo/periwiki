package storage

import (
	"testing"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// openMemDB opens a fresh in-memory SQLite database.
func openMemDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// requireSchemaVersion reads schema_version from Setting and asserts it equals want.
func requireSchemaVersion(t *testing.T, db *sqlx.DB, want int) {
	t.Helper()
	v, err := getSchemaVersion(db)
	if err != nil {
		t.Fatalf("getSchemaVersion: %v", err)
	}
	if v != want {
		t.Fatalf("schema_version = %d, want %d", v, want)
	}
}

func TestRunMigrations_NewDatabase(t *testing.T) {
	db := openMemDB(t)

	if err := RunMigrations(db, DialectSQLite); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	requireSchemaVersion(t, db, latestVersion)

	// Verify key tables exist
	for _, table := range []string{"Article", "Revision", "User", "Setting", "Password"} {
		if !tableExists(db, table) {
			t.Errorf("expected table %s to exist", table)
		}
	}

	// Verify anonymous user has empty role
	var role string
	if err := db.Get(&role, `SELECT role FROM User WHERE id = 0`); err != nil {
		t.Fatalf("get anonymous user role: %v", err)
	}
	if role != "" {
		t.Errorf("anonymous user role = %q, want empty", role)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openMemDB(t)

	if err := RunMigrations(db, DialectSQLite); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	requireSchemaVersion(t, db, latestVersion)

	// Running again should be a no-op.
	if err := RunMigrations(db, DialectSQLite); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
	requireSchemaVersion(t, db, latestVersion)
}

func TestRunMigrations_LegacyFullyMigrated(t *testing.T) {
	db := openMemDB(t)

	// Simulate a fully-migrated legacy database: run schema.sql to create
	// all tables, then remove the schema_version row so it looks like a
	// pre-version-tracking database.
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("exec schema.sql: %v", err)
	}
	db.Exec(`DELETE FROM Setting WHERE key = ?`, wiki.SettingSchemaVersion)

	// RunMigrations should detect the legacy version and not re-run migrations.
	if err := RunMigrations(db, DialectSQLite); err != nil {
		t.Fatalf("RunMigrations on legacy DB: %v", err)
	}
	requireSchemaVersion(t, db, latestVersion)
}

func TestDetectLegacyVersion(t *testing.T) {
	tests := []struct {
		name    string
		setup   string // SQL to build partial schema
		want    int
	}{
		{
			name: "version 0: no render_status, has title",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE);
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT NOT NULL, title TEXT,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 0,
		},
		{
			name: "version 1: has render_status, has title",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE);
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT NOT NULL, title TEXT,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					render_status TEXT NOT NULL DEFAULT 'rendered',
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 1,
		},
		{
			name: "version 2: has render_status, no title, html NOT NULL",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE);
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT NOT NULL,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					render_status TEXT NOT NULL DEFAULT 'rendered',
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 2,
		},
		{
			name: "version 3: has frontmatter, html NOT NULL",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE, frontmatter BLOB);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE);
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT NOT NULL,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					render_status TEXT NOT NULL DEFAULT 'rendered',
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 3,
		},
		{
			name: "version 5: has frontmatter, html nullable (4+5 co-detected)",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE, frontmatter BLOB);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE);
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					render_status TEXT NOT NULL DEFAULT 'rendered',
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 5,
		},
		{
			name: "version 6: has role, no created_at",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE, frontmatter BLOB);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE, role TEXT NOT NULL DEFAULT 'user');
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					render_status TEXT NOT NULL DEFAULT 'rendered',
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 6,
		},
		{
			name: "version 7: has created_at on User (current schema)",
			setup: `
				CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE, frontmatter BLOB);
				CREATE TABLE User (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, email TEXT NOT NULL UNIQUE, screenname TEXT NOT NULL UNIQUE, role TEXT NOT NULL DEFAULT 'user', created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
				CREATE TABLE Revision (
					id INTEGER NOT NULL, article_id INT NOT NULL, hashval TEXT NOT NULL,
					markdown TEXT NOT NULL, html TEXT,
					user_id INTEGER NOT NULL, created TIMESTAMP NOT NULL, previous_id INT NOT NULL, comment TEXT,
					render_status TEXT NOT NULL DEFAULT 'rendered',
					PRIMARY KEY (id, article_id)
				);
				CREATE TABLE Setting (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
			`,
			want: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openMemDB(t)
			if _, err := db.Exec(tt.setup); err != nil {
				t.Fatalf("setup SQL: %v", err)
			}
			got, err := detectLegacyVersion(db)
			if err != nil {
				t.Fatalf("detectLegacyVersion: %v", err)
			}
			if got != tt.want {
				t.Errorf("detectLegacyVersion = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRecreateTable(t *testing.T) {
	db := openMemDB(t)

	// Create a table with extra column, insert data, then recreate without it.
	_, err := db.Exec(`
		CREATE TABLE TestTable (id INTEGER PRIMARY KEY, name TEXT NOT NULL, extra TEXT);
		INSERT INTO TestTable (id, name, extra) VALUES (1, 'alice', 'drop-me');
		INSERT INTO TestTable (id, name, extra) VALUES (2, 'bob', 'drop-me-too');
	`)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	err = recreateTable(db, "TestTable",
		`CREATE TABLE TestTable_new (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO TestTable_new (id, name) SELECT id, name FROM TestTable`,
	)
	if err != nil {
		t.Fatalf("recreateTable: %v", err)
	}

	// Verify data preserved
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM TestTable`); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("row count = %d, want 2", count)
	}

	// Verify column removed
	var extraExists int
	if err := db.Get(&extraExists, `SELECT COUNT(*) FROM pragma_table_info('TestTable') WHERE name = 'extra'`); err != nil {
		t.Fatalf("pragma check: %v", err)
	}
	if extraExists != 0 {
		t.Error("extra column should not exist after recreateTable")
	}
}

func TestRecreateTable_RestoresFKOnError(t *testing.T) {
	db := openMemDB(t)

	// Enable FK checks first
	db.Exec(`PRAGMA foreign_keys = ON`)

	// Attempt recreateTable with invalid SQL — it should fail but restore FKs.
	err := recreateTable(db, "NoSuchTable",
		`CREATE TABLE NoSuchTable_new (id INTEGER PRIMARY KEY)`,
		`INSERT INTO NoSuchTable_new SELECT * FROM NoSuchTable`, // NoSuchTable doesn't exist
	)
	if err == nil {
		t.Fatal("expected error from recreateTable with nonexistent source table")
	}

	// Verify foreign_keys is back ON. Since recreateTable uses a pinned connection
	// that is closed after returning, we check a new connection from the pool.
	// For a single-connection :memory: db this is the same connection.
	var fk int
	if err := db.Get(&fk, `PRAGMA foreign_keys`); err != nil {
		t.Fatalf("check PRAGMA: %v", err)
	}
	if fk != 1 {
		t.Errorf("PRAGMA foreign_keys = %d after error, want 1", fk)
	}
}

func TestRunMigrations_FromVersion0(t *testing.T) {
	db := openMemDB(t)

	// Build a minimal version-0 database (pre-migration era):
	// Article without frontmatter, Revision with title and html NOT NULL,
	// no render_status, User without role/created_at.
	_, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE Article (id INTEGER PRIMARY KEY, url TEXT NOT NULL UNIQUE);
		CREATE TABLE User (
			id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			screenname TEXT NOT NULL UNIQUE
		);
		CREATE TABLE Revision (
			id INTEGER NOT NULL,
			article_id INT NOT NULL,
			hashval TEXT NOT NULL,
			markdown TEXT NOT NULL,
			html TEXT NOT NULL,
			title TEXT,
			user_id INTEGER NOT NULL,
			created TIMESTAMP NOT NULL,
			previous_id INT NOT NULL,
			comment TEXT,
			PRIMARY KEY (id, article_id),
			FOREIGN KEY(article_id) REFERENCES Article(id),
			FOREIGN KEY(user_id) REFERENCES User(id)
		);
		CREATE TABLE Password (
			user_id INTEGER PRIMARY KEY NOT NULL,
			passwordhash TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES User(id)
		);
		CREATE TABLE AnonymousEdit (
			id INT PRIMARY KEY NOT NULL,
			ip TEXT NOT NULL,
			revision_id INT NOT NULL,
			FOREIGN KEY(revision_id) REFERENCES Revision(id)
		);
		CREATE TABLE Setting (
			key TEXT PRIMARY KEY NOT NULL,
			value TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		INSERT INTO User(id, email, screenname) VALUES (0, '', 'Anonymous');
		INSERT INTO User(email, screenname) VALUES ('admin@test.com', 'admin');

		INSERT INTO Article(id, url) VALUES (1, 'Test_Article');
		INSERT INTO Revision(id, article_id, hashval, markdown, html, title, user_id, created, previous_id)
			VALUES (1, 1, 'abc', '# Test', '<h1>Test</h1>', 'Test', 1, datetime('now'), 0);
	`)
	if err != nil {
		t.Fatalf("setup v0 database: %v", err)
	}

	// Run all migrations from version 0
	if err := RunMigrations(db, DialectSQLite); err != nil {
		t.Fatalf("RunMigrations from v0: %v", err)
	}

	requireSchemaVersion(t, db, latestVersion)

	// Verify migration 1: render_status exists
	var hasRenderStatus int
	db.Get(&hasRenderStatus, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'render_status'`)
	if hasRenderStatus == 0 {
		t.Error("migration 1: render_status column missing")
	}

	// Verify migration 2: title column removed
	var hasTitle int
	db.Get(&hasTitle, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'title'`)
	if hasTitle != 0 {
		t.Error("migration 2: title column still exists")
	}

	// Verify migration 3: frontmatter column exists
	var hasFM int
	db.Get(&hasFM, `SELECT COUNT(*) FROM pragma_table_info('Article') WHERE name = 'frontmatter'`)
	if hasFM == 0 {
		t.Error("migration 3: frontmatter column missing")
	}

	// Verify migration 4: html is nullable
	var htmlNotNull int
	db.Get(&htmlNotNull, `SELECT "notnull" FROM pragma_table_info('Revision') WHERE name = 'html'`)
	if htmlNotNull != 0 {
		t.Error("migration 4: html column is still NOT NULL")
	}

	// Verify migration 6: role column exists
	var hasRole int
	db.Get(&hasRole, `SELECT COUNT(*) FROM pragma_table_info('User') WHERE name = 'role'`)
	if hasRole == 0 {
		t.Error("migration 6: role column missing")
	}

	// Verify first user promoted to admin
	var role string
	db.Get(&role, `SELECT role FROM User WHERE id = 1`)
	if role != "admin" {
		t.Errorf("migration 6: first user role = %q, want 'admin'", role)
	}

	// Verify migration 7: created_at column exists and is NOT NULL
	var createdAtNotNull int
	db.Get(&createdAtNotNull, `SELECT "notnull" FROM pragma_table_info('User') WHERE name = 'created_at'`)
	if createdAtNotNull != 1 {
		t.Error("migration 7: created_at is not NOT NULL")
	}

	// Verify anonymous user fixup
	var anonRole string
	db.Get(&anonRole, `SELECT role FROM User WHERE id = 0`)
	if anonRole != "" {
		t.Errorf("anonymous user role = %q, want empty", anonRole)
	}

	// Verify data preserved
	var count int
	db.Get(&count, `SELECT COUNT(*) FROM Revision`)
	if count != 1 {
		t.Errorf("revision count = %d, want 1", count)
	}
}
