package ast

import (
	gast "github.com/yuin/goldmark/ast"
)

type WikiLink struct {
	gast.BaseInline
	Destination []byte
	Title       []byte
	// title       *ast.Text
}

// Dump implements Node.Dump.
func (l *WikiLink) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Destination"] = string(l.Destination)
	m["Title"] = string(l.Title)

	gast.DumpHelper(l, source, level, m, nil)
}

// KindWikiLink is a NodeKind of the WikiLink node.
var KindWikiLink = gast.NewNodeKind("WikiLink")

// Kind implements Node.Kind.
func (l *WikiLink) Kind() gast.NodeKind {
	return KindWikiLink
}

// NewWikiLink returns a new WikiLink node.
func NewWikiLink(dest, title []byte) *WikiLink {
	return &WikiLink{
		BaseInline:  gast.BaseInline{},
		Destination: dest,
		Title:       title,
	}
}
