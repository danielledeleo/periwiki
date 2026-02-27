package repository

import "github.com/danielledeleo/periwiki/wiki"

// LinkRepository defines the interface for article link graph persistence.
type LinkRepository interface {
	// ReplaceArticleLinks replaces all outgoing links for the given source article.
	ReplaceArticleLinks(sourceURL string, targetSlugs []string) error

	// SelectBacklinks returns articles that link to the given target slug.
	SelectBacklinks(targetSlug string) ([]*wiki.ArticleSummary, error)

	// CountLinks returns the total number of rows in the ArticleLink table.
	CountLinks() (int, error)
}
