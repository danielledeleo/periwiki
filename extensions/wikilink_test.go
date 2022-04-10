package extensions

import (
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

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

			node.Dump(source, 0)
		})
	}
}
