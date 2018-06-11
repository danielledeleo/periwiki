package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"time"

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
	conn                              *sqlx.DB
	selectArticleByLatestRevisionStmt *sqlx.Stmt
	selectArticleByRevisionStmt       *sqlx.Stmt
	selectUserScreennameStmt          *sqlx.Stmt
	selectUserScreennameWithHashStmt  *sqlx.Stmt
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
	// q := `select title, markdown, html, hashval, created from Revision where article_id = (select id from Article where url = ?) ORDER BY created DESC LIMIT 1`
	q := `SELECT url, Revision.id, title, markdown, html, hashval, created, previous_hash, comment 
			FROM Article JOIN Revision ON Article.id = Revision.article_id WHERE Article.url = ?`
	db.selectArticleByLatestRevisionStmt, err = db.conn.Preparex(q + ` ORDER BY created DESC LIMIT 1`)
	check(err)

	db.selectArticleByRevisionStmt, err = db.conn.Preparex(q + ` AND Revision.hashval = ?`)
	check(err)

	db.selectUserScreennameStmt, err = db.conn.Preparex(`SELECT id, screenname, email FROM User WHERE screenname = ?`)
	db.selectUserScreennameWithHashStmt, err = db.conn.Preparex(`
		SELECT id, screenname, email, passwordhash FROM User JOIN Password ON Password.user_id = User.id WHERE screenname = ?`)
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
	_, _ = tx.Exec(`INSERT INTO Password(user_id, passwordhash) VALUES (last_insert_rowid(), ?)`, user.PasswordHash)

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
	article.Revision = &model.Revision{}
	err := db.selectArticleByLatestRevisionStmt.Get(article, url)
	if err != nil {
		return nil, err
	}
	return article, err
}

func (db *sqliteDb) SelectArticleByRevision(url string, hash string) (*model.Article, error) {
	article := &model.Article{}
	article.Revision = &model.Revision{}

	err := db.selectArticleByRevisionStmt.Get(article, url, hash)
	if err != nil {
		return nil, err
	}

	return article, err
}

func (db *sqliteDb) SelectRevision(hash string) (*model.Revision, error) {
	r := &model.Revision{}
	x := &struct {
		ID           int
		Title        sql.NullString
		Markdown     sql.NullString
		HTML         sql.NullString
		Hash         sql.NullString `db:"hashval"`
		PreviousHash sql.NullString `db:"previous_hash"`
		Created      time.Time
	}{}
	err := db.conn.Get(x, "SELECT id, title, markdown, html, hashval, created, previous_hash FROM Revision WHERE hashval = ?", hash)
	if err != nil {
		return nil, err
	}
	return r, err
}

func (db *sqliteDb) SelectRevisionHistory(url string) ([]*model.Revision, error) {
	rows, err := db.conn.Queryx(
		`SELECT title, hashval, created, comment, User.screenname, length(markdown)
			FROM Article JOIN Revision ON Article.id = Revision.article_id 
					     JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ? ORDER BY created DESC`, url)
	if err != nil {
		return nil, err
	}
	result := struct {
		Title, Hashval, Comment, Screenname string
		Length                              int `db:"length(markdown)"`
		Created                             time.Time
	}{}
	results := make([]*model.Revision, 0)
	for rows.Next() {
		rev := &model.Revision{Creator: &model.User{}}
		err := rows.StructScan(&result)
		if err != nil {
			return nil, err
		}
		rev.Title = result.Title
		rev.Created = result.Created
		rev.Hash = result.Hashval
		rev.Comment = result.Comment
		rev.Markdown = fmt.Sprint(result.Length) // dirty hack
		rev.Creator.ScreenName = result.Screenname
		results = append(results, rev)
	}
	if len(results) < 1 {
		return nil, model.ErrGenericNotFound
	}
	return results, nil
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

func (db *sqliteDb) InsertArticle(article *model.Article) error {
	testArticle, err := db.SelectArticle(article.URL)
	if err == sql.ErrNoRows { // New article.
		tx, err := db.conn.Beginx()
		if err != nil {
			return err
		}
		tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL)
		tx.Exec(`INSERT INTO Revision (title, hashval, markdown, html, article_id, user_id, created, previous_hash, comment)
			VALUES (?, ?, ?, ?, last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.Creator.ID,
			article.PreviousHash,
			article.Comment)

		err = tx.Commit()
		if err != nil {
			return err
		}

	} else if err == nil && testArticle != nil { // New revision to article
		tx, err := db.conn.Beginx()
		if err != nil {
			return err
		}
		tx.Exec(`INSERT INTO Revision (title, hashval, markdown, html, article_id, user_id, created, previous_hash, comment)
			VALUES (?, ?, ?, ?, (SELECT id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.URL,
			article.Creator.ID,
			article.PreviousHash,
			article.Comment)

		err = tx.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}
