package extensions

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

var shouldDump bool

func testmain() {
	shouldDump = os.Getenv("DUMP_AST") == "1"
}

func dump(node ast.Node, source []byte) {
	if shouldDump {
		node.Dump(source, 0)
	}
}

// testWikiLinkTemplates creates simple templates for testing.
func testWikiLinkTemplates() map[string]*template.Template {
	return map[string]*template.Template{
		"link": template.Must(template.New("link").Parse(
			`<a href="{{.Destination}}"{{if .OriginalDest}} title="{{.OriginalDest}}"{{end}}{{if .Classes}} class="{{.Classes}}"{{end}}>{{.Title}}</a>`,
		)),
	}
}

func TestWikiLink(t *testing.T) {
	tests := []struct {
		name string
		md   string
	}{
		{name: "not", md: "Nothing _here_."},
		{name: "single", md: `[[Hello]] is a way to greet someone in English.`},
		{name: "replaced", md: `[[Bye|Goodbye]] is a parting greeting in English.`},
		{name: "exclaim", md: `[[Exclamation point|Boo!]]`},
		{name: "fancy", md: `Fancy: [[Some Ugly (URL)|A Pretty Link]]`},
		{name: "inside", md: `Part of a

- greater
- [[List]]
- [[Of|Things]]
`},
		{name: "nothing", md: `[[]]`},
		{name: "url", md: `[[[url](/inside)]]`},
		{name: "url2", md: `[[[inside]]](/url)`}, // broken

	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(nil, []WikiLinkRendererOption{WithWikiLinkTemplates(testWikiLinkTemplates())}),
		),
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.md)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			if node == nil {
				t.Fatal("empty node")
			}

			if err := markdown.Renderer().Render(log.Writer(), source, node); err != nil {
				t.Error(err)
			}

			dump(node, source)
		})
	}
}

func TestWikiLinkDefaultUnderscoreResolver(t *testing.T) {
	tests := []struct {
		name string
		md   string
	}{
		{name: "basic", md: "[[From here]]"},
		{name: "multiple", md: "[[List of Canadian provinces]]"},
		{name: "leading spaces", md: "[[ 	List of Canadian provinces ]]"},
		{name: "replaced", md: "[[Disambiguation (Disambiguation)|Disambiguation]]"},
		{name: "inner spaces", md: "[[From   here ]]"},
		{name: "inner tabs and spaces", md: "[[ From  		 here ]]"},
		{name: "everything", md: "[[From   here|  To 	here]]"},
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(
				[]WikiLinkerOption{WithUnderscoreResolver()},
				[]WikiLinkRendererOption{WithWikiLinkTemplates(testWikiLinkTemplates())},
			),
		),
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.md)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			if node == nil {
				t.Fatal("empty node")
			}

			dump(node, source)

			if err := markdown.Renderer().Render(log.Writer(), source, node); err != nil {
				t.Error(err)
			}

		})
	}
}

type customResolver struct {
	internal WikiLinkResolver
	t        *testing.T
}

func (r *customResolver) Resolve(original []byte) ([]byte, [][]byte) {
	resolved, classes := r.internal.Resolve(original)
	if resolved[0] == '/' {
		classes = [][]byte{[]byte("something")}
	}
	r.t.Logf("resolved: '%s' => '%s' %s", original, resolved, classes)

	return resolved, classes
}

