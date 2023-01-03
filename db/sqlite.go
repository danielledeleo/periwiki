package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
	"github.com/michaeljs1990/sqlitestore"
	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

type sqliteDb struct {
	*sqlitestore.SqliteStore
	conn                              *sqlx.DB
	selectArticleByLatestRevisionStmt *sqlx.Stmt
	selectArticleByRevisionHashStmt   *sqlx.Stmt
	selectArticleByRevisionIDStmt     *sqlx.Stmt
	selectUserScreennameStmt          *sqlx.Stmt
	selectUserScreennameWithHashStmt  *sqlx.Stmt
}

func Init(config *wiki.Config) (*sqliteDb, error) {
	conn, err := sqlx.Open("sqlite3", config.DatabaseFile)

	if err != nil {
		return nil, err
	}

	sqlFile, err := ioutil.ReadFile("db/schema.sql")
	if err != nil {
		return nil, err
	}

	sqlStmt := string(sqlFile)
	_, err = conn.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}

	db := &sqliteDb{conn: conn}
	db.SqliteStore, err = sqlitestore.NewSqliteStoreFromConnection(conn, "sessions", "/", config.CookieExpiry, config.CookieSecret)
	if err != nil {
		return nil, err
	}

	// Add prepared statements
	q := `SELECT url, Revision.id, title, markdown, html, hashval, created, previous_id, comment 
			FROM Article JOIN Revision ON Article.id = Revision.article_id WHERE Article.url = ?`
	db.selectArticleByLatestRevisionStmt, err = db.conn.Preparex(q + ` ORDER BY created DESC LIMIT 1`)
	if err != nil {
		return nil, err
	}

	db.selectArticleByRevisionHashStmt, err = db.conn.Preparex(q + ` AND Revision.hashval = ?`)
	if err != nil {
		return nil, err
	}

	db.selectArticleByRevisionIDStmt, err = db.conn.Preparex(q + ` AND Revision.id = ?`)
	if err != nil {
		return nil, err
	}

	db.selectUserScreennameStmt, err = db.conn.Preparex(`SELECT id, screenname, email FROM User WHERE screenname = ?`)
	if err != nil {
		return nil, err
	}

	db.selectUserScreennameWithHashStmt, err = db.conn.Preparex(`
		SELECT id, screenname, email, passwordhash FROM User JOIN Password ON Password.user_id = User.id WHERE screenname = ?`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *sqliteDb) InsertUser(user *wiki.User) (err error) {
	var tx *sqlx.Tx
	tx, err = db.conn.Beginx()

	defer func() {
		if err != nil {
			log.Println(err)
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Println(rbErr)
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				log.Println(commitErr)
			}
		}
	}()

	_, err = tx.Exec(`INSERT INTO User(screenname, email) VALUES (?, ?)`, user.ScreenName, user.Email)

	if err != nil {
		if err.Error() == "UNIQUE constraint failed: User.screenname" {
			return wiki.ErrUsernameTaken
		} else if err.Error() == "UNIQUE constraint failed: User.email" {
			return wiki.ErrEmailTaken
		}
		return
	}
	if _, err = tx.Exec(`INSERT INTO Password(user_id, passwordhash) VALUES (last_insert_rowid(), ?)`, user.PasswordHash); err != nil {
		return
	}

	return nil
}

func (db *sqliteDb) InsertPreference(pref *wiki.Preference) error {
	_, err := db.conn.Exec(`INSERT OR REPLACE INTO Preference (pref_label, pref_type, help_text, pref_int, pref_text, pref_selection) 
		VALUES(?, ?, ?, ?, ?, ?)`,
		pref.Label,
		pref.Type,
		pref.HelpText,
		pref.IntValue,
		pref.TextValue,
		pref.SelectionValue,
	)
	return err
}

func (db *sqliteDb) SelectPreference(key string) (*wiki.Preference, error) {
	pref := &wiki.Preference{}
	err := db.conn.Select(`SELECT * FROM Preference WHERE pref_label = ?`, key)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrGenericNotFound
	} else if err != nil {
		return nil, err
	}
	return pref, err
}

func (db *sqliteDb) SelectArticle(url string) (*wiki.Article, error) {
	article := &wiki.Article{}
	article.Revision = &wiki.Revision{}
	err := db.selectArticleByLatestRevisionStmt.Get(article, url)
	if err != nil {
		return nil, err
	}
	return article, err
}

