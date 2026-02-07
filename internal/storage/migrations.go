package storage

import (
	_ "embed"
	"encoding/json"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
)

//go:embed schema.sql
var schemaSQL string

// RunMigrations executes the database schema and any necessary migrations.
// This function is idempotent and safe to run multiple times.
func RunMigrations(db *sqlx.DB) error {
	// Execute the embedded schema
	_, err := db.Exec(schemaSQL)
	if err != nil {
		return err
	}

	// Migration: Add render_status column to Revision table if it doesn't exist.
	// This is idempotent - safe to run multiple times on the same database.
	var colExists int
	err = db.Get(&colExists, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'render_status'`)
	if err != nil {
		return err
	}
	if colExists == 0 {
		_, err = db.Exec(`ALTER TABLE Revision ADD COLUMN render_status TEXT NOT NULL DEFAULT 'rendered'`)
		if err != nil {
			return err
		}
	}

	// Migration: Drop title column from Revision table (titles now come from frontmatter).
	// SQLite doesn't support DROP COLUMN directly, so we recreate the table.
	var titleColExists int
	err = db.Get(&titleColExists, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'title'`)
	if err != nil {
		return err
	}
	if titleColExists == 1 {
		_, err = db.Exec(`
			-- Create new table without title column
			CREATE TABLE Revision_new (
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
			);
			-- Copy data from old table (excluding title)
			INSERT INTO Revision_new (id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status)
			SELECT id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status FROM Revision;
			-- Drop old table and rename new one
			DROP TABLE Revision;
			ALTER TABLE Revision_new RENAME TO Revision;
		`)
		if err != nil {
			return err
		}
	}

	// Migration: Add frontmatter JSONB column to Article table.
	var fmColExists int
	err = db.Get(&fmColExists, `SELECT COUNT(*) FROM pragma_table_info('Article') WHERE name = 'frontmatter'`)
	if err != nil {
		return err
	}
	if fmColExists == 0 {
		_, err = db.Exec(`ALTER TABLE Article ADD COLUMN frontmatter BLOB`)
		if err != nil {
			return err
		}
	}

	// Migration: Make html column nullable in Revision table.
	// This allows old revisions to have their HTML cleared (set to NULL) when the
	// render pipeline changes, reclaiming storage. They re-render lazily on access.
	var htmlNotNull int
	err = db.Get(&htmlNotNull, `SELECT "notnull" FROM pragma_table_info('Revision') WHERE name = 'html'`)
	if err != nil {
		return err
	}
	if htmlNotNull == 1 {
		_, err = db.Exec(`
			CREATE TABLE Revision_new (
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
			);
			INSERT INTO Revision_new (id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status)
			SELECT id, article_id, hashval, markdown, html, user_id, created, previous_id, comment, render_status FROM Revision;
			DROP TABLE Revision;
			ALTER TABLE Revision_new RENAME TO Revision;
		`)
		if err != nil {
			return err
		}
	}

	// Backfill any articles with NULL frontmatter.
	// Collect data first, then close rows before updating to avoid SQLite locking.
	if err := backfillFrontmatter(db); err != nil {
		return err
	}

	// Migration: Add role column to User table.
	var roleColExists int
	err = db.Get(&roleColExists, `SELECT COUNT(*) FROM pragma_table_info('User') WHERE name = 'role'`)
	if err != nil {
		return err
	}
	if roleColExists == 0 {
		_, err = db.Exec(`ALTER TABLE User ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`)
		if err != nil {
			return err
		}
		// Promote first registered user to admin
		_, err = db.Exec(`UPDATE User SET role = 'admin' WHERE id = 1`)
		if err != nil {
			return err
		}
	}

	// Always ensure anonymous user has empty role. On new databases the column
	// DEFAULT is 'user', so this corrects it. On existing databases it's a no-op.
	_, err = db.Exec(`UPDATE User SET role = '' WHERE id = 0`)
	if err != nil {
		return err
	}

	// Migration: Add created_at column to User table, or fix it if it was
	// added as nullable by an earlier buggy migration.
	var createdAtNotNull int
	err = db.Get(&createdAtNotNull, `SELECT COALESCE(
		(SELECT "notnull" FROM pragma_table_info('User') WHERE name = 'created_at'),
		-1)`)
	if err != nil {
		return err
	}
	if createdAtNotNull < 1 {
		_, err = db.Exec(`PRAGMA foreign_keys = OFF`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`
			CREATE TABLE User_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
				email TEXT NOT NULL UNIQUE,
				screenname TEXT NOT NULL UNIQUE,
				role TEXT NOT NULL DEFAULT 'user',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			INSERT INTO User_new (id, email, screenname, role, created_at)
				SELECT id, email, screenname, role, COALESCE(created_at, CURRENT_TIMESTAMP) FROM User;
			DROP TABLE User;
			ALTER TABLE User_new RENAME TO User;
		`)
		if err != nil {
			db.Exec(`PRAGMA foreign_keys = ON`)
			return err
		}
		_, err = db.Exec(`PRAGMA foreign_keys = ON`)
		if err != nil {
			return err
		}
	}

	return nil
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
	rows.Close() // Close before updating

	// Now update each article
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
