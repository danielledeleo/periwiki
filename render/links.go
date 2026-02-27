package render

import (
	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/extensions/ast"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// LinkExtractor extracts wikilink target slugs from markdown content.
type LinkExtractor struct {
	md goldmark.Markdown
}

// NewLinkExtractor creates a new LinkExtractor with a lightweight Goldmark
// instance configured only for wikilink parsing.
func NewLinkExtractor() *LinkExtractor {
	return &LinkExtractor{
		md: goldmark.New(
			goldmark.WithExtensions(
				extensions.NewWikiLinker(
					[]extensions.WikiLinkerOption{extensions.WithUnderscoreResolver()},
					nil,
				),
			),
		),
	}
}

// ExtractLinks parses markdown and returns a deduplicated list of wikilink
// target slugs. Wikilinks inside code blocks are ignored (handled by Goldmark).
func (e *LinkExtractor) ExtractLinks(markdown string) []string {
	_, content := wiki.ParseFrontmatter(markdown)

	reader := text.NewReader([]byte(content))
	doc := e.md.Parser().Parse(reader)

	seen := make(map[string]struct{})
	var slugs []string

	gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		wl, ok := n.(*ast.WikiLink)
		if !ok {
			return gast.WalkContinue, nil
		}
		slug := wiki.TitleToSlug(string(wl.OriginalDest))
		if _, exists := seen[slug]; !exists {
			seen[slug] = struct{}{}
			slugs = append(slugs, slug)
		}
		return gast.WalkContinue, nil
	})

	return slugs
}
