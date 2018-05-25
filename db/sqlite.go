package db

import (
	"database/sql"
	"io/ioutil"
	"log"

	"github.com/jagger27/iwikii/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/michaeljs1990/sqlitestore"
)

type SqliteConfig struct {
	DatabaseFile    string
	CookieSecretKey string
}

type sqliteDb struct {
	*sqlitestore.SqliteStore
	conn                             *sqlx.DB
	selectArticleStmt                *sqlx.Stmt
	selectRevisionStmt               *sqlx.Stmt
	selectUserScreennameStmt         *sqlx.Stmt
	selectUserScreennameWithHashStmt *sqlx.Stmt
}

func Init(config SqliteConfig) (*sqliteDb, error) {
	conn, err := sqlx.Open("sqlite3", config.DatabaseFile)

	if err != nil {
		log.Fatal(err)
	}
	// defer conn.Close()

	sqlFile, err := ioutil.ReadFile("db/schema.sql")
	if err != nil {
		log.Fatal(err)
	}

	sqlStmt := string(sqlFile)
	_, err = conn.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, sqlStmt)
	}

	db := &sqliteDb{conn: conn}

	db.SqliteStore, err = sqlitestore.NewSqliteStoreFromConnection(conn, "sessions", "/", 86400, []byte(config.CookieSecretKey))
	check(err)
	// timenowquery := `strftime("%Y-%m-%d %H:%M:%f", "now")`

	// Add prepared statements
	db.selectArticleStmt, err = db.conn.Preparex("SELECT url FROM Article WHERE url = ?")
	check(err)

	q := `select title, markdown, html, hashval, created from Revision where article_id = (select id from Article where url = ?) ORDER BY created DESC LIMIT 1`

	db.selectRevisionStmt, err = db.conn.Preparex(q)
	check(err)

	db.selectUserScreennameStmt, err = db.conn.Preparex(`SELECT id, screenname, email FROM User WHERE screenname = ?`)
	db.selectUserScreennameWithHashStmt, err = db.conn.Preparex(`
		SELECT id, screenname, email, passwordhash FROM User JOIN Passwords ON Passwords.user_id = User.id WHERE screenname = ?`)
	check(err)

	return db, err
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (db *sqliteDb) InsertUser(user *model.User) error {
	tx, err := db.conn.Beginx()
	_, userErr := tx.Exec(`INSERT INTO User(screenname, email) VALUES (?, ?)`, user.ScreenName, user.Email)
	_, _ = tx.Exec(`INSERT INTO Passwords(user_id, passwordhash) VALUES (last_insert_rowid(), ?)`, user.PasswordHash)

	err = tx.Commit()
	if err != nil {
		return err
	}
	if userErr != nil {
		if userErr.Error() == "UNIQUE constraint failed: User.screenname" {
			return model.ErrUsernameTaken
		} else if userErr.Error() == "UNIQUE constraint failed: User.email" {
			return model.ErrEmailTaken
		}
		return userErr
	}
	return nil
}

func (db *sqliteDb) SelectArticle(url string) (*model.Article, error) {
	article := &model.Article{}
	err := db.selectArticleStmt.Get(article, url)
	if err != nil {
		return nil, err
	}
	article.Revision = &model.Revision{}
	err = db.selectRevisionStmt.Get(article.Revision, url)
	if err != nil {
		return nil, err
	}
	return article, err
}

func (db *sqliteDb) SelectUserByScreenname(screenname string, withHash bool) (*model.User, error) {
	user := &model.User{}

	var err error
	if withHash {
		err = db.selectUserScreennameWithHashStmt.Get(user, screenname)
	} else {
		err = db.selectUserScreennameStmt.Get(user, screenname)
	}

	return user, err
}

func (db *sqliteDb) SelectLatestRevision(url string) {

}

func (db *sqliteDb) InsertArticle(article *model.Article) error {
	testArticle, err := db.SelectArticle(article.URL)
	if err == sql.ErrNoRows { // New article.
		tx, err := db.conn.Beginx()
		check(err)
		_, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL)
		check(err)
		_, err = tx.Exec(`INSERT INTO Revision (title, hashval, markdown, html, article_id, user_id, created)
			VALUES (?, ?, ?, ?, last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"))`,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.Creator.ID,
		)
		check(err)

		err = tx.Commit()
		check(err)

	} else if err == nil && testArticle != nil { // New revision to article
		tx, err := db.conn.Beginx()
		check(err)
		_, err = tx.Exec(`INSERT INTO Revision (title, hashval, markdown, html, article_id, user_id, created)
			VALUES (?, ?, ?, ?, (SELECT id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"))`,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.URL,
			article.Creator.ID,
		)
		check(err)

		err = tx.Commit()
		check(err)
	}
	return nil
}
