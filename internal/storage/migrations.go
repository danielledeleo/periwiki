package storage

import (
	"os"

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
				html TEXT NOT NULL,
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

	return nil
}
