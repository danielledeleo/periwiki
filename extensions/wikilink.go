package extensions

import (
	"bytes"
	"text/template"

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

var wikiLinkRegexp = regexp.MustCompile(`\[\[\s*((?P<truelink>[^\[\]]+?)\s*(?:\\\||\|)\s*(?P<replacement>[^\[\]]+?)\s*|(?P<link>[^\[\]]+?))\s*\]\]`)

const (
	optWikiLinkerResolver parser.OptionName = "WikiLinkResolver"
)

// WikiLinkData holds data passed to the wikilink template.
type WikiLinkData struct {
	Destination  string // URL-escaped destination
	Title        string // display text (HTML-escaped)
	OriginalDest string // original destination before resolution
	Classes      string // space-separated CSS classes
	IsDeadlink   bool   // true if link points to non-existent page
}

// WikiLinkTemplates holds templates for wikilink rendering.
type WikiLinkTemplates struct {
	Link *template.Template
}

// WikiLinkRendererConfig holds configuration for wikilink rendering.
type WikiLinkRendererConfig struct {
	Templates WikiLinkTemplates
}

// WikiLinkRendererOption configures the wikilink renderer.
type WikiLinkRendererOption func(*WikiLinkRendererConfig)

// WithWikiLinkTemplates sets templates from a map.
// Required keys: "link"
func WithWikiLinkTemplates(templates map[string]*template.Template) WikiLinkRendererOption {
	return func(c *WikiLinkRendererConfig) {
		c.Templates.Link = templates["link"]
	}
}

// ExistenceChecker is a function that checks if a page exists at the given URL.
// It is used by resolvers to determine if a WikiLink points to an existing page.
type ExistenceChecker func(url string) bool

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
		// Strip trailing backslash from escaped pipe syntax (e.g., [[Page\|Text]] in tables)
		if len(originalDest) > 0 && originalDest[len(originalDest)-1] == '\\' {
			originalDest = originalDest[:len(originalDest)-1]
		}
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
	parserOptions   []WikiLinkerOption
	rendererOptions []WikiLinkRendererOption
}

// NewWikiLinker creates a new wikilinker extension.
// Parser options (WithCustomResolver, WithExistenceAwareResolver, etc.) configure link resolution.
// Renderer options (WithWikiLinkTemplates) configure HTML output.
func NewWikiLinker(parserOpts []WikiLinkerOption, rendererOpts []WikiLinkRendererOption) goldmark.Extender {
	return &wikiLinker{
		parserOptions:   parserOpts,
		rendererOptions: rendererOpts,
	}
}

// wikiLinkerHTMLRenderer is a renderer.NodeRenderer implementation that
// renders WikiLinker nodes.
type wikiLinkerHTMLRenderer struct {
	html.Config
	WikiLinkRendererConfig
	buf bytes.Buffer
}

// NewWikiLinkerHTMLRenderer returns a new WikiLinkerHTMLRenderer.
func NewWikiLinkerHTMLRenderer(opts ...WikiLinkRendererOption) renderer.NodeRenderer {
	r := &wikiLinkerHTMLRenderer{
		Config: html.NewConfig(),
	}

	for _, opt := range opts {
		opt(&r.WikiLinkRendererConfig)
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
	if !entering {
		return gast.WalkContinue, nil
	}

	node := n.(*ast.WikiLink)

	// SECURITY: Always check for dangerous URLs (javascript:, vbscript:, data:, etc.)
	// regardless of the Unsafe setting. WikiLinks should never allow script execution.
	var destination string
	if !html.IsDangerousURL(node.Link.Destination) {
		destination = string(util.URLEscape(node.Link.Destination, true))
	}

	// Build classes string and check for deadlink
	var classes string
	var isDeadlink bool
	if node.Classes != nil {
		classes = string(bytes.Join(node.Classes, []byte{' '}))
		isDeadlink = bytes.Contains(bytes.Join(node.Classes, []byte{' '}), []byte("pw-deadlink"))
	}

	data := WikiLinkData{
		Destination:  destination,
		Title:        string(util.EscapeHTML(node.Link.Title)),
		OriginalDest: string(util.EscapeHTML(node.OriginalDest)),
		Classes:      classes,
		IsDeadlink:   isDeadlink,
	}

	r.buf.Reset()
	if err := r.Templates.Link.Execute(&r.buf, data); err != nil {
		return gast.WalkStop, err
	}
	_, _ = w.Write(r.buf.Bytes())

	return gast.WalkContinue, nil
}

func (e *wikiLinker) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(NewWikiLinkerParser(e.parserOptions...), 100),
		),
	)

	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewWikiLinkerHTMLRenderer(e.rendererOptions...), 500),
	))
}
