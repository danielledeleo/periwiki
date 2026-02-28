package extensions

import (
	"bytes"
	"strconv"
	"text/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Template data types

type FootnoteLinkData struct {
	Entering bool
	Index    int    // footnote number
	RefCount int    // total references to this footnote
	RefIndex int    // which reference (0-indexed)
	ID       string // anchor for footnote definition
	RefID    string // anchor for this reference
}

type FootnoteBacklinkData struct {
	Entering bool
	Index    int
	RefCount int // total references to this footnote
	RefIndex int // which reference (0-indexed)
	RefID    string
}

type FootnoteListData struct {
	Entering bool
}

type FootnoteItemData struct {
	Entering bool
	Index    int    // footnote number
	Ref      string // original reference name (e.g., "1" or "note")
	ID       string // anchor ID
}

// FootnoteTemplates holds all footnote templates.
type FootnoteTemplates struct {
	Link     *template.Template
	Backlink *template.Template
	List     *template.Template
	Item     *template.Template
}

// FootnoteConfig holds configuration for footnote rendering.
type FootnoteConfig struct {
	IDPrefix  string
	Templates FootnoteTemplates
}

// FootnoteOption configures the footnote extension.
type FootnoteOption func(*FootnoteConfig)

// WithFootnoteIDPrefix sets the prefix for footnote IDs.
func WithFootnoteIDPrefix(prefix string) FootnoteOption {
	return func(c *FootnoteConfig) { c.IDPrefix = prefix }
}

// WithFootnoteTemplates sets templates from a map.
// Required keys: "link", "backlink", "list", "item"
func WithFootnoteTemplates(templates map[string]*template.Template) FootnoteOption {
	return func(c *FootnoteConfig) {
		c.Templates.Link = templates["link"]
		c.Templates.Backlink = templates["backlink"]
		c.Templates.List = templates["list"]
		c.Templates.Item = templates["item"]
	}
}

func defaultFootnoteConfig() FootnoteConfig {
	// No default templates - must be provided via WithFootnoteTemplates
	return FootnoteConfig{
		IDPrefix: "fn-",
	}
}

// footnoteRenderer renders footnote nodes to HTML.
type footnoteRenderer struct {
	FootnoteConfig
}

// NewFootnoteRenderer creates a new footnote renderer.
func NewFootnoteRenderer(opts ...FootnoteOption) renderer.NodeRenderer {
	cfg := defaultFootnoteConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &footnoteRenderer{FootnoteConfig: cfg}
}

// RegisterFuncs implements renderer.NodeRenderer.
func (r *footnoteRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindFootnoteLink, r.renderFootnoteLink)
	reg.Register(east.KindFootnoteBacklink, r.renderFootnoteBacklink)
	reg.Register(east.KindFootnoteList, r.renderFootnoteList)
	reg.Register(east.KindFootnote, r.renderFootnote)
}

func (r *footnoteRenderer) renderFootnoteLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*east.FootnoteLink)
	data := FootnoteLinkData{
		Entering: entering,
		Index:    n.Index,
		RefCount: n.RefCount,
		RefIndex: n.RefIndex,
		ID:       r.footnoteID(n.Index),
		RefID:    r.footnoteRefID(n.Index, n.RefIndex),
	}
	var buf bytes.Buffer
	if err := r.Templates.Link.Execute(&buf, data); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.Write(buf.Bytes())
	return ast.WalkContinue, nil
}

func (r *footnoteRenderer) renderFootnoteBacklink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*east.FootnoteBacklink)
	data := FootnoteBacklinkData{
		Entering: entering,
		Index:    n.Index,
		RefCount: n.RefCount,
		RefIndex: n.RefIndex,
		RefID:    r.footnoteRefID(n.Index, n.RefIndex),
	}
	var buf bytes.Buffer
	if err := r.Templates.Backlink.Execute(&buf, data); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.Write(buf.Bytes())
	return ast.WalkContinue, nil
}

func (r *footnoteRenderer) renderFootnoteList(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	var buf bytes.Buffer
	data := FootnoteListData{Entering: entering}
	if err := r.Templates.List.Execute(&buf, data); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.Write(buf.Bytes())
	return ast.WalkContinue, nil
}

func (r *footnoteRenderer) renderFootnote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*east.Footnote)
	data := FootnoteItemData{
		Entering: entering,
		Index:    n.Index,
		Ref:      string(n.Ref),
		ID:       r.footnoteID(n.Index),
	}
	var buf bytes.Buffer
	if err := r.Templates.Item.Execute(&buf, data); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.Write(buf.Bytes())
	return ast.WalkContinue, nil
}

func (r *footnoteRenderer) footnoteID(index int) string {
	return r.IDPrefix + strconv.Itoa(index)
}

func (r *footnoteRenderer) footnoteRefID(index, refIndex int) string {
	return r.IDPrefix + "ref-" + strconv.Itoa(index) + "-" + strconv.Itoa(refIndex)
}

// FootnoteBacklinkReorderer moves backlinks from end to beginning of footnotes.
type FootnoteBacklinkReorderer struct{}

// NewFootnoteBacklinkReorderer creates a new AST transformer that reorders backlinks.
func NewFootnoteBacklinkReorderer() parser.ASTTransformer {
	return &FootnoteBacklinkReorderer{}
}

// Transform implements parser.ASTTransformer.
func (r *FootnoteBacklinkReorderer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		list, ok := node.(*east.FootnoteList)
		if !ok {
			return ast.WalkContinue, nil
		}

		for footnote := list.FirstChild(); footnote != nil; footnote = footnote.NextSibling() {
			fn, ok := footnote.(*east.Footnote)
			if !ok {
				continue
			}

			// Find container (same logic as goldmark)
			var container ast.Node = fn
			if fc := fn.LastChild(); fc != nil && ast.IsParagraph(fc) {
				container = fc
			}

			// Collect backlinks
			var backlinks []ast.Node
			for child := container.FirstChild(); child != nil; child = child.NextSibling() {
				if _, ok := child.(*east.FootnoteBacklink); ok {
					backlinks = append(backlinks, child)
				}
			}

			// Remove and reinsert at beginning
			for _, bl := range backlinks {
				container.RemoveChild(container, bl)
			}
			for i := len(backlinks) - 1; i >= 0; i-- {
				if first := container.FirstChild(); first != nil {
					container.InsertBefore(container, first, backlinks[i])
				} else {
					container.AppendChild(container, backlinks[i])
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

// footnote is the extension that combines parser and renderer.
type footnote struct {
	options []FootnoteOption
}

// NewFootnote creates a new footnote extension with custom rendering.
func NewFootnote(opts ...FootnoteOption) goldmark.Extender {
	return &footnote{options: opts}
}

// Extend implements goldmark.Extender.
func (e *footnote) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(extension.NewFootnoteBlockParser(), 999),
		),
		parser.WithInlineParsers(
			util.Prioritized(extension.NewFootnoteParser(), 101),
		),
		parser.WithASTTransformers(
			util.Prioritized(extension.NewFootnoteASTTransformer(), 999),
			util.Prioritized(NewFootnoteBacklinkReorderer(), 1000), // runs after 999 (higher = later)
		),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewFootnoteRenderer(e.options...), 101),
	))
}
