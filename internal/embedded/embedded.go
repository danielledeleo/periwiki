package embedded

//go:generate go run ./gen/main.go

import (
	"embed"
	"io/fs"
	"strings"

	"github.com/danielledeleo/periwiki/wiki"
)

//go:embed help/*.md
var helpFS embed.FS

const embeddedPrefix = "Periwiki:"

// IsEmbeddedURL returns true if the URL is for an embedded article.
func IsEmbeddedURL(url string) bool {
	return strings.HasPrefix(url, embeddedPrefix)
}

// EmbeddedArticles holds pre-rendered embedded help articles.
type EmbeddedArticles struct {
	articles map[string]*wiki.Article
}

// RenderFunc is a function that renders markdown to HTML.
type RenderFunc func(markdown string) (string, error)

// New creates a new EmbeddedArticles instance by loading and rendering
// all markdown files from the embedded filesystem.
func New(render RenderFunc) (*EmbeddedArticles, error) {
	ea := &EmbeddedArticles{
		articles: make(map[string]*wiki.Article),
	}

	entries, err := fs.ReadDir(helpFS, "help")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := fs.ReadFile(helpFS, "help/"+entry.Name())
		if err != nil {
			return nil, err
		}

		// Derive URL from filename: "Syntax.md" -> "Periwiki:Syntax"
		name := strings.TrimSuffix(entry.Name(), ".md")
		url := embeddedPrefix + name

		html, err := render(string(content))
		if err != nil {
			return nil, err
		}

		ea.articles[url] = &wiki.Article{
			URL:      url,
			ReadOnly: true,
			Revision: &wiki.Revision{
				Markdown: string(content),
				HTML:     html,
			},
		}
	}

	return ea, nil
}

// Get returns an embedded article by URL, or nil if not found.
func (ea *EmbeddedArticles) Get(url string) *wiki.Article {
	return ea.articles[url]
}

// List returns all embedded article URLs.
func (ea *EmbeddedArticles) List() []string {
	urls := make([]string, 0, len(ea.articles))
	for url := range ea.articles {
		urls = append(urls, url)
	}
	return urls
}

// SourceURL returns the URL to view the source file for an embedded article.
// Returns empty string if the base URL isn't configured or the URL isn't embedded.
func SourceURL(articleURL string) string {
	if SourceBaseURL == "" || !IsEmbeddedURL(articleURL) {
		return ""
	}
	// articleURL is like "Periwiki:Syntax", file is "Syntax.md"
	name := strings.TrimPrefix(articleURL, embeddedPrefix)
	return SourceBaseURL + "/internal/embedded/help/" + name + ".md"
}
