package wiki

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
