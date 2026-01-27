package wiki

import (
	"fmt"
	"time"
)

type Article struct {
	URL string
	*Revision
}

func NewArticle(url, title, markdownBody string) *Article {
	article := &Article{URL: url, Revision: &Revision{}}
	article.Title = title
	article.Markdown = markdownBody

	return article
}

func (article *Article) String() string {
	return fmt.Sprintf("%s %v", article.URL, *article.Revision)
}

// DisplayTitle returns the article's title for display.
// If the title is empty, it falls back to inferring a title from the URL.
func (a *Article) DisplayTitle() string {
	if a.Title != "" {
		return a.Title
	}
	return InferTitle(a.URL)
}

// ArticleSummary represents minimal article info for sitemaps.
type ArticleSummary struct {
	URL          string    `db:"url"`
	Title        string    `db:"title"`
	LastModified time.Time `db:"last_modified"`
}
