package wiki

import (
	"fmt"
	"time"
)

type Article struct {
	URL      string
	ReadOnly bool // True for embedded/system articles
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

// Layout returns the article's layout from frontmatter.
func (a *Article) Layout() string {
	fm, _ := ParseFrontmatter(a.Markdown)
	return fm.Layout
}

// ArticleSummary represents minimal article info for sitemaps.
// Note: Does not include markdown for performance - use InferTitle for display.
// ArticleSummary is a lightweight article representation for listings.
type ArticleSummary struct {
	URL          string    `db:"url"`
	LastModified time.Time `db:"last_modified"`
	Title        string    `db:"title"` // Cached from frontmatter, may be empty
}

// DisplayTitle returns the display title for the article summary.
// Uses cached frontmatter title if available, otherwise infers from URL.
func (s *ArticleSummary) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	return InferTitle(s.URL)
}
