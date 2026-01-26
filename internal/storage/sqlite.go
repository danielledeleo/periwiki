package storage

import (
	"os"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
	"github.com/michaeljs1990/sqlitestore"
	_ "modernc.org/sqlite"
)

// PreparedStatements holds the prepared SQL statements used for database queries.
// This struct is exported to allow reuse in test utilities.
type PreparedStatements struct {
	SelectArticleByLatestRevisionStmt *sqlx.Stmt
	SelectArticleByRevisionHashStmt   *sqlx.Stmt
	SelectArticleByRevisionIDStmt     *sqlx.Stmt
	SelectUserScreennameStmt          *sqlx.Stmt
	SelectUserScreennameWithHashStmt  *sqlx.Stmt
}

// InitializeStatements prepares all the SQL statements needed for database operations.
// This function is exported to allow reuse in test utilities.
func InitializeStatements(conn *sqlx.DB) (*PreparedStatements, error) {
	stmts := &PreparedStatements{}
	var err error

	q := `SELECT url, Revision.id, title, markdown, html, hashval, created, previous_id, comment, User.id AS user_id, User.screenname
			FROM Article 
			JOIN Revision ON Article.id = Revision.article_id 
			JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ?`

	stmts.SelectArticleByLatestRevisionStmt, err = conn.Preparex(q + ` ORDER BY created DESC LIMIT 1`)
	if err != nil {
		return nil, err
	}

	stmts.SelectArticleByRevisionHashStmt, err = conn.Preparex(q + ` AND Revision.hashval = ?`)
	if err != nil {
		return nil, err
	}

	stmts.SelectArticleByRevisionIDStmt, err = conn.Preparex(q + ` AND Revision.id = ?`)
	if err != nil {
		return nil, err
	}

	stmts.SelectUserScreennameStmt, err = conn.Preparex(`SELECT id, screenname, email FROM User WHERE screenname = ?`)
	if err != nil {
		return nil, err
	}

	stmts.SelectUserScreennameWithHashStmt, err = conn.Preparex(`
		SELECT id, screenname, email, passwordhash FROM User JOIN Password ON Password.user_id = User.id WHERE screenname = ?`)
	if err != nil {
		return nil, err
	}

	return stmts, nil
}

// sqliteDb is the main database struct that embeds all repository functionality.
// Methods are defined in separate files:
//   - article_repo.go: Article and Revision operations
//   - user_repo.go: User operations
//   - preference_repo.go: Preference operations
//
// Session operations are handled by the embedded SqliteStore.
type sqliteDb struct {
	*sqlitestore.SqliteStore
	*PreparedStatements
	conn *sqlx.DB
}

func Init(config *wiki.Config) (*sqliteDb, error) {
	conn, err := sqlx.Open("sqlite3", config.DatabaseFile)

	if err != nil {
		return nil, err
	}

	sqlFile, err := os.ReadFile("internal/storage/schema.sql")
	if err != nil {
		return nil, err
	}

	sqlStmt := string(sqlFile)
	_, err = conn.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Migration: Add render_status column to Revision table if it doesn't exist.
	// This is idempotent - safe to run multiple times on the same database.
	var colExists int
	err = conn.Get(&colExists, `SELECT COUNT(*) FROM pragma_table_info('Revision') WHERE name = 'render_status'`)
	if err != nil {
		return nil, err
	}
	if colExists == 0 {
		_, err = conn.Exec(`ALTER TABLE Revision ADD COLUMN render_status TEXT NOT NULL DEFAULT 'rendered'`)
		if err != nil {
			return nil, err
		}
	}

	db := &sqliteDb{conn: conn}
	db.SqliteStore, err = sqlitestore.NewSqliteStoreFromConnection(conn, "sessions", "/", config.CookieExpiry, config.CookieSecret)
	if err != nil {
		return nil, err
	}

	// Initialize prepared statements using shared function
	db.PreparedStatements, err = InitializeStatements(conn)
	if err != nil {
		return nil, err
	}

	return db, nil
}
