package render

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/PuerkitoBio/goquery"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"

	"github.com/danielledeleo/periwiki/extensions"
)

type HTMLRenderer struct {
	md goldmark.Markdown
}

// NewHTMLRenderer creates a new HTMLRenderer. If existenceChecker is provided,
// WikiLinks to non-existent pages will be styled with the pw-deadlink class.
func NewHTMLRenderer(existenceChecker extensions.ExistenceChecker) *HTMLRenderer {
	var wikiLinkerOpt extensions.WikiLinkerOption
	if existenceChecker != nil {
		wikiLinkerOpt = extensions.WithExistenceAwareResolver(existenceChecker)
	} else {
		wikiLinkerOpt = extensions.WithUnderscoreResolver()
	}

	r := &HTMLRenderer{
		md: goldmark.New(
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
			goldmark.WithExtensions(
				extensions.NewWikiLinker(wikiLinkerOpt),
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

	headers := document.Find("h2")
	if headers.Length() == 0 {
		if err != nil {
			return "", err
		}
		return string(rawhtml), nil
	}

	tmpl, err := template.ParseFiles("templates/helpers/toc.html")
	if err != nil {
		return "", err
	}

	outbuf := &bytes.Buffer{}
	err = tmpl.Execute(outbuf, map[string]interface{}{"Headers": headers.Nodes})
	if err != nil {
		return "", err
	}

	fakeBody := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}

	newnode, err := html.ParseFragment(outbuf, fakeBody)
	if err != nil {
		return "", err
	}

	root.InsertBefore(newnode[0], headers.Nodes[0])

	outbuf.Reset()
	err = html.Render(outbuf, root)
	if err != nil {
		return "", err
	}

	return outbuf.String(), nil
}
