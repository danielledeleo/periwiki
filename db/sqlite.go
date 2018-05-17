package db

import (
	"io/ioutil"
	"log"

	"github.com/jagger27/iwikii/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type SqliteConfig struct {
	DatabaseFile string
}

type sqliteDb struct {
	conn          *sqlx.DB
	selectArticle *sqlx.Stmt
}

func Init(config SqliteConfig) (*sqliteDb, error) {
	conn, err := sqlx.Open("sqlite3", config.DatabaseFile)

	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	sqlFile, err := ioutil.ReadFile("db/schema.sql")
	if err != nil {
		log.Fatal(err)
	}

	sqlStmt := string(sqlFile)
	_, err = conn.Exec(sqlStmt)
	if err != nil {
		log.Fatal("%q: %s\n", err, sqlStmt)
	}

	db := &sqliteDb{conn: conn}

	// Add prepared statements
	db.selectArticle, err = db.conn.Preparex("SELECT url, title FROM Article WHERE url = $1")
	if err != nil {
		log.Fatal(err)
	}

	return db, err
}

func (db *sqliteDb) GetArticle(url string) (*model.Article, error) {
	article := &model.Article{}
	// db.selectArticle.Get()

	return article, nil
}
