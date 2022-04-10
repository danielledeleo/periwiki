package extensions

import (
	localast "github.com/jagger27/periwiki/extensions/ast"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"regexp"
)

var wikiLinkRegexp = regexp.MustCompile(`\[\[\s*((?P<truelink>.+?)\s*\|\s*(?P<replacement>.+?)\s*|(?P<link>.+?))\s*\]\]`)

type WikiLinkerConfig struct {
	WikiLinkRegexp *regexp.Regexp

	trueLinkIdx, replacementIdx, linkIdx int
}

type WikiLinkerOption interface {
	parser.Option
	SetWikiLinkerOption(*WikiLinkerConfig)
}

type wikiLinkerParser struct {
	WikiLinkerConfig
}

func NewWikiLinkerParser(opts ...WikiLinkerOption) parser.InlineParser {
	return &wikiLinkerParser{
		WikiLinkerConfig: WikiLinkerConfig{
			WikiLinkRegexp: wikiLinkRegexp,

			trueLinkIdx:    wikiLinkRegexp.SubexpIndex("truelink") * 2,
			replacementIdx: wikiLinkRegexp.SubexpIndex("replacement") * 2,
			linkIdx:        wikiLinkRegexp.SubexpIndex("link") * 2,
		},
	}
}

func (p *wikiLinkerParser) Trigger() []byte {
	return []byte{'['}
}

func (p *wikiLinkerParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
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

	var destination []byte
	var title []byte

	if m[p.linkIdx] != -1 {
		destination = line[m[p.linkIdx]:m[p.linkIdx+1]]
		title = destination

	} else if m[p.trueLinkIdx] != -1 && m[p.replacementIdx] != -1 {
		destination = line[m[p.trueLinkIdx]:m[p.trueLinkIdx+1]]
		title = line[m[p.replacementIdx]:m[p.replacementIdx+1]]

	} else {
		return nil
	}

	s := segment.WithStop(segment.Start)
	ast.MergeOrAppendTextSegment(parent, s)

	block.Advance(length)
	return localast.NewWikiLink(destination, title)
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

func (e *wikiLinker) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(NewWikiLinkerParser(e.options...), 100),
		),
	)
}