func (db *sqliteDb) SelectArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	article := &wiki.Article{}
	article.Revision = &wiki.Revision{}

	err := db.selectArticleByRevisionHashStmt.Get(article, url, hash)
	if err != nil {
		return nil, err
	}

	return article, err
}

func (db *sqliteDb) SelectArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	article := &wiki.Article{}
	article.Revision = &wiki.Revision{}

	err := db.selectArticleByRevisionIDStmt.Get(article, url, id)
	if err != nil {
		return nil, err
	}

	return article, err
}

func (db *sqliteDb) SelectRevision(hash string) (*wiki.Revision, error) {
	r := &wiki.Revision{}
	x := &struct {
		ID         int
		Title      sql.NullString
		Markdown   sql.NullString
		HTML       sql.NullString
		Hash       sql.NullString `db:"hashval"`
		PreviousID int            `db:"previous_id"`
		Created    time.Time
	}{}
	err := db.conn.Get(x, "SELECT id, title, markdown, html, hashval, created, previous_id FROM Revision WHERE hashval = ?", hash)
	if err != nil {
		return nil, err
	}
	return r, err
}

func (db *sqliteDb) SelectRevisionHistory(url string) ([]*wiki.Revision, error) {
	rows, err := db.conn.Queryx(
		`SELECT Revision.id, title, hashval, created, comment, User.screenname, length(markdown)
			FROM Article JOIN Revision ON Article.id = Revision.article_id 
					     JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ? ORDER BY created DESC`, url)
	if err != nil {
		return nil, err
	}
	result := struct {
		Title, Hashval, Comment, Screenname string
		ID                                  int
		Length                              int `db:"length(markdown)"`
		Created                             time.Time
	}{}
	results := make([]*wiki.Revision, 0)
	for rows.Next() {
		rev := &wiki.Revision{Creator: &wiki.User{}}
		err := rows.StructScan(&result)
		if err != nil {
			return nil, err
		}
		rev.Title = result.Title
		rev.Created = result.Created
		rev.Hash = result.Hashval
		rev.ID = result.ID
		rev.Comment = result.Comment
		rev.Markdown = fmt.Sprint(result.Length) // dirty hack
		rev.Creator.ScreenName = result.Screenname
		results = append(results, rev)
	}
	if len(results) < 1 {
		return nil, wiki.ErrGenericNotFound
	}
	return results, nil
}

func (db *sqliteDb) SelectUserByScreenname(screenname string, withHash bool) (*wiki.User, error) {
	user := &wiki.User{}

	var err error
	if withHash {
		err = db.selectUserScreennameWithHashStmt.Get(user, screenname)
	} else {
		err = db.selectUserScreennameStmt.Get(user, screenname)
	}

	return user, err
}

func (db *sqliteDb) InsertArticle(article *wiki.Article) (err error) {
	testArticle, insertErr := db.SelectArticle(article.URL)

	var tx *sqlx.Tx
	tx, err = db.conn.Beginx()

	if err != nil {
		log.Println(err)
		return
	}

	defer func() {
		if err != nil {
			log.Println(errors.Wrap(err, "failed to InsertArticle"))
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Println(rbErr)
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				log.Println(commitErr)
			}
		}
	}()

	if insertErr == sql.ErrNoRows { // New article.
		if _, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL); err != nil {
			return
		}

		_, err = tx.Exec(`INSERT INTO Revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
			VALUES (?, ?, ?, ?, ?, last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.PreviousID+1,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil {
			return
		}

		// if article.Creator.ID == 0 { // Anonymous User
		// 	if _, err = tx.Exec(`INSERT INTO AnonymousEdit (ip, revision_id) VALUES (?, last_insert_rowid())`,
		// 		article.Creator.IPAddress); err != nil {
		// 		return
		// 	}
		// }

	} else if insertErr == nil && testArticle != nil { // New revision to article

		_, err = tx.Exec(`INSERT INTO Revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
			VALUES (?, ?, ?, ?, ?, (SELECT Article.id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.PreviousID+1,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.URL,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil {
			if err.Error() == "UNIQUE constraint failed: Revision.id, Revision.article_id" {
				return wiki.ErrRevisionAlreadyExists
			}
		}

		// if article.Creator.ID == 0 { // Anonymous
		// 	_, err = tx.Exec(`INSERT INTO AnonymousEdit (ip, revision_id) VALUES (?, last_insert_rowid())`,
		// 		article.Creator.IPAddress)
		// 	if err != nil {
		// 		return
		// 	}
		// }
	}

	// Success!
	return nil
}
