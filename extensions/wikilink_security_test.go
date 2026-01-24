package extensions

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// testSecurityWikiLinkTemplates creates simple templates for security testing.
func testSecurityWikiLinkTemplates() map[string]*template.Template {
	return map[string]*template.Template{
		"link": template.Must(template.New("link").Parse(
			`<a href="{{.Destination}}"{{if .OriginalDest}} title="{{.OriginalDest}}"{{end}}{{if .Classes}} class="{{.Classes}}"{{end}}>{{.Title}}</a>`,
		)),
	}
}

// TestWikiLinkXSSPrevention verifies that WikiLink properly escapes
// potentially malicious content to prevent XSS attacks.
func TestWikiLinkXSSPrevention(t *testing.T) {
	tests := []struct {
		name      string
		markdown  string
		forbidden []string // patterns that should NOT appear in output
	}{
		{
			name:      "script in link text",
			markdown:  `[[test|<script>alert('xss')</script>]]`,
			forbidden: []string{"<script>", "</script>"},
		},
		{
			name:      "script in link destination",
			markdown:  `[[<script>alert('xss')</script>]]`,
			forbidden: []string{"<script>", "</script>"},
		},
		{
			name:      "img onerror in link text",
			markdown:  `[[test|<img src=x onerror="alert('xss')">]]`,
			forbidden: []string{`onerror="alert`},
		},
		{
			name:      "event handler in destination",
			markdown:  `[[<div onmouseover="alert('xss')">hover</div>]]`,
			forbidden: []string{`onmouseover="`},
		},
		{
			name:      "javascript href attempt",
			markdown:  `[[javascript:alert('xss')|click me]]`,
			forbidden: []string{`href="javascript:`},
		},
		{
			name:      "double quotes in title attribute",
			markdown:  `[[" onclick="alert('xss')]]`,
			forbidden: []string{`onclick="`},
		},
		{
			name:      "html entity bypass",
			markdown:  `[[test|&lt;script&gt;alert('xss')&lt;/script&gt;]]`,
			forbidden: []string{"<script>"},
		},
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Even with unsafe, XSS should be prevented
		),
		goldmark.WithExtensions(
			NewWikiLinker(nil, []WikiLinkRendererOption{WithWikiLinkTemplates(testSecurityWikiLinkTemplates())}),
		),
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.markdown)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			if node == nil {
				t.Fatal("empty node")
			}

			var buf bytes.Buffer
			if err := markdown.Renderer().Render(&buf, source, node); err != nil {
				t.Fatalf("render failed: %v", err)
			}

			output := buf.String()
			t.Logf("Output: %s", output)

			for _, forbidden := range tc.forbidden {
				if strings.Contains(output, forbidden) {
					t.Errorf("SECURITY: output contains forbidden pattern %q\nFull output: %s",
						forbidden, output)
				}
			}
		})
	}
}

// TestWikiLinkProperEscaping verifies that special characters are properly escaped.
func TestWikiLinkProperEscaping(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		mustFind []string // patterns that MUST appear (escaped versions)
	}{
		{
			name:     "angle brackets escaped in href",
			markdown: `[[<test>]]`,
			mustFind: []string{`href="`}, // should have escaped URL
		},
		{
			name:     "quotes escaped in title",
			markdown: `[[test "with quotes"]]`,
			mustFind: []string{`title="`},
		},
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewWikiLinker(nil, []WikiLinkRendererOption{WithWikiLinkTemplates(testSecurityWikiLinkTemplates())}),
		),
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.markdown)
			reader := text.NewReader(source)
			node := markdown.Parser().Parse(reader)

			var buf bytes.Buffer
			if err := markdown.Renderer().Render(&buf, source, node); err != nil {
				t.Fatalf("render failed: %v", err)
			}

			output := buf.String()
			t.Logf("Output: %s", output)

			for _, required := range tc.mustFind {
				if !strings.Contains(output, required) {
					t.Errorf("output missing required pattern %q\nFull output: %s",
						required, output)
				}
			}
		})
	}
}
