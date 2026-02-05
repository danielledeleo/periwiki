package service

import (
	"context"

	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/wiki"
)

// embeddedArticleService wraps an ArticleService to intercept embedded articles.
type embeddedArticleService struct {
	base     ArticleService
	embedded *embedded.EmbeddedArticles
}

// NewEmbeddedArticleService creates a decorator that serves embedded articles
// for Periwiki:* URLs while delegating other requests to the base service.
func NewEmbeddedArticleService(base ArticleService, ea *embedded.EmbeddedArticles) ArticleService {
	return &embeddedArticleService{
		base:     base,
		embedded: ea,
	}
}

func (s *embeddedArticleService) GetArticle(url string) (*wiki.Article, error) {
	if article := s.embedded.Get(url); article != nil {
		return article, nil
	}
	return s.base.GetArticle(url)
}

func (s *embeddedArticleService) PostArticle(article *wiki.Article) error {
	if embedded.IsEmbeddedURL(article.URL) {
		return wiki.ErrReadOnlyArticle
	}
	return s.base.PostArticle(article)
}

func (s *embeddedArticleService) PostArticleWithContext(ctx context.Context, article *wiki.Article) error {
	if embedded.IsEmbeddedURL(article.URL) {
		return wiki.ErrReadOnlyArticle
	}
	return s.base.PostArticleWithContext(ctx, article)
}

func (s *embeddedArticleService) Preview(markdown string) (string, error) {
	return s.base.Preview(markdown)
}

func (s *embeddedArticleService) GetArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, wiki.ErrRevisionNotFound
	}
	return s.base.GetArticleByRevisionID(url, id)
}

func (s *embeddedArticleService) GetArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, wiki.ErrRevisionNotFound
	}
	return s.base.GetArticleByRevisionHash(url, hash)
}

func (s *embeddedArticleService) GetRevisionHistory(url string) ([]*wiki.Revision, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, nil // Empty history for embedded articles
	}
	return s.base.GetRevisionHistory(url)
}

func (s *embeddedArticleService) GetRandomArticleURL() (string, error) {
	return s.base.GetRandomArticleURL()
}

func (s *embeddedArticleService) GetAllArticles() ([]*wiki.ArticleSummary, error) {
	return s.base.GetAllArticles()
}

func (s *embeddedArticleService) RerenderRevision(ctx context.Context, url string, revisionID int) error {
	if embedded.IsEmbeddedURL(url) {
		return wiki.ErrReadOnlyArticle
	}
	return s.base.RerenderRevision(ctx, url, revisionID)
}

func (s *embeddedArticleService) QueueRerenderRevision(ctx context.Context, url string, revisionID int) (<-chan RerenderResult, error) {
	if embedded.IsEmbeddedURL(url) {
		return nil, wiki.ErrReadOnlyArticle
	}
	return s.base.QueueRerenderRevision(ctx, url, revisionID)
}