func TestWikiLinkCustomResolver(t *testing.T) {
	tests := []struct {
		name string
		md   string
	}{
		{name: "not", md: "Nothing _here_."},
		{name: "basic", md: "[[From here]]"},
		{name: "multiple", md: "[[List of Canadian provinces]]"},
		{name: "replaced", md: "[[Disambiguation (Disambiguation)|Disambiguation]]"},
		{name: "weird title", md: `[[weird "title"]]`},
		{name: "slash title", md: `[[/thing with class|I have a class]]`},
	}

	resolver := &customResolver{
		internal: &underscoreResolver{},
		t:        t,
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(
				[]WikiLinkerOption{WithCustomResolver(resolver)},
				[]WikiLinkRendererOption{WithWikiLinkTemplates(testWikiLinkTemplates())},
			),
		),
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.md)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			if node == nil {
				t.Fatal("empty node")
			}

			dump(node, source)

			if err := markdown.Renderer().Render(log.Writer(), source, node); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestWikiLinkExistenceAwareResolver(t *testing.T) {
	// Mock existence checker: simulates DB, embedded, and special page lookups
	existingPages := map[string]bool{
		"/wiki/Existing_Page":    true,
		"/wiki/Periwiki:Syntax":  true,
		"/wiki/Special:Sitemap":  true,
	}
	checker := func(url string) bool {
		return existingPages[url]
	}

	tests := []struct {
		name           string
		md             string
		expectDeadlink bool
	}{
		{name: "existing page", md: "[[Existing Page]]", expectDeadlink: false},
		{name: "non-existing page", md: "[[Non Existing Page]]", expectDeadlink: true},
		{name: "existing with display text", md: "[[Existing Page|Click here]]", expectDeadlink: false},
		{name: "non-existing with display text", md: "[[Dead Link|Click here]]", expectDeadlink: true},
		{name: "embedded help page exists", md: "[[Periwiki:Syntax]]", expectDeadlink: false},
		{name: "embedded help page not found", md: "[[Periwiki:NonExistent]]", expectDeadlink: true},
		{name: "special page exists", md: "[[Special:Sitemap]]", expectDeadlink: false},
		{name: "special page not found", md: "[[Special:NonExistent]]", expectDeadlink: true},
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(
				[]WikiLinkerOption{WithExistenceAwareResolver(checker)},
				[]WikiLinkRendererOption{WithWikiLinkTemplates(testWikiLinkTemplates())},
			),
		),
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.md)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			if node == nil {
				t.Fatal("empty node")
			}

			var buf bytes.Buffer
			if err := markdown.Renderer().Render(&buf, source, node); err != nil {
				t.Error(err)
			}

			result := buf.String()
			hasDeadlinkClass := strings.Contains(result, `class="pw-deadlink"`)

			if test.expectDeadlink && !hasDeadlinkClass {
				t.Errorf("expected deadlink class in output, got: %s", result)
			}
			if !test.expectDeadlink && hasDeadlinkClass {
				t.Errorf("did not expect deadlink class in output, got: %s", result)
			}

			t.Logf("output: %s", result)
		})
	}
}

func TestWikiLinkExistenceAwareResolver_NilChecker(t *testing.T) {
	// Test with nil checker - should not mark any links as dead
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(
				[]WikiLinkerOption{WithExistenceAwareResolver(nil)},
				[]WikiLinkRendererOption{WithWikiLinkTemplates(testWikiLinkTemplates())},
			),
		),
	)

	source := []byte("[[Some Page]]")
	reader := text.NewReader(source)
	node := markdown.Parser().Parse(reader)

	var buf bytes.Buffer
	if err := markdown.Renderer().Render(&buf, source, node); err != nil {
		t.Error(err)
	}

	result := buf.String()
	if strings.Contains(result, `class="pw-deadlink"`) {
		t.Errorf("with nil checker, should not have deadlink class, got: %s", result)
	}
}


// TestWikiLinkEdgeCases tests bug fixes for wikilink parsing edge cases.
func TestWikiLinkEdgeCases(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(nil, []WikiLinkRendererOption{WithWikiLinkTemplates(testWikiLinkTemplates())}),
		),
	)

	tests := []struct {
		name     string
		input    string
		wantHref string // expected href value (or substring)
		wantText string // expected link text
	}{
		{
			name:     "multiple wikilinks not greedy",
			input:    "([[abalone]], [[periwinkle|snails]])",
			wantHref: `href="abalone"`,
			wantText: ">abalone</a>",
		},
		{
			name:     "escaped pipe in table context",
			input:    `[[Sea snail\|sea snails]]`,
			wantHref: `href="Sea%20snail"`, // URL-encoded, no backslash
			wantText: ">sea snails</a>",
		},
		{
			name:     "escaped pipe destination has no backslash",
			input:    `[[Page Name\|Display]]`,
			wantHref: `href="Page%20Name"`, // URL-encoded, no backslash
			wantText: ">Display</a>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := []byte(tt.input)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			var buf bytes.Buffer
			if err := markdown.Renderer().Render(&buf, source, node); err != nil {
				t.Fatal(err)
			}

			result := buf.String()
			if !strings.Contains(result, tt.wantHref) {
				t.Errorf("expected href containing %q, got: %s", tt.wantHref, result)
			}
			if !strings.Contains(result, tt.wantText) {
				t.Errorf("expected text containing %q, got: %s", tt.wantText, result)
			}
		})
	}
}
