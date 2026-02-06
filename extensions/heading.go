package extensions

import (
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/danielledeleo/periwiki/wiki"
)

// WikiHeadingIDs is a goldmark extension that generates MediaWiki-style
// heading IDs: preserving case and using underscores instead of hyphens.
type WikiHeadingIDs struct{}

// NewWikiHeadingIDs creates a new WikiHeadingIDs extension.
func NewWikiHeadingIDs() goldmark.Extender {
	return &WikiHeadingIDs{}
}

// Extend implements goldmark.Extender.
func (e *WikiHeadingIDs) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithAutoHeadingID(),
		parser.WithASTTransformers(
			util.Prioritized(&wikiHeadingIDTransformer{}, 500),
		),
	)
}

// wikiHeadingIDTransformer replaces goldmark's default heading IDs with
// MediaWiki-style IDs.
type wikiHeadingIDTransformer struct{}

func (t *wikiHeadingIDTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	used := map[string]bool{}

	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		// Build the heading text from source lines
		var raw []byte
		for i := 0; i < heading.Lines().Len(); i++ {
			line := heading.Lines().At(i)
			raw = append(raw, line.Value(reader.Source())...)
		}

		id := wikiHeadingID(string(raw), used)
		heading.SetAttribute([]byte("id"), []byte(id))

		return ast.WalkContinue, nil
	})
}

// wikiHeadingID converts heading text to a MediaWiki-style anchor ID.
// Follows MediaWiki's html5 mode: spaces become underscores, everything
// else is kept as-is. Goldmark and html/template handle escaping in
// attribute context; bluemonday sanitizes the final output.
func wikiHeadingID(text string, used map[string]bool) string {
	slug := wiki.TitleToSlug(text)

	if slug == "" {
		slug = "heading"
	}

	if !used[slug] {
		used[slug] = true
		return slug
	}
	for i := 1; ; i++ {
		deduped := fmt.Sprintf("%s_%d", slug, i)
		if !used[deduped] {
			used[deduped] = true
			return deduped
		}
	}
}
