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

	return nil
}
