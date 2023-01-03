package extensions

import (
	"bytes"

	"github.com/danielledeleo/periwiki/extensions/ast"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"regexp"
)

var wikiLinkRegexp = regexp.MustCompile(`\[\[\s*((?P<truelink>.+?)\s*\|\s*(?P<replacement>.+?)\s*|(?P<link>.+?))\s*\]\]`)

const (
	optWikiLinkerResolver parser.OptionName = "WikiLinkResolver"
)

// WikiLinkResolver resolves link destinations. If actual == nil parsing
// is skipped. CSS classes returned here are applied to the resulting <a>.
type WikiLinkResolver interface {
	Resolve(dest []byte) (actual []byte, classes [][]byte)
}

type identityResolver struct{}

func (r *identityResolver) Resolve(dest []byte) (actual []byte, classes [][]byte) {
	return dest, nil
}

type WikiLinkerConfig struct {
	WikiLinkRegexp *regexp.Regexp

	WikiLinkResolver
}

type WikiLinkerOption interface {
	parser.Option
	SetWikiLinkerOption(*WikiLinkerConfig)
}

type wikiLinkerParser struct {
	WikiLinkerConfig

	trueLinkIdx, replacementIdx, linkIdx int
}

type withWikiLinkerResolver struct {
	value WikiLinkResolver
}

func (o *withWikiLinkerResolver) SetParserOption(c *parser.Config) {
	c.Options[optWikiLinkerResolver] = o.value
}

func (o *withWikiLinkerResolver) SetWikiLinkerOption(p *WikiLinkerConfig) {
	p.WikiLinkResolver = o.value
}

// WithCustomResolver is a functional option to define how WikiLinks
// are resolved.
func WithCustomResolver(value WikiLinkResolver) WikiLinkerOption {
	return &withWikiLinkerResolver{
		value: value,
	}
}

func NewWikiLinkerParser(opts ...WikiLinkerOption) parser.InlineParser {
	parser := &wikiLinkerParser{
		WikiLinkerConfig: WikiLinkerConfig{
			WikiLinkRegexp:   wikiLinkRegexp,
			WikiLinkResolver: &identityResolver{},
		},

		trueLinkIdx:    wikiLinkRegexp.SubexpIndex("truelink") * 2,
		replacementIdx: wikiLinkRegexp.SubexpIndex("replacement") * 2,
		linkIdx:        wikiLinkRegexp.SubexpIndex("link") * 2,
	}

	for _, o := range opts {
		o.SetWikiLinkerOption(&parser.WikiLinkerConfig)
	}

	return parser
}

func (p *wikiLinkerParser) Trigger() []byte {
	return []byte{'['}
}

func (p *wikiLinkerParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, segment := block.PeekLine()

	// Must be at least 5 chars long: [[X]]
	if len(line) < 5 {
		return nil
	}

	if line[0] != '[' || line[1] != '[' {
		return nil
	}

	m := p.WikiLinkRegexp.FindSubmatchIndex(line)
	if m == nil {
		return nil
	}

	length := m[1] - m[0]

	var originalDest []byte
	var title []byte

	if m[p.linkIdx] != -1 {
		originalDest = line[m[p.linkIdx]:m[p.linkIdx+1]]
		title = originalDest

	} else if m[p.trueLinkIdx] != -1 && m[p.replacementIdx] != -1 {
		originalDest = line[m[p.trueLinkIdx]:m[p.trueLinkIdx+1]]
		title = line[m[p.replacementIdx]:m[p.replacementIdx+1]]

	} else {
		return nil
	}

	actualDest, classes := p.Resolve(originalDest)
	if actualDest == nil {
		return nil
	}

	s := segment.WithStop(segment.Start)
	gast.MergeOrAppendTextSegment(parent, s)

	block.Advance(length)

	return ast.NewWikiLink(title, originalDest, actualDest, classes)
}

type wikiLinker struct {
	options []WikiLinkerOption
}

// WikiLinker is an extension that allow you to parse text that seems like a URL.
var WikiLinker = &wikiLinker{}

func NewWikiLinker(opts ...WikiLinkerOption) goldmark.Extender {
	return &wikiLinker{
		options: opts,
	}
}

// wikiLinkerHTMLRenderer is a renderer.NodeRenderer implementation that
// renders WikiLinker nodes.
type wikiLinkerHTMLRenderer struct {
	html.Config
}

// NewWikiLinkerHTMLRenderer returns a new WikiLinkerHTMLRenderer.
func NewWikiLinkerHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &wikiLinkerHTMLRenderer{
		Config: html.NewConfig(),
	}

	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}

	return r
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *wikiLinkerHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindWikiLink, r.renderWikiLinker)
}

// WikiLinkAttributeFilter defines attribute names which WikiLink elements can have.
var WikiLinkAttributeFilter = html.GlobalAttributeFilter

func (r *wikiLinkerHTMLRenderer) renderWikiLinker(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	// adapted from Link renderer
	node := n.(*ast.WikiLink)
	if entering {
		_, _ = w.WriteString(`<a href="`)
		if r.Unsafe || !html.IsDangerousURL(node.Link.Destination) {
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(node.Link.Destination, true)))
		}
		_ = w.WriteByte('"')

		if node.Link.Title != nil {
			node.SetAttributeString("title", node.OriginalDest)
		}

		if node.Classes != nil {
			classes := bytes.Join(node.Classes, []byte{' '})
			node.SetAttributeString("class", classes)
		}

		if n.Attributes() != nil {
			html.RenderAttributes(w, node, WikiLinkAttributeFilter)
		}
		_ = w.WriteByte('>')
	} else {
		_, _ = w.Write(node.Link.Title)
		_, _ = w.WriteString("</a>")
	}
	return gast.WalkContinue, nil
}

func (e *wikiLinker) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(NewWikiLinkerParser(e.options...), 100),
		),
	)

	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewWikiLinkerHTMLRenderer(), 500),
	))
}
