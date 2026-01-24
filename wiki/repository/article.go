package repository

import "github.com/danielledeleo/periwiki/wiki"

// ArticleRepository defines the interface for article persistence operations.
type ArticleRepository interface {
	// SelectArticle retrieves an article by its URL.
	SelectArticle(url string) (*wiki.Article, error)

	// SelectArticleByRevisionHash retrieves an article by URL and revision hash.
	SelectArticleByRevisionHash(url string, hash string) (*wiki.Article, error)

	// SelectArticleByRevisionID retrieves an article by URL and revision ID.
	SelectArticleByRevisionID(url string, id int) (*wiki.Article, error)

	// SelectRevision retrieves a revision by its hash.
	SelectRevision(hash string) (*wiki.Revision, error)

	// SelectRevisionHistory retrieves all revisions for an article.
	SelectRevisionHistory(url string) ([]*wiki.Revision, error)

	// InsertArticle inserts a new article revision.
	InsertArticle(article *wiki.Article) error

	// SelectRandomArticleURL returns a random article URL.
	SelectRandomArticleURL() (string, error)

	// SelectAllArticles retrieves all articles with their last modified time.
	SelectAllArticles() ([]*wiki.ArticleSummary, error)
}
