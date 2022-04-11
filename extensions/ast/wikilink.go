package ast

import (
	"fmt"

	gast "github.com/yuin/goldmark/ast"
)

type WikiLink struct {
	gast.BaseInline
	Link         *gast.Link
	OriginalDest []byte
	Classes      [][]byte
}

// Dump implements Node.Dump.
func (l *WikiLink) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Destination(actual)"] = string(l.Link.Destination)
	m["Destination(original)"] = string(l.OriginalDest)
	m["Title"] = string(l.Link.Title)
	m["Classes"] = fmt.Sprintf("%s", l.Classes)

	gast.DumpHelper(l, source, level, m, nil)
}

// KindWikiLink is a NodeKind of the WikiLink node.
var KindWikiLink = gast.NewNodeKind("WikiLink")

// Kind implements Node.Kind.
func (l *WikiLink) Kind() gast.NodeKind {
	return KindWikiLink
}

// NewWikiLink returns a new WikiLink node.
func NewWikiLink(title, originalDest, actualDest []byte, classes [][]byte) *WikiLink {
	link := gast.NewLink()
	link.Destination = actualDest
	link.Title = title

	return &WikiLink{
		BaseInline:   gast.BaseInline{},
		Link:         link,
		OriginalDest: originalDest,
		Classes:      classes,
	}
}
