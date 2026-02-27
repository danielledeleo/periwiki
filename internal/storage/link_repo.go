package storage

import (
	"time"

	"github.com/danielledeleo/periwiki/wiki"
)

// Link repository methods for sqliteDb

func (db *sqliteDb) ReplaceArticleLinks(sourceURL string, targetSlugs []string) error {
	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM ArticleLink WHERE source_url = ?`, sourceURL); err != nil {
		return err
	}

	for _, slug := range targetSlugs {
		if _, err = tx.Exec(`INSERT INTO ArticleLink (source_url, target_slug) VALUES (?, ?)`, sourceURL, slug); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *sqliteDb) SelectBacklinks(targetSlug string) ([]*wiki.ArticleSummary, error) {
	rows, err := db.conn.Queryx(`
		SELECT a.url, MAX(r.created) AS last_modified,
		       COALESCE(json_extract(a.frontmatter, '$.display_title'), '') AS title
		FROM ArticleLink al
		JOIN Article a ON al.source_url = a.url
		JOIN Revision r ON a.id = r.article_id
		WHERE al.target_slug = ?
		GROUP BY a.url
		ORDER BY a.url ASC
	`, targetSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*wiki.ArticleSummary
	for rows.Next() {
		var url, lastModStr, title string
		if err := rows.Scan(&url, &lastModStr, &title); err != nil {
			return nil, err
		}
		lastMod, err := time.Parse("2006-01-02 15:04:05.000", lastModStr)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &wiki.ArticleSummary{
			URL:          url,
			LastModified: lastMod,
			Title:        title,
		})
	}
	return articles, rows.Err()
}

func (db *sqliteDb) CountLinks() (int, error) {
	var count int
	err := db.conn.Get(&count, `SELECT COUNT(*) FROM ArticleLink`)
	return count, err
}
