package service

import (
	"context"
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/danielledeleo/periwiki/internal/renderqueue"
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

	// PostArticleWithContext creates or updates an article with context for cancellation.
	PostArticleWithContext(ctx context.Context, article *wiki.Article) error

	// Preview renders markdown to HTML without persisting (bypasses queue).
	Preview(markdown string) (string, error)

	// GetArticleByRevisionID retrieves an article by URL and revision ID.
	GetArticleByRevisionID(url string, id int) (*wiki.Article, error)

	// GetArticleByRevisionHash retrieves an article by URL and revision hash.
	GetArticleByRevisionHash(url string, hash string) (*wiki.Article, error)

	// GetRevisionHistory retrieves all revisions for an article.
	GetRevisionHistory(url string) ([]*wiki.Revision, error)

	// GetRandomArticleURL returns a random article URL.
	GetRandomArticleURL() (string, error)

	// GetAllArticles retrieves all articles with their last modified time.
	GetAllArticles() ([]*wiki.ArticleSummary, error)

	// RerenderRevision re-renders an existing revision's markdown and updates its HTML.
	// If revisionID is 0, re-renders the current (latest) revision.
	RerenderRevision(ctx context.Context, url string, revisionID int) error

	// QueueRerenderRevision queues a rerender without waiting for completion.
	// Returns a channel that receives the result when the render and DB update complete.
	// If revisionID is 0, re-renders the current (latest) revision.
	QueueRerenderRevision(ctx context.Context, url string, revisionID int) (<-chan RerenderResult, error)
}

// articleService is the default implementation of ArticleService.
type articleService struct {
	repo      repository.ArticleRepository
	rendering RenderingService
	queue     *renderqueue.Queue
}

// NewArticleService creates a new ArticleService.
// If queue is nil, rendering is done synchronously (useful for tests).
func NewArticleService(repo repository.ArticleRepository, rendering RenderingService, queue *renderqueue.Queue) ArticleService {
	return &articleService{
		repo:      repo,
		rendering: rendering,
		queue:     queue,
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

	if err := s.ensureHTML(article); err != nil {
		return nil, err
	}

	return article, nil
}

// PostArticle creates or updates an article.
// This is a convenience wrapper that uses context.Background().
func (s *articleService) PostArticle(article *wiki.Article) error {
	return s.PostArticleWithContext(context.Background(), article)
}

// PostArticleWithContext creates or updates an article with context for cancellation.
func (s *articleService) PostArticleWithContext(ctx context.Context, article *wiki.Article) error {
	// Talk pages require the subject article to exist
	if wiki.IsTalkPage(article.URL) {
		subjectURL := wiki.SubjectPageURL(article.URL)
		if _, err := s.repo.SelectArticle(subjectURL); err != nil {
			return wiki.ErrSubjectPageNotFound
		}
	}

	// Hash based on markdown only - title is deprecated
	x := sha512.Sum384([]byte(article.Markdown))
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
	article.Comment = strip.Sanitize(article.Comment)

	// If no queue is configured, use synchronous rendering (useful for tests)
	if s.queue == nil {
		html, err := s.rendering.Render(article.Markdown)
		if err != nil {
			return err
		}
		article.HTML = html
		if err := s.repo.InsertArticle(article); err != nil {
			return err
		}
		// Update article.ID to match the queued path behavior
		article.ID = article.PreviousID + 1
		return nil
	}

	// Insert revision with render_status='queued' and empty HTML
	revisionID, err := s.repo.InsertArticleQueued(article)
	if err != nil {
		return err
	}

	// Submit job to queue and wait for result
	waitCh := make(chan renderqueue.Result, 1)
	job := renderqueue.Job{
		ArticleURL:  article.URL,
		RevisionID:  revisionID,
		Markdown:    article.Markdown,
		Tier:        renderqueue.TierInteractive,
	}

	if err := s.queue.Submit(ctx, job, waitCh); err != nil {
		// Queue closed or other error - mark as failed
		_ = s.repo.UpdateRevisionHTML(article.URL, int(revisionID), "", "failed")
		return err
	}

	// Wait for render result with context cancellation
	select {
	case result := <-waitCh:
		if result.Err != nil {
			// Render failed - update status
			_ = s.repo.UpdateRevisionHTML(article.URL, int(revisionID), "", "failed")
			return result.Err
		}
		// Success - update HTML and status
		if err := s.repo.UpdateRevisionHTML(article.URL, int(revisionID), result.HTML, "rendered"); err != nil {
			return err
		}
		article.HTML = result.HTML
		article.ID = int(revisionID)
		return nil

	case <-ctx.Done():
		// Context cancelled (e.g., request timeout)
		_ = s.repo.UpdateRevisionHTML(article.URL, int(revisionID), "", "failed")
		return ctx.Err()
	}
}

// Preview renders markdown to HTML without persisting (bypasses queue).
func (s *articleService) Preview(markdown string) (string, error) {
	return s.rendering.Render(markdown)
}

// GetArticleByRevisionID retrieves an article by URL and revision ID.
func (s *articleService) GetArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	article, err := s.repo.SelectArticleByRevisionID(url, id)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrRevisionNotFound
	} else if err != nil {
		return nil, err
	}

	if err := s.ensureHTML(article); err != nil {
		return nil, err
	}

	return article, nil
}

