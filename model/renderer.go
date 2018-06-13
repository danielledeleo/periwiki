package model

import (
	"bytes"
	"log"

	"html/template"

	"github.com/PuerkitoBio/goquery"

	"github.com/jagger27/iwikii/pandoc"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type HTMLRenderer struct{}

func RenderHTML(markdown string) (string, error) {
	rawhtml, err := pandoc.MarkdownToHTML(markdown)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(rawhtml)
	root, err := html.Parse(buf)
	if err != nil {
		return "", err
	}
	document := goquery.NewDocumentFromNode(root)

	headers := document.Find("h2")
	if headers.Length() == 0 {
		return "", nil
	}

	tmpl, err := template.ParseFiles("templates/helpers/helper.html")
	if err != nil {
		log.Panic(err)
	}

	outbuf := &bytes.Buffer{}
	err = tmpl.Execute(outbuf, map[string]interface{}{"Headers": headers.Nodes})
	if err != nil {
		log.Panic(err)
	}

	fakeBody := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}
	newnode, err := html.ParseFragment(outbuf, fakeBody)
	if err != nil {
		log.Panic(err)
	}
	root.InsertBefore(newnode[0], headers.Nodes[0])

	outbuf.Reset()
	err = html.Render(outbuf, root)
	if err != nil {
		log.Panic(err)
	}

	return outbuf.String(), nil
}
