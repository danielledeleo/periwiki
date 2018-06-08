package db

import (
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/jagger27/iwikii/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/gorilla/sessions"
)

type PgConfig struct {
	DatabaseFile    string
	CookieSecretKey string
}

type pgDb struct {
	*sessions.CookieStore
	conn                              *sqlx.DB
	selectArticleByLatestRevisionStmt *sqlx.Stmt
	selectArticleByRevisionStmt       *sqlx.Stmt
	selectUserScreennameStmt          *sqlx.Stmt
	selectUserScreennameWithHashStmt  *sqlx.Stmt
}

func (db *pgDb) Delete(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error {
	return nil
}

func InitPg(config PgConfig) (*pgDb, error) {
	conn, err := sqlx.Open("postgres", "user=jagger dbname=iwikii sslmode=disable")
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

	db := &pgDb{conn: conn}
	db.CookieStore = sessions.NewCookieStore([]byte(config.CookieSecretKey))
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

func (db *pgDb) InsertUser(user *model.User) error {
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

func (db *pgDb) SelectArticle(url string) (*model.Article, error) {
	article := &model.Article{}
	article.Revision = &model.Revision{}
	err := db.selectArticleByLatestRevisionStmt.Get(article, url)
	if err != nil {
		return nil, err
	}
	return article, err
}

func (db *pgDb) SelectArticleByRevision(url string, hash string) (*model.Article, error) {
	article := &model.Article{}
	article.Revision = &model.Revision{}

	err := db.selectArticleByRevisionStmt.Get(article, url, hash)
	if err != nil {
		return nil, err
	}

	return article, err
}

func (db *pgDb) SelectRevision(hash string) (*model.Revision, error) {
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
	log.Println(x)
	return r, err
}

func (db *pgDb) SelectRevisionHistory(url string) ([]*model.Revision, error) {
	rows, err := db.conn.Queryx(
		`SELECT title, hashval, created, comment, User.screenname
			FROM Article JOIN Revision ON Article.id = Revision.article_id 
					     JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ? ORDER BY created DESC`, url)
	if err != nil {
		log.Panic(err)
	}
	result := struct {
		Title, Hashval, Comment, Screenname string
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
		rev.Creator.ScreenName = result.Screenname
		results = append(results, rev)
	}
	return results, nil
}

func (db *pgDb) SelectUserByScreenname(screenname string, withHash bool) (*model.User, error) {
	user := &model.User{}

	var err error
	if withHash {
		err = db.selectUserScreennameWithHashStmt.Get(user, screenname)
	} else {
		err = db.selectUserScreennameStmt.Get(user, screenname)
	}

	return user, err
}

func (db *pgDb) InsertArticle(article *model.Article) error {
	testArticle, err := db.SelectArticle(article.URL)
	if err == sql.ErrNoRows { // New article.
		tx, err := db.conn.Beginx()
		check(err)
		_, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL)
		check(err)
		_, err = tx.Exec(`INSERT INTO Revision (title, hashval, markdown, html, article_id, user_id, created, previous_hash, comment)
			VALUES (?, ?, ?, ?, last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)  ORDER BY created DESC LIMIT 100`,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.Creator.ID,
			article.PreviousHash,
			article.Comment)
		check(err)

		err = tx.Commit()
		check(err)

	} else if err == nil && testArticle != nil { // New revision to article
		tx, err := db.conn.Beginx()
		check(err)
		_, err = tx.Exec(`INSERT INTO Revision (title, hashval, markdown, html, article_id, user_id, created, previous_hash, comment)
			VALUES (?, ?, ?, ?, (SELECT id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.URL,
			article.Creator.ID,
			article.PreviousHash,
			article.Comment)
		check(err)

		err = tx.Commit()
		check(err)
	}
	return nil
}