// GetArticleByRevisionHash retrieves an article by URL and revision hash.
func (s *articleService) GetArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	article, err := s.repo.SelectArticleByRevisionHash(url, hash)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrRevisionNotFound
	} else if err != nil {
		return nil, err
	}

	if err := s.ensureHTML(article); err != nil {
		return nil, err
	}

	return article, nil
}

// ensureHTML lazily renders and persists HTML for a revision that has none.
// This handles old revisions whose cached HTML was invalidated after a
// render pipeline change.
func (s *articleService) ensureHTML(article *wiki.Article) error {
	if article.HTML != "" {
		return nil
	}

	html, err := s.rendering.Render(article.Markdown)
	if err != nil {
		return fmt.Errorf("lazy render failed: %w", err)
	}

	article.HTML = html

	if err := s.repo.UpdateRevisionHTML(article.URL, article.ID, html, "rendered"); err != nil {
		slog.Warn("failed to persist lazy-rendered HTML", "url", article.URL, "revision", article.ID, "error", err)
		// Non-fatal: we have the HTML in memory to serve this request
	}

	return nil
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

// GetAllArticles retrieves all articles with their last modified time.
func (s *articleService) GetAllArticles() ([]*wiki.ArticleSummary, error) {
	return s.repo.SelectAllArticles()
}

// RerenderRevision re-renders an existing revision's markdown and updates its HTML.
// If revisionID is 0, re-renders the current (latest) revision.
func (s *articleService) RerenderRevision(ctx context.Context, url string, revisionID int) error {
	// Get the revision to re-render
	var article *wiki.Article
	var err error

	if revisionID == 0 {
		article, err = s.GetArticle(url)
	} else {
		article, err = s.GetArticleByRevisionID(url, revisionID)
	}
	if err != nil {
		return err
	}

	// If no queue, render synchronously
	if s.queue == nil {
		html, err := s.rendering.Render(article.Markdown)
		if err != nil {
			return err
		}
		return s.repo.UpdateRevisionHTML(url, article.ID, html, "rendered")
	}

	// Mark as queued
	if err := s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "queued"); err != nil {
		return err
	}

	// Submit to queue and wait
	waitCh := make(chan renderqueue.Result, 1)
	job := renderqueue.Job{
		ArticleURL:  url,
		RevisionID:  int64(article.ID),
		Markdown:    article.Markdown,
		Tier:        renderqueue.TierInteractive,
	}

	if err := s.queue.Submit(ctx, job, waitCh); err != nil {
		_ = s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "failed")
		return err
	}

	select {
	case result := <-waitCh:
		if result.Err != nil {
			_ = s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "failed")
			return result.Err
		}
		return s.repo.UpdateRevisionHTML(url, article.ID, result.HTML, "rendered")

	case <-ctx.Done():
		_ = s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "failed")
		return ctx.Err()
	}
}

// RerenderResult contains the outcome of an async rerender operation.
type RerenderResult struct {
	URL string
	Err error
}

// QueueRerenderRevision queues a rerender without waiting for completion.
// Returns a channel that will receive the result when rendering and DB update complete.
// The DB update is handled internally by a goroutine.
func (s *articleService) QueueRerenderRevision(ctx context.Context, url string, revisionID int) (<-chan RerenderResult, error) {
	// Get the revision to re-render
	var article *wiki.Article
	var err error

	if revisionID == 0 {
		article, err = s.GetArticle(url)
	} else {
		article, err = s.GetArticleByRevisionID(url, revisionID)
	}
	if err != nil {
		return nil, err
	}

	// If no queue, render synchronously and return completed channel
	if s.queue == nil {
		html, err := s.rendering.Render(article.Markdown)
		if err != nil {
			return nil, err
		}
		if err := s.repo.UpdateRevisionHTML(url, article.ID, html, "rendered"); err != nil {
			return nil, err
		}
		ch := make(chan RerenderResult, 1)
		ch <- RerenderResult{URL: url}
		close(ch)
		return ch, nil
	}

	// Mark as queued
	if err := s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "queued"); err != nil {
		return nil, err
	}

	// Submit to queue
	waitCh := make(chan renderqueue.Result, 1)
	job := renderqueue.Job{
		ArticleURL:  url,
		RevisionID:  int64(article.ID),
		Markdown:    article.Markdown,
		Tier:        renderqueue.TierBackground,
	}

	if err := s.queue.Submit(ctx, job, waitCh); err != nil {
		_ = s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "failed")
		return nil, err
	}

	// Spawn goroutine to wait for result and update DB
	resultCh := make(chan RerenderResult, 1)
	go func() {
		defer close(resultCh)
		result := <-waitCh
		if result.Err != nil {
			_ = s.repo.UpdateRevisionHTML(url, article.ID, article.HTML, "failed")
			resultCh <- RerenderResult{URL: url, Err: result.Err}
			return
		}
		if err := s.repo.UpdateRevisionHTML(url, article.ID, result.HTML, "rendered"); err != nil {
			resultCh <- RerenderResult{URL: url, Err: err}
			return
		}
		resultCh <- RerenderResult{URL: url}
	}()

	return resultCh, nil
}
