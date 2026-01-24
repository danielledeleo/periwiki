package service

import (
	"crypto/sha512"
	"database/sql"
	"encoding/base64"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/repository"
	"github.com/microcosm-cc/bluemonday"
)

// ArticleService defines the interface for article operations.
type ArticleService interface {
	// GetArticle retrieves an article by its URL.
	GetArticle(url string) (*wiki.Article, error)

	// PostArticle creates or updates an article.
	PostArticle(article *wiki.Article) error

	// GetArticleByRevisionID retrieves an article by URL and revision ID.
	GetArticleByRevisionID(url string, id int) (*wiki.Article, error)

	// GetArticleByRevisionHash retrieves an article by URL and revision hash.
	GetArticleByRevisionHash(url string, hash string) (*wiki.Article, error)

	// GetRevisionHistory retrieves all revisions for an article.
	GetRevisionHistory(url string) ([]*wiki.Revision, error)

	// GetRandomArticleURL returns a random article URL.
	GetRandomArticleURL() (string, error)
}

// articleService is the default implementation of ArticleService.
type articleService struct {
	repo      repository.ArticleRepository
	rendering RenderingService
}

// NewArticleService creates a new ArticleService.
func NewArticleService(repo repository.ArticleRepository, rendering RenderingService) ArticleService {
	return &articleService{
		repo:      repo,
		rendering: rendering,
	}
}

// GetArticle retrieves an article by its URL.
func (s *articleService) GetArticle(url string) (*wiki.Article, error) {
	article, err := s.repo.SelectArticle(url)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrGenericNotFound
	} else if err != nil {
		return nil, err
	}

	return article, err
}

// PostArticle creates or updates an article.
func (s *articleService) PostArticle(article *wiki.Article) error {
	x := sha512.Sum384([]byte(article.Title + article.Markdown))
	article.Hash = base64.URLEncoding.EncodeToString(x[:])

	sourceRevision, err := s.GetArticleByRevisionID(article.URL, article.PreviousID)
	if err != wiki.ErrRevisionNotFound {
		if sourceRevision.Hash == article.Hash {
			return wiki.ErrArticleNotModified
		} else if err != nil {
			return err
		}
	}

	strip := bluemonday.StrictPolicy()

	article.Title = strip.Sanitize(article.Title)
	article.Comment = strip.Sanitize(article.Comment)

	html, err := s.rendering.Render(article.Markdown)
	if err != nil {
		return err
	}

	article.HTML = html

	return s.repo.InsertArticle(article)
}

// GetArticleByRevisionID retrieves an article by URL and revision ID.
func (s *articleService) GetArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	revision, err := s.repo.SelectArticleByRevisionID(url, id)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrRevisionNotFound
	}

	return revision, err
}

// GetArticleByRevisionHash retrieves an article by URL and revision hash.
func (s *articleService) GetArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	revision, err := s.repo.SelectArticleByRevisionHash(url, hash)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrRevisionNotFound
	}

	return revision, err
}

// GetRevisionHistory retrieves all revisions for an article.
func (s *articleService) GetRevisionHistory(url string) ([]*wiki.Revision, error) {
	return s.repo.SelectRevisionHistory(url)
}

// GetRandomArticleURL returns a random article URL.
func (s *articleService) GetRandomArticleURL() (string, error) {
	url, err := s.repo.SelectRandomArticleURL()
	if err == sql.ErrNoRows {
		return "", wiki.ErrNoArticles
	}
	return url, err
}
