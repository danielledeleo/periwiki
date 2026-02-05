package render

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"github.com/danielledeleo/periwiki/extensions"
)

type HTMLRenderer struct {
	md goldmark.Markdown
}

// TOCEntry represents a heading in the table of contents.
type TOCEntry struct {
	ID       string
	Text     string
	Children []TOCEntry
}

// buildTOCTree constructs a nested TOC from a flat list of heading nodes.
// Expects h2, h3, and h4 nodes. h2 is top-level, h3 nests under h2, h4 under h3.
// Headings that appear before a parent of the expected level are dropped
// (e.g. an h3 before any h2 is not included).
func buildTOCTree(nodes []*html.Node) []TOCEntry {
	var root []TOCEntry

	for _, n := range nodes {
		level := headingLevel(n)
		if level < 2 || level > 4 {
			continue
		}

		entry := TOCEntry{
			ID:   getAttr(n, "id"),
			Text: textContent(n),
		}

		switch level {
		case 2:
			root = append(root, entry)
		case 3:
			if len(root) > 0 {
				root[len(root)-1].Children = append(root[len(root)-1].Children, entry)
			}
		case 4:
			if len(root) > 0 {
				parent := &root[len(root)-1]
				if len(parent.Children) > 0 {
					parent.Children[len(parent.Children)-1].Children = append(
						parent.Children[len(parent.Children)-1].Children, entry)
				}
			}
		}
	}

	return root
}

func headingLevel(n *html.Node) int {
	switch n.Data {
	case "h2":
		return 2
	case "h3":
		return 3
	case "h4":
		return 4
	default:
		return 0
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}

// NewHTMLRenderer creates a new HTMLRenderer. If existenceChecker is provided,
// WikiLinks to non-existent pages will be styled with the pw-deadlink class.
// WikiLink and footnote options can be passed to customize rendering.
func NewHTMLRenderer(
	existenceChecker extensions.ExistenceChecker,
	wikiLinkRendererOpts []extensions.WikiLinkRendererOption,
	footnoteOpts []extensions.FootnoteOption,
) *HTMLRenderer {
	var wikiLinkerParserOpts []extensions.WikiLinkerOption
	if existenceChecker != nil {
		wikiLinkerParserOpts = []extensions.WikiLinkerOption{extensions.WithExistenceAwareResolver(existenceChecker)}
	} else {
		wikiLinkerParserOpts = []extensions.WikiLinkerOption{extensions.WithUnderscoreResolver()}
	}

	r := &HTMLRenderer{
		md: goldmark.New(
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
			goldmark.WithExtensions(
				extension.Table,
				extensions.NewWikiLinker(wikiLinkerParserOpts, wikiLinkRendererOpts),
				extensions.NewFootnote(footnoteOpts...),
			),
		),
	}

	return r
}

func (r *HTMLRenderer) Render(md string) (string, error) {
	buf := &bytes.Buffer{}

	if err := r.md.Convert([]byte(md), buf); err != nil {
		return "", fmt.Errorf("failed to Convert: %w", err)
	}
	rawhtml := buf.Bytes()
	htmlreader := bytes.NewReader(rawhtml)

	root, err := html.Parse(htmlreader)
	if err != nil {
		return "", err
	}

	document := goquery.NewDocumentFromNode(root)

	headers := document.Find("h2, h3, h4")
	if headers.Length() == 0 {
		return string(rawhtml), nil
	}

	var nodes []*html.Node
	headers.Each(func(_ int, s *goquery.Selection) {
		nodes = append(nodes, s.Nodes[0])
	})
	tocTree := buildTOCTree(nodes)

	if len(tocTree) == 0 {
		return string(rawhtml), nil
	}

	tmpl, err := template.ParseFiles("templates/_render/toc.html")
	if err != nil {
		return "", err
	}

	outbuf := &bytes.Buffer{}
	err = tmpl.Execute(outbuf, map[string]any{"Entries": tocTree})
	if err != nil {
		return "", err
	}

	fakeBody := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}

	newnodes, err := html.ParseFragment(outbuf, fakeBody)
	if err != nil {
		return "", err
	}

	// Find the TOC div element among parsed fragment nodes (skip whitespace text nodes).
	var tocNode *html.Node
	for _, n := range newnodes {
		if n.Type == html.ElementNode {
			tocNode = n
			break
		}
	}
	if tocNode == nil {
		return string(rawhtml), nil
	}

	h2s := document.Find("h2")
	if h2s.Length() == 0 {
		return string(rawhtml), nil
	}
	firstH2 := h2s.Nodes[0]
	firstH2.Parent.InsertBefore(tocNode, firstH2)

	outbuf.Reset()
	err = html.Render(outbuf, root)
	if err != nil {
		return "", err
	}

	return outbuf.String(), nil
}
