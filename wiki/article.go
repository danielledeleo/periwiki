package wiki

import (
	"fmt"
	"time"
)

type Article struct {
	URL string
	*Revision
}

func NewArticle(url, markdownBody string) *Article {
	article := &Article{URL: url, Revision: &Revision{}}
	article.Markdown = markdownBody

	return article
}

func (article *Article) String() string {
	return fmt.Sprintf("%s %v", article.URL, *article.Revision)
}

// DisplayTitle returns the article's title for display.
// Priority: frontmatter display_title > inferred from URL.
func (a *Article) DisplayTitle() string {
	fm, _ := ParseFrontmatter(a.Markdown)
	if fm.DisplayTitle != "" {
		return fm.DisplayTitle
	}
	return InferTitle(a.URL)
}

// ArticleSummary represents minimal article info for sitemaps.
// Note: Does not include markdown for performance - use InferTitle for display.
// For frontmatter-based titles, use full Article with DisplayTitle().
type ArticleSummary struct {
	URL          string    `db:"url"`
	LastModified time.Time `db:"last_modified"`
}

// DisplayTitle returns the display title for the article summary.
// Uses URL inference only - ArticleSummary doesn't fetch markdown for performance.
func (s *ArticleSummary) DisplayTitle() string {
	return InferTitle(s.URL)
}
