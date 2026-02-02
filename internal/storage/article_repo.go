package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
)

// Article repository methods for sqliteDb

// articleResult is used for scanning article queries that include user info
type articleResult struct {
	URL        string
	ID         int
	Markdown   string
	HTML       string
	Hash       string    `db:"hashval"`
	Created    time.Time
	PreviousID int       `db:"previous_id"`
	Comment    string
	UserID     int       `db:"user_id"`
	ScreenName string    `db:"screenname"`
}

func (r *articleResult) toArticle() *wiki.Article {
	return &wiki.Article{
		URL: r.URL,
		Revision: &wiki.Revision{
			ID:         r.ID,
			Markdown:   r.Markdown,
			HTML:       r.HTML,
			Hash:       r.Hash,
			Created:    r.Created,
			PreviousID: r.PreviousID,
			Comment:    r.Comment,
			Creator:    &wiki.User{ID: r.UserID, ScreenName: r.ScreenName},
		},
	}
}

func (db *sqliteDb) SelectArticle(url string) (*wiki.Article, error) {
	result := &articleResult{}
	err := db.SelectArticleByLatestRevisionStmt.Get(result, url)
	if err != nil {
		return nil, err
	}
	return result.toArticle(), nil
}

func (db *sqliteDb) SelectArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	result := &articleResult{}
	err := db.SelectArticleByRevisionHashStmt.Get(result, url, hash)
	if err != nil {
		return nil, err
	}
	return result.toArticle(), nil
}

func (db *sqliteDb) SelectArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	result := &articleResult{}
	err := db.SelectArticleByRevisionIDStmt.Get(result, url, id)
	if err != nil {
		return nil, err
	}
	return result.toArticle(), nil
}

func (db *sqliteDb) SelectRevision(hash string) (*wiki.Revision, error) {
	r := &wiki.Revision{}
	x := &struct {
		ID         int
		Markdown   sql.NullString
		HTML       sql.NullString
		Hash       sql.NullString `db:"hashval"`
		PreviousID int            `db:"previous_id"`
		Created    time.Time
	}{}
	err := db.conn.Get(x, "SELECT id, markdown, html, hashval, created, previous_id FROM Revision WHERE hashval = ?", hash)
	if err != nil {
		return nil, err
	}
	return r, err
}

func (db *sqliteDb) SelectRevisionHistory(url string) ([]*wiki.Revision, error) {
	rows, err := db.conn.Queryx(
		`SELECT Revision.id, hashval, created, comment, previous_id, User.screenname, length(markdown)
			FROM Article JOIN Revision ON Article.id = Revision.article_id
					     JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ? ORDER BY created DESC`, url)
	if err != nil {
		return nil, err
	}
	result := struct {
		Hashval, Comment, Screenname string
		ID                           int
		PreviousID                   int `db:"previous_id"`
		Length                       int `db:"length(markdown)"`
		Created                      time.Time
	}{}
	results := make([]*wiki.Revision, 0)
	for rows.Next() {
		rev := &wiki.Revision{Creator: &wiki.User{}}
		err := rows.StructScan(&result)
		if err != nil {
			return nil, err
		}
		rev.Created = result.Created
		rev.Hash = result.Hashval
		rev.ID = result.ID
		rev.PreviousID = result.PreviousID
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

func (db *sqliteDb) SelectRandomArticleURL() (string, error) {
	var url string
	err := db.conn.Get(&url, `SELECT url FROM Article ORDER BY ABS(RANDOM()) LIMIT 1`)
	return url, err
}

func (db *sqliteDb) SelectAllArticles() ([]*wiki.ArticleSummary, error) {
	rows, err := db.conn.Queryx(`
		SELECT a.url, MAX(r.created) as last_modified
		FROM Article a
		JOIN Revision r ON a.id = r.article_id
		GROUP BY a.id
		ORDER BY a.url ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*wiki.ArticleSummary
	for rows.Next() {
		var url, lastModStr string
		if err := rows.Scan(&url, &lastModStr); err != nil {
			return nil, err
		}
		lastMod, err := time.Parse("2006-01-02 15:04:05.000", lastModStr)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &wiki.ArticleSummary{
			URL:          url,
			LastModified: lastMod,
		})
	}
	return articles, rows.Err()
}

func (db *sqliteDb) InsertArticle(article *wiki.Article) (err error) {
	testArticle, insertErr := db.SelectArticle(article.URL)

	var tx *sqlx.Tx
	tx, err = db.conn.Beginx()

	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		return
	}

	defer func() {
		if err != nil {
			slog.Error("article insert failed", "operation", "InsertArticle", "error", err)
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("transaction rollback failed", "error", rbErr)
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				slog.Error("transaction commit failed", "error", commitErr)
			}
		}
	}()

	if insertErr == sql.ErrNoRows { // New article.
		if _, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL); err != nil {
			return
		}

		_, err = tx.Exec(`INSERT INTO Revision (id, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
			VALUES (?, ?, ?, ?, last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.PreviousID+1,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil {
			return
		}

	} else if insertErr == nil && testArticle != nil { // New revision to article

		_, err = tx.Exec(`INSERT INTO Revision (id, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
			VALUES (?, ?, ?, ?, (SELECT Article.id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.PreviousID+1,
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
	}

	// Success!
	return nil
}

func (db *sqliteDb) InsertArticleQueued(article *wiki.Article) (revisionID int64, err error) {
	testArticle, insertErr := db.SelectArticle(article.URL)

	var tx *sqlx.Tx
	tx, err = db.conn.Beginx()

	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		return 0, err
	}

	defer func() {
		if err != nil {
			slog.Error("article insert failed", "operation", "InsertArticleQueued", "error", err)
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("transaction rollback failed", "error", rbErr)
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				slog.Error("transaction commit failed", "error", commitErr)
			}
		}
	}()

	newRevisionID := int64(article.PreviousID + 1)

	if insertErr == sql.ErrNoRows { // New article.
		if _, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL); err != nil {
			return 0, err
		}

		_, err = tx.Exec(`INSERT INTO Revision (id, hashval, markdown, html, article_id, user_id, created, previous_id, comment, render_status)
			VALUES (?, ?, ?, '', last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?, 'queued')`,
			newRevisionID,
			article.Hash,
			article.Markdown,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil {
			return 0, err
		}

	} else if insertErr == nil && testArticle != nil { // New revision to article

		_, err = tx.Exec(`INSERT INTO Revision (id, hashval, markdown, html, article_id, user_id, created, previous_id, comment, render_status)
			VALUES (?, ?, ?, '', (SELECT Article.id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?, 'queued')`,
			newRevisionID,
			article.Hash,
			article.Markdown,
			article.URL,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil {
			if err.Error() == "UNIQUE constraint failed: Revision.id, Revision.article_id" {
				return 0, wiki.ErrRevisionAlreadyExists
			}
			return 0, err
		}
	} else {
		return 0, insertErr
	}

	return newRevisionID, nil
}

func (db *sqliteDb) UpdateRevisionHTML(url string, revisionID int, html string, renderStatus string) error {
	result, err := db.conn.Exec(`
		UPDATE Revision
		SET html = ?, render_status = ?
		WHERE id = ? AND article_id = (SELECT id FROM Article WHERE url = ?)`,
		html, renderStatus, revisionID, url)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return wiki.ErrRevisionNotFound
	}

	return nil
}
