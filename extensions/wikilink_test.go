package extensions

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

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
			WikiLinker,
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
				WithUnderscoreResolver(),
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
				WithCustomResolver(resolver),
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
	// Mock existence checker: only "Existing_Page" exists
	existingPages := map[string]bool{
		"/wiki/Existing_Page": true,
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
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(
				WithExistenceAwareResolver(checker),
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
				WithExistenceAwareResolver(nil),
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
