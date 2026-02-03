package storage

import (
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

	q := `SELECT url, Revision.id, markdown, html, hashval, created, previous_id, comment, User.id AS user_id, User.screenname
			FROM Article
			JOIN Revision ON Article.id = Revision.article_id
			JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ?`

	stmts.SelectArticleByLatestRevisionStmt, err = conn.Preparex(q + ` ORDER BY Revision.id DESC LIMIT 1`)
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

// Init initializes the storage layer with an existing database connection and runtime config.
// The database connection should already have migrations applied via RunMigrations.
func Init(db *sqlx.DB, runtimeConfig *wiki.RuntimeConfig) (*sqliteDb, error) {
	var err error

	store := &sqliteDb{conn: db}
	store.SqliteStore, err = sqlitestore.NewSqliteStoreFromConnection(db, "sessions", "/", runtimeConfig.CookieExpiry, runtimeConfig.CookieSecret)
	if err != nil {
		return nil, err
	}

	// Initialize prepared statements using shared function
	store.PreparedStatements, err = InitializeStatements(db)
	if err != nil {
		return nil, err
	}

	return store, nil
}
