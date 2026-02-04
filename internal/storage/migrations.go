package storage

import (
	"encoding/json"
	"os"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
)

// RunMigrations executes the database schema and any necessary migrations.
// This function is idempotent and safe to run multiple times.
func RunMigrations(db *sqlx.DB) error {
	// Read and execute the main schema
	sqlFile, err := os.ReadFile("internal/storage/schema.sql")
	if err != nil {
		return err
	}

	_, err = db.Exec(string(sqlFile))
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
