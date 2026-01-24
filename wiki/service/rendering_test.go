package service_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
)

func TestRender(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name     string
		markdown string
		contains string
	}{
		{
			name:     "heading",
			markdown: "# Hello World",
			contains: "<h1", // heading with possible id attribute
		},
		{
			name:     "paragraph",
			markdown: "This is a paragraph.",
			contains: "<p>This is a paragraph.</p>",
		},
		{
			name:     "bold text",
			markdown: "**bold**",
			contains: "<strong>bold</strong>",
		},
		{
			name:     "link",
			markdown: "[example](https://example.com)",
			contains: `<a href="https://example.com"`,
		},
		{
			name:     "code block",
			markdown: "```go\nfunc main() {}\n```",
			contains: "<pre",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html, err := app.Rendering.Render(tc.markdown)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if !strings.Contains(html, tc.contains) {
				t.Errorf("expected HTML to contain %q, got: %s", tc.contains, html)
			}
		})
	}
}

func TestRenderSanitizesHTML(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name       string
		markdown   string
		shouldHave string
		shouldNot  string
	}{
		{
			name:       "removes script tags",
			markdown:   "<script>alert('xss')</script>",
			shouldNot:  "<script>",
			shouldHave: "",
		},
		{
			name:       "removes onclick",
			markdown:   `<a href="#" onclick="alert('xss')">click</a>`,
			shouldNot:  "onclick",
			shouldHave: "", // anchor may be stripped entirely, which is fine
		},
		{
			name:       "removes javascript protocol",
			markdown:   `<a href="javascript:alert('xss')">click</a>`,
			shouldNot:  "javascript:",
			shouldHave: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html, err := app.Rendering.Render(tc.markdown)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if tc.shouldNot != "" && strings.Contains(html, tc.shouldNot) {
				t.Errorf("HTML should not contain %q, got: %s", tc.shouldNot, html)
			}

			if tc.shouldHave != "" && !strings.Contains(html, tc.shouldHave) {
				t.Errorf("HTML should contain %q, got: %s", tc.shouldHave, html)
			}
		})
	}
}
